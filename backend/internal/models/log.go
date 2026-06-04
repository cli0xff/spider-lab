package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelDebug = "debug"
)

type TaskLog struct {
	Id        primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	TaskId    primitive.ObjectID `json:"task_id" bson:"task_id"`
	NodeKey   string             `json:"node_key" bson:"node_key"`
	Content   string             `json:"content" bson:"content"`
	Level     string             `json:"level" bson:"level"`
	Timestamp time.Time          `json:"timestamp" bson:"timestamp"`
}

type SystemLog struct {
	Id        primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Level     string             `json:"level" bson:"level"`
	Module    string             `json:"module" bson:"module"`
	Message   string             `json:"message" bson:"message"`
	Detail    string             `json:"detail" bson:"detail"`
	Timestamp time.Time          `json:"timestamp" bson:"timestamp"`
}

type ResultItem struct {
	Id          primitive.ObjectID     `json:"_id" bson:"_id,omitempty"`
	TaskId      primitive.ObjectID     `json:"task_id" bson:"task_id"`
	SpiderId    primitive.ObjectID     `json:"spider_id" bson:"spider_id"`
	Item        map[string]interface{} `json:"item" bson:"item"`
	ItemHash    string                 `json:"item_hash" bson:"item_hash"`
	PublishedAt time.Time              `json:"published_at" bson:"published_at"`
	Timestamp   time.Time              `json:"timestamp" bson:"timestamp"`
}
