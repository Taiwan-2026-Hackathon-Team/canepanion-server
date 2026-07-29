package firmwareauth

import (
	"net/http"

	http_helper "canepanion-server/pkg/http"

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

func (h *Handler) ActivateDevice(c *gin.Context) {
	req, err := http_helper.BindJSON[ActivateRequest](c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response, err := h.service.ActivateDevice(req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) CreateSession(c *gin.Context) {
	req, err := http_helper.BindJSON[CreateSessionRequest](c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response, err := h.service.CreateSession(req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) Heartbeat(c *gin.Context) {
	notImplemented(c, "record device heartbeat")
}

func notImplemented(c *gin.Context, operation string) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": operation + " is not implemented"})
}
