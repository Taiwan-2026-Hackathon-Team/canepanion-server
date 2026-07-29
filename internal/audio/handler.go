package audio

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

func (h *Handler) CreateUpload(c *gin.Context) {
	notImplemented(c, "create audio upload")
}

func (h *Handler) CompleteUpload(c *gin.Context) {
	notImplemented(c, "complete audio upload")
}

func notImplemented(c *gin.Context, operation string) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": operation + " is not implemented"})
}
