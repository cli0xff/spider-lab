package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/crawlab-team/spider-lab/internal/models"
	mastergrpc "github.com/crawlab-team/spider-lab/internal/master/grpc"
	"github.com/crawlab-team/spider-lab/internal/utils"
	pb "github.com/crawlab-team/spider-lab/proto"
	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Scheduler struct {
	cron       *cron.Cron
	grpcServer *mastergrpc.MasterServer
	stopCh     chan struct{}
}

func NewScheduler(grpcServer *mastergrpc.MasterServer) *Scheduler {
	return &Scheduler{
		cron:       cron.New(cron.WithSeconds()),
		grpcServer: grpcServer,
		stopCh:     make(chan struct{}),
	}
}

func (s *Scheduler) Start() error {
	utils.Logger.Info("Starting task scheduler")
	utils.WriteSystemLog("info", "scheduler", "任务调度器启动", "")

	if err := s.loadSchedules(); err != nil {
		utils.Logger.Warnf("Failed to load schedules: %v", err)
	}

	s.cron.Start()
	go s.pollPendingTasks()
	go s.monitorNodeHealth()

	return nil
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.cron.Stop()
}

func (s *Scheduler) CreateTask(spider *models.Spider, param string, createdBy primitive.ObjectID) (*models.Task, error) {
	if spider.Mode == models.SpiderModeAllNodes {
		return s.createFanOutTasks(spider, param, createdBy)
	}
	return s.createSingleTask(spider, param, createdBy)
}

func (s *Scheduler) createSingleTask(spider *models.Spider, param string, createdBy primitive.ObjectID) (*models.Task, error) {
	ctx := context.Background()
	now := time.Now()

	task := &models.Task{
		SpiderId:  spider.Id,
		Status:    models.TaskStatusPending,
		Cmd:       spider.Cmd,
		Param:     param,
		Priority:  spider.Priority,
		Type:      "spider",
		RunTs:     now,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: createdBy,
	}

	col := utils.GetCollection("tasks")
	result, err := col.InsertOne(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	task.Id = result.InsertedID.(primitive.ObjectID)
	utils.WriteSystemLog("info", "scheduler", fmt.Sprintf("创建任务: %s (爬虫: %s)", task.Id.Hex(), spider.Id.Hex()), fmt.Sprintf("cmd=%s", spider.Cmd))

	// Xueqiu spiders require a cookie from the pool; wait if none available
	if isXueqiuSpider(spider) && !s.grpcServer.HasXueqiuCookieAvailable(ctx) {
		_, _ = col.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{
			"$set": bson.M{"status": models.TaskStatusWaiting, "error": "等待Cookie池中的可用Cookie"},
		})
		task.Status = models.TaskStatusWaiting
		utils.WriteSystemLog("info", "scheduler", fmt.Sprintf("任务等待Cookie: %s", task.Id.Hex()), "Cookie池中暂无可用Cookie")
		return task, nil
	}

	// WeChat spiders require a cookie from the pool; wait if none available
	if isWechatSpider(spider) && !s.grpcServer.HasWechatCookieAvailable(ctx) {
		_, _ = col.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{
			"$set": bson.M{"status": models.TaskStatusWaiting, "error": "等待微信Cookie池中的可用Cookie"},
		})
		task.Status = models.TaskStatusWaiting
		utils.WriteSystemLog("info", "scheduler", fmt.Sprintf("任务等待微信Cookie: %s", task.Id.Hex()), "微信Cookie池中暂无可用Cookie")
		return task, nil
	}

	if err := s.dispatchTask(ctx, task, spider); err != nil {
		_, _ = col.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{
			"$set": bson.M{"status": models.TaskStatusError, "error": err.Error()},
		})
		return task, fmt.Errorf("failed to dispatch task: %w", err)
	}

	return task, nil
}

func (s *Scheduler) createFanOutTasks(spider *models.Spider, param string, createdBy primitive.ObjectID) (*models.Task, error) {
	ctx := context.Background()

	// Xueqiu spiders require a cookie from the pool
	if isXueqiuSpider(spider) && !s.grpcServer.HasXueqiuCookieAvailable(ctx) {
		col := utils.GetCollection("tasks")
		now := time.Now()
		task := &models.Task{
			SpiderId:  spider.Id,
			Status:    models.TaskStatusWaiting,
			Cmd:       spider.Cmd,
			Param:     param,
			Error:     "等待Cookie池中的可用Cookie",
			Priority:  spider.Priority,
			Type:      "spider",
			RunTs:     now,
			CreatedAt: now,
			UpdatedAt: now,
			CreatedBy: createdBy,
		}
		result, err := col.InsertOne(ctx, task)
		if err != nil {
			return nil, fmt.Errorf("failed to create waiting task: %w", err)
		}
		task.Id = result.InsertedID.(primitive.ObjectID)
		utils.WriteSystemLog("info", "scheduler", fmt.Sprintf("任务等待Cookie: %s", task.Id.Hex()), "Cookie池中暂无可用Cookie")
		return task, nil
	}

	// WeChat spiders require a cookie from the pool
	if isWechatSpider(spider) && !s.grpcServer.HasWechatCookieAvailable(ctx) {
		col := utils.GetCollection("tasks")
		now := time.Now()
		task := &models.Task{
			SpiderId:  spider.Id,
			Status:    models.TaskStatusWaiting,
			Cmd:       spider.Cmd,
			Param:     param,
			Error:     "等待微信Cookie池中的可用Cookie",
			Priority:  spider.Priority,
			Type:      "spider",
			RunTs:     now,
			CreatedAt: now,
			UpdatedAt: now,
			CreatedBy: createdBy,
		}
		result, err := col.InsertOne(ctx, task)
		if err != nil {
			return nil, fmt.Errorf("failed to create waiting task: %w", err)
		}
		task.Id = result.InsertedID.(primitive.ObjectID)
		utils.WriteSystemLog("info", "scheduler", fmt.Sprintf("任务等待微信Cookie: %s", task.Id.Hex()), "微信Cookie池中暂无可用Cookie")
		return task, nil
	}

	nodeCol := utils.GetCollection("nodes")

	cursor, err := nodeCol.Find(ctx, bson.M{
		"status":            models.NodeStatusOnline,
		"available_runners": bson.M{"$gt": 0},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query online nodes: %w", err)
	}
	var nodes []models.Node
	if err := cursor.All(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("failed to decode nodes: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no available online nodes for all_nodes fan-out")
	}

	col := utils.GetCollection("tasks")
	now := time.Now()
	var firstTask *models.Task

	for _, node := range nodes {
		task := &models.Task{
			SpiderId:  spider.Id,
			NodeId:    node.Id,
			Status:    models.TaskStatusPending,
			Cmd:       spider.Cmd,
			Param:     param,
			Priority:  spider.Priority,
			Type:      "spider",
			RunTs:     now,
			CreatedAt: now,
			UpdatedAt: now,
			CreatedBy: createdBy,
		}

		result, err := col.InsertOne(ctx, task)
		if err != nil {
			utils.Logger.Warnf("Failed to create fan-out task for node %s: %v", node.Key, err)
			continue
		}
		task.Id = result.InsertedID.(primitive.ObjectID)

		if firstTask == nil {
			firstTask = task
		}

		if err := s.dispatchToNode(ctx, task, &node); err != nil {
			utils.Logger.Warnf("Failed to dispatch fan-out task %s to node %s: %v", task.Id.Hex(), node.Key, err)
			_, _ = col.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{
				"$set": bson.M{"status": models.TaskStatusError, "error": err.Error()},
			})
		}
	}

	if firstTask == nil {
		return nil, fmt.Errorf("failed to create any fan-out tasks")
	}
	return firstTask, nil
}

func (s *Scheduler) dispatchTask(ctx context.Context, task *models.Task, spider *models.Spider) error {
	node, err := s.selectNode(ctx, spider)
	if err != nil {
		return err
	}
	return s.dispatchToNode(ctx, task, node)
}

func (s *Scheduler) dispatchToNode(ctx context.Context, task *models.Task, node *models.Node) error {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Select a different node for retry
			var retryNode *models.Node
			retryNode, lastErr = s.selectNode(ctx, &models.Spider{Mode: models.SpiderModeRandom})
			if lastErr != nil {
				utils.Logger.Warnf("Retry %d: failed to select alternate node for task %s: %v", attempt, task.Id.Hex(), lastErr)
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			node = retryNode
			utils.Logger.Infof("Retry %d: attempting to dispatch task %s to alternate node %s", attempt, task.Id.Hex(), node.Key)
		}

		// Deploy spider files to worker before running the task
		resp, err := s.grpcServer.DeploySpider(ctx, &pb.SpiderDeployRequest{
			SpiderId: task.SpiderId.Hex(),
			NodeKey:  node.Key,
		})
		if err != nil {
			utils.Logger.Warnf("Failed to deploy spider %s to node %s: %v", task.SpiderId.Hex(), node.Key, err)
			lastErr = err
			continue
		} else if resp != nil && !resp.Success {
			utils.Logger.Warnf("Deploy spider %s to node %s: %s", task.SpiderId.Hex(), node.Key, resp.Message)
			lastErr = fmt.Errorf(resp.Message)
			continue
		}

		// Track which cookie was injected for this task (for error reporting)
		taskUpdate := bson.M{"node_id": node.Id, "status": models.TaskStatusRunning, "start_ts": time.Now(), "updated_at": time.Now()}
		if cookieId, ok := s.grpcServer.PopLastCookieId(task.SpiderId.Hex()); ok {
			taskUpdate["cookie_id"] = cookieId
			task.CookieId = cookieId
		}

		task.NodeId = node.Id
		col := utils.GetCollection("tasks")
		_, err = col.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{"$set": taskUpdate})
		if err != nil {
			lastErr = err
			continue
		}

		task.Status = models.TaskStatusRunning
		if err := s.grpcServer.SendTaskToWorker(node.Key, task); err != nil {
			utils.Logger.Warnf("Failed to send task %s to worker %s (attempt %d): %v", task.Id.Hex(), node.Key, attempt+1, err)
			lastErr = err
			// Continue to next retry
			continue
		}

		utils.WriteSystemLog("info", "scheduler", fmt.Sprintf("任务已分发: %s -> 节点 %s", task.Id.Hex(), node.Name), "")
		return nil
	}

	// All retries failed, set task back to pending
	col := utils.GetCollection("tasks")
	_, _ = col.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{
		"$set": bson.M{"status": models.TaskStatusPending, "updated_at": time.Now()},
	})
	utils.WriteSystemLog("error", "scheduler", fmt.Sprintf("任务分发失败，已重试%d次: %s", maxRetries, task.Id.Hex()), lastErr.Error())
	return fmt.Errorf("failed to dispatch task after %d retries: %w", maxRetries, lastErr)
}

func (s *Scheduler) selectNode(ctx context.Context, spider *models.Spider) (*models.Node, error) {
	col := utils.GetCollection("nodes")

	switch spider.Mode {
	case models.SpiderModeSelectedNodes:
		if len(spider.NodeIds) == 0 {
			return nil, fmt.Errorf("no nodes specified for spider")
		}
		var node models.Node
		err := col.FindOne(ctx, bson.M{
			"_id":    bson.M{"$in": spider.NodeIds},
			"status": models.NodeStatusOnline,
		}, options.FindOne().SetSort(bson.D{{Key: "available_runners", Value: -1}})).Decode(&node)
		if err != nil {
			return nil, fmt.Errorf("no available selected node: %w", err)
		}
		return &node, nil

	case models.SpiderModeRandom:
		var nodes []models.Node
		cursor, err := col.Find(ctx, bson.M{"status": models.NodeStatusOnline, "available_runners": bson.M{"$gt": 0}})
		if err != nil {
			return nil, err
		}
		if err := cursor.All(ctx, &nodes); err != nil {
			return nil, err
		}
		if len(nodes) == 0 {
			return nil, fmt.Errorf("no available nodes")
		}
		return &nodes[rand.Intn(len(nodes))], nil

	default:
		var node models.Node
		err := col.FindOne(ctx, bson.M{
			"status":            models.NodeStatusOnline,
			"available_runners": bson.M{"$gt": 0},
		}, options.FindOne().SetSort(bson.D{{Key: "available_runners", Value: -1}})).Decode(&node)
		if err != nil {
			return nil, fmt.Errorf("no available nodes: %w", err)
		}
		return &node, nil
	}
}

func (s *Scheduler) loadSchedules() error {
	ctx := context.Background()
	col := utils.GetCollection("schedules")

	cursor, err := col.Find(ctx, bson.M{"enabled": true})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var schedules []models.Schedule
	if err := cursor.All(ctx, &schedules); err != nil {
		return err
	}

	for _, sched := range schedules {
		if err := s.AddSchedule(&sched); err != nil {
			utils.Logger.Warnf("Failed to add schedule %s: %v", sched.Id.Hex(), err)
		}
	}

	utils.Logger.Infof("Loaded %d schedules", len(schedules))
	return nil
}

func (s *Scheduler) AddSchedule(schedule *models.Schedule) error {
	schedCopy := *schedule
	entryId, err := s.cron.AddFunc(schedule.Cron, func() {
		ctx := context.Background()
		spiderCol := utils.GetCollection("spiders")
		var spider models.Spider
		if err := spiderCol.FindOne(ctx, bson.M{"_id": schedCopy.SpiderId}).Decode(&spider); err != nil {
			utils.Logger.Errorf("Schedule: spider not found %s", schedCopy.SpiderId.Hex())
			return
		}

		if schedCopy.Cmd != "" {
			spider.Cmd = schedCopy.Cmd
		}
		if schedCopy.Mode != "" {
			spider.Mode = schedCopy.Mode
		}
		if len(schedCopy.NodeIds) > 0 {
			spider.NodeIds = schedCopy.NodeIds
		}

		task, err := s.CreateTask(&spider, schedCopy.Param, schedCopy.CreatedBy)
		if err != nil {
			utils.Logger.Errorf("Schedule: failed to create task: %v", err)
			return
		}
		task.ScheduleId = schedCopy.Id

		taskCol := utils.GetCollection("tasks")
		_, _ = taskCol.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{"$set": bson.M{"schedule_id": schedCopy.Id}})
		utils.Logger.Infof("Schedule triggered: %s -> task %s", schedCopy.Id.Hex(), task.Id.Hex())
		utils.WriteSystemLog("info", "scheduler", fmt.Sprintf("定时任务触发: %s -> 任务 %s", schedCopy.Id.Hex(), task.Id.Hex()), schedCopy.Cron)
	})

	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	schedule.EntryId = int(entryId)
	schedCol := utils.GetCollection("schedules")
	_, _ = schedCol.UpdateOne(context.Background(), bson.M{"_id": schedule.Id}, bson.M{"$set": bson.M{"entry_id": entryId}})

	return nil
}

func (s *Scheduler) RemoveSchedule(entryId int) {
	s.cron.Remove(cron.EntryID(entryId))
}

func (s *Scheduler) pollPendingTasks() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ctx := context.Background()
			col := utils.GetCollection("tasks")

			cursor, err := col.Find(ctx, bson.M{
				"status": bson.M{"$in": []string{models.TaskStatusPending, models.TaskStatusWaiting}},
				"node_id": primitive.NilObjectID,
			}, options.Find().SetSort(bson.D{
				{Key: "priority", Value: -1},
				{Key: "created_at", Value: 1},
			}).SetLimit(10))

			if err != nil {
				utils.Logger.Warnf("pollPendingTasks: query error: %v", err)
				continue
			}

			var tasks []models.Task
			if err := cursor.All(ctx, &tasks); err != nil {
				utils.Logger.Warnf("pollPendingTasks: cursor error: %v", err)
				continue
			}

			for _, task := range tasks {
				spiderCol := utils.GetCollection("spiders")
				var spider models.Spider
				if err := spiderCol.FindOne(ctx, bson.M{"_id": task.SpiderId}).Decode(&spider); err != nil {
					utils.Logger.Warnf("pollPendingTasks: spider %s not found for task %s: %v", task.SpiderId.Hex(), task.Id.Hex(), err)
					continue
				}
				// Skip Xueqiu spiders if no cookie is available
				if isXueqiuSpider(&spider) && !s.grpcServer.HasXueqiuCookieAvailable(ctx) {
					if task.Status != models.TaskStatusWaiting {
						_, _ = col.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{
							"$set": bson.M{"status": models.TaskStatusWaiting, "error": "等待Cookie池中的可用Cookie"},
						})
					}
					continue
				}
				// Skip WeChat spiders if no cookie is available
				if isWechatSpider(&spider) && !s.grpcServer.HasWechatCookieAvailable(ctx) {
					if task.Status != models.TaskStatusWaiting {
						_, _ = col.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{
							"$set": bson.M{"status": models.TaskStatusWaiting, "error": "等待微信Cookie池中的可用Cookie"},
						})
					}
					continue
				}
				// Transition waiting tasks back to pending before dispatch
				if task.Status == models.TaskStatusWaiting {
					_, _ = col.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{
						"$set": bson.M{"status": models.TaskStatusPending, "error": ""},
					})
					task.Status = models.TaskStatusPending
				}
				if err := s.dispatchTask(ctx, &task, &spider); err != nil {
					utils.Logger.Warnf("pollPendingTasks: dispatch failed for task %s: %v", task.Id.Hex(), err)
				}
			}
		}
	}
}

func (s *Scheduler) monitorNodeHealth() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ctx := context.Background()
			col := utils.GetCollection("nodes")
			threshold := time.Now().Add(-60 * time.Second)

			_, _ = col.UpdateMany(ctx, bson.M{
				"status":         models.NodeStatusOnline,
				"last_heartbeat": bson.M{"$lt": threshold},
			}, bson.M{
				"$set": bson.M{"status": models.NodeStatusOffline, "updated_at": time.Now()},
			})
		}
	}
}

// isXueqiuSpider returns true if the spider is a Xueqiu template spider
// that requires a cookie from the pool.
func isXueqiuSpider(spider *models.Spider) bool {
	return spider.Type == models.SpiderTypeTemplate && strings.HasPrefix(spider.TemplateId, "xueqiu_")
}

// isWechatSpider returns true if the spider is a WeChat template spider
// that requires a cookie from the pool.
func isWechatSpider(spider *models.Spider) bool {
	return spider.Type == models.SpiderTypeTemplate && strings.HasPrefix(spider.TemplateId, "wechat_")
}
