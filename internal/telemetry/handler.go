package telemetry

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

func (h *Handler) Submit(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "submit telemetry is not implemented"})
}
