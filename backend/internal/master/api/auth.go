package api

import (
	"context"
	"time"

	"github.com/crawlab-team/spider-lab/internal/models"
	"github.com/crawlab-team/spider-lab/internal/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuthController struct{}

func NewAuthController() *AuthController {
	return &AuthController{}
}

func (ctrl *AuthController) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "invalid request body")
		return
	}

	col := utils.GetCollection("users")
	var user models.User
	err := col.FindOne(context.Background(), bson.M{"username": req.Username}).Decode(&user)
	if err != nil {
		utils.RespondUnauthorized(c, "invalid username or password")
		return
	}

	if !utils.CheckPassword(req.Password, user.Password) {
		utils.RespondUnauthorized(c, "invalid username or password")
		return
	}

	token, expiresAt, err := utils.GenerateToken(user.Id.Hex(), user.Username, user.Role)
	if err != nil {
		utils.RespondInternalError(c, "failed to generate token")
		return
	}

	utils.RespondSuccess(c, models.TokenResponse{Token: token, ExpiresAt: expiresAt})
}

func (ctrl *AuthController) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "invalid request body")
		return
	}

	col := utils.GetCollection("users")

	var existing models.User
	err := col.FindOne(context.Background(), bson.M{"username": req.Username}).Decode(&existing)
	if err == nil {
		utils.RespondBadRequest(c, "username already exists")
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		utils.RespondInternalError(c, "failed to hash password")
		return
	}

	now := time.Now()
	user := models.User{
		Username:  req.Username,
		Password:  hashedPassword,
		Email:     req.Email,
		Role:      models.RoleNormal,
		CreatedAt: now,
		UpdatedAt: now,
	}

	result, err := col.InsertOne(context.Background(), user)
	if err != nil {
		utils.RespondInternalError(c, "failed to create user")
		return
	}

	insertedId := result.InsertedID.(primitive.ObjectID)
	token, expiresAt, err := utils.GenerateToken(insertedId.Hex(), user.Username, user.Role)
	if err != nil {
		utils.RespondInternalError(c, "failed to generate token")
		return
	}

	utils.RespondCreated(c, models.TokenResponse{Token: token, ExpiresAt: expiresAt})
}
