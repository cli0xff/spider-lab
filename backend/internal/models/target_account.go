package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TargetAccount represents a user account on a platform whose posts will be scraped.
type TargetAccount struct {
	Id             primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Platform       string             `json:"platform" bson:"platform"`               // "xueqiu", "weibo", "xhs"
	UserId         string             `json:"user_id" bson:"user_id"`                 // platform-specific user ID
	Nickname       string             `json:"nickname" bson:"nickname"`               // display name
	AvatarUrl      string             `json:"avatar_url" bson:"avatar_url"`           // avatar URL
	Description    string             `json:"description" bson:"description"`         // bio/description
	FollowersCount int                `json:"followers_count" bson:"followers_count"` // follower count at add time
	Verified       bool               `json:"verified" bson:"verified"`               // verified badge
	CreatedAt      time.Time          `json:"created_at" bson:"created_at"`
	Notes          string             `json:"notes" bson:"notes"`
}
