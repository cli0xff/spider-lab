package models

import (
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	SpiderTypeCustom   = "custom"
	SpiderTypeTemplate = "template"

	SpiderModeAllNodes      = "all_nodes"
	SpiderModeSelectedNodes = "selected_nodes"
	SpiderModeRandom        = "random"
)

type Spider struct {
	Id          primitive.ObjectID     `json:"_id" bson:"_id,omitempty"`
	Name        string                 `json:"name" bson:"name" binding:"required"`
	Type        string                 `json:"type" bson:"type" binding:"required"`
	TemplateId  string                 `json:"template_id" bson:"template_id"`
	Config      map[string]interface{} `json:"config" bson:"config"`
	Description string                 `json:"description" bson:"description"`
	Cmd         string                 `json:"cmd" bson:"cmd"`
	Param       string                 `json:"param" bson:"param"`
	Mode        string                 `json:"mode" bson:"mode" binding:"required"`
	NodeIds     []primitive.ObjectID   `json:"node_ids" bson:"node_ids"`
	ColName     string                 `json:"col_name" bson:"col_name"`
	Priority    int                    `json:"priority" bson:"priority"`
	Project     string                 `json:"project" bson:"project"`
	Tags        []string               `json:"tags" bson:"tags"`
	Stat        *SpiderStat            `json:"stat,omitempty" bson:"-"`
	CreatedAt   time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" bson:"updated_at"`
	CreatedBy   primitive.ObjectID     `json:"created_by" bson:"created_by"`
}

var validSpiderTypes = map[string]bool{
	SpiderTypeCustom:   true,
	SpiderTypeTemplate: true,
}

var validSpiderModes = map[string]bool{
	SpiderModeAllNodes:      true,
	SpiderModeSelectedNodes: true,
	SpiderModeRandom:        true,
}

func (s *Spider) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("spider name is required")
	}
	if !validSpiderTypes[s.Type] {
		return fmt.Errorf("invalid spider type: %s", s.Type)
	}
	if s.Type == SpiderTypeTemplate && s.TemplateId == "" {
		return fmt.Errorf("template_id is required for template spiders")
	}
	if s.Type == SpiderTypeCustom && strings.TrimSpace(s.Cmd) == "" {
		return fmt.Errorf("cmd is required for custom spiders")
	}
	if !validSpiderModes[s.Mode] {
		return fmt.Errorf("invalid spider mode: %s", s.Mode)
	}
	if s.Priority < 1 || s.Priority > 10 {
		s.Priority = 5
	}
	if s.Mode == SpiderModeSelectedNodes && len(s.NodeIds) == 0 {
		return fmt.Errorf("node_ids required when mode is selected_nodes")
	}
	return nil
}

type SpiderStat struct {
	TotalTasks      int64     `json:"total_tasks" bson:"total_tasks"`
	TotalResults    int64     `json:"total_results" bson:"total_results"`
	AverageDuration float64   `json:"average_duration" bson:"average_duration"`
	SuccessRate     float64   `json:"success_rate" bson:"success_rate"`
	LastRunAt       time.Time `json:"last_run_at" bson:"last_run_at"`
}
