package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/crawlab-team/spider-lab/internal/models"
	"github.com/crawlab-team/spider-lab/internal/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ==================== WeChat Cookie Pool Controller ====================

type WechatCookiePoolController struct {
	mu       sync.RWMutex
	sessions map[string]*QRLoginSession // reuse QRLoginSession struct from cookie_pool.go
}

func NewWechatCookiePoolController() *WechatCookiePoolController {
	return &WechatCookiePoolController{
		sessions: make(map[string]*QRLoginSession),
	}
}

// GetList returns all WeChat cookies in the pool.
func (ctrl *WechatCookiePoolController) GetList(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("wechat_cookies")

	cursor, err := col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch wechat cookies")
		return
	}
	defer cursor.Close(ctx)

	var cookies []models.WechatCookie
	if err := cursor.All(ctx, &cookies); err != nil {
		utils.RespondInternalError(c, "failed to decode wechat cookies")
		return
	}
	if cookies == nil {
		cookies = []models.WechatCookie{}
	}

	now := time.Now()
	type cookieResponse struct {
		models.WechatCookie
		HasCookie       bool   `json:"has_cookie"`
		EffectiveStatus string `json:"effective_status"`
		CooldownLeft    int    `json:"cooldown_left"`
	}
	var resp []cookieResponse
	for _, ck := range cookies {
		effective := ck.Status
		cooldownLeft := 0
		if ck.Status == models.WechatCookieStatusActive && ck.CooldownTill.After(now) {
			effective = models.WechatCookieStatusCooldown
			cooldownLeft = int(ck.CooldownTill.Sub(now).Seconds())
		}
		resp = append(resp, cookieResponse{
			WechatCookie:    ck,
			HasCookie:       ck.Cookie != "",
			EffectiveStatus: effective,
			CooldownLeft:    cooldownLeft,
		})
	}
	if resp == nil {
		resp = []cookieResponse{}
	}

	utils.RespondSuccess(c, resp)
}

// Delete removes a WeChat cookie from the pool.
func (ctrl *WechatCookiePoolController) Delete(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid cookie id")
		return
	}

	col := utils.GetCollection("wechat_cookies")
	result, err := col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		utils.RespondInternalError(c, "failed to delete wechat cookie")
		return
	}
	if result.DeletedCount == 0 {
		utils.RespondNotFound(c, "cookie not found")
		return
	}

	utils.WriteSystemLog("info", "cookie_pool", fmt.Sprintf("微信公众号Cookie已删除: %s", id.Hex()), "")
	utils.RespondSuccess(c, nil)
}

// ToggleStatus enables or disables a WeChat cookie.
func (ctrl *WechatCookiePoolController) ToggleStatus(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid cookie id")
		return
	}

	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondBadRequest(c, "status is required")
		return
	}

	if body.Status != models.WechatCookieStatusActive && body.Status != models.WechatCookieStatusDisabled {
		utils.RespondBadRequest(c, "status must be 'active' or 'disabled'")
		return
	}

	col := utils.GetCollection("wechat_cookies")
	update := bson.M{"$set": bson.M{"status": body.Status}}
	if body.Status == models.WechatCookieStatusActive {
		update["$set"].(bson.M)["error_count"] = 0
		update["$set"].(bson.M)["last_error"] = ""
	}
	_, err = col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		utils.RespondInternalError(c, "failed to update wechat cookie status")
		return
	}

	utils.RespondSuccess(c, nil)
}

// CheckHealth validates a WeChat cookie by attempting to load a protected MP page.
func (ctrl *WechatCookiePoolController) CheckHealth(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid cookie id")
		return
	}

	col := utils.GetCollection("wechat_cookies")
	var ck models.WechatCookie
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&ck); err != nil {
		utils.RespondNotFound(c, "cookie not found")
		return
	}

	valid := checkWechatCookieInline(ck.Cookie, ck.Token)
	now := time.Now()
	if valid {
		col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
			"$set": bson.M{
				"status":        models.WechatCookieStatusActive,
				"last_check_at": now,
				"error_count":   0,
				"last_error":    "",
			},
		})
		utils.RespondSuccess(c, gin.H{"valid": true})
	} else {
		col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
			"$set": bson.M{
				"status":        models.WechatCookieStatusExpired,
				"last_check_at": now,
				"last_error":    "cookie health check failed",
			},
			"$inc": bson.M{"error_count": 1},
		})
		utils.RespondSuccess(c, gin.H{"valid": false, "error": "cookie health check failed"})
	}
}

// checkWechatCookieInline does a lightweight HTTP check for a WeChat MP cookie.
func checkWechatCookieInline(cookie, token string) bool {
	if cookie == "" {
		return false
	}
	checkCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	urlPath := "https://mp.weixin.qq.com/cgi-bin/home"
	if token != "" {
		urlPath = fmt.Sprintf("https://mp.weixin.qq.com/cgi-bin/appmsg?action=list_ex&begin=0&count=1&type=9&token=%s&lang=zh_CN", token)
	}

	cmd := exec.CommandContext(checkCtx, "python3", "-c", fmt.Sprintf(`
import json, urllib.request
cookie = %q
url = %q
req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0", "Cookie": cookie})
try:
    resp = urllib.request.urlopen(req, timeout=10)
    body = resp.read().decode()
    # Not redirected to loginpage and no error in response = valid
    valid = "loginpage" not in resp.url and "请重新登录" not in body
    print(json.dumps({"valid": valid}))
except Exception as e:
    print(json.dumps({"valid": False, "error": str(e)}))
`, cookie, urlPath))
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	var result map[string]interface{}
	if json.Unmarshal(output, &result) == nil {
		v, _ := result["valid"].(bool)
		return v
	}
	return false
}

// ==================== QR Login Session Management ====================

// QRStart initiates a WeChat MP QR login session.
func (ctrl *WechatCookiePoolController) QRStart(c *gin.Context) {
	sessionID := primitive.NewObjectID().Hex()

	session := &QRLoginSession{
		ID:        sessionID,
		Status:    "pending",
		CreatedAt: time.Now().Unix(),
	}

	bgCtx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	session.cancel = cancel

	ctrl.mu.Lock()
	ctrl.sessions[sessionID] = session
	ctrl.mu.Unlock()

	go ctrl.runQRLogin(bgCtx, session)

	utils.RespondSuccess(c, gin.H{"session_id": sessionID})
}

// QRStatus polls the WeChat QR login session status.
func (ctrl *WechatCookiePoolController) QRStatus(c *gin.Context) {
	sessionID := c.Param("id")

	ctrl.mu.RLock()
	session, ok := ctrl.sessions[sessionID]
	var snapshot QRLoginSession
	if ok {
		snapshot = *session
	}
	ctrl.mu.RUnlock()

	if !ok {
		utils.RespondNotFound(c, "session not found")
		return
	}

	utils.RespondSuccess(c, &snapshot)
}

// QRCancel cancels a WeChat QR login session.
func (ctrl *WechatCookiePoolController) QRCancel(c *gin.Context) {
	sessionID := c.Param("id")

	ctrl.mu.Lock()
	session, ok := ctrl.sessions[sessionID]
	if ok {
		if session.cancel != nil {
			session.cancel()
		}
		delete(ctrl.sessions, sessionID)
	}
	ctrl.mu.Unlock()

	utils.RespondSuccess(c, nil)
}

// runQRLogin runs the wechat_qr_login.py script as a subprocess.
func (ctrl *WechatCookiePoolController) runQRLogin(ctx context.Context, session *QRLoginSession) {
	defer func() {
		time.AfterFunc(5*time.Minute, func() {
			ctrl.mu.Lock()
			delete(ctrl.sessions, session.ID)
			ctrl.mu.Unlock()
		})
	}()

	scriptPath := findScript("wechat_qr_login.py")
	if scriptPath == "" {
		ctrl.updateWechatSession(session, func(s *QRLoginSession) {
			s.Status = "error"
			s.Error = "WeChat QR login script not found"
		})
		return
	}

	cmd := exec.CommandContext(ctx, "python3", scriptPath, "120")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ctrl.updateWechatSession(session, func(s *QRLoginSession) {
			s.Status = "error"
			s.Error = "Failed to start QR login script"
		})
		return
	}

	if err := cmd.Start(); err != nil {
		ctrl.updateWechatSession(session, func(s *QRLoginSession) {
			s.Status = "error"
			s.Error = "Failed to start QR login script: " + err.Error()
		})
		return
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		var msg map[string]interface{}
		if json.Unmarshal([]byte(line), &msg) != nil {
			continue
		}

		msgType, _ := msg["type"].(string)
		switch msgType {
		case "qr":
			image, _ := msg["image"].(string)
			ctrl.updateWechatSession(session, func(s *QRLoginSession) {
				s.Status = "qr_ready"
				s.QRImage = image
			})

		case "success":
			cookies, _ := msg["cookies"].(string)
			token, _ := msg["token"].(string)
			nickname, _ := msg["nickname"].(string)

			ctrl.saveWechatCookie(cookies, token, nickname)

			ctrl.updateWechatSession(session, func(s *QRLoginSession) {
				s.Status = "success"
				s.Nickname = nickname
				s.QRImage = ""
			})

		case "expired":
			ctrl.updateWechatSession(session, func(s *QRLoginSession) {
				s.Status = "expired"
				s.QRImage = ""
			})

		case "error":
			errMsg, _ := msg["message"].(string)
			ctrl.updateWechatSession(session, func(s *QRLoginSession) {
				s.Status = "error"
				s.Error = errMsg
				s.QRImage = ""
			})
		}
	}

	cmd.Wait()
}

func (ctrl *WechatCookiePoolController) updateWechatSession(session *QRLoginSession, fn func(*QRLoginSession)) {
	ctrl.mu.Lock()
	fn(session)
	ctrl.mu.Unlock()
}

func (ctrl *WechatCookiePoolController) saveWechatCookie(cookie, token, nickname string) {
	if cookie == "" {
		return
	}

	ctx := context.Background()
	col := utils.GetCollection("wechat_cookies")
	now := time.Now()

	update := bson.M{
		"$set": bson.M{
			"nickname":      nickname,
			"cookie":        cookie,
			"token":         token,
			"status":        models.WechatCookieStatusActive,
			"last_check_at": now,
			"error_count":   0,
			"last_error":    "",
		},
		"$setOnInsert": bson.M{
			"created_at":     now,
			"daily_reset_at": now.Truncate(24 * time.Hour),
			"daily_tasks":    int64(0),
			"total_tasks":    int64(0),
			"cooldown_till":  time.Time{},
		},
	}

	// Upsert by nickname to handle re-login of same account
	filter := bson.M{"nickname": nickname}
	if nickname == "" {
		// No nickname: always insert new
		doc := models.WechatCookie{
			Nickname:     nickname,
			Cookie:       cookie,
			Token:        token,
			Status:       models.WechatCookieStatusActive,
			CreatedAt:    now,
			LastCheckAt:  now,
			DailyResetAt: now.Truncate(24 * time.Hour),
		}
		_, err := col.InsertOne(ctx, doc)
		if err != nil {
			utils.Logger.Warnf("Failed to insert WeChat cookie: %v", err)
		}
		return
	}

	_, err := col.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		utils.Logger.Warnf("Failed to save WeChat cookie: %v", err)
		return
	}

	utils.WriteSystemLog("info", "cookie_pool",
		fmt.Sprintf("微信公众号账号已添加: %s", nickname), "")
}

// AddManual allows manual addition of WeChat cookies (for advanced users).
func (ctrl *WechatCookiePoolController) AddManual(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("wechat_cookies")

	var body struct {
		Nickname  string `json:"nickname" binding:"required"`
		Cookie    string `json:"cookie" binding:"required"`
		Token     string `json:"token"`
		MpAccount string `json:"mp_account"` // Optional: WeChat MP account name
		Notes     string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondBadRequest(c, "nickname and cookie are required")
		return
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"nickname":      body.Nickname,
			"cookie":        body.Cookie,
			"token":         body.Token,
			"mp_account":    body.MpAccount,
			"notes":         body.Notes,
			"status":        models.WechatCookieStatusActive,
			"last_check_at": now,
			"error_count":   0,
			"last_error":    "",
		},
		"$setOnInsert": bson.M{
			"created_at":     now,
			"daily_reset_at": now.Truncate(24 * time.Hour),
			"daily_tasks":    int64(0),
			"total_tasks":    int64(0),
			"cooldown_till":  time.Time{},
		},
	}

	// Upsert by nickname to handle re-login of same account
	filter := bson.M{"nickname": body.Nickname}
	result, err := col.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		utils.RespondInternalError(c, "failed to save WeChat cookie")
		return
	}

	action := "更新"
	if result.UpsertedCount > 0 {
		action = "添加"
	}
	utils.WriteSystemLog("info", "cookie_pool",
		fmt.Sprintf("微信公众号Cookie已%s: %s", action, body.Nickname), "")
	utils.RespondSuccess(c, nil)
}

// ==================== Cookie Selection (used by spider deployer) ====================

// SelectWechatCookie atomically picks and reserves the best available WeChat
// cookie from the pool using a least-recently-used strategy.
func SelectWechatCookie(ctx context.Context) (*models.WechatCookie, error) {
	col := utils.GetCollection("wechat_cookies")
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	_, _ = col.UpdateMany(ctx, bson.M{
		"daily_reset_at": bson.M{"$lt": today},
	}, bson.M{
		"$set": bson.M{"daily_tasks": 0, "daily_reset_at": today},
	})

	filter := bson.M{
		"status":        models.WechatCookieStatusActive,
		"cooldown_till": bson.M{"$not": bson.M{"$gt": now}},
		"daily_tasks":   bson.M{"$lt": int64(models.WechatCookieMaxDailyTasks)},
	}
	cooldownUntil := now.Add(time.Duration(models.WechatCookieCooldownMin) * time.Minute)

	var cookie models.WechatCookie
	err := col.FindOneAndUpdate(
		ctx,
		filter,
		bson.M{
			"$set": bson.M{
				"last_used_at":  now,
				"cooldown_till": cooldownUntil,
			},
			"$inc": bson.M{
				"total_tasks": 1,
				"daily_tasks": 1,
			},
		},
		options.FindOneAndUpdate().
			SetSort(bson.D{{Key: "last_used_at", Value: 1}}).
			SetReturnDocument(options.Before),
	).Decode(&cookie)
	if err != nil {
		return nil, fmt.Errorf("no available WeChat cookie in pool: %w", err)
	}

	return &cookie, nil
}

// MarkWechatCookieError records an error for a WeChat cookie with progressive cooldown.
func MarkWechatCookieError(ctx context.Context, cookieID primitive.ObjectID, errMsg string) {
	col := utils.GetCollection("wechat_cookies")
	now := time.Now()

	var ck models.WechatCookie
	err := col.FindOneAndUpdate(
		ctx,
		bson.M{"_id": cookieID},
		bson.M{
			"$inc": bson.M{"error_count": 1},
			"$set": bson.M{"last_error": errMsg},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&ck)
	if err != nil {
		return
	}

	setFields := bson.M{}
	if ck.ErrorCount >= models.WechatCookieMaxConsecError {
		setFields["status"] = models.WechatCookieStatusExpired
		utils.Logger.Warnf("WeChat cookie %s expired after %d consecutive errors: %s", cookieID.Hex(), ck.ErrorCount, errMsg)
	} else {
		var cooldownMin int
		if ck.ErrorCount == 1 {
			cooldownMin = models.WechatCookieCooldownErr1
		} else {
			cooldownMin = models.WechatCookieCooldownErr2
		}
		setFields["cooldown_till"] = now.Add(time.Duration(cooldownMin) * time.Minute)
	}

	if len(setFields) > 0 {
		_, _ = col.UpdateOne(ctx, bson.M{"_id": cookieID}, bson.M{"$set": setFields})
	}
}

// ClearWechatCookieError resets error count after a successful WeChat task.
func ClearWechatCookieError(ctx context.Context, cookieID primitive.ObjectID) {
	col := utils.GetCollection("wechat_cookies")
	_, _ = col.UpdateOne(ctx, bson.M{"_id": cookieID}, bson.M{
		"$set": bson.M{"error_count": 0, "last_error": ""},
	})
}

// HasWechatCookieAvailable checks if at least one WeChat cookie is available.
func HasWechatCookieAvailable(ctx context.Context) bool {
	col := utils.GetCollection("wechat_cookies")
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	_, _ = col.UpdateMany(ctx, bson.M{
		"daily_reset_at": bson.M{"$lt": today},
	}, bson.M{
		"$set": bson.M{"daily_tasks": 0, "daily_reset_at": today},
	})

	filter := bson.M{
		"status":        models.WechatCookieStatusActive,
		"cooldown_till": bson.M{"$not": bson.M{"$gt": now}},
		"daily_tasks":   bson.M{"$lt": int64(models.WechatCookieMaxDailyTasks)},
	}
	count, err := col.CountDocuments(ctx, filter)
	return err == nil && count > 0
}
