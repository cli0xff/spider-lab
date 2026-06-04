package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/crawlab-team/spider-lab/internal/config"
	"github.com/crawlab-team/spider-lab/internal/models"
	"github.com/crawlab-team/spider-lab/internal/storage"
	"github.com/crawlab-team/spider-lab/internal/utils"
	workergrpc "github.com/crawlab-team/spider-lab/internal/worker/grpc"
	"github.com/crawlab-team/spider-lab/internal/worker/handler"
	"github.com/google/uuid"
)

func main() {
	utils.InitLogger()
	cfg := config.Load()

	utils.Logger.Info("Starting Spider Lab Worker Node...")
	utils.Logger.Infof("Master gRPC: %s", cfg.MasterGRPCAddr)

	nodeKey := cfg.NodeKey
	if nodeKey == "" {
		hostname, _ := os.Hostname()
		nodeKey = "worker-" + hostname + "-" + uuid.New().String()[:8]
	}

	client := workergrpc.NewWorkerClient(cfg.MasterGRPCAddr, nodeKey, cfg.SpiderDir)
	if err := client.Connect(); err != nil {
		utils.Logger.Fatalf("Failed to connect to master: %v", err)
	}
	defer client.Close()

	hostname, _ := os.Hostname()
	ip := getLocalIP()
	nodeId, err := client.Register(hostname, ip, 0, cfg.MaxRunners)
	if err != nil {
		utils.Logger.Fatalf("Failed to register: %v", err)
	}
	utils.Logger.Infof("Registered as node %s (id: %s)", nodeKey, nodeId)

	// Initialize storage client (optional — nil if not configured)
	var storageClient *storage.Client
	if cfg.StorageEndpoint != "" {
		sc, err := storage.NewClient(storage.Config{
			Endpoint:  cfg.StorageEndpoint,
			AccessKey: cfg.StorageAccessKey,
			SecretKey: cfg.StorageSecretKey,
			Bucket:    cfg.StorageBucket,
			UseSSL:    cfg.StorageUseSSL,
		})
		if err != nil {
			utils.Logger.Warnf("Storage init failed (will use local only): %v", err)
		} else if sc != nil {
			if err := sc.EnsureBucket(context.Background()); err != nil {
				utils.Logger.Warnf("Storage bucket init failed: %v", err)
			} else {
				storageClient = sc
				utils.Logger.Info("Storage (MinIO) connected")
			}
		}
	}

	taskHandler := handler.NewHandler(client, cfg.SpiderDir, cfg.MaxRunners, storageClient)

	client.SetTaskHandler(
		func(task *models.Task) { taskHandler.HandleTask(task) },
		func(taskId string) { taskHandler.CancelTask(taskId) },
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.StartHeartbeat(ctx, 15*time.Second, func() (float64, float64, float64, int32, int32) {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		cpuUsage := float64(runtime.NumGoroutine()) / float64(runtime.NumCPU()) * 100
		memUsage := float64(memStats.Alloc) / float64(memStats.Sys) * 100
		return cpuUsage, memUsage, 0, int32(taskHandler.GetActiveRunners()), int32(taskHandler.GetAvailableRunners())
	})

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			err := client.Subscribe(ctx)
			if err != nil {
				utils.Logger.Errorf("Stream disconnected: %v, reconnecting in 5s...", err)
				select {
				case <-time.After(5 * time.Second):
				case <-ctx.Done():
					return
				}
				if err := client.Connect(); err != nil {
					utils.Logger.Errorf("Reconnect failed: %v", err)
					continue
				}
				client.Register(hostname, ip, 0, cfg.MaxRunners)
			}
		}
	}()

	utils.Logger.Info("Spider Lab Worker Node started successfully")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	utils.Logger.Info("Shutting down Worker Node...")
	cancel()
	client.Close()
	utils.Logger.Info("Worker Node stopped")
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}
