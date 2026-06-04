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

// ==================== Xueqiu Cookie Pool Controller ====================

type CookiePoolController struct {
	mu       sync.RWMutex
	sessions map[string]*QRLoginSession
}

type QRLoginSession struct {
	ID        string `json:"id"`
	Status    string `json:"status"` // pending, qr_ready, success, expired, error
	QRImage   string `json:"qr_image,omitempty"`
	Nickname  string `json:"nickname,omitempty"`
	UID       string `json:"uid,omitempty"`
	AvatarUrl string `json:"avatar_url,omitempty"`
	Error     string `json:"error,omitempty"`
	CreatedAt int64  `json:"created_at"`
	cancel    context.CancelFunc
}

func NewCookiePoolController() *CookiePoolController {
	return &CookiePoolController{
		sessions: make(map[string]*QRLoginSession),
	}
}

// GetList returns all Xueqiu cookies in the pool.
func (ctrl *CookiePoolController) GetList(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("xueqiu_cookies")

	cursor, err := col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch cookies")
		return
	}
	defer cursor.Close(ctx)

	var cookies []models.XueqiuCookie
	if err := cursor.All(ctx, &cookies); err != nil {
		utils.RespondInternalError(c, "failed to decode cookies")
		return
	}
	if cookies == nil {
		cookies = []models.XueqiuCookie{}
	}

	// Return cookie field as masked (since json:"-" hides it completely)
	// Compute effective_status: if stored status is "active" but cookie is
	// still in cooldown, report "cooldown" so the UI reflects reality.
	now := time.Now()
	type cookieResponse struct {
		models.XueqiuCookie
		HasCookie       bool   `json:"has_cookie"`
		EffectiveStatus string `json:"effective_status"`
		CooldownLeft    int    `json:"cooldown_left"` // seconds remaining, 0 if not in cooldown
	}
	var resp []cookieResponse
	for _, ck := range cookies {
		effective := ck.Status
		cooldownLeft := 0
		if ck.Status == models.XueqiuCookieStatusActive && ck.CooldownTill.After(now) {
			effective = models.XueqiuCookieStatusCooldown
			cooldownLeft = int(ck.CooldownTill.Sub(now).Seconds())
		}
		resp = append(resp, cookieResponse{
			XueqiuCookie:    ck,
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

// Delete removes a cookie from the pool.
func (ctrl *CookiePoolController) Delete(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid cookie id")
		return
	}

	col := utils.GetCollection("xueqiu_cookies")
	result, err := col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		utils.RespondInternalError(c, "failed to delete cookie")
		return
	}
	if result.DeletedCount == 0 {
		utils.RespondNotFound(c, "cookie not found")
		return
	}

	utils.WriteSystemLog("info", "cookie_pool", fmt.Sprintf("雪球Cookie已删除: %s", id.Hex()), "")
	utils.RespondSuccess(c, nil)
}

// ToggleStatus enables or disables a cookie.
func (ctrl *CookiePoolController) ToggleStatus(c *gin.Context) {
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

	if body.Status != models.XueqiuCookieStatusActive && body.Status != models.XueqiuCookieStatusDisabled {
		utils.RespondBadRequest(c, "status must be 'active' or 'disabled'")
		return
	}

	col := utils.GetCollection("xueqiu_cookies")
	update := bson.M{"$set": bson.M{"status": body.Status}}
	if body.Status == models.XueqiuCookieStatusActive {
		update["$set"].(bson.M)["error_count"] = 0
		update["$set"].(bson.M)["last_error"] = ""
	}
	_, err = col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		utils.RespondInternalError(c, "failed to update cookie")
		return
	}

	utils.RespondSuccess(c, nil)
}

// CheckHealth validates that a cookie is still working.
func (ctrl *CookiePoolController) CheckHealth(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid cookie id")
		return
	}

	col := utils.GetCollection("xueqiu_cookies")
	var ck models.XueqiuCookie
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&ck); err != nil {
		utils.RespondNotFound(c, "cookie not found")
		return
	}

	// Run health check script
	scriptPath := findScript("xueqiu_cookie_check.py")
	valid := false
	errMsg := ""

	if scriptPath != "" {
		checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(checkCtx, "python3", scriptPath, ck.Cookie)
		output, err := cmd.Output()
		if err == nil {
			var result map[string]interface{}
			if json.Unmarshal(output, &result) == nil {
				valid, _ = result["valid"].(bool)
				if msg, ok := result["error"].(string); ok {
					errMsg = msg
				}
			}
		}
	} else {
		// Inline check using the cookie to call a simple API
		valid = checkCookieInline(ck.Cookie)
	}

	now := time.Now()
	if valid {
		col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
			"$set": bson.M{
				"status":        models.XueqiuCookieStatusActive,
				"last_check_at": now,
				"error_count":   0,
				"last_error":    "",
			},
		})
		utils.RespondSuccess(c, gin.H{"valid": true})
	} else {
		col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
			"$set": bson.M{
				"status":        models.XueqiuCookieStatusExpired,
				"last_check_at": now,
				"last_error":    errMsg,
			},
			"$inc": bson.M{"error_count": 1},
		})
		utils.RespondSuccess(c, gin.H{"valid": false, "error": errMsg})
	}
}

// checkCookieInline does a quick HTTP check to verify a Xueqiu cookie is valid.
func checkCookieInline(cookie string) bool {
	// Try calling the v4 timeline API page 2 — only works with valid login cookie
	scriptPath := findScript("xueqiu_user_info.py")
	if scriptPath == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Use a known user ID to test pagination
	cmd := exec.CommandContext(ctx, "python3", "-c", fmt.Sprintf(`
import json, urllib.request
cookie = %q
req = urllib.request.Request(
    "https://xueqiu.com/v4/statuses/user_timeline.json?user_id=1247347556&page=2&page_size=5",
    headers={"User-Agent": "Mozilla/5.0", "Cookie": cookie},
)
try:
    resp = urllib.request.urlopen(req, timeout=10)
    data = json.loads(resp.read())
    print(json.dumps({"valid": not data.get("error_code")}))
except Exception as e:
    print(json.dumps({"valid": False, "error": str(e)}))
`, cookie))
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

// QRStart initiates a QR login session.
func (ctrl *CookiePoolController) QRStart(c *gin.Context) {
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

// QRStatus polls the QR login session status.
func (ctrl *CookiePoolController) QRStatus(c *gin.Context) {
	sessionID := c.Param("id")

	ctrl.mu.RLock()
	session, ok := ctrl.sessions[sessionID]
	ctrl.mu.RUnlock()

	if !ok {
		utils.RespondNotFound(c, "session not found")
		return
	}

	utils.RespondSuccess(c, session)
}

// QRCancel cancels a QR login session.
func (ctrl *CookiePoolController) QRCancel(c *gin.Context) {
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

// runQRLogin runs the Playwright QR login script as a subprocess.
func (ctrl *CookiePoolController) runQRLogin(ctx context.Context, session *QRLoginSession) {
	defer func() {
		// Clean up session after 5 minutes
		time.AfterFunc(5*time.Minute, func() {
			ctrl.mu.Lock()
			delete(ctrl.sessions, session.ID)
			ctrl.mu.Unlock()
		})
	}()

	scriptPath := findScript("xueqiu_qr_login.py")
	if scriptPath == "" {
		ctrl.updateSession(session, func(s *QRLoginSession) {
			s.Status = "error"
			s.Error = "QR login script not found"
		})
		return
	}

	cmd := exec.CommandContext(ctx, "python3", scriptPath, "120")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ctrl.updateSession(session, func(s *QRLoginSession) {
			s.Status = "error"
			s.Error = "Failed to start script"
		})
		return
	}

	if err := cmd.Start(); err != nil {
		ctrl.updateSession(session, func(s *QRLoginSession) {
			s.Status = "error"
			s.Error = "Failed to start script: " + err.Error()
		})
		return
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer for base64 images
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
			ctrl.updateSession(session, func(s *QRLoginSession) {
				s.Status = "qr_ready"
				s.QRImage = image
			})

		case "success":
			cookies, _ := msg["cookies"].(string)
			nickname, _ := msg["nickname"].(string)
			uid, _ := msg["uid"].(string)
			avatar, _ := msg["avatar"].(string)

			// Store cookie in MongoDB
			ctrl.saveCookie(cookies, nickname, uid, avatar)

			ctrl.updateSession(session, func(s *QRLoginSession) {
				s.Status = "success"
				s.Nickname = nickname
				s.UID = uid
				s.AvatarUrl = avatar
				s.QRImage = "" // clear image data
			})

		case "expired":
			ctrl.updateSession(session, func(s *QRLoginSession) {
				s.Status = "expired"
				s.QRImage = "" // clear image data
			})

		case "error":
			errMsg, _ := msg["message"].(string)
			ctrl.updateSession(session, func(s *QRLoginSession) {
				s.Status = "error"
				s.Error = errMsg
				s.QRImage = ""
			})
		}
	}

	cmd.Wait()
}

func (ctrl *CookiePoolController) updateSession(session *QRLoginSession, fn func(*QRLoginSession)) {
	ctrl.mu.Lock()
	fn(session)
	ctrl.mu.Unlock()
}

func (ctrl *CookiePoolController) saveCookie(cookie, nickname, uid, avatar string) {
	if cookie == "" {
		return
	}

	ctx := context.Background()
	col := utils.GetCollection("xueqiu_cookies")
	now := time.Now()

	doc := models.XueqiuCookie{
		Nickname:     nickname,
		XueqiuUID:    uid,
		AvatarUrl:    avatar,
		Cookie:       cookie,
		Status:       models.XueqiuCookieStatusActive,
		CreatedAt:    now,
		LastUsedAt:   time.Time{},
		LastCheckAt:  now,
		CooldownTill: time.Time{},
		DailyResetAt: now.Truncate(24 * time.Hour),
	}

	// Upsert by xueqiu_uid to handle re-login of existing accounts
	if uid != "" {
		filter := bson.M{"xueqiu_uid": uid}
		update := bson.M{
			"$set": bson.M{
				"nickname":      nickname,
				"avatar_url":    avatar,
				"cookie":        cookie,
				"status":        models.XueqiuCookieStatusActive,
				"last_check_at": now,
				"error_count":   0,
				"last_error":    "",
			},
			"$setOnInsert": bson.M{
				"xueqiu_uid":    uid,
				"created_at":    now,
				"daily_reset_at": now.Truncate(24 * time.Hour),
			},
		}
		_, err := col.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
		if err != nil {
			utils.Logger.Warnf("Failed to save Xueqiu cookie: %v", err)
			return
		}
	} else {
		_, err := col.InsertOne(ctx, doc)
		if err != nil {
			utils.Logger.Warnf("Failed to insert Xueqiu cookie: %v", err)
			return
		}
	}

	utils.WriteSystemLog("info", "cookie_pool",
		fmt.Sprintf("雪球账号已添加: %s (%s)", nickname, uid), "")
}

// ==================== Cookie Selection (used by spider deployer) ====================

// SelectCookie picks the best available cookie from the pool using
// least-recently-used strategy with anti-bot protections.
func SelectXueqiuCookie(ctx context.Context) (*models.XueqiuCookie, error) {
	col := utils.GetCollection("xueqiu_cookies")
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	// Reset daily counters for new day
	_, _ = col.UpdateMany(ctx, bson.M{
		"daily_reset_at": bson.M{"$lt": today},
	}, bson.M{
		"$set": bson.M{"daily_tasks": 0, "daily_reset_at": today},
	})

	// Find active cookie that:
	// 1. status = "active"
	// 2. cooldown has expired
	// 3. daily limit not reached
	// Sort by last_used_at ASC (least recently used first)
	filter := bson.M{
		"status":        models.XueqiuCookieStatusActive,
		"cooldown_till": bson.M{"$not": bson.M{"$gt": now}},
		"daily_tasks":   bson.M{"$lt": int64(models.XueqiuCookieMaxDailyTasks)},
	}

	var cookie models.XueqiuCookie
	err := col.FindOne(ctx, filter,
		options.FindOne().SetSort(bson.D{{Key: "last_used_at", Value: 1}}),
	).Decode(&cookie)
	if err != nil {
		return nil, fmt.Errorf("no available cookie in pool: %w", err)
	}

	// Reserve this cookie: update usage stats and set cooldown
	cooldownUntil := now.Add(time.Duration(models.XueqiuCookieCooldownMin) * time.Minute)
	_, _ = col.UpdateOne(ctx, bson.M{"_id": cookie.Id}, bson.M{
		"$set": bson.M{
			"last_used_at":  now,
			"cooldown_till": cooldownUntil,
		},
		"$inc": bson.M{
			"total_tasks": 1,
			"daily_tasks": 1,
		},
	})

	return &cookie, nil
}

// MarkCookieError records an error for a cookie with progressive cooldown.
// 1st error → 5min cooldown, 2nd → 10min, 3rd+ → mark expired.
func MarkCookieError(ctx context.Context, cookieID primitive.ObjectID, errMsg string) {
	col := utils.GetCollection("xueqiu_cookies")
	now := time.Now()

	var ck models.XueqiuCookie
	if err := col.FindOne(ctx, bson.M{"_id": cookieID}).Decode(&ck); err != nil {
		return
	}

	newErrCount := ck.ErrorCount + 1
	setFields := bson.M{
		"last_error": errMsg,
		"error_count": newErrCount,
	}

	if newErrCount >= models.XueqiuCookieMaxConsecError {
		// 3+ consecutive errors → expire the cookie
		setFields["status"] = models.XueqiuCookieStatusExpired
		utils.Logger.Warnf("Cookie %s expired after %d consecutive errors: %s", cookieID.Hex(), newErrCount, errMsg)
	} else {
		// Progressive cooldown based on error count
		var cooldownMin int
		switch newErrCount {
		case 1:
			cooldownMin = models.XueqiuCookieCooldownErr1
		default:
			cooldownMin = models.XueqiuCookieCooldownErr2
		}
		setFields["cooldown_till"] = now.Add(time.Duration(cooldownMin) * time.Minute)
		utils.Logger.Infof("Cookie %s error #%d, cooldown %dmin: %s", cookieID.Hex(), newErrCount, cooldownMin, errMsg)
	}

	_, _ = col.UpdateOne(ctx, bson.M{"_id": cookieID}, bson.M{"$set": setFields})
}

// ClearCookieError resets error count after a successful task.
func ClearCookieError(ctx context.Context, cookieID primitive.ObjectID) {
	col := utils.GetCollection("xueqiu_cookies")
	_, _ = col.UpdateOne(ctx, bson.M{"_id": cookieID}, bson.M{
		"$set": bson.M{"error_count": 0, "last_error": ""},
	})
}
