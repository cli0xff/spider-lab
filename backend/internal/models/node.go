package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	NodeStatusOnline  = "online"
	NodeStatusOffline = "offline"
)

type Node struct {
	Id               primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Key              string             `json:"key" bson:"key"`
	Name             string             `json:"name" bson:"name"`
	IsMaster         bool               `json:"is_master" bson:"is_master"`
	Status           string             `json:"status" bson:"status"`
	Enabled          bool               `json:"enabled" bson:"enabled"`
	IP               string             `json:"ip" bson:"ip"`
	Port             int                `json:"port" bson:"port"`
	MaxRunners       int                `json:"max_runners" bson:"max_runners"`
	ActiveRunners    int                `json:"active_runners" bson:"active_runners"`
	AvailableRunners int                `json:"available_runners" bson:"available_runners"`
	Description      string             `json:"description" bson:"description"`
	CPUUsage         float64            `json:"cpu_usage" bson:"cpu_usage"`
	MemoryUsage      float64            `json:"memory_usage" bson:"memory_usage"`
	DiskUsage        float64            `json:"disk_usage" bson:"disk_usage"`
	LastHeartbeat    time.Time          `json:"last_heartbeat" bson:"last_heartbeat"`
	CreatedAt        time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at" bson:"updated_at"`
}
