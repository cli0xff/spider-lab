package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crawlab-team/spider-lab/internal/config"
	"github.com/crawlab-team/spider-lab/internal/master/api"
	mastergrpc "github.com/crawlab-team/spider-lab/internal/master/grpc"
	"github.com/crawlab-team/spider-lab/internal/master/scheduler"
	"github.com/crawlab-team/spider-lab/internal/master/spider"
	"github.com/crawlab-team/spider-lab/internal/models"
	"github.com/crawlab-team/spider-lab/internal/storage"
	"github.com/crawlab-team/spider-lab/internal/utils"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	utils.InitLogger()
	cfg := config.Load()

	utils.Logger.Info("Starting Spider Lab Master Node...")
	utils.Logger.Infof("MongoDB: %s, Database: %s", cfg.MongoURI, cfg.MongoDatabase)
	utils.Logger.Infof("gRPC Address: %s, API Address: %s", cfg.GRPCAddress, cfg.APIAddress)

	if err := utils.InitMongo(cfg.MongoURI, cfg.MongoDatabase); err != nil {
		utils.Logger.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	utils.Logger.Info("Connected to MongoDB")

	utils.InitJWT(cfg.JWTSecret)
	ensureAdminUser()

	nodeKey := cfg.NodeKey
	if nodeKey == "" {
		nodeKey = "master-" + uuid.New().String()[:8]
	}
	registerMasterNode(nodeKey, cfg)

	grpcServer := mastergrpc.NewMasterServer(cfg.GRPCAddress, cfg.GRPCAuthKey, cfg.SpiderDir)
	go func() {
		if err := grpcServer.Start(); err != nil {
			utils.Logger.Fatalf("gRPC server failed: %v", err)
		}
	}()

	sched := scheduler.NewScheduler(grpcServer)
	if err := sched.Start(); err != nil {
		utils.Logger.Fatalf("Scheduler failed: %v", err)
	}

	deployer := spider.NewDeployer(cfg.SpiderDir)

	// Initialize storage (MinIO) — optional, nil if endpoint not configured
	var storageClient *storage.Client
	if cfg.StorageEndpoint != "" {
		var err error
		storageClient, err = storage.NewClient(storage.Config{
			Endpoint:  cfg.StorageEndpoint,
			AccessKey: cfg.StorageAccessKey,
			SecretKey: cfg.StorageSecretKey,
			Bucket:    cfg.StorageBucket,
			UseSSL:    cfg.StorageUseSSL,
		})
		if err != nil {
			utils.Logger.Warnf("Storage init failed: %v (file serving will use local FS)", err)
		} else {
			if err := storageClient.EnsureBucket(context.Background()); err != nil {
				utils.Logger.Warnf("Storage bucket creation failed: %v", err)
			} else {
				utils.Logger.Infof("Storage connected: %s (bucket: %s)", cfg.StorageEndpoint, cfg.StorageBucket)
			}
		}
	}

	router := api.NewRouter(sched, deployer, grpcServer, storageClient)
	go func() {
		utils.Logger.Infof("API server listening on %s", cfg.APIAddress)
		if err := router.Run(cfg.APIAddress); err != nil {
			utils.Logger.Fatalf("API server failed: %v", err)
		}
	}()

	utils.Logger.Info("Spider Lab Master Node started successfully")
	utils.WriteSystemLog("info", "master", "Spider Lab 主节点启动成功", cfg.APIAddress)

	// One-time backfill: populate published_at on existing results/posts
	go backfillPublishedAt()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	utils.Logger.Info("Shutting down Master Node...")
	sched.Stop()
	grpcServer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = utils.MongoClient.Disconnect(ctx)

	utils.Logger.Info("Master Node stopped")
}

func ensureAdminUser() {
	col := utils.GetCollection("users")
	var user models.User
	err := col.FindOne(context.Background(), bson.M{"username": "admin"}).Decode(&user)
	if err != nil {
		hashedPw, _ := utils.HashPassword("admin123")
		now := time.Now()
		_, err = col.InsertOne(context.Background(), models.User{
			Username: "admin", Password: hashedPw, Email: "admin@spider-lab.io", Role: models.RoleAdmin, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			utils.Logger.Warnf("Failed to create admin user: %v", err)
		} else {
			utils.Logger.Info("Default admin user created (admin/admin123)")
		}
	}
}

func registerMasterNode(nodeKey string, cfg *config.Config) {
	col := utils.GetCollection("nodes")
	now := time.Now()
	ip := getLocalIP()

	upsert := true
	_, _ = col.UpdateOne(context.Background(), bson.M{"key": nodeKey}, bson.M{
		"$set": bson.M{
			"name": "Master Node", "is_master": true, "status": models.NodeStatusOnline,
			"ip": ip, "max_runners": cfg.MaxRunners, "last_heartbeat": now, "updated_at": now,
		},
		"$setOnInsert": bson.M{"key": nodeKey, "created_at": now},
	}, &options.UpdateOptions{Upsert: &upsert})

	utils.Logger.Infof("Master node registered: key=%s, ip=%s", nodeKey, ip)
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

// backfillPublishedAt is a one-time migration that populates the published_at
// field on existing results and posts documents by parsing item.created_at.
func backfillPublishedAt() {
	ctx := context.Background()
	zeroTime := time.Time{}

	for _, colName := range []string{"results", "posts"} {
		col := utils.GetCollection(colName)
		// Find docs where published_at is missing or zero
		filter := bson.M{"$or": bson.A{
			bson.M{"published_at": bson.M{"$exists": false}},
			bson.M{"published_at": zeroTime},
		}}
		cursor, err := col.Find(ctx, filter)
		if err != nil {
			utils.Logger.Warnf("backfill %s: query failed: %v", colName, err)
			continue
		}
		updated := 0
		for cursor.Next(ctx) {
			var doc struct {
				Id   bson.RawValue              `bson:"_id"`
				Item map[string]interface{} `bson:"item"`
			}
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			pubAt := mastergrpc.ParsePublishedAt(doc.Item)
			if pubAt.IsZero() {
				continue
			}
			_, _ = col.UpdateOne(ctx, bson.M{"_id": doc.Id}, bson.M{"$set": bson.M{"published_at": pubAt}})
			updated++
		}
		cursor.Close(ctx)
		if updated > 0 {
			utils.Logger.Infof("backfill %s: updated %d documents with published_at", colName, updated)
		}
	}
}
