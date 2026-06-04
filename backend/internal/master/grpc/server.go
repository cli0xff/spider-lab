package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/crawlab-team/spider-lab/internal/models"
	"github.com/crawlab-team/spider-lab/internal/master/spider"
	"github.com/crawlab-team/spider-lab/internal/utils"
	pb "github.com/crawlab-team/spider-lab/proto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
)

type MasterServer struct {
	pb.UnimplementedNodeServiceServer
	pb.UnimplementedTaskServiceServer
	pb.UnimplementedSpiderServiceServer

	grpcServer        *grpc.Server
	address           string
	authKey           string
	spiderDir         string
	mu                sync.RWMutex
	workerStreams     map[string]pb.NodeService_SubscribeServer
	lastCookieInjected sync.Map // spiderId -> primitive.ObjectID (for cookie tracking)
}

func NewMasterServer(address, authKey, spiderDir string) *MasterServer {
	return &MasterServer{
		address:      address,
		authKey:      authKey,
		spiderDir:    spiderDir,
		workerStreams: make(map[string]pb.NodeService_SubscribeServer),
	}
}

func (s *MasterServer) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.address, err)
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterNodeServiceServer(s.grpcServer, s)
	pb.RegisterTaskServiceServer(s.grpcServer, s)
	pb.RegisterSpiderServiceServer(s.grpcServer, s)

	utils.Logger.Infof("Master gRPC server listening on %s", s.address)
	return s.grpcServer.Serve(listener)
}

func (s *MasterServer) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

func (s *MasterServer) Register(ctx context.Context, req *pb.NodeRegisterRequest) (*pb.NodeRegisterResponse, error) {
	utils.Logger.Infof("Node registration: key=%s, name=%s, ip=%s", req.Key, req.Name, req.Ip)
	utils.WriteSystemLog("info", "grpc", fmt.Sprintf("节点注册: %s (%s)", req.Name, req.Ip), fmt.Sprintf("key=%s, master=%v", req.Key, req.IsMaster))

	col := utils.GetCollection("nodes")
	now := time.Now()

	var existingNode models.Node
	err := col.FindOne(ctx, bson.M{"key": req.Key}).Decode(&existingNode)

	if err != nil {
		node := models.Node{
			Key:              req.Key,
			Name:             req.Name,
			IsMaster:         req.IsMaster,
			Status:           models.NodeStatusOnline,
			Enabled:          true,
			IP:               req.Ip,
			Port:             int(req.Port),
			MaxRunners:       int(req.MaxRunners),
			AvailableRunners: int(req.MaxRunners),
			Description:      req.Description,
			LastHeartbeat:    now,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		result, err := col.InsertOne(ctx, node)
		if err != nil {
			return &pb.NodeRegisterResponse{Success: false, Message: err.Error()}, nil
		}
		nodeId := result.InsertedID.(primitive.ObjectID).Hex()
		return &pb.NodeRegisterResponse{Success: true, NodeId: nodeId, Message: "registered"}, nil
	}

	_, err = col.UpdateOne(ctx, bson.M{"key": req.Key}, bson.M{
		"$set": bson.M{
			"status":         statusOnReregister(existingNode.Enabled),
			"ip":             req.Ip,
			"port":           req.Port,
			"max_runners":    req.MaxRunners,
			"last_heartbeat": now,
			"updated_at":     now,
		},
	})
	if err != nil {
		return &pb.NodeRegisterResponse{Success: false, Message: err.Error()}, nil
	}

	msg := "re-registered"
	if !existingNode.Enabled {
		msg = "re-registered (disabled)"
	}
	return &pb.NodeRegisterResponse{Success: true, NodeId: existingNode.Id.Hex(), Message: msg}, nil
}

// statusOnReregister returns the appropriate status when a worker re-registers.
// Disabled nodes stay offline so the admin's choice is respected.
func statusOnReregister(enabled bool) string {
	if enabled {
		return models.NodeStatusOnline
	}
	return models.NodeStatusOffline
}

func (s *MasterServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	col := utils.GetCollection("nodes")
	now := time.Now()

	// Check if node still exists (may have been deleted)
	var node models.Node
	err := col.FindOne(ctx, bson.M{"key": req.NodeKey}).Decode(&node)
	if err != nil {
		// Node not found — deleted or never registered
		return &pb.HeartbeatResponse{Success: false}, nil
	}

	update := bson.M{
		"cpu_usage":         req.CpuUsage,
		"memory_usage":      req.MemoryUsage,
		"disk_usage":        req.DiskUsage,
		"active_runners":    req.ActiveRunners,
		"available_runners": req.AvailableRunners,
		"last_heartbeat":    now,
		"updated_at":        now,
	}
	// Only set status to online if node is enabled; disabled nodes stay offline
	if node.Enabled {
		update["status"] = models.NodeStatusOnline
	}

	_, err = col.UpdateOne(ctx, bson.M{"key": req.NodeKey}, bson.M{"$set": update})
	if err != nil {
		return &pb.HeartbeatResponse{Success: false}, nil
	}

	return &pb.HeartbeatResponse{Success: true, Timestamp: now.Unix()}, nil
}

func (s *MasterServer) Subscribe(stream pb.NodeService_SubscribeServer) error {
	var nodeKey string

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			s.removeWorkerStream(nodeKey)
			return nil
		}
		if err != nil {
			s.removeWorkerStream(nodeKey)
			return err
		}

		if nodeKey == "" {
			nodeKey = msg.NodeKey
			s.addWorkerStream(nodeKey, stream)
			utils.Logger.Infof("Worker subscribed: %s", nodeKey)
		}

		switch msg.Code {
		case "heartbeat":
			_ = stream.Send(&pb.StreamMessage{Code: "heartbeat_ack", Timestamp: time.Now().Unix()})
		case "task_status":
			var report models.Task
			if err := json.Unmarshal(msg.Data, &report); err == nil {
				s.handleTaskStatusUpdate(&report)
			}
		case "log":
			var log models.TaskLog
			if err := json.Unmarshal(msg.Data, &log); err == nil {
				s.handleLogMessage(&log)
			}
		}
	}
}

func (s *MasterServer) GetNodeInfo(ctx context.Context, req *pb.NodeInfoRequest) (*pb.NodeInfoResponse, error) {
	col := utils.GetCollection("nodes")
	var node models.Node
	err := col.FindOne(ctx, bson.M{"key": req.NodeKey}).Decode(&node)
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	return &pb.NodeInfoResponse{
		NodeId:        node.Id.Hex(),
		Name:          node.Name,
		Status:        node.Status,
		CpuUsage:      node.CPUUsage,
		MemoryUsage:   node.MemoryUsage,
		ActiveRunners: int32(node.ActiveRunners),
		MaxRunners:    int32(node.MaxRunners),
	}, nil
}

func (s *MasterServer) AssignTask(ctx context.Context, req *pb.TaskAssignRequest) (*pb.TaskAssignResponse, error) {
	utils.Logger.Infof("Assigning task %s (spider: %s)", req.TaskId, req.SpiderId)
	return &pb.TaskAssignResponse{Accepted: true, Message: "task assigned"}, nil
}

func (s *MasterServer) CancelTask(ctx context.Context, req *pb.TaskCancelRequest) (*pb.TaskCancelResponse, error) {
	utils.Logger.Infof("Cancelling task %s: %s", req.TaskId, req.Reason)
	col := utils.GetCollection("tasks")
	taskId, err := primitive.ObjectIDFromHex(req.TaskId)
	if err != nil {
		return &pb.TaskCancelResponse{Success: false, Message: "invalid task id"}, nil
	}

	_, err = col.UpdateOne(ctx, bson.M{"_id": taskId}, bson.M{
		"$set": bson.M{
			"status":     models.TaskStatusCancelled,
			"error":      req.Reason,
			"end_ts":     time.Now(),
			"updated_at": time.Now(),
		},
	})
	if err != nil {
		return &pb.TaskCancelResponse{Success: false, Message: err.Error()}, nil
	}

	return &pb.TaskCancelResponse{Success: true, Message: "cancelled"}, nil
}

func (s *MasterServer) ReportTaskStatus(ctx context.Context, req *pb.TaskStatusReport) (*pb.TaskStatusResponse, error) {
	col := utils.GetCollection("tasks")
	taskId, err := primitive.ObjectIDFromHex(req.TaskId)
	if err != nil {
		return &pb.TaskStatusResponse{Acknowledged: false}, nil
	}

	update := bson.M{"status": req.Status, "result_count": req.ResultCount, "updated_at": time.Now()}
	if req.Error != "" {
		update["error"] = req.Error
	}
	if req.Pid > 0 {
		update["pid"] = req.Pid
	}
	if req.StartTs > 0 {
		update["start_ts"] = time.Unix(req.StartTs, 0)
	}
	if req.EndTs > 0 {
		update["end_ts"] = time.Unix(req.EndTs, 0)
	}

	_, err = col.UpdateOne(ctx, bson.M{"_id": taskId}, bson.M{"$set": update})
	if err != nil {
		return &pb.TaskStatusResponse{Acknowledged: false}, nil
	}

	// Track cookie errors/successes for Xueqiu and WeChat cookie pools
	if req.Status == models.TaskStatusFinished || req.Status == models.TaskStatusError {
		var task models.Task
		if err := col.FindOne(ctx, bson.M{"_id": taskId}).Decode(&task); err == nil && !task.CookieId.IsZero() {
			// Determine which cookie pool to use based on spider template
			spiderCol := utils.GetCollection("spiders")
			var spider models.Spider
			if err := spiderCol.FindOne(ctx, bson.M{"_id": task.SpiderId}).Decode(&spider); err == nil {
				isXueqiu := spider.Type == models.SpiderTypeTemplate && strings.HasPrefix(spider.TemplateId, "xueqiu_")
				isWechat := spider.Type == models.SpiderTypeTemplate && strings.HasPrefix(spider.TemplateId, "wechat_")

				if req.Status == models.TaskStatusFinished {
					if isXueqiu {
						s.clearCookieError(ctx, task.CookieId)
					} else if isWechat {
						s.clearWechatCookieError(ctx, task.CookieId)
					}
				} else if req.Status == models.TaskStatusError {
					if isXueqiu {
						s.markCookieError(ctx, task.CookieId, req.Error)
					} else if isWechat {
						s.markWechatCookieError(ctx, task.CookieId, req.Error)
					}
				}
			}
		}
	}

	return &pb.TaskStatusResponse{Acknowledged: true}, nil
}

func (s *MasterServer) StreamLogs(stream pb.TaskService_StreamLogsServer) error {
	col := utils.GetCollection("task_logs")
	resultCol := utils.GetCollection("results")
	postCol := utils.GetCollection("posts")
	var logCount int64
	var resultCount int64

	// Cache spider_id -> platform mapping to avoid repeated DB lookups
	platformCache := make(map[string]string)

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.LogAck{Success: true, LinesReceived: logCount + resultCount})
		}
		if err != nil {
			return err
		}

		if msg.Level == "result" {
			// Parse result payload: {"spider_id": "...", "item": {...}}
			var payload struct {
				SpiderId string                 `json:"spider_id"`
				Item     map[string]interface{} `json:"item"`
			}
			if err := json.Unmarshal([]byte(msg.Content), &payload); err == nil && len(payload.Item) > 0 {
				taskId, _ := primitive.ObjectIDFromHex(msg.TaskId)
				spiderId, _ := primitive.ObjectIDFromHex(payload.SpiderId)
				itemHash := computeItemHash(payload.Item)

				// Upsert: if same spider+hash exists, update with latest data; otherwise insert new
				filter := bson.M{"spider_id": spiderId, "item_hash": itemHash}
				pubAt := ParsePublishedAt(payload.Item)
				update := bson.M{
					"$set": bson.M{
						"task_id":      taskId,
						"item":         payload.Item,
						"item_hash":    itemHash,
						"timestamp":    time.Unix(msg.Timestamp, 0),
						"published_at": pubAt,
					},
					"$setOnInsert": bson.M{
						"spider_id": spiderId,
					},
				}
				opts := options.Update().SetUpsert(true)
				_, _ = resultCol.UpdateOne(stream.Context(), filter, update, opts)
				resultCount++

				// Also upsert into posts collection for centralized storage
				s.upsertPost(stream.Context(), postCol, spiderId, taskId, payload.Item, platformCache)
			}
		} else {
			taskId, _ := primitive.ObjectIDFromHex(msg.TaskId)
			log := models.TaskLog{
				TaskId:    taskId,
				NodeKey:   msg.NodeKey,
				Content:   msg.Content,
				Level:     msg.Level,
				Timestamp: time.Unix(msg.Timestamp, 0),
			}
			_, _ = col.InsertOne(stream.Context(), log)
			logCount++
		}
	}
}

// upsertPost saves a crawled item to the centralized posts collection.
func (s *MasterServer) upsertPost(ctx context.Context, postCol *mongo.Collection, spiderId, taskId primitive.ObjectID, item map[string]interface{}, platformCache map[string]string) {
	platform := s.getSpiderPlatform(ctx, spiderId, platformCache)
	if platform == "" {
		return
	}
	platformId := extractPlatformId(item)
	if platformId == "" {
		return
	}

	itemType, _ := item["type"].(string)
	now := time.Now()
	pubAt := ParsePublishedAt(item)

	setFields := bson.M{
		"item":         item,
		"task_id":      taskId,
		"type":         itemType,
		"updated_at":   now,
		"published_at": pubAt,
	}

	filter := bson.M{"platform": platform, "platform_id": platformId}
	update := bson.M{
		"$set": setFields,
		"$addToSet": bson.M{
			"spider_ids": spiderId,
		},
		"$setOnInsert": bson.M{
			"platform":    platform,
			"platform_id": platformId,
			"created_at":  now,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, _ = postCol.UpdateOne(ctx, filter, update, opts)
}

// ParsePublishedAt extracts the original publish time from a spider result item.
// Handles: millisecond/second Unix timestamps (number or string),
// RFC-like date strings ("Mon Jan 02 15:04:05 +0800 2006"),
// and ISO/common date formats.
func ParsePublishedAt(item map[string]interface{}) time.Time {
	// Try created_at first, then crawled_at as fallback
	for _, key := range []string{"created_at", "crawled_at"} {
		raw, ok := item[key]
		if !ok {
			continue
		}
		var t time.Time
		switch v := raw.(type) {
		case float64:
			t = tsToTime(v)
		case int64:
			t = tsToTime(float64(v))
		case json.Number:
			if f, err := v.Float64(); err == nil {
				t = tsToTime(f)
			}
		case string:
			t = parseTimeString(v)
		}
		if !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}

// tsToTime converts a numeric timestamp (seconds or milliseconds) to time.Time.
func tsToTime(v float64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	// Distinguish seconds vs milliseconds: timestamps in seconds are < 1e11
	if v > 1e11 {
		sec := int64(v / 1000)
		ms := int64(math.Mod(v, 1000))
		return time.Unix(sec, ms*1e6)
	}
	return time.Unix(int64(v), 0)
}

// parseTimeString parses various date string formats.
func parseTimeString(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	// Try as numeric string first (e.g. "1681234567890")
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 1e8 {
		return tsToTime(f)
	}
	// Common formats from Chinese social platforms
	formats := []string{
		"Mon Jan 02 15:04:05 -0700 2006",   // Weibo: "Tue Apr 11 15:30:00 +0800 2023"
		"Mon Jan 02 15:04:05 +0800 2006",    // Weibo variant
		time.RFC3339,                         // ISO 8601
		"2006-01-02T15:04:05Z",              // ISO without tz
		"2006-01-02 15:04:05",               // Common
		"2006-01-02",                         // Date only
		"2006/01/02 15:04:05",               // Slash variant
		"01-02",                              // Weibo short (month-day)
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, s); err == nil {
			// For month-day only, assume current year
			if layout == "01-02" {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			return t
		}
	}
	return time.Time{}
}

// getSpiderPlatform resolves the platform name for a spider via its template.
func (s *MasterServer) getSpiderPlatform(ctx context.Context, spiderId primitive.ObjectID, cache map[string]string) string {
	key := spiderId.Hex()
	if p, ok := cache[key]; ok {
		return p
	}
	col := utils.GetCollection("spiders")
	var sp models.Spider
	if err := col.FindOne(ctx, bson.M{"_id": spiderId}).Decode(&sp); err != nil {
		cache[key] = ""
		return ""
	}
	tmpl := spider.GetTemplate(sp.TemplateId)
	if tmpl == nil {
		cache[key] = ""
		return ""
	}
	cache[key] = tmpl.Platform
	return tmpl.Platform
}

// extractPlatformId extracts a stable platform-specific identifier from a result item.
func extractPlatformId(item map[string]interface{}) string {
	// XHS: note_id
	if v, ok := item["note_id"]; ok {
		s := fmt.Sprintf("%v", v)
		if s != "" {
			return s
		}
	}
	// Weibo / Xueqiu: id
	if v, ok := item["id"]; ok {
		s := fmt.Sprintf("%v", v)
		if s != "" {
			return s
		}
	}
	// Xueqiu hot_stock: symbol
	if v, ok := item["symbol"]; ok {
		s := fmt.Sprintf("%v", v)
		if s != "" {
			return "symbol:" + s
		}
	}
	// Topics: title as fallback
	if v, ok := item["title"]; ok {
		s := fmt.Sprintf("%v", v)
		if s != "" {
			return "title:" + s
		}
	}
	return ""
}

// computeItemHash generates a dedup key for a result item.
// Uses item["id"] for posts (Weibo post ID), item["title"] for topics, else SHA256.
func computeItemHash(item map[string]interface{}) string {
	// Posts: use Weibo post ID
	if id, ok := item["id"]; ok {
		idStr := fmt.Sprintf("%v", id)
		if idStr != "" {
			return "id:" + idStr
		}
	}
	// Topics: use title as stable dedup key (same topic across runs gets updated)
	if title, ok := item["title"]; ok {
		titleStr := fmt.Sprintf("%v", title)
		if titleStr != "" {
			return "title:" + titleStr
		}
	}
	// Fallback: SHA256 of sorted key-value pairs
	keys := make([]string, 0, len(item))
	for k := range item {
		// Skip volatile fields
		if k == "task_id" || k == "crawled_at" || k == "rank" || k == "hot_value" || k == "post_count" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		v, _ := json.Marshal(item[k])
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write(v)
		h.Write([]byte(";"))
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

func (s *MasterServer) DeploySpider(ctx context.Context, req *pb.SpiderDeployRequest) (*pb.SpiderDeployResponse, error) {
	utils.Logger.Infof("Deploying spider %s to node %s", req.SpiderId, req.NodeKey)

	spiderDir := filepath.Join(s.spiderDir, req.SpiderId)

	// Always regenerate template files on deploy so code updates take effect
	s.regenerateTemplateFiles(ctx, req.SpiderId, spiderDir)

	if _, err := os.Stat(spiderDir); os.IsNotExist(err) {
		return &pb.SpiderDeployResponse{Success: true, Message: "no files to deploy"}, nil
	}

	type fileEntry struct {
		RelPath string `json:"relPath"`
		Data    []byte `json:"data"`
	}

	var files []fileEntry
	err := filepath.Walk(spiderDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		relPath, _ := filepath.Rel(spiderDir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", relPath, err)
		}
		files = append(files, fileEntry{RelPath: relPath, Data: data})
		return nil
	})
	if err != nil {
		return &pb.SpiderDeployResponse{Success: false, Message: err.Error()}, nil
	}

	if len(files) == 0 {
		return &pb.SpiderDeployResponse{Success: true, Message: "no files to deploy"}, nil
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"spider_id": req.SpiderId,
		"files":     files,
	})

	s.mu.RLock()
	stream, ok := s.workerStreams[req.NodeKey]
	s.mu.RUnlock()

	if !ok {
		return &pb.SpiderDeployResponse{Success: false, Message: fmt.Sprintf("worker %s not connected", req.NodeKey)}, nil
	}

	if err := stream.Send(&pb.StreamMessage{
		Code:      "deploy_spider",
		NodeKey:   req.NodeKey,
		Data:      payload,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		// Stream is broken, remove the worker from the map and mark as offline
		s.removeWorkerStream(req.NodeKey)
		return &pb.SpiderDeployResponse{Success: false, Message: fmt.Sprintf("failed to deploy to worker %s: %v", req.NodeKey, err)}, nil
	}

	return &pb.SpiderDeployResponse{Success: true, Message: fmt.Sprintf("deployed %d files", len(files))}, nil
}

// regenerateTemplateFiles fetches the spider from DB, checks if it's a
// template spider, and writes fresh template files + config.json if so.
func (s *MasterServer) regenerateTemplateFiles(ctx context.Context, spiderId, spiderDir string) {
	oid, err := primitive.ObjectIDFromHex(spiderId)
	if err != nil {
		return
	}
	col := utils.GetCollection("spiders")
	var sp models.Spider
	if err := col.FindOne(ctx, bson.M{"_id": oid}).Decode(&sp); err != nil {
		return
	}
	if sp.Type != "template" || sp.TemplateId == "" {
		return
	}
	tmpl := spider.GetTemplate(sp.TemplateId)
	if tmpl == nil {
		utils.Logger.Warnf("Template %s not found for spider %s", sp.TemplateId, spiderId)
		return
	}
	os.MkdirAll(spiderDir, 0755)
	for filename, content := range tmpl.Files {
		fp := filepath.Join(spiderDir, filename)
		if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
			utils.Logger.Warnf("Failed to write %s: %v", filename, err)
			return
		}
	}
	if sp.Config != nil {
		// Inject cookie from pool for Xueqiu and WeChat spiders
		config := sp.Config
		if strings.HasPrefix(sp.TemplateId, "xueqiu_") {
			config = s.injectPoolCookie(ctx, config, spiderId)
		} else if strings.HasPrefix(sp.TemplateId, "wechat_") {
			config = s.injectWechatPoolCookie(ctx, config, spiderId)
		}
		configBytes, err := spider.RenderConfig(config)
		if err == nil {
			os.WriteFile(filepath.Join(spiderDir, "config.json"), configBytes, 0644)
		}
	}

	utils.Logger.Infof("Regenerated template files for spider %s (template: %s)", spiderId, sp.TemplateId)
}

// injectPoolCookie selects a cookie from the Xueqiu cookie pool and injects
// it into the spider config. Returns a copy of config with cookie set.
func (s *MasterServer) injectPoolCookie(ctx context.Context, config map[string]interface{}, spiderId string) map[string]interface{} {
	col := utils.GetCollection("xueqiu_cookies")
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	// Reset daily counters
	_, _ = col.UpdateMany(ctx, bson.M{
		"daily_reset_at": bson.M{"$lt": today},
	}, bson.M{
		"$set": bson.M{"daily_tasks": 0, "daily_reset_at": today},
	})

	// Select least-recently-used active cookie within limits
	filter := bson.M{
		"status":        models.XueqiuCookieStatusActive,
		"cooldown_till": bson.M{"$not": bson.M{"$gt": now}},
		"daily_tasks":   bson.M{"$lt": int64(models.XueqiuCookieMaxDailyTasks)},
	}
	var cookie models.XueqiuCookie
	err := col.FindOne(ctx, filter,
		options.FindOne().SetSort(bson.D{{Key: "last_used_at", Value: 1}}),
	).Decode(&cookie)
	if err != nil {
		utils.Logger.Infof("No cookie available in pool for spider %s", spiderId)
		return config
	}

	// Reserve: update usage stats and set cooldown
	cooldownUntil := now.Add(time.Duration(models.XueqiuCookieCooldownMin) * time.Minute)
	_, _ = col.UpdateOne(ctx, bson.M{"_id": cookie.Id}, bson.M{
		"$set": bson.M{
			"last_used_at":  now,
			"cooldown_till": cooldownUntil,
		},
		"$inc": bson.M{
			"total_tasks": 1,
			"daily_tasks": 1,
		},
	})

	// Create a copy of config with cookie injected
	injected := make(map[string]interface{}, len(config)+1)
	for k, v := range config {
		injected[k] = v
	}
	injected["cookie"] = cookie.Cookie

	utils.Logger.Infof("Injected pool cookie %s (%s) for spider %s",
		cookie.Nickname, cookie.Id.Hex(), spiderId)
	s.lastCookieInjected.Store(spiderId, cookie.Id)
	return injected
}

// PopLastCookieId returns and removes the last injected cookie ID for a spider.
func (s *MasterServer) PopLastCookieId(spiderId string) (primitive.ObjectID, bool) {
	if v, ok := s.lastCookieInjected.LoadAndDelete(spiderId); ok {
		return v.(primitive.ObjectID), true
	}
	return primitive.NilObjectID, false
}

// markCookieError records an error for a cookie with progressive cooldown.
// 1st error → 5min cooldown, 2nd → 10min, 3rd+ → mark expired.
func (s *MasterServer) markCookieError(ctx context.Context, cookieID primitive.ObjectID, errMsg string) {
	col := utils.GetCollection("xueqiu_cookies")
	now := time.Now()

	var ck models.XueqiuCookie
	if err := col.FindOne(ctx, bson.M{"_id": cookieID}).Decode(&ck); err != nil {
		return
	}

	newErrCount := ck.ErrorCount + 1
	setFields := bson.M{
		"last_error":  errMsg,
		"error_count": newErrCount,
	}

	if newErrCount >= models.XueqiuCookieMaxConsecError {
		setFields["status"] = models.XueqiuCookieStatusExpired
		utils.Logger.Warnf("Cookie %s expired after %d consecutive errors: %s", cookieID.Hex(), newErrCount, errMsg)
	} else {
		var cooldownMin int
		switch newErrCount {
		case 1:
			cooldownMin = models.XueqiuCookieCooldownErr1
		default:
			cooldownMin = models.XueqiuCookieCooldownErr2
		}
		setFields["cooldown_till"] = now.Add(time.Duration(cooldownMin) * time.Minute)
		utils.Logger.Infof("Cookie %s error #%d, cooldown %dmin: %s", cookieID.Hex(), newErrCount, cooldownMin, errMsg)
	}

	_, _ = col.UpdateOne(ctx, bson.M{"_id": cookieID}, bson.M{"$set": setFields})
}

// clearCookieError resets error count after a successful task.
func (s *MasterServer) clearCookieError(ctx context.Context, cookieID primitive.ObjectID) {
	col := utils.GetCollection("xueqiu_cookies")
	_, _ = col.UpdateOne(ctx, bson.M{"_id": cookieID}, bson.M{
		"$set": bson.M{"error_count": 0, "last_error": ""},
	})
}

// HasXueqiuCookieAvailable checks if at least one cookie is available
// in the Xueqiu cookie pool without reserving it.
func (s *MasterServer) HasXueqiuCookieAvailable(ctx context.Context) bool {
	col := utils.GetCollection("xueqiu_cookies")
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	// Reset daily counters
	_, _ = col.UpdateMany(ctx, bson.M{
		"daily_reset_at": bson.M{"$lt": today},
	}, bson.M{
		"$set": bson.M{"daily_tasks": 0, "daily_reset_at": today},
	})

	filter := bson.M{
		"status":        models.XueqiuCookieStatusActive,
		"cooldown_till": bson.M{"$not": bson.M{"$gt": now}},
		"daily_tasks":   bson.M{"$lt": int64(models.XueqiuCookieMaxDailyTasks)},
	}
	count, err := col.CountDocuments(ctx, filter)
	if err != nil {
		utils.Logger.Warnf("HasXueqiuCookieAvailable: query error: %v", err)
	}
	return err == nil && count > 0
}

// injectWechatPoolCookie selects a cookie from the WeChat cookie pool and injects
// it into the spider config. Returns a copy of config with cookie and token set.
func (s *MasterServer) injectWechatPoolCookie(ctx context.Context, config map[string]interface{}, spiderId string) map[string]interface{} {
	col := utils.GetCollection("wechat_cookies")
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	// Reset daily counters
	_, _ = col.UpdateMany(ctx, bson.M{
		"daily_reset_at": bson.M{"$lt": today},
	}, bson.M{
		"$set": bson.M{"daily_tasks": 0, "daily_reset_at": today},
	})

	// Select least-recently-used active cookie within limits
	filter := bson.M{
		"status":        models.WechatCookieStatusActive,
		"cooldown_till": bson.M{"$not": bson.M{"$gt": now}},
		"daily_tasks":   bson.M{"$lt": int64(models.WechatCookieMaxDailyTasks)},
	}
	var cookie models.WechatCookie
	err := col.FindOne(ctx, filter,
		options.FindOne().SetSort(bson.D{{Key: "last_used_at", Value: 1}}),
	).Decode(&cookie)
	if err != nil {
		utils.Logger.Infof("No WeChat cookie available in pool for spider %s", spiderId)
		return config
	}

	// Reserve: update usage stats and set cooldown
	cooldownUntil := now.Add(time.Duration(models.WechatCookieCooldownMin) * time.Minute)
	_, _ = col.UpdateOne(ctx, bson.M{"_id": cookie.Id}, bson.M{
		"$set": bson.M{
			"last_used_at":  now,
			"cooldown_till": cooldownUntil,
		},
		"$inc": bson.M{
			"total_tasks": 1,
			"daily_tasks": 1,
		},
	})

	// Create a copy of config with cookie and token injected
	injected := make(map[string]interface{}, len(config)+2)
	for k, v := range config {
		injected[k] = v
	}
	injected["cookie"] = cookie.Cookie
	if cookie.Token != "" {
		injected["token"] = cookie.Token
	}

	utils.Logger.Infof("Injected WeChat pool cookie %s (%s) for spider %s",
		cookie.Nickname, cookie.Id.Hex(), spiderId)
	s.lastCookieInjected.Store(spiderId, cookie.Id)
	return injected
}

// HasWechatCookieAvailable checks if at least one WeChat cookie is available
// in the WeChat cookie pool without reserving it.
func (s *MasterServer) HasWechatCookieAvailable(ctx context.Context) bool {
	col := utils.GetCollection("wechat_cookies")
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	// Reset daily counters
	_, _ = col.UpdateMany(ctx, bson.M{
		"daily_reset_at": bson.M{"$lt": today},
	}, bson.M{
		"$set": bson.M{"daily_tasks": 0, "daily_reset_at": today},
	})

	filter := bson.M{
		"status":        models.WechatCookieStatusActive,
		"cooldown_till": bson.M{"$not": bson.M{"$gt": now}},
		"daily_tasks":   bson.M{"$lt": int64(models.WechatCookieMaxDailyTasks)},
	}
	count, err := col.CountDocuments(ctx, filter)
	if err != nil {
		utils.Logger.Warnf("HasWechatCookieAvailable: query error: %v", err)
	}
	return err == nil && count > 0
}

// markWechatCookieError records an error for a WeChat cookie with progressive cooldown.
func (s *MasterServer) markWechatCookieError(ctx context.Context, cookieID primitive.ObjectID, errMsg string) {
	col := utils.GetCollection("wechat_cookies")
	now := time.Now()

	var ck models.WechatCookie
	if err := col.FindOne(ctx, bson.M{"_id": cookieID}).Decode(&ck); err != nil {
		return
	}

	newErrCount := ck.ErrorCount + 1
	setFields := bson.M{
		"last_error":  errMsg,
		"error_count": newErrCount,
	}

	if newErrCount >= models.WechatCookieMaxConsecError {
		setFields["status"] = models.WechatCookieStatusExpired
		utils.Logger.Warnf("WeChat cookie %s expired after %d consecutive errors: %s", cookieID.Hex(), newErrCount, errMsg)
	} else {
		var cooldownMin int
		switch newErrCount {
		case 1:
			cooldownMin = models.WechatCookieCooldownErr1
		default:
			cooldownMin = models.WechatCookieCooldownErr2
		}
		setFields["cooldown_till"] = now.Add(time.Duration(cooldownMin) * time.Minute)
		utils.Logger.Infof("WeChat cookie %s error #%d, cooldown %dmin: %s", cookieID.Hex(), newErrCount, cooldownMin, errMsg)
	}

	_, _ = col.UpdateOne(ctx, bson.M{"_id": cookieID}, bson.M{"$set": setFields})
}

// clearWechatCookieError resets error count after a successful task.
func (s *MasterServer) clearWechatCookieError(ctx context.Context, cookieID primitive.ObjectID) {
	col := utils.GetCollection("wechat_cookies")
	_, _ = col.UpdateOne(ctx, bson.M{"_id": cookieID}, bson.M{
		"$set": bson.M{"error_count": 0, "last_error": ""},
	})
}

func (s *MasterServer) SyncFiles(stream pb.SpiderService_SyncFilesServer) error {
	var filesCount int32
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.SyncResponse{Success: true, FilesSynced: filesCount})
		}
		if err != nil {
			return err
		}
		if chunk.IsLast {
			filesCount++
		}
	}
}

func (s *MasterServer) SendTaskToWorker(nodeKey string, task *models.Task) error {
	s.mu.RLock()
	stream, ok := s.workerStreams[nodeKey]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("worker %s not connected", nodeKey)
	}

	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	err = stream.Send(&pb.StreamMessage{Code: "run_task", NodeKey: nodeKey, Data: data, Timestamp: time.Now().Unix()})
	if err != nil {
		// Stream is broken, remove the worker from the map and mark as offline
		s.removeWorkerStream(nodeKey)
		return fmt.Errorf("failed to send task to worker %s: %w", nodeKey, err)
	}

	return nil
}

func (s *MasterServer) SendCancelToWorker(nodeKey, taskId string) error {
	s.mu.RLock()
	stream, ok := s.workerStreams[nodeKey]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("worker %s not connected", nodeKey)
	}

	data, _ := json.Marshal(map[string]string{"task_id": taskId})
	err := stream.Send(&pb.StreamMessage{Code: "cancel_task", NodeKey: nodeKey, Data: data, Timestamp: time.Now().Unix()})
	if err != nil {
		// Stream is broken, remove the worker from the map and mark as offline
		s.removeWorkerStream(nodeKey)
		return fmt.Errorf("failed to send cancel to worker %s: %w", nodeKey, err)
	}

	return nil
}

func (s *MasterServer) GetConnectedWorkers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.workerStreams))
	for k := range s.workerStreams {
		keys = append(keys, k)
	}
	return keys
}

// DisconnectWorker sends a disconnect message to the worker and removes its stream.
// Called when a node is deleted from the API to ensure immediate disconnection.
func (s *MasterServer) DisconnectWorker(nodeKey string) {
	s.mu.RLock()
	stream, ok := s.workerStreams[nodeKey]
	s.mu.RUnlock()

	if !ok {
		return
	}

	// Send disconnect message so the worker knows to reconnect
	_ = stream.Send(&pb.StreamMessage{
		Code:      "disconnect",
		NodeKey:   nodeKey,
		Timestamp: time.Now().Unix(),
	})

	// Remove from active streams (also sets node offline in DB if doc still exists)
	s.removeWorkerStream(nodeKey)
	utils.Logger.Infof("Disconnected worker: %s (node deleted)", nodeKey)
	utils.WriteSystemLog("info", "grpc", fmt.Sprintf("主动断开工作节点: %s", nodeKey), "节点已被删除")
}

func (s *MasterServer) addWorkerStream(nodeKey string, stream pb.NodeService_SubscribeServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workerStreams[nodeKey] = stream
}

func (s *MasterServer) removeWorkerStream(nodeKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workerStreams, nodeKey)

	ctx := context.Background()
	col := utils.GetCollection("nodes")
	_, _ = col.UpdateOne(ctx, bson.M{"key": nodeKey}, bson.M{
		"$set": bson.M{"status": models.NodeStatusOffline, "updated_at": time.Now()},
	})
	utils.Logger.Infof("Worker disconnected: %s", nodeKey)
	utils.WriteSystemLog("warn", "grpc", fmt.Sprintf("工作节点断开连接: %s", nodeKey), "")
}

func (s *MasterServer) handleTaskStatusUpdate(task *models.Task) {
	ctx := context.Background()
	col := utils.GetCollection("tasks")
	update := bson.M{"status": task.Status, "updated_at": time.Now()}
	if task.Error != "" {
		update["error"] = task.Error
	}
	if !task.EndTs.IsZero() {
		update["end_ts"] = task.EndTs
	}
	if task.ResultCount > 0 {
		update["result_count"] = task.ResultCount
	}
	_, _ = col.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{"$set": update})

	switch task.Status {
	case models.TaskStatusFinished:
		utils.WriteSystemLog("info", "task", fmt.Sprintf("任务完成: %s, 采集 %d 条数据", task.Id.Hex(), task.ResultCount), "")
	case models.TaskStatusError:
		utils.WriteSystemLog("error", "task", fmt.Sprintf("任务失败: %s", task.Id.Hex()), task.Error)
	}
}

func (s *MasterServer) handleLogMessage(log *models.TaskLog) {
	ctx := context.Background()
	col := utils.GetCollection("task_logs")
	_, _ = col.InsertOne(ctx, log)
}
