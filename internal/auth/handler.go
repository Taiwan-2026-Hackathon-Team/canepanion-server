package auth

import (
	appErr "canepanion-server/pkg/errors"
	http_helper "canepanion-server/pkg/http"
	"canepanion-server/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(db *gorm.DB) *Handler {
	repo := NewRepository(db)
	service := NewService(repo)
	return &Handler{service: service}
}

func (h *Handler) SignUp(c *gin.Context) {
	req, err := http_helper.BindJSON[SignUpRequest](c)
	if err != nil {
		c.Error(err)
		return
	}

	user, token, err := h.service.SignUp(*req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SetCookie(c, token, 3600*5)

	c.JSON(200, gin.H{
		"message": "Signed In Successfully",
		"user":    user,
	})
}

func (h *Handler) LogIn(c *gin.Context) {
	req, err := http_helper.BindJSON[SignInRequest](c)
	if err != nil {
		c.Error(err)
		return
	}

	user, token, err := h.service.LogIn(*req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SetCookie(c, token, 3600*5)

	c.JSON(200, gin.H{
		"message": "Signed In Successfully",
		"user":    user,
	})
}

func (h *Handler) LogOut(c *gin.Context) {
	utils.ClearCookie(c)

	c.JSON(200, gin.H{
		"message": "You have signed out successfully",
	})
}

func (h *Handler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.Error(appErr.NewUnauthorized("Missing authenticated user", nil))
		return
	}

	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		c.Error(appErr.NewUnauthorized("Invalid authenticated user", nil))
		return
	}

	user, err := h.service.GetCurrentUser(userIDStr)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(200, user)
}
