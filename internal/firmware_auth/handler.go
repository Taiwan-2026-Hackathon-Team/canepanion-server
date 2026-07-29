package firmwareauth

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

func (h *Handler) Activate(c *gin.Context) {
	notImplemented(c, "activate device")
}

func (h *Handler) CreateSession(c *gin.Context) {
	notImplemented(c, "create firmware session")
}

func (h *Handler) Heartbeat(c *gin.Context) {
	notImplemented(c, "record device heartbeat")
}

func notImplemented(c *gin.Context, operation string) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": operation + " is not implemented"})
}
