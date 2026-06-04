package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/crawlab-team/spider-lab/internal/models"
	"github.com/crawlab-team/spider-lab/internal/storage"
	"github.com/crawlab-team/spider-lab/internal/utils"
	workergrpc "github.com/crawlab-team/spider-lab/internal/worker/grpc"
)

type Runner struct {
	task      *models.Task
	spiderDir string
	client    *workergrpc.WorkerClient
	storage   *storage.Client
	cmd       *exec.Cmd
	mu        sync.Mutex
	cancelled bool
}

func NewRunner(task *models.Task, spiderDir string, client *workergrpc.WorkerClient, sc *storage.Client) *Runner {
	return &Runner{task: task, spiderDir: spiderDir, client: client, storage: sc}
}

func (r *Runner) Run() {
	taskId := r.task.Id.Hex()
	spiderId := r.task.SpiderId.Hex()
	startTs := time.Now()

	_ = r.client.ReportTaskStatus(taskId, models.TaskStatusRunning, "", 0, startTs.Unix(), 0, 0)

	// Open persistent gRPC log stream for real-time log/result delivery
	logStream, err := r.client.OpenLogStream()
	if err != nil {
		utils.Logger.Warnf("Task %s: failed to open log stream (logs will not be streamed): %v", taskId, err)
	}
	defer func() {
		if logStream != nil {
			logStream.Close()
		}
	}()

	sendLog := func(content, level string) {
		if logStream != nil {
			logStream.Log(taskId, content, level)
		}
	}

	workDir := filepath.Join(r.spiderDir, r.task.SpiderId.Hex())
	if err := os.MkdirAll(workDir, 0755); err != nil {
		r.reportError(taskId, startTs, fmt.Sprintf("failed to create work dir: %v", err))
		return
	}

	// Install dependencies if dependency files exist
	sendLog("检查并安装依赖...", "info")
	if err := r.installDeps(workDir); err != nil {
		sendLog(fmt.Sprintf("依赖安装警告: %v", err), "warn")
		utils.Logger.Warnf("Task %s: dependency install warning: %v", taskId, err)
	}

	cmdStr := r.task.Cmd
	if r.task.Param != "" {
		cmdStr = cmdStr + " " + r.task.Param
	}

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		r.reportError(taskId, startTs, "empty command")
		return
	}

	r.mu.Lock()
	r.cmd = exec.Command(parts[0], parts[1:]...)
	r.cmd.Dir = workDir
	r.cmd.Env = append(os.Environ(),
		fmt.Sprintf("SPIDER_LAB_TASK_ID=%s", taskId),
		fmt.Sprintf("SPIDER_LAB_SPIDER_ID=%s", spiderId),
		"PYTHONUNBUFFERED=1",
	)
	r.mu.Unlock()

	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		r.reportError(taskId, startTs, fmt.Sprintf("failed to get stdout: %v", err))
		return
	}
	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		r.reportError(taskId, startTs, fmt.Sprintf("failed to get stderr: %v", err))
		return
	}

	sendLog(fmt.Sprintf("启动命令: %s", cmdStr), "info")

	if err := r.cmd.Start(); err != nil {
		r.reportError(taskId, startTs, fmt.Sprintf("failed to start: %v", err))
		return
	}

	pid := r.cmd.Process.Pid
	_ = r.client.ReportTaskStatus(taskId, models.TaskStatusRunning, "", pid, startTs.Unix(), 0, 0)
	utils.Logger.Infof("Task %s started with PID %d", taskId, pid)
	sendLog(fmt.Sprintf("进程已启动 (PID %d)", pid), "info")

	// Capture stdout: parse JSON lines as result items, send others as logs
	var resultItems []map[string]interface{}
	var resultMu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer
		for scanner.Scan() {
			line := scanner.Text()
			utils.Logger.Debugf("[%s] %s", taskId, line)
			var item map[string]interface{}
			if json.Unmarshal([]byte(line), &item) == nil && len(item) > 0 {
				resultMu.Lock()
				resultItems = append(resultItems, item)
				resultMu.Unlock()
				// Stream result to master in real-time
				if logStream != nil {
					logStream.Result(taskId, spiderId, item)
				}
			} else if line != "" {
				// Stream non-JSON output as log message
				sendLog(line, "info")
			}
		}
	}()

	var lastError string
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			lastError = line
			utils.Logger.Warnf("[%s] STDERR: %s", taskId, line)
			sendLog(line, "warn")
		}
	}()

	err = r.cmd.Wait()
	wg.Wait() // ensure all stdout/stderr is processed
	endTs := time.Now()

	r.mu.Lock()
	cancelled := r.cancelled
	r.mu.Unlock()

	if cancelled {
		sendLog("任务已被用户取消", "warn")
		_ = r.client.ReportTaskStatus(taskId, models.TaskStatusCancelled, "cancelled by user", pid, startTs.Unix(), endTs.Unix(), 0)
		return
	}

	if err != nil {
		errMsg := lastError
		if errMsg == "" {
			errMsg = err.Error()
		}
		sendLog(fmt.Sprintf("任务失败: %s", errMsg), "error")
		_ = r.client.ReportTaskStatus(taskId, models.TaskStatusError, errMsg, pid, startTs.Unix(), endTs.Unix(), 0)
		utils.Logger.Errorf("Task %s failed: %s", taskId, errMsg)
		return
	}

	// Send result items to master (fallback for when real-time stream was unavailable)
	resultCount := int64(len(resultItems))
	if logStream == nil && resultCount > 0 {
		utils.Logger.Infof("Task %s: sending %d result items via backup stream", taskId, resultCount)
		if err := r.client.SendResults(taskId, spiderId, resultItems); err != nil {
			utils.Logger.Warnf("Task %s: failed to send results: %v", taskId, err)
		}
	}

	// Upload output directory to MinIO if storage is configured
	r.uploadOutput(spiderId, workDir, sendLog)

	sendLog(fmt.Sprintf("任务完成 — 共采集 %d 条数据", resultCount), "info")
	_ = r.client.ReportTaskStatus(taskId, models.TaskStatusFinished, "", pid, startTs.Unix(), endTs.Unix(), resultCount)
	utils.Logger.Infof("Task %s completed successfully (%d results)", taskId, resultCount)
}

func (r *Runner) Cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancelled = true
	if r.cmd != nil && r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
	}
}

func (r *Runner) reportError(taskId string, startTs time.Time, errMsg string) {
	endTs := time.Now()
	_ = r.client.ReportTaskStatus(taskId, models.TaskStatusError, errMsg, 0, startTs.Unix(), endTs.Unix(), 0)
	utils.Logger.Errorf("Task %s error: %s", taskId, errMsg)
}

// uploadOutput uploads per-task data files and shared images to MinIO storage.
func (r *Runner) uploadOutput(spiderId, workDir string, sendLog func(string, string)) {
	if r.storage == nil {
		return
	}
	taskId := r.task.Id.Hex()
	ctx := context.Background()
	totalCount := 0

	// 1. Upload per-task output (data files: JSON, CSV)
	outputDir := filepath.Join(workDir, "output", taskId)
	if _, err := os.Stat(outputDir); err == nil {
		key := "spiders/" + spiderId + "/tasks/" + taskId + "/output"
		n, err := r.storage.UploadDir(ctx, key, outputDir)
		if err != nil {
			utils.Logger.Warnf("Task %s: output upload failed: %v", taskId, err)
			sendLog(fmt.Sprintf("数据文件上传失败: %v", err), "warn")
		} else {
			totalCount += n
		}
	}

	// 2. Upload shared images (deduped across tasks by filename)
	imagesDir := filepath.Join(workDir, "images")
	if _, err := os.Stat(imagesDir); err == nil {
		key := "spiders/" + spiderId + "/images"
		n, err := r.storage.UploadDir(ctx, key, imagesDir)
		if err != nil {
			utils.Logger.Warnf("Task %s: images upload failed: %v", taskId, err)
			sendLog(fmt.Sprintf("图片上传失败: %v", err), "warn")
		} else {
			totalCount += n
		}
	}

	if totalCount > 0 {
		utils.Logger.Infof("Task %s: uploaded %d files to storage", taskId, totalCount)
		sendLog(fmt.Sprintf("已上传 %d 个文件到存储服务", totalCount), "info")
	}
}

// installDeps detects dependency files in the working directory and installs them.
func (r *Runner) installDeps(workDir string) error {
	taskId := r.task.Id.Hex()

	// Python: requirements.txt
	reqFile := filepath.Join(workDir, "requirements.txt")
	if info, err := os.Stat(reqFile); err == nil && info.Size() > 0 {
		if hasRealContent(reqFile) {
			utils.Logger.Infof("Task %s: installing Python dependencies", taskId)
			cmd := exec.Command("pip", "install", "-r", "requirements.txt", "-q", "--disable-pip-version-check")
			cmd.Dir = workDir
			cmd.Env = os.Environ()
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("pip install failed: %s: %w", string(out), err)
			}
		}
	}

	// Playwright: install browsers if playwright is in requirements
	if hasRealContent(reqFile) && fileContains(reqFile, "playwright") {
		pwCheck := filepath.Join(workDir, ".playwright_installed")
		if _, err := os.Stat(pwCheck); os.IsNotExist(err) {
			utils.Logger.Infof("Task %s: installing Playwright browsers", taskId)
			cmd := exec.Command("python3", "-m", "playwright", "install", "chromium")
			cmd.Dir = workDir
			cmd.Env = os.Environ()
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("playwright install failed: %s: %w", string(out), err)
			}
			os.WriteFile(pwCheck, []byte("ok"), 0644)
		}
	}

	// Node.js: package.json (only if node_modules missing)
	pkgFile := filepath.Join(workDir, "package.json")
	nodeModules := filepath.Join(workDir, "node_modules")
	if _, err := os.Stat(pkgFile); err == nil {
		if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
			utils.Logger.Infof("Task %s: installing Node.js dependencies", taskId)
			cmd := exec.Command("npm", "install", "--production", "--no-audit", "--no-fund")
			cmd.Dir = workDir
			cmd.Env = os.Environ()
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("npm install failed: %s: %w", string(out), err)
			}
		}
	}

	// Go: go.mod (go run handles deps automatically, but pre-download for speed)
	goMod := filepath.Join(workDir, "go.mod")
	if _, err := os.Stat(goMod); err == nil {
		goSum := filepath.Join(workDir, "go.sum")
		if _, err := os.Stat(goSum); os.IsNotExist(err) {
			utils.Logger.Infof("Task %s: downloading Go dependencies", taskId)
			cmd := exec.Command("go", "mod", "download")
			cmd.Dir = workDir
			cmd.Env = os.Environ()
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("go mod download failed: %s: %w", string(out), err)
			}
		}
	}

	return nil
}

// fileContains checks if a file contains a given substring (case-insensitive).
func fileContains(path, substr string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(substr))
}

// hasRealContent checks if a file has non-comment, non-blank lines.
func hasRealContent(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			return true
		}
	}
	return false
}
