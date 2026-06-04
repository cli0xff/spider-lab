package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Setting struct {
	Id        primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Key       string             `json:"key" bson:"key"`
	Value     interface{}        `json:"value" bson:"value"`
	Category  string             `json:"category" bson:"category"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
}

type NotificationChannel struct {
	Id        primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Type      string             `json:"type" bson:"type"`
	Name      string             `json:"name" bson:"name"`
	Enabled   bool               `json:"enabled" bson:"enabled"`
	Config    map[string]string  `json:"config" bson:"config"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
}
