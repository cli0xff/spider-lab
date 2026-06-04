package config

import (
	"os"
	"strconv"
)

type Config struct {
	IsMaster       bool
	MongoURI       string
	MongoDatabase  string
	GRPCAddress    string
	GRPCAuthKey    string
	APIAddress     string
	MasterGRPCAddr string
	JWTSecret      string
	NodeKey        string
	MaxRunners     int
	SpiderDir      string
	// Storage (MinIO)
	StorageEndpoint  string
	StorageAccessKey string
	StorageSecretKey string
	StorageBucket    string
	StorageUseSSL    bool
}

func Load() *Config {
	cfg := &Config{
		IsMaster:       getEnvBool("SPIDER_LAB_MASTER", true),
		MongoURI:       getEnv("SPIDER_LAB_MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:  getEnv("SPIDER_LAB_MONGO_DB", "spider_lab"),
		GRPCAddress:    getEnv("SPIDER_LAB_GRPC_ADDRESS", "0.0.0.0:9666"),
		GRPCAuthKey:    getEnv("SPIDER_LAB_GRPC_AUTH_KEY", "spider-lab-secret"),
		APIAddress:     getEnv("SPIDER_LAB_API_ADDRESS", "0.0.0.0:8080"),
		MasterGRPCAddr: getEnv("SPIDER_LAB_MASTER_GRPC", "localhost:9666"),
		JWTSecret:      getEnv("SPIDER_LAB_JWT_SECRET", "spider-lab-jwt-secret-key"),
		NodeKey:        getEnv("SPIDER_LAB_NODE_KEY", ""),
		MaxRunners:     getEnvInt("SPIDER_LAB_MAX_RUNNERS", 8),
		SpiderDir:      getEnv("SPIDER_LAB_SPIDER_DIR", "/tmp/spider-lab/spiders"),
		// Storage
		StorageEndpoint:  getEnv("SPIDER_LAB_STORAGE_ENDPOINT", ""),
		StorageAccessKey: getEnv("SPIDER_LAB_STORAGE_ACCESS_KEY", "spider-lab"),
		StorageSecretKey: getEnv("SPIDER_LAB_STORAGE_SECRET_KEY", "spider-lab-secret"),
		StorageBucket:    getEnv("SPIDER_LAB_STORAGE_BUCKET", "spider-lab"),
		StorageUseSSL:    getEnvBool("SPIDER_LAB_STORAGE_USE_SSL", false),
	}
	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		i, err := strconv.Atoi(val)
		if err == nil {
			return i
		}
	}
	return defaultVal
}
