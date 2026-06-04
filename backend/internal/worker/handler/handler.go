package handler

import (
	"sync"

	"github.com/crawlab-team/spider-lab/internal/models"
	"github.com/crawlab-team/spider-lab/internal/storage"
	"github.com/crawlab-team/spider-lab/internal/utils"
	workergrpc "github.com/crawlab-team/spider-lab/internal/worker/grpc"
	"github.com/crawlab-team/spider-lab/internal/worker/runner"
)

type Handler struct {
	mu         sync.RWMutex
	runners    map[string]*runner.Runner
	client     *workergrpc.WorkerClient
	spiderDir  string
	maxRunners int
	storage    *storage.Client
}

func NewHandler(client *workergrpc.WorkerClient, spiderDir string, maxRunners int, sc *storage.Client) *Handler {
	return &Handler{runners: make(map[string]*runner.Runner), client: client, spiderDir: spiderDir, maxRunners: maxRunners, storage: sc}
}

func (h *Handler) HandleTask(task *models.Task) {
	taskId := task.Id.Hex()
	utils.Logger.Infof("Received task: %s (cmd: %s)", taskId, task.Cmd)

	h.mu.RLock()
	activeCount := len(h.runners)
	h.mu.RUnlock()

	if activeCount >= h.maxRunners {
		utils.Logger.Warnf("Max runners reached (%d), rejecting task %s", h.maxRunners, taskId)
		_ = h.client.ReportTaskStatus(taskId, models.TaskStatusError, "max runners reached", 0, 0, 0, 0)
		return
	}

	r := runner.NewRunner(task, h.spiderDir, h.client, h.storage)
	h.mu.Lock()
	h.runners[taskId] = r
	h.mu.Unlock()

	go func() {
		r.Run()
		h.mu.Lock()
		delete(h.runners, taskId)
		h.mu.Unlock()
	}()
}

func (h *Handler) CancelTask(taskId string) {
	h.mu.RLock()
	r, ok := h.runners[taskId]
	h.mu.RUnlock()

	if !ok {
		utils.Logger.Warnf("Task %s not found for cancellation", taskId)
		return
	}
	r.Cancel()
}

func (h *Handler) GetActiveRunners() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.runners)
}

func (h *Handler) GetAvailableRunners() int {
	return h.maxRunners - h.GetActiveRunners()
}
