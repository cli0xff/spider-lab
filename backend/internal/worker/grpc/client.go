package grpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/crawlab-team/spider-lab/internal/models"
	"github.com/crawlab-team/spider-lab/internal/utils"
	pb "github.com/crawlab-team/spider-lab/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type WorkerClient struct {
	conn             *grpc.ClientConn
	nodeClient       pb.NodeServiceClient
	taskClient       pb.TaskServiceClient
	spiderClient     pb.SpiderServiceClient
	masterAddr       string
	nodeKey          string
	spiderDir        string
	onTaskReceived   func(task *models.Task)
	onTaskCancel     func(taskId string)
}

func NewWorkerClient(masterAddr, nodeKey, spiderDir string) *WorkerClient {
	return &WorkerClient{masterAddr: masterAddr, nodeKey: nodeKey, spiderDir: spiderDir}
}

func (c *WorkerClient) SetTaskHandler(onTask func(task *models.Task), onCancel func(taskId string)) {
	c.onTaskReceived = onTask
	c.onTaskCancel = onCancel
}

func (c *WorkerClient) Connect() error {
	conn, err := grpc.NewClient(c.masterAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to master: %w", err)
	}

	c.conn = conn
	c.nodeClient = pb.NewNodeServiceClient(conn)
	c.taskClient = pb.NewTaskServiceClient(conn)
	c.spiderClient = pb.NewSpiderServiceClient(conn)

	return nil
}

func (c *WorkerClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *WorkerClient) Register(name, ip string, port, maxRunners int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.nodeClient.Register(ctx, &pb.NodeRegisterRequest{
		Key: c.nodeKey, Name: name, Ip: ip, Port: int32(port), IsMaster: false, MaxRunners: int32(maxRunners),
	})
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("registration failed: %s", resp.Message)
	}

	utils.Logger.Infof("Registered with master: nodeId=%s", resp.NodeId)
	return resp.NodeId, nil
}

func (c *WorkerClient) StartHeartbeat(ctx context.Context, interval time.Duration, getMetrics func() (float64, float64, float64, int32, int32)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cpu, mem, disk, active, available := getMetrics()
			resp, err := c.nodeClient.Heartbeat(ctx, &pb.HeartbeatRequest{
				NodeKey: c.nodeKey, CpuUsage: cpu, MemoryUsage: mem, DiskUsage: disk, ActiveRunners: active, AvailableRunners: available,
			})
			if err != nil {
				utils.Logger.Warnf("Heartbeat failed: %v", err)
			} else if !resp.Success {
				utils.Logger.Warn("Heartbeat rejected — node may have been deleted or disabled")
			}
		}
	}
}

func (c *WorkerClient) Subscribe(ctx context.Context) error {
	stream, err := c.nodeClient.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	err = stream.Send(&pb.StreamMessage{Code: "identify", NodeKey: c.nodeKey, Timestamp: time.Now().Unix()})
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = stream.Send(&pb.StreamMessage{Code: "heartbeat", NodeKey: c.nodeKey, Timestamp: time.Now().Unix()})
			}
		}
	}()

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch msg.Code {
		case "run_task":
			var task models.Task
			if err := json.Unmarshal(msg.Data, &task); err == nil && c.onTaskReceived != nil {
				go c.onTaskReceived(&task)
			}
		case "cancel_task":
			var data map[string]string
			if err := json.Unmarshal(msg.Data, &data); err == nil && c.onTaskCancel != nil {
				if taskId, ok := data["task_id"]; ok {
					go c.onTaskCancel(taskId)
				}
			}
		case "deploy_spider":
			c.handleDeploySpider(msg.Data)
		case "disconnect":
			utils.Logger.Warn("Received disconnect from master — node was deleted, will re-register")
			return fmt.Errorf("disconnected by master")
		}
	}
}

func (c *WorkerClient) ReportTaskStatus(taskId, status, errMsg string, pid int, startTs, endTs int64, resultCount int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.taskClient.ReportTaskStatus(ctx, &pb.TaskStatusReport{
		TaskId: taskId, NodeKey: c.nodeKey, Status: status, Error: errMsg, Pid: int32(pid), StartTs: startTs, EndTs: endTs, ResultCount: resultCount,
	})
	return err
}

// SendResults sends spider result items to master via the StreamLogs RPC with level="result".
// Each item is sent as a JSON-encoded LogMessage.
func (c *WorkerClient) SendResults(taskId, spiderId string, items []map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := c.taskClient.StreamLogs(ctx)
	if err != nil {
		return fmt.Errorf("failed to open result stream: %w", err)
	}

	for _, item := range items {
		// Encode spider_id into the item payload so master can extract it
		payload := map[string]interface{}{
			"spider_id": spiderId,
			"item":      item,
		}
		data, _ := json.Marshal(payload)
		if err := stream.Send(&pb.LogMessage{
			TaskId:    taskId,
			NodeKey:   c.nodeKey,
			Content:   string(data),
			Timestamp: time.Now().Unix(),
			Level:     "result",
		}); err != nil {
			return fmt.Errorf("failed to send result: %w", err)
		}
	}

	_, err = stream.CloseAndRecv()
	return err
}

func (c *WorkerClient) handleDeploySpider(data []byte) {
	var payload struct {
		SpiderId string `json:"spider_id"`
		Files    []struct {
			RelPath string `json:"relPath"`
			Data    string `json:"data"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		utils.Logger.Errorf("Failed to parse deploy_spider payload: %v", err)
		return
	}

	spiderDir := filepath.Join(c.spiderDir, payload.SpiderId)
	if err := os.MkdirAll(spiderDir, 0755); err != nil {
		utils.Logger.Errorf("Failed to create spider dir %s: %v", spiderDir, err)
		return
	}

	for _, f := range payload.Files {
		filePath := filepath.Join(spiderDir, f.RelPath)
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			utils.Logger.Errorf("Failed to create dir %s: %v", dir, err)
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(f.Data)
		if err != nil {
			utils.Logger.Errorf("Failed to decode file %s: %v", f.RelPath, err)
			continue
		}
		if err := os.WriteFile(filePath, decoded, 0644); err != nil {
			utils.Logger.Errorf("Failed to write file %s: %v", filePath, err)
			continue
		}
	}

	utils.Logger.Infof("Deployed spider %s: %d files", payload.SpiderId, len(payload.Files))
}

// TaskLogStream wraps a StreamLogs gRPC stream for thread-safe real-time log/result sending.
type TaskLogStream struct {
	stream  pb.TaskService_StreamLogsClient
	mu      sync.Mutex
	nodeKey string
	cancel  context.CancelFunc
}

// OpenLogStream creates a persistent gRPC stream for sending task logs and results in real-time.
func (c *WorkerClient) OpenLogStream() (*TaskLogStream, error) {
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := c.taskClient.StreamLogs(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to open log stream: %w", err)
	}
	return &TaskLogStream{stream: stream, nodeKey: c.nodeKey, cancel: cancel}, nil
}

// Log sends a log message to the master via the persistent stream.
func (tls *TaskLogStream) Log(taskId, content, level string) {
	tls.mu.Lock()
	defer tls.mu.Unlock()
	_ = tls.stream.Send(&pb.LogMessage{
		TaskId:    taskId,
		NodeKey:   tls.nodeKey,
		Content:   content,
		Timestamp: time.Now().Unix(),
		Level:     level,
	})
}

// Result sends a result item to the master via the persistent stream.
func (tls *TaskLogStream) Result(taskId, spiderId string, item map[string]interface{}) {
	payload := map[string]interface{}{
		"spider_id": spiderId,
		"item":      item,
	}
	data, _ := json.Marshal(payload)
	tls.mu.Lock()
	defer tls.mu.Unlock()
	_ = tls.stream.Send(&pb.LogMessage{
		TaskId:    taskId,
		NodeKey:   tls.nodeKey,
		Content:   string(data),
		Timestamp: time.Now().Unix(),
		Level:     "result",
	})
}

// Close closes the log stream gracefully.
func (tls *TaskLogStream) Close() {
	tls.mu.Lock()
	defer tls.mu.Unlock()
	_, _ = tls.stream.CloseAndRecv()
	tls.cancel()
}
