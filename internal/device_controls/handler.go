package devicecontrol

import (
	"net/http"

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

func (h *Handler) GetConfig(c *gin.Context) {
	notImplemented(c, "get device configuration")
}

func (h *Handler) ListCommands(c *gin.Context) {
	notImplemented(c, "list device commands")
}

func (h *Handler) AcknowledgeCommand(c *gin.Context) {
	notImplemented(c, "acknowledge device command")
}

func notImplemented(c *gin.Context, operation string) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": operation + " is not implemented"})
}
