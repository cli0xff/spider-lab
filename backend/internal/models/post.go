package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Post represents a crawled item stored in the centralized posts collection.
// The {platform, platform_id} compound index enforces platform-level dedup.
type Post struct {
	Id          primitive.ObjectID     `json:"_id" bson:"_id,omitempty"`
	Platform    string                 `json:"platform" bson:"platform"`
	PlatformId  string                 `json:"platform_id" bson:"platform_id"`
	Type        string                 `json:"type" bson:"type"`
	SpiderIds   []primitive.ObjectID   `json:"spider_ids" bson:"spider_ids"`
	TaskId      primitive.ObjectID     `json:"task_id" bson:"task_id"`
	Item        map[string]interface{} `json:"item" bson:"item"`
	PublishedAt time.Time              `json:"published_at" bson:"published_at"`
	CreatedAt   time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" bson:"updated_at"`
}
