package firmwareupdates

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

func (h *Handler) GetLatest(c *gin.Context) {
	notImplemented(c, "get latest firmware")
}

func (h *Handler) Report(c *gin.Context) {
	notImplemented(c, "report firmware installation")
}

func notImplemented(c *gin.Context, operation string) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": operation + " is not implemented"})
}
