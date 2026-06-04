package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Schedule struct {
	Id          primitive.ObjectID   `json:"_id" bson:"_id,omitempty"`
	SpiderId    primitive.ObjectID   `json:"spider_id" bson:"spider_id"`
	Cron        string               `json:"cron" bson:"cron"`
	Cmd         string               `json:"cmd" bson:"cmd"`
	Param       string               `json:"param" bson:"param"`
	Mode        string               `json:"mode" bson:"mode"`
	NodeIds     []primitive.ObjectID  `json:"node_ids" bson:"node_ids"`
	Enabled     bool                 `json:"enabled" bson:"enabled"`
	EntryId     int                  `json:"entry_id" bson:"entry_id"`
	Name        string               `json:"name" bson:"name"`
	Description string               `json:"description" bson:"description"`
	CreatedAt   time.Time            `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at" bson:"updated_at"`
	CreatedBy   primitive.ObjectID   `json:"created_by" bson:"created_by"`
}
