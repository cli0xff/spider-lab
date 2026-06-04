package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/crawlab-team/spider-lab/internal/models"
	mastergrpc "github.com/crawlab-team/spider-lab/internal/master/grpc"
	"github.com/crawlab-team/spider-lab/internal/master/scheduler"
	"github.com/crawlab-team/spider-lab/internal/master/spider"
	"github.com/crawlab-team/spider-lab/internal/storage"
	"github.com/crawlab-team/spider-lab/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ==================== Node Controller ====================

type NodeController struct {
	grpcServer *mastergrpc.MasterServer
}

func NewNodeController(grpcServer *mastergrpc.MasterServer) *NodeController {
	return &NodeController{grpcServer: grpcServer}
}

func (ctrl *NodeController) GetList(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("nodes")
	page, size := getPagination(c)

	filter := bson.M{}
	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}

	total, _ := col.CountDocuments(ctx, filter)

	opts := options.Find().SetSkip(int64((page - 1) * size)).SetLimit(int64(size)).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch nodes")
		return
	}
	defer cursor.Close(ctx)

	var nodes []models.Node
	if err := cursor.All(ctx, &nodes); err != nil {
		utils.RespondInternalError(c, "failed to decode nodes")
		return
	}

	utils.RespondPaginated(c, nodes, total, page, size)
}

func (ctrl *NodeController) GetById(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid node id")
		return
	}

	col := utils.GetCollection("nodes")
	var node models.Node
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&node); err != nil {
		utils.RespondNotFound(c, "node not found")
		return
	}

	utils.RespondSuccess(c, node)
}

func (ctrl *NodeController) Delete(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid node id")
		return
	}

	// Look up the node key before deleting so we can disconnect the worker
	col := utils.GetCollection("nodes")
	var node models.Node
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&node); err != nil {
		utils.RespondNotFound(c, "node not found")
		return
	}

	_, err = col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		utils.RespondInternalError(c, "failed to delete node")
		return
	}

	// Immediately disconnect the worker's gRPC stream
	ctrl.grpcServer.DisconnectWorker(node.Key)

	utils.RespondSuccess(c, nil)
}

func (ctrl *NodeController) Update(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid node id")
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.RespondBadRequest(c, "invalid request body")
		return
	}

	// Only allow safe fields to be updated
	allowed := bson.M{"updated_at": time.Now()}
	for _, key := range []string{"name", "description", "max_runners", "enabled"} {
		if val, ok := updates[key]; ok {
			allowed[key] = val
		}
	}

	col := utils.GetCollection("nodes")
	_, err = col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": allowed})
	if err != nil {
		utils.RespondInternalError(c, "failed to update node")
		return
	}

	utils.RespondSuccess(c, nil)
}

func (ctrl *NodeController) Enable(c *gin.Context) {
	ctrl.setEnabled(c, true)
}

func (ctrl *NodeController) Disable(c *gin.Context) {
	ctrl.setEnabled(c, false)
}

func (ctrl *NodeController) setEnabled(c *gin.Context, enabled bool) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid node id")
		return
	}

	col := utils.GetCollection("nodes")
	update := bson.M{"enabled": enabled, "updated_at": time.Now()}
	if !enabled {
		update["status"] = models.NodeStatusOffline
	}
	_, err = col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		utils.RespondInternalError(c, "failed to update node")
		return
	}

	utils.RespondSuccess(c, nil)
}

// ==================== Spider Controller ====================

type SpiderController struct {
	deployer  *spider.Deployer
	scheduler *scheduler.Scheduler
	storage   *storage.Client
}

func NewSpiderController(deployer *spider.Deployer, sched *scheduler.Scheduler, store *storage.Client) *SpiderController {
	return &SpiderController{deployer: deployer, scheduler: sched, storage: store}
}

func (ctrl *SpiderController) GetList(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("spiders")
	page, size := getPagination(c)

	filter := bson.M{}
	if q := c.Query("q"); q != "" {
		filter["name"] = bson.M{"$regex": regexp.QuoteMeta(q), "$options": "i"}
	}
	if spType := c.Query("type"); spType != "" {
		filter["type"] = spType
	}

	total, _ := col.CountDocuments(ctx, filter)

	opts := options.Find().SetSkip(int64((page - 1) * size)).SetLimit(int64(size)).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch spiders")
		return
	}
	defer cursor.Close(ctx)

	var spiders []models.Spider
	if err := cursor.All(ctx, &spiders); err != nil {
		utils.RespondInternalError(c, "failed to decode spiders")
		return
	}

	utils.RespondPaginated(c, spiders, total, page, size)
}

func (ctrl *SpiderController) GetById(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid spider id")
		return
	}

	col := utils.GetCollection("spiders")
	var sp models.Spider
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&sp); err != nil {
		utils.RespondNotFound(c, "spider not found")
		return
	}

	utils.RespondSuccess(c, sp)
}

func (ctrl *SpiderController) Create(c *gin.Context) {
	var sp models.Spider
	if err := c.ShouldBindJSON(&sp); err != nil {
		utils.RespondBadRequest(c, "invalid request body")
		return
	}

	if err := sp.Validate(); err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	userId, _ := c.Get("user_id")
	userOid, _ := primitive.ObjectIDFromHex(userId.(string))
	sp.CreatedBy = userOid

	// For template spiders, auto-fill cmd from template
	if sp.Type == "template" && sp.TemplateId != "" {
		tmpl := spider.GetTemplate(sp.TemplateId)
		if tmpl == nil {
			utils.RespondBadRequest(c, "unknown template_id: "+sp.TemplateId)
			return
		}
		if sp.Cmd == "" {
			sp.Cmd = tmpl.Cmd
		}
		if sp.Description == "" {
			sp.Description = tmpl.Description
		}
	}

	if err := ctrl.deployer.CreateSpider(context.Background(), &sp); err != nil {
		utils.RespondInternalError(c, err.Error())
		return
	}

	utils.RespondCreated(c, sp)
}

func (ctrl *SpiderController) Update(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid spider id")
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.RespondBadRequest(c, "invalid request body")
		return
	}

	allowed := bson.M{"updated_at": time.Now()}
	for _, key := range []string{"name", "description", "cmd", "param", "mode", "node_ids", "col_name", "priority", "project", "tags"} {
		if val, ok := updates[key]; ok {
			allowed[key] = val
		}
	}

	col := utils.GetCollection("spiders")
	_, err = col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": allowed})
	if err != nil {
		utils.RespondInternalError(c, "failed to update spider")
		return
	}

	utils.RespondSuccess(c, nil)
}

func (ctrl *SpiderController) Delete(c *gin.Context) {
	spiderId := c.Param("id")
	if err := ctrl.deployer.DeleteSpider(context.Background(), spiderId); err != nil {
		utils.RespondInternalError(c, err.Error())
		return
	}
	utils.RespondSuccess(c, nil)
}

func (ctrl *SpiderController) Run(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid spider id")
		return
	}

	col := utils.GetCollection("spiders")
	var sp models.Spider
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&sp); err != nil {
		utils.RespondNotFound(c, "spider not found")
		return
	}

	var body struct {
		Param string `json:"param"`
	}
	c.ShouldBindJSON(&body)

	userId, _ := c.Get("user_id")
	userOid, _ := primitive.ObjectIDFromHex(userId.(string))

	task, err := ctrl.scheduler.CreateTask(&sp, body.Param, userOid)
	if err != nil {
		utils.RespondInternalError(c, err.Error())
		return
	}

	utils.RespondCreated(c, task)
}

func (ctrl *SpiderController) GetFiles(c *gin.Context) {
	spiderId := c.Param("id")
	files, err := ctrl.deployer.ListFiles(spiderId)
	if err != nil {
		utils.RespondInternalError(c, err.Error())
		return
	}
	utils.RespondSuccess(c, files)
}

func (ctrl *SpiderController) Upload(c *gin.Context) {
	spiderId := c.Param("id")
	file, err := c.FormFile("file")
	if err != nil {
		utils.RespondBadRequest(c, "no file uploaded")
		return
	}

	src, err := file.Open()
	if err != nil {
		utils.RespondInternalError(c, "failed to open file")
		return
	}
	defer src.Close()

	if err := ctrl.deployer.UploadFile(spiderId, file.Filename, src); err != nil {
		utils.RespondInternalError(c, err.Error())
		return
	}

	utils.RespondSuccess(c, gin.H{"filename": file.Filename})
}

func (ctrl *SpiderController) GetFileContent(c *gin.Context) {
	spiderId := c.Param("id")
	filePath := c.Query("path")
	if filePath == "" {
		utils.RespondBadRequest(c, "path query parameter required")
		return
	}

	content, err := ctrl.deployer.ReadFile(spiderId, filePath)
	if err != nil {
		utils.RespondNotFound(c, "file not found")
		return
	}

	utils.RespondSuccess(c, gin.H{"path": filePath, "content": string(content)})
}

func (ctrl *SpiderController) SaveFileContent(c *gin.Context) {
	spiderId := c.Param("id")
	var body struct {
		Path    string `json:"path" binding:"required"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondBadRequest(c, "invalid request body: path is required")
		return
	}

	if err := ctrl.deployer.WriteFile(spiderId, body.Path, []byte(body.Content)); err != nil {
		utils.RespondInternalError(c, err.Error())
		return
	}

	utils.RespondSuccess(c, gin.H{"path": body.Path})
}

// ServeOutputFile serves raw files from per-task output directories.
// Route: /api/spiders/:id/tasks/:taskId/output/*filepath
func (ctrl *SpiderController) ServeOutputFile(c *gin.Context) {
	spiderId := c.Param("id")
	taskId := c.Param("taskId")
	filePath := c.Param("filepath")
	if filePath == "" {
		utils.RespondBadRequest(c, "file path required")
		return
	}
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		utils.RespondBadRequest(c, "invalid path")
		return
	}

	// Try MinIO storage first
	if ctrl.storage != nil {
		key := "spiders/" + spiderId + "/tasks/" + taskId + "/output/" + strings.ReplaceAll(cleanPath, string(os.PathSeparator), "/")
		if ctrl.storage.Exists(c.Request.Context(), key) {
			if err := ctrl.storage.ServeHTTP(c.Request.Context(), c.Writer, key); err == nil {
				c.Abort()
				return
			}
		}
	}

	// Fallback to local filesystem: {spiderDir}/{spiderId}/output/{taskId}/{filepath}
	fullPath := filepath.Join(ctrl.deployer.GetSpiderDir(spiderId), "output", taskId, cleanPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		utils.RespondNotFound(c, "file not found")
		return
	}
	c.File(fullPath)
}

// ServeImageFile serves shared images from the spider-level images/ directory.
// Route: /api/spiders/:id/images/*filepath
func (ctrl *SpiderController) ServeImageFile(c *gin.Context) {
	spiderId := c.Param("id")
	filePath := c.Param("filepath")
	if filePath == "" {
		utils.RespondBadRequest(c, "file path required")
		return
	}
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		utils.RespondBadRequest(c, "invalid path")
		return
	}

	// Try MinIO storage first
	if ctrl.storage != nil {
		key := "spiders/" + spiderId + "/images/" + strings.ReplaceAll(cleanPath, string(os.PathSeparator), "/")
		if ctrl.storage.Exists(c.Request.Context(), key) {
			if err := ctrl.storage.ServeHTTP(c.Request.Context(), c.Writer, key); err == nil {
				c.Abort()
				return
			}
		}
	}

	// Fallback to local filesystem: {spiderDir}/{spiderId}/images/{filepath}
	fullPath := filepath.Join(ctrl.deployer.GetSpiderDir(spiderId), "images", cleanPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		utils.RespondNotFound(c, "file not found")
		return
	}
	c.File(fullPath)
}

func (ctrl *SpiderController) GetTemplates(c *gin.Context) {
	templates := spider.ListTemplates()
	utils.RespondSuccess(c, templates)
}

func (ctrl *SpiderController) SearchWeiboUser(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		utils.RespondBadRequest(c, "query parameter 'q' is required")
		return
	}

	// Find the Playwright helper script relative to the executable
	scriptPath := findScript("weibo_user_search.py")
	if scriptPath == "" {
		utils.RespondInternalError(c, "weibo search script not found")
		return
	}

	// Run the script with a 30s timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", scriptPath, q, "8")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		utils.Logger.Warnf("Weibo user search script failed: %v, stderr: %s", err, stderr.String())
		utils.RespondSuccess(c, []struct{}{})
		return
	}

	// Parse raw user list from script output
	var rawUsers []map[string]interface{}
	if err := json.Unmarshal(output, &rawUsers); err != nil {
		utils.Logger.Warnf("Weibo user search: failed to parse output: %v, raw: %s", err, string(output))
		utils.RespondSuccess(c, []struct{}{})
		return
	}

	type WeiboUserResult struct {
		UID            string `json:"uid"`
		ScreenName     string `json:"screen_name"`
		AvatarURL      string `json:"avatar_url"`
		Description    string `json:"description"`
		FollowersCount int    `json:"followers_count"`
		Verified       bool   `json:"verified"`
		VerifiedType   int    `json:"verified_type"`
		VerifiedReason string `json:"verified_reason"`
	}

	var users []WeiboUserResult
	for _, um := range rawUsers {
		uid := ""
		switch v := um["id"].(type) {
		case float64:
			uid = strconv.FormatInt(int64(v), 10)
		case string:
			uid = v
		}
		if uid == "" {
			continue
		}

		followersCount := 0
		switch fc := um["followers_count"].(type) {
		case float64:
			followersCount = int(fc)
		case string:
			followersCount = parseChineseNumber(fc)
		}

		verifiedType := -1
		if vt, ok := um["verified_type"].(float64); ok {
			verifiedType = int(vt)
		}

		avatarURL := ""
		if av, ok := um["avatar_hd"].(string); ok && av != "" {
			avatarURL = av
		} else if av, ok := um["avatar_large"].(string); ok && av != "" {
			avatarURL = av
		} else if av, ok := um["profile_image_url"].(string); ok {
			avatarURL = av
		}

		u := WeiboUserResult{
			UID:            uid,
			ScreenName:     getStr(um, "screen_name"),
			AvatarURL:      avatarURL,
			Description:    getStr(um, "description"),
			FollowersCount: followersCount,
			Verified:       getBool(um, "verified"),
			VerifiedType:   verifiedType,
			VerifiedReason: getStr(um, "verified_reason"),
		}
		users = append(users, u)
	}

	if users == nil {
		users = []WeiboUserResult{}
	}

	utils.RespondSuccess(c, users)
}

func (ctrl *SpiderController) SearchXueqiuUser(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		utils.RespondBadRequest(c, "query parameter 'q' is required")
		return
	}

	// Find the Playwright helper script relative to the executable
	scriptPath := findScript("xueqiu_user_search.py")
	if scriptPath == "" {
		utils.RespondInternalError(c, "xueqiu search script not found")
		return
	}

	// Run the script with a 30s timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", scriptPath, q, "8")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		utils.Logger.Warnf("Xueqiu user search script failed: %v, stderr: %s", err, stderr.String())
		utils.RespondSuccess(c, []struct{}{})
		return
	}

	// Parse raw user list from script output
	var rawUsers []map[string]interface{}
	if err := json.Unmarshal(output, &rawUsers); err != nil {
		utils.Logger.Warnf("Xueqiu user search: failed to parse output: %v, raw: %s", err, string(output))
		utils.RespondSuccess(c, []struct{}{})
		return
	}

	type XueqiuUserResult struct {
		ID                  string `json:"id"`
		ScreenName          string `json:"screen_name"`
		AvatarURL           string `json:"avatar_url"`
		Description         string `json:"description"`
		FollowersCount      int    `json:"followers_count"`
		Verified            bool   `json:"verified"`
		VerifiedDescription string `json:"verified_description"`
	}

	var users []XueqiuUserResult
	for _, um := range rawUsers {
		id := ""
		switch v := um["id"].(type) {
		case float64:
			id = strconv.FormatInt(int64(v), 10)
		case string:
			id = v
		}
		if id == "" {
			continue
		}

		followersCount := 0
		if fc, ok := um["followers_count"].(float64); ok {
			followersCount = int(fc)
		}

		avatarURL := ""
		if photo := getStr(um, "photo_domain"); photo != "" {
			raw := getStr(um, "profile_image_url")
			// profile_image_url is comma-separated; first part is the original image
			if parts := strings.SplitN(raw, ",", 2); len(parts) > 0 {
				avatarURL = photo + parts[0]
			}
		}

		u := XueqiuUserResult{
			ID:                  id,
			ScreenName:          getStr(um, "screen_name"),
			AvatarURL:           avatarURL,
			Description:         getStr(um, "description"),
			FollowersCount:      followersCount,
			Verified:            getBool(um, "verified"),
			VerifiedDescription: getStr(um, "verified_description"),
		}
		users = append(users, u)
	}

	if users == nil {
		users = []XueqiuUserResult{}
	}

	utils.RespondSuccess(c, users)
}

func (ctrl *SpiderController) SearchWechatUser(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		utils.RespondBadRequest(c, "query parameter 'q' is required")
		return
	}

	// Get an active WeChat cookie from the pool
	ctx := context.Background()
	col := utils.GetCollection("wechat_cookies")
	cursor, err := col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "last_used_at", Value: 1}}))
	if err != nil {
		utils.Logger.Errorf("Failed to get WeChat cookies: %v", err)
		utils.RespondInternalError(c, "Failed to get WeChat cookies")
		return
	}
	defer cursor.Close(ctx)

	var cookies []models.WechatCookie
	if err := cursor.All(ctx, &cookies); err != nil {
		utils.Logger.Errorf("Failed to decode WeChat cookies: %v", err)
		utils.RespondInternalError(c, "Failed to get WeChat cookies")
		return
	}

	if len(cookies) == 0 {
		utils.RespondSuccess(c, []struct{}{})
		return
	}

	// Find an active cookie not in cooldown
	var selectedCookie *models.WechatCookie
	now := time.Now()
	for i := range cookies {
		if cookies[i].Status == "active" && (cookies[i].CooldownTill.IsZero() || cookies[i].CooldownTill.Before(now)) {
			selectedCookie = &cookies[i]
			break
		}
	}

	if selectedCookie == nil {
		utils.RespondSuccess(c, []struct{}{})
		return
	}

	// Find the Playwright helper script relative to the executable
	scriptPath := findScript("wechat_user_search.py")
	if scriptPath == "" {
		utils.RespondInternalError(c, "wechat search script not found")
		return
	}

	// Run the script with a 30s timeout
	scriptCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(scriptCtx, "python3", scriptPath, q, "8", selectedCookie.Cookie, selectedCookie.Token)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		utils.Logger.Warnf("WeChat user search script failed: %v, stderr: %s", err, stderr.String())
		utils.RespondSuccess(c, []struct{}{})
		return
	}

	// Parse raw account list from script output
	var rawAccounts []map[string]interface{}
	if err := json.Unmarshal(output, &rawAccounts); err != nil {
		utils.Logger.Warnf("WeChat user search: failed to parse output: %v, raw: %s", err, string(output))
		utils.RespondSuccess(c, []struct{}{})
		return
	}

	type WechatUserResult struct {
		ID             string `json:"id"`
		Nickname       string `json:"nickname"`
		AvatarURL      string `json:"avatar_url"`
		Description    string `json:"description"`
		FollowersCount int    `json:"followers_count"`
		Verified       bool   `json:"verified"`
	}

	var accounts []WechatUserResult
	for _, am := range rawAccounts {
		id := getStr(am, "user_id")
		if id == "" {
			continue
		}

		followersCount := 0
		if fc, ok := am["followers_count"].(float64); ok {
			followersCount = int(fc)
		}

		a := WechatUserResult{
			ID:             id,
			Nickname:       getStr(am, "nickname"),
			AvatarURL:      getStr(am, "avatar_url"),
			Description:    getStr(am, "description"),
			FollowersCount: followersCount,
			Verified:       getBool(am, "verified"),
		}
		accounts = append(accounts, a)
	}

	if accounts == nil {
		accounts = []WechatUserResult{}
	}

	utils.RespondSuccess(c, accounts)
}

// findScript locates a helper script in the backend/scripts directory,
// searching relative to the executable, working directory, and Docker paths.
func findScript(name string) string {
	// Try relative to the executable (backend/bin/master → backend/scripts/)
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "scripts", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Try relative to working directory (e.g. running from project root)
	for _, rel := range []string{
		filepath.Join("backend", "scripts", name),
		filepath.Join("scripts", name),
		// Docker: scripts copied to /app/scripts/
		filepath.Join("/app", "scripts", name),
	} {
		if _, err := os.Stat(rel); err == nil {
			return rel
		}
	}
	return ""
}

func (ctrl *SpiderController) LookupXhsUser(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		utils.RespondBadRequest(c, "query parameter 'id' is required")
		return
	}

	type XhsUserResult struct {
		UserID         string `json:"user_id"`
		Nickname       string `json:"nickname"`
		Avatar         string `json:"avatar"`
		Desc           string `json:"desc"`
		RedID          string `json:"red_id"`
		FollowersCount int    `json:"followers_count"`
		Verified       bool   `json:"verified"`
	}

	profileURL := "https://www.xiaohongshu.com/user/profile/" + id
	req, err := http.NewRequest("GET", profileURL, nil)
	if err != nil {
		utils.RespondSuccess(c, XhsUserResult{UserID: id})
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		utils.RespondSuccess(c, XhsUserResult{UserID: id})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		utils.RespondSuccess(c, XhsUserResult{UserID: id})
		return
	}

	html := string(body)

	// Extract __INITIAL_STATE__ from SSR HTML
	re := regexp.MustCompile(`__INITIAL_STATE__\s*=\s*(\{.+?\})\s*</script>`)
	match := re.FindStringSubmatch(html)
	if match == nil {
		re = regexp.MustCompile(`__INITIAL_STATE__\s*=\s*(\{.+\})`)
		match = re.FindStringSubmatch(html)
	}

	if match != nil {
		stateStr := strings.ReplaceAll(match[1], "undefined", "null")
		var state map[string]interface{}
		if err := json.Unmarshal([]byte(stateStr), &state); err == nil {
			userMap, _ := state["user"].(map[string]interface{})
			pageData, _ := userMap["userPageData"].(map[string]interface{})
			basicInfo, _ := pageData["basicInfo"].(map[string]interface{})
			if basicInfo != nil {
				result := XhsUserResult{
					UserID:   id,
					Nickname: getStr(basicInfo, "nickname"),
					Avatar:   getStr(basicInfo, "imageb"),
					Desc:     getStr(basicInfo, "desc"),
					RedID:    getStr(basicInfo, "redId"),
				}
				// Extract followers count from interactions array
				if interactions, ok := pageData["interactions"].([]interface{}); ok {
					for _, item := range interactions {
						if m, ok := item.(map[string]interface{}); ok {
							if getStr(m, "type") == "fans" {
								if cnt, ok := m["count"].(string); ok {
									if n, err := strconv.Atoi(cnt); err == nil {
										result.FollowersCount = n
									}
								} else if cnt, ok := m["count"].(float64); ok {
									result.FollowersCount = int(cnt)
								}
							}
						}
					}
				}
				// Check verified status from tags
				if tags, ok := pageData["tags"].([]interface{}); ok {
					for _, tag := range tags {
						if m, ok := tag.(map[string]interface{}); ok {
							tagType := getStr(m, "tagType")
							if tagType == "official" || tagType == "identity" {
								result.Verified = true
								break
							}
						}
					}
				}
				if result.Nickname != "" {
					utils.RespondSuccess(c, result)
					return
				}
			}
		}
	}

	utils.RespondSuccess(c, XhsUserResult{UserID: id})
}

func (ctrl *SpiderController) LookupXueqiuUser(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		utils.RespondBadRequest(c, "query parameter 'id' is required")
		return
	}

	type XueqiuUserInfo struct {
		ID         string `json:"id"`
		ScreenName string `json:"screen_name"`
		AvatarURL  string `json:"avatar_url"`
	}

	scriptPath := findScript("xueqiu_user_info.py")
	if scriptPath == "" {
		utils.RespondSuccess(c, XueqiuUserInfo{ID: id})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", scriptPath, id)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		utils.Logger.Warnf("Xueqiu user info script failed: %v, stderr: %s", err, stderr.String())
		utils.RespondSuccess(c, XueqiuUserInfo{ID: id})
		return
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(output, &raw); err != nil || len(raw) == 0 {
		utils.RespondSuccess(c, XueqiuUserInfo{ID: id})
		return
	}

	avatarURL := ""
	if photo := getStr(raw, "photo_domain"); photo != "" {
		rawURL := getStr(raw, "profile_image_url")
		if parts := strings.SplitN(rawURL, ",", 2); len(parts) > 0 {
			avatarURL = photo + parts[0]
		}
	}

	uid := id
	switch v := raw["id"].(type) {
	case float64:
		uid = strconv.FormatInt(int64(v), 10)
	case string:
		uid = v
	}

	utils.RespondSuccess(c, XueqiuUserInfo{
		ID:         uid,
		ScreenName: getStr(raw, "screen_name"),
		AvatarURL:  avatarURL,
	})
}

func (ctrl *SpiderController) LookupWeiboUser(c *gin.Context) {
	uid := c.Query("uid")
	if uid == "" {
		utils.RespondBadRequest(c, "query parameter 'uid' is required")
		return
	}

	type WeiboUserInfo struct {
		UID        string `json:"uid"`
		ScreenName string `json:"screen_name"`
		AvatarURL  string `json:"avatar_url"`
	}

	// Use m.weibo.cn mobile API — works without authentication
	apiURL := "https://m.weibo.cn/api/container/getIndex?type=uid&value=" + uid
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		utils.RespondSuccess(c, WeiboUserInfo{UID: uid})
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://m.weibo.cn/")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		utils.RespondSuccess(c, WeiboUserInfo{UID: uid})
		return
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		utils.RespondSuccess(c, WeiboUserInfo{UID: uid})
		return
	}

	data, _ := raw["data"].(map[string]interface{})
	userInfo, _ := data["userInfo"].(map[string]interface{})
	if userInfo == nil {
		utils.RespondSuccess(c, WeiboUserInfo{UID: uid})
		return
	}

	avatarURL := getStr(userInfo, "avatar_hd")
	if avatarURL == "" {
		avatarURL = getStr(userInfo, "profile_image_url")
	}

	utils.RespondSuccess(c, WeiboUserInfo{
		UID:        uid,
		ScreenName: getStr(userInfo, "screen_name"),
		AvatarURL:  avatarURL,
	})
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// parseChineseNumber converts strings like "1.4亿", "183.7万", "5379" to int.
func parseChineseNumber(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.Contains(s, "亿") {
		s = strings.Replace(s, "亿", "", 1)
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int(f * 100_000_000)
		}
	}
	if strings.Contains(s, "万") {
		s = strings.Replace(s, "万", "", 1)
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int(f * 10_000)
		}
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return 0
}

func (ctrl *SpiderController) UpdateConfig(c *gin.Context) {
	ctx := context.Background()
	spiderId := c.Param("id")
	id, err := primitive.ObjectIDFromHex(spiderId)
	if err != nil {
		utils.RespondBadRequest(c, "invalid spider id")
		return
	}

	var body struct {
		Config map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondBadRequest(c, "invalid request body")
		return
	}

	// Update config in MongoDB
	col := utils.GetCollection("spiders")
	_, err = col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{"config": body.Config, "updated_at": time.Now()},
	})
	if err != nil {
		utils.RespondInternalError(c, "failed to update config")
		return
	}

	// Write config.json to spider directory
	if err := ctrl.deployer.UpdateConfig(spiderId, body.Config); err != nil {
		utils.RespondInternalError(c, err.Error())
		return
	}

	utils.RespondSuccess(c, nil)
}

// ==================== Task Controller ====================

type TaskController struct {
	scheduler  *scheduler.Scheduler
	grpcServer *mastergrpc.MasterServer
}

func NewTaskController(sched *scheduler.Scheduler, grpcServer *mastergrpc.MasterServer) *TaskController {
	return &TaskController{scheduler: sched, grpcServer: grpcServer}
}

func (ctrl *TaskController) GetList(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("tasks")
	page, size := getPagination(c)

	filter := bson.M{}
	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}
	if spiderId := c.Query("spider_id"); spiderId != "" {
		oid, _ := primitive.ObjectIDFromHex(spiderId)
		filter["spider_id"] = oid
	}
	if nodeId := c.Query("node_id"); nodeId != "" {
		oid, _ := primitive.ObjectIDFromHex(nodeId)
		filter["node_id"] = oid
	}

	total, _ := col.CountDocuments(ctx, filter)

	opts := options.Find().SetSkip(int64((page - 1) * size)).SetLimit(int64(size)).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch tasks")
		return
	}
	defer cursor.Close(ctx)

	var tasks []models.Task
	if err := cursor.All(ctx, &tasks); err != nil {
		utils.RespondInternalError(c, "failed to decode tasks")
		return
	}

	// Batch-populate spider and node names
	spiderIds := make(map[primitive.ObjectID]bool)
	nodeIds := make(map[primitive.ObjectID]bool)
	for _, t := range tasks {
		if !t.SpiderId.IsZero() {
			spiderIds[t.SpiderId] = true
		}
		if !t.NodeId.IsZero() {
			nodeIds[t.NodeId] = true
		}
	}

	spiderMap := make(map[primitive.ObjectID]*models.Spider)
	if len(spiderIds) > 0 {
		ids := make([]primitive.ObjectID, 0, len(spiderIds))
		for id := range spiderIds {
			ids = append(ids, id)
		}
		spiderCol := utils.GetCollection("spiders")
		sCursor, _ := spiderCol.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
		if sCursor != nil {
			defer sCursor.Close(ctx)
			var spiders []models.Spider
			if err := sCursor.All(ctx, &spiders); err == nil {
				for i := range spiders {
					spiderMap[spiders[i].Id] = &spiders[i]
				}
			}
		}
	}

	nodeMap := make(map[primitive.ObjectID]*models.Node)
	if len(nodeIds) > 0 {
		ids := make([]primitive.ObjectID, 0, len(nodeIds))
		for id := range nodeIds {
			ids = append(ids, id)
		}
		nodeCol := utils.GetCollection("nodes")
		nCursor, _ := nodeCol.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
		if nCursor != nil {
			defer nCursor.Close(ctx)
			var nodes []models.Node
			if err := nCursor.All(ctx, &nodes); err == nil {
				for i := range nodes {
					nodeMap[nodes[i].Id] = &nodes[i]
				}
			}
		}
	}

	for i := range tasks {
		if sp, ok := spiderMap[tasks[i].SpiderId]; ok {
			tasks[i].Spider = sp
		}
		if nd, ok := nodeMap[tasks[i].NodeId]; ok {
			tasks[i].Node = nd
		}
	}

	utils.RespondPaginated(c, tasks, total, page, size)
}

func (ctrl *TaskController) GetById(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid task id")
		return
	}

	col := utils.GetCollection("tasks")
	var task models.Task
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&task); err != nil {
		utils.RespondNotFound(c, "task not found")
		return
	}

	spiderCol := utils.GetCollection("spiders")
	var sp models.Spider
	if err := spiderCol.FindOne(ctx, bson.M{"_id": task.SpiderId}).Decode(&sp); err == nil {
		task.Spider = &sp
	}

	nodeCol := utils.GetCollection("nodes")
	var node models.Node
	if err := nodeCol.FindOne(ctx, bson.M{"_id": task.NodeId}).Decode(&node); err == nil {
		task.Node = &node
	}

	utils.RespondSuccess(c, task)
}

func (ctrl *TaskController) Cancel(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid task id")
		return
	}

	col := utils.GetCollection("tasks")
	var task models.Task
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&task); err != nil {
		utils.RespondNotFound(c, "task not found")
		return
	}

	if task.Status != models.TaskStatusRunning && task.Status != models.TaskStatusPending {
		utils.RespondBadRequest(c, "task is not running or pending")
		return
	}

	nodeCol := utils.GetCollection("nodes")
	var node models.Node
	if err := nodeCol.FindOne(ctx, bson.M{"_id": task.NodeId}).Decode(&node); err == nil {
		_ = ctrl.grpcServer.SendCancelToWorker(node.Key, task.Id.Hex())
	}

	_, err = col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{"status": models.TaskStatusCancelled, "end_ts": time.Now(), "updated_at": time.Now()},
	})
	if err != nil {
		utils.RespondInternalError(c, "failed to cancel task")
		return
	}

	utils.RespondSuccess(c, nil)
}

func (ctrl *TaskController) Restart(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid task id")
		return
	}

	col := utils.GetCollection("tasks")
	var task models.Task
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&task); err != nil {
		utils.RespondNotFound(c, "task not found")
		return
	}

	spiderCol := utils.GetCollection("spiders")
	var sp models.Spider
	if err := spiderCol.FindOne(ctx, bson.M{"_id": task.SpiderId}).Decode(&sp); err != nil {
		utils.RespondNotFound(c, "spider not found")
		return
	}

	userId, _ := c.Get("user_id")
	userOid, _ := primitive.ObjectIDFromHex(userId.(string))
	newTask, err := ctrl.scheduler.CreateTask(&sp, task.Param, userOid)
	if err != nil {
		utils.RespondInternalError(c, err.Error())
		return
	}

	utils.RespondCreated(c, newTask)
}

func (ctrl *TaskController) Delete(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid task id")
		return
	}

	col := utils.GetCollection("tasks")
	var task models.Task
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&task); err != nil {
		utils.RespondNotFound(c, "task not found")
		return
	}

	// Only allow deleting finished, error, or cancelled tasks
	if task.Status == "running" || task.Status == "pending" || task.Status == "waiting" {
		utils.RespondBadRequest(c, "cannot delete a task that is still active")
		return
	}

	// Delete the task itself
	col.DeleteOne(ctx, bson.M{"_id": id})

	// Clean up associated logs and results
	utils.GetCollection("task_logs").DeleteMany(ctx, bson.M{"task_id": id})
	utils.GetCollection("results").DeleteMany(ctx, bson.M{"task_id": id})

	utils.RespondSuccess(c, nil)
}

func (ctrl *TaskController) GetLogs(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid task id")
		return
	}

	page, size := getPagination(c)
	col := utils.GetCollection("task_logs")

	total, _ := col.CountDocuments(ctx, bson.M{"task_id": id})

	opts := options.Find().SetSkip(int64((page - 1) * size)).SetLimit(int64(size)).SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cursor, err := col.Find(ctx, bson.M{"task_id": id}, opts)
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch logs")
		return
	}
	defer cursor.Close(ctx)

	var logs []models.TaskLog
	if err := cursor.All(ctx, &logs); err != nil {
		utils.RespondInternalError(c, "failed to decode logs")
		return
	}

	utils.RespondPaginated(c, logs, total, page, size)
}

func (ctrl *TaskController) GetResults(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid task id")
		return
	}

	page, size := getPagination(c)
	col := utils.GetCollection("results")

	total, _ := col.CountDocuments(ctx, bson.M{"task_id": id})

	opts := options.Find().SetSkip(int64((page - 1) * size)).SetLimit(int64(size)).SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cursor, err := col.Find(ctx, bson.M{"task_id": id}, opts)
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch results")
		return
	}
	defer cursor.Close(ctx)

	var results []models.ResultItem
	if err := cursor.All(ctx, &results); err != nil {
		utils.RespondInternalError(c, "failed to decode results")
		return
	}

	utils.RespondPaginated(c, results, total, page, size)
}

// ==================== Schedule Controller ====================

type ScheduleController struct {
	scheduler *scheduler.Scheduler
}

func NewScheduleController(sched *scheduler.Scheduler) *ScheduleController {
	return &ScheduleController{scheduler: sched}
}

func (ctrl *ScheduleController) GetList(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("schedules")
	page, size := getPagination(c)

	total, _ := col.CountDocuments(ctx, bson.M{})
	opts := options.Find().SetSkip(int64((page - 1) * size)).SetLimit(int64(size)).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := col.Find(ctx, bson.M{}, opts)
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch schedules")
		return
	}
	defer cursor.Close(ctx)

	var schedules []models.Schedule
	if err := cursor.All(ctx, &schedules); err != nil {
		utils.RespondInternalError(c, "failed to decode schedules")
		return
	}

	utils.RespondPaginated(c, schedules, total, page, size)
}

func (ctrl *ScheduleController) GetById(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid schedule id")
		return
	}

	col := utils.GetCollection("schedules")
	var sched models.Schedule
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&sched); err != nil {
		utils.RespondNotFound(c, "schedule not found")
		return
	}

	utils.RespondSuccess(c, sched)
}

func (ctrl *ScheduleController) Create(c *gin.Context) {
	var sched models.Schedule
	if err := c.ShouldBindJSON(&sched); err != nil {
		utils.RespondBadRequest(c, "invalid request body")
		return
	}

	userId, _ := c.Get("user_id")
	userOid, _ := primitive.ObjectIDFromHex(userId.(string))
	sched.CreatedBy = userOid

	// Validate cron expression before inserting into DB to avoid orphaned records
	if sched.Cron != "" {
		parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		if _, err := parser.Parse(sched.Cron); err != nil {
			utils.RespondBadRequest(c, "invalid cron expression: "+err.Error())
			return
		}
	}

	now := time.Now()
	sched.CreatedAt = now
	sched.UpdatedAt = now

	col := utils.GetCollection("schedules")
	result, err := col.InsertOne(context.Background(), sched)
	if err != nil {
		utils.RespondInternalError(c, "failed to create schedule")
		return
	}
	sched.Id = result.InsertedID.(primitive.ObjectID)

	if sched.Enabled {
		if err := ctrl.scheduler.AddSchedule(&sched); err != nil {
			utils.RespondInternalError(c, err.Error())
			return
		}
	}

	utils.RespondCreated(c, sched)
}

func (ctrl *ScheduleController) Update(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid schedule id")
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.RespondBadRequest(c, "invalid request body")
		return
	}
	updates["updated_at"] = time.Now()

	col := utils.GetCollection("schedules")

	var oldSched models.Schedule
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&oldSched); err != nil {
		utils.RespondNotFound(c, "schedule not found")
		return
	}

	if oldSched.EntryId > 0 {
		ctrl.scheduler.RemoveSchedule(oldSched.EntryId)
	}

	_, err = col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		utils.RespondInternalError(c, "failed to update schedule")
		return
	}

	var newSched models.Schedule
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&newSched); err == nil && newSched.Enabled {
		_ = ctrl.scheduler.AddSchedule(&newSched)
	}

	utils.RespondSuccess(c, nil)
}

func (ctrl *ScheduleController) Delete(c *gin.Context) {
	ctx := context.Background()
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "invalid schedule id")
		return
	}

	col := utils.GetCollection("schedules")
	var sched models.Schedule
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&sched); err == nil && sched.EntryId > 0 {
		ctrl.scheduler.RemoveSchedule(sched.EntryId)
	}

	_, err = col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		utils.RespondInternalError(c, "failed to delete schedule")
		return
	}

	utils.RespondSuccess(c, nil)
}

// ==================== Setting Controller ====================

type SettingController struct{}

func NewSettingController() *SettingController { return &SettingController{} }

func (ctrl *SettingController) GetList(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("settings")

	cursor, err := col.Find(ctx, bson.M{})
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch settings")
		return
	}
	defer cursor.Close(ctx)

	var settings []models.Setting
	if err := cursor.All(ctx, &settings); err != nil {
		utils.RespondInternalError(c, "failed to decode settings")
		return
	}

	utils.RespondSuccess(c, settings)
}

func (ctrl *SettingController) Update(c *gin.Context) {
	key := c.Param("key")

	var body struct {
		Value interface{} `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondBadRequest(c, "invalid request body")
		return
	}

	col := utils.GetCollection("settings")
	filter := bson.M{"key": key}
	update := bson.M{
		"$set":         bson.M{"value": body.Value, "updated_at": time.Now()},
		"$setOnInsert": bson.M{"key": key},
	}

	opts := options.Update().SetUpsert(true)
	_, err := col.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		utils.RespondInternalError(c, "failed to update setting")
		return
	}

	utils.RespondSuccess(c, nil)
}

// ==================== Stats Controller ====================

type StatsController struct{}

func NewStatsController() *StatsController { return &StatsController{} }

func (ctrl *StatsController) GetOverview(c *gin.Context) {
	ctx := context.Background()

	nodeCol := utils.GetCollection("nodes")
	totalNodes, _ := nodeCol.CountDocuments(ctx, bson.M{})
	onlineNodes, _ := nodeCol.CountDocuments(ctx, bson.M{"status": "online"})

	taskCol := utils.GetCollection("tasks")
	totalTasks, _ := taskCol.CountDocuments(ctx, bson.M{})
	runningTasks, _ := taskCol.CountDocuments(ctx, bson.M{"status": "running"})
	pendingTasks, _ := taskCol.CountDocuments(ctx, bson.M{"status": "pending"})
	finishedTasks, _ := taskCol.CountDocuments(ctx, bson.M{"status": "finished"})
	errorTasks, _ := taskCol.CountDocuments(ctx, bson.M{"status": "error"})

	spiderCol := utils.GetCollection("spiders")
	totalSpiders, _ := spiderCol.CountDocuments(ctx, bson.M{})

	schedCol := utils.GetCollection("schedules")
	activeSchedules, _ := schedCol.CountDocuments(ctx, bson.M{"enabled": true})

	utils.RespondSuccess(c, gin.H{
		"nodes":            gin.H{"total": totalNodes, "online": onlineNodes},
		"tasks":            gin.H{"total": totalTasks, "running": runningTasks, "pending": pendingTasks, "finished": finishedTasks, "error": errorTasks},
		"spiders":          totalSpiders,
		"active_schedules": activeSchedules,
	})
}

func (ctrl *StatsController) GetTaskStats(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("tasks")

	now := time.Now()
	stats := make([]gin.H, 7)
	for i := 6; i >= 0; i-- {
		dayStart := time.Date(now.Year(), now.Month(), now.Day()-i, 0, 0, 0, 0, now.Location())
		dayEnd := dayStart.Add(24 * time.Hour)

		filter := bson.M{"created_at": bson.M{"$gte": dayStart, "$lt": dayEnd}}
		count, _ := col.CountDocuments(ctx, filter)
		finished, _ := col.CountDocuments(ctx, bson.M{"created_at": bson.M{"$gte": dayStart, "$lt": dayEnd}, "status": "finished"})
		errored, _ := col.CountDocuments(ctx, bson.M{"created_at": bson.M{"$gte": dayStart, "$lt": dayEnd}, "status": "error"})

		stats[6-i] = gin.H{"date": dayStart.Format("2006-01-02"), "total": count, "finished": finished, "error": errored}
	}

	utils.RespondSuccess(c, stats)
}

// ==================== Log Controller ====================

type LogController struct{}

func NewLogController() *LogController { return &LogController{} }

func (ctrl *LogController) GetSystemLogs(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("system_logs")
	page, size := getPagination(c)

	filter := bson.M{}
	if level := c.Query("level"); level != "" {
		filter["level"] = level
	}

	total, _ := col.CountDocuments(ctx, filter)
	opts := options.Find().SetSkip(int64((page - 1) * size)).SetLimit(int64(size)).SetSort(bson.D{{Key: "timestamp", Value: -1}})
	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch logs")
		return
	}
	defer cursor.Close(ctx)

	var logs []models.SystemLog
	if err := cursor.All(ctx, &logs); err != nil {
		utils.RespondInternalError(c, "failed to decode logs")
		return
	}

	utils.RespondPaginated(c, logs, total, page, size)
}

// ==================== User Controller ====================

type UserController struct{}

func NewUserController() *UserController { return &UserController{} }

func (ctrl *UserController) GetMe(c *gin.Context) {
	userId, _ := c.Get("user_id")
	id, err := primitive.ObjectIDFromHex(userId.(string))
	if err != nil {
		utils.RespondBadRequest(c, "invalid user id")
		return
	}

	col := utils.GetCollection("users")
	var user models.User
	if err := col.FindOne(context.Background(), bson.M{"_id": id}).Decode(&user); err != nil {
		utils.RespondNotFound(c, "user not found")
		return
	}

	utils.RespondSuccess(c, user)
}

func (ctrl *UserController) UpdateMe(c *gin.Context) {
	userId, _ := c.Get("user_id")
	id, err := primitive.ObjectIDFromHex(userId.(string))
	if err != nil {
		utils.RespondBadRequest(c, "invalid user id")
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.RespondBadRequest(c, "invalid request body")
		return
	}
	delete(updates, "password")
	delete(updates, "role")
	updates["updated_at"] = time.Now()

	col := utils.GetCollection("users")
	_, err = col.UpdateOne(context.Background(), bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		utils.RespondInternalError(c, "failed to update user")
		return
	}

	utils.RespondSuccess(c, nil)
}

// ==================== Result Controller ====================

type ResultController struct{}

func NewResultController() *ResultController { return &ResultController{} }

func (ctrl *ResultController) GetList(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("results")
	page, size := getPagination(c)

	filter := bson.M{}
	if spiderId := c.Query("spider_id"); spiderId != "" {
		oid, err := primitive.ObjectIDFromHex(spiderId)
		if err == nil {
			filter["spider_id"] = oid
		}
	}
	if taskId := c.Query("task_id"); taskId != "" {
		oid, err := primitive.ObjectIDFromHex(taskId)
		if err == nil {
			filter["task_id"] = oid
		}
	}
	// Server-side text search across common item fields
	if q := c.Query("q"); q != "" {
		regex := bson.M{"$regex": regexp.QuoteMeta(q), "$options": "i"}
		filter["$or"] = bson.A{
			bson.M{"item.text": regex},
			bson.M{"item.title": regex},
			bson.M{"item.user_name": regex},
			bson.M{"item.keyword": regex},
			bson.M{"item.full_text": regex},
			bson.M{"item.link_content.summary": regex},
		}
	}

	total, _ := col.CountDocuments(ctx, filter)

	opts := options.Find().SetSkip(int64((page - 1) * size)).SetLimit(int64(size)).SetSort(bson.D{{Key: "published_at", Value: -1}, {Key: "timestamp", Value: -1}})
	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch results")
		return
	}
	defer cursor.Close(ctx)

	var results []models.ResultItem
	if err := cursor.All(ctx, &results); err != nil {
		utils.RespondInternalError(c, "failed to decode results")
		return
	}

	// Batch-populate spider names
	spiderIds := make(map[primitive.ObjectID]bool)
	for _, r := range results {
		if !r.SpiderId.IsZero() {
			spiderIds[r.SpiderId] = true
		}
	}
	spiderMap := make(map[primitive.ObjectID]string)
	if len(spiderIds) > 0 {
		ids := make([]primitive.ObjectID, 0, len(spiderIds))
		for id := range spiderIds {
			ids = append(ids, id)
		}
		spiderCol := utils.GetCollection("spiders")
		sCursor, err := spiderCol.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
		if err == nil {
			defer sCursor.Close(ctx)
			var spiders []models.Spider
			if sCursor.All(ctx, &spiders) == nil {
				for _, s := range spiders {
					spiderMap[s.Id] = s.Name
				}
			}
		}
	}

	type EnrichedResult struct {
		models.ResultItem `bson:",inline"`
		SpiderName        string `json:"spider_name"`
	}

	enriched := make([]EnrichedResult, len(results))
	for i, r := range results {
		enriched[i] = EnrichedResult{
			ResultItem: r,
			SpiderName: spiderMap[r.SpiderId],
		}
	}

	utils.RespondPaginated(c, enriched, total, page, size)
}

func (ctrl *ResultController) GetStats(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("results")

	total, _ := col.CountDocuments(ctx, bson.M{})

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayCount, _ := col.CountDocuments(ctx, bson.M{"timestamp": bson.M{"$gte": todayStart}})

	utils.RespondSuccess(c, gin.H{
		"total":       total,
		"today_count": todayCount,
	})
}

// ==================== Post Controller ====================

type PostController struct{}

func NewPostController() *PostController { return &PostController{} }

func (ctrl *PostController) GetList(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("posts")
	page, size := getPagination(c)

	filter := bson.M{}
	if platform := c.Query("platform"); platform != "" {
		filter["platform"] = platform
	}
	if itemType := c.Query("type"); itemType != "" {
		filter["type"] = itemType
	}
	if spiderId := c.Query("spider_id"); spiderId != "" {
		oid, err := primitive.ObjectIDFromHex(spiderId)
		if err == nil {
			filter["spider_ids"] = oid
		}
	}
	if q := c.Query("q"); q != "" {
		regex := bson.M{"$regex": regexp.QuoteMeta(q), "$options": "i"}
		filter["$or"] = bson.A{
			bson.M{"item.text": regex},
			bson.M{"item.title": regex},
			bson.M{"item.user_name": regex},
			bson.M{"item.full_text": regex},
			bson.M{"platform_id": regex},
		}
	}

	total, _ := col.CountDocuments(ctx, filter)

	opts := options.Find().SetSkip(int64((page - 1) * size)).SetLimit(int64(size)).SetSort(bson.D{{Key: "published_at", Value: -1}, {Key: "updated_at", Value: -1}})
	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		utils.RespondInternalError(c, "failed to fetch posts")
		return
	}
	defer cursor.Close(ctx)

	var posts []models.Post
	if err := cursor.All(ctx, &posts); err != nil {
		utils.RespondInternalError(c, "failed to decode posts")
		return
	}

	utils.RespondPaginated(c, posts, total, page, size)
}

func (ctrl *PostController) GetStats(c *gin.Context) {
	ctx := context.Background()
	col := utils.GetCollection("posts")

	total, _ := col.CountDocuments(ctx, bson.M{})

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayCount, _ := col.CountDocuments(ctx, bson.M{"updated_at": bson.M{"$gte": todayStart}})

	// Count per platform
	platforms := []string{"weibo", "xhs", "xueqiu"}
	platformCounts := make(map[string]int64)
	for _, p := range platforms {
		cnt, _ := col.CountDocuments(ctx, bson.M{"platform": p})
		platformCounts[p] = cnt
	}

	utils.RespondSuccess(c, gin.H{
		"total":           total,
		"today_count":     todayCount,
		"platform_counts": platformCounts,
	})
}

// ==================== Helpers ====================

func getPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}
