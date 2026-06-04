package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusFinished  = "finished"
	TaskStatusError     = "error"
	TaskStatusCancelled = "cancelled"
	TaskStatusWaiting   = "waiting"
)

type Task struct {
	Id          primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	SpiderId    primitive.ObjectID `json:"spider_id" bson:"spider_id"`
	NodeId      primitive.ObjectID `json:"node_id" bson:"node_id"`
	CookieId    primitive.ObjectID `json:"cookie_id,omitempty" bson:"cookie_id,omitempty"`
	Status      string             `json:"status" bson:"status"`
	Cmd         string             `json:"cmd" bson:"cmd"`
	Param       string             `json:"param" bson:"param"`
	Error       string             `json:"error" bson:"error"`
	Pid         int                `json:"pid" bson:"pid"`
	Priority    int                `json:"priority" bson:"priority"`
	Type        string             `json:"type" bson:"type"`
	RunTs       time.Time          `json:"run_ts" bson:"run_ts"`
	StartTs     time.Time          `json:"start_ts" bson:"start_ts"`
	EndTs       time.Time          `json:"end_ts" bson:"end_ts"`
	ResultCount int64              `json:"result_count" bson:"result_count"`
	ScheduleId  primitive.ObjectID `json:"schedule_id,omitempty" bson:"schedule_id,omitempty"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" bson:"updated_at"`
	CreatedBy   primitive.ObjectID `json:"created_by" bson:"created_by"`
	Spider      *Spider            `json:"spider,omitempty" bson:"-"`
	Node        *Node              `json:"node,omitempty" bson:"-"`
}

type TaskStat struct {
	TaskId      primitive.ObjectID `json:"task_id" bson:"task_id"`
	ResultCount int64              `json:"result_count" bson:"result_count"`
	ErrorCount  int64              `json:"error_count" bson:"error_count"`
	WarnCount   int64              `json:"warn_count" bson:"warn_count"`
}
