package api

import (
	"context"
	"fmt"
	"time"

	"github.com/crawlab-team/spider-lab/internal/models"
	"github.com/crawlab-team/spider-lab/internal/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TargetAccountController struct{}

func NewTargetAccountController() *TargetAccountController {
	return &TargetAccountController{}
}

// GetList returns target accounts, optionally filtered by platform.
func (ctrl *TargetAccountController) GetList(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("target_accounts")

	filter := bson.M{}
	if platform := c.Query("platform"); platform != "" {
		filter["platform"] = platform
	}

	cursor, err := col.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch target accounts")
		return
	}
	defer cursor.Close(ctx)

	var accounts []models.TargetAccount
	if err := cursor.All(ctx, &accounts); err != nil {
		utils.RespondInternalError(c, "failed to decode target accounts")
		return
	}
	if accounts == nil {
		accounts = []models.TargetAccount{}
	}

	utils.RespondSuccess(c, accounts)
}

// Create adds a new target account.
func (ctrl *TargetAccountController) Create(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("target_accounts")

	var body struct {
		Platform       string `json:"platform" binding:"required"`
		UserId         string `json:"user_id" binding:"required"`
		Nickname       string `json:"nickname"`
		AvatarUrl      string `json:"avatar_url"`
		Description    string `json:"description"`
		FollowersCount int    `json:"followers_count"`
		Verified       bool   `json:"verified"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondBadRequest(c, "platform and user_id are required")
		return
	}

	validPlatforms := map[string]bool{"xueqiu": true, "weibo": true, "xhs": true, "wechat": true}
	if !validPlatforms[body.Platform] {
		utils.RespondBadRequest(c, "platform must be xueqiu, weibo, xhs, or wechat")
		return
	}

	// Upsert: if already exists for this platform+user_id, update info
	now := time.Now()
	filter := bson.M{"platform": body.Platform, "user_id": body.UserId}
	update := bson.M{
		"$set": bson.M{
			"nickname":        body.Nickname,
			"avatar_url":      body.AvatarUrl,
			"description":     body.Description,
			"followers_count": body.FollowersCount,
			"verified":        body.Verified,
		},
		"$setOnInsert": bson.M{
			"platform":   body.Platform,
			"user_id":    body.UserId,
			"created_at": now,
			"notes":      "",
		},
	}

	result, err := col.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		utils.RespondInternalError(c, "failed to save target account")
		return
	}

	action := "更新"
	if result.UpsertedCount > 0 {
		action = "添加"
	}
	utils.WriteSystemLog("info", "target_accounts",
		fmt.Sprintf("目标账号已%s: %s (%s/%s)", action, body.Nickname, body.Platform, body.UserId), "")
	utils.RespondSuccess(c, nil)
}

// Delete removes a target account.
func (ctrl *TargetAccountController) Delete(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid account id")
		return
	}

	col := utils.GetCollection("target_accounts")
	result, err := col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		utils.RespondInternalError(c, "failed to delete target account")
		return
	}
	if result.DeletedCount == 0 {
		utils.RespondNotFound(c, "target account not found")
		return
	}

	utils.WriteSystemLog("info", "target_accounts",
		fmt.Sprintf("目标账号已删除: %s", id.Hex()), "")
	utils.RespondSuccess(c, nil)
}

// GetByPlatform returns target accounts for a specific platform (for dropdown population).
func (ctrl *TargetAccountController) GetByPlatform(c *gin.Context) {
	ctx := context.Background()
	platform := c.Query("platform")
	if platform == "" {
		utils.RespondBadRequest(c, "platform is required")
		return
	}

	col := utils.GetCollection("target_accounts")
	cursor, err := col.Find(ctx, bson.M{"platform": platform},
		options.Find().SetSort(bson.D{{Key: "nickname", Value: 1}}))
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch target accounts")
		return
	}
	defer cursor.Close(ctx)

	var accounts []models.TargetAccount
	if err := cursor.All(ctx, &accounts); err != nil {
		utils.RespondInternalError(c, "failed to decode target accounts")
		return
	}
	if accounts == nil {
		accounts = []models.TargetAccount{}
	}

	// Return simplified format for dropdown: id as value, nickname as label
	type dropdownOption struct {
		Value    string `json:"value"`
		Label    string `json:"label"`
		UserId   string `json:"user_id"`
		Nickname string `json:"nickname"`
	}
	options := make([]dropdownOption, 0, len(accounts))
	for _, acc := range accounts {
		options = append(options, dropdownOption{
			Value:    acc.Id.Hex(),
			Label:    acc.Nickname,
			UserId:   acc.UserId,
			Nickname: acc.Nickname,
		})
	}

	utils.RespondSuccess(c, options)
}
