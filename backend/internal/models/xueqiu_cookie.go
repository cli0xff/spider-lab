package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	XueqiuCookieStatusActive   = "active"
	XueqiuCookieStatusCooldown = "cooldown"
	XueqiuCookieStatusExpired  = "expired"
	XueqiuCookieStatusDisabled = "disabled"
)

// Anti-bot protection constants
const (
	XueqiuCookieMaxDailyTasks  = 20 // max spider tasks per cookie per day
	XueqiuCookieCooldownMin    = 1  // default cooldown (minutes) after successful use
	XueqiuCookieCooldownErr1   = 5  // cooldown (minutes) after 1st consecutive error
	XueqiuCookieCooldownErr2   = 10 // cooldown (minutes) after 2nd consecutive error
	XueqiuCookieMaxConsecError = 3  // auto-expire after this many consecutive errors
)

type XueqiuCookie struct {
	Id           primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Nickname     string             `json:"nickname" bson:"nickname"`
	XueqiuUID    string             `json:"xueqiu_uid" bson:"xueqiu_uid"`
	AvatarUrl    string             `json:"avatar_url" bson:"avatar_url"`
	Cookie       string             `json:"-" bson:"cookie"` // hidden from JSON API responses
	Status       string             `json:"status" bson:"status"`
	CreatedAt    time.Time          `json:"created_at" bson:"created_at"`
	LastUsedAt   time.Time          `json:"last_used_at" bson:"last_used_at"`
	LastCheckAt  time.Time          `json:"last_check_at" bson:"last_check_at"`
	CooldownTill time.Time          `json:"cooldown_till" bson:"cooldown_till"`
	TotalTasks   int64              `json:"total_tasks" bson:"total_tasks"`
	DailyTasks   int64              `json:"daily_tasks" bson:"daily_tasks"`
	DailyResetAt time.Time          `json:"daily_reset_at" bson:"daily_reset_at"`
	ErrorCount   int                `json:"error_count" bson:"error_count"`
	LastError    string             `json:"last_error" bson:"last_error"`
	Notes        string             `json:"notes" bson:"notes"`
}
