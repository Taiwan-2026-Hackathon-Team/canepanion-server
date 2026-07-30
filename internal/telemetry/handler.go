package telemetry

import (
	"net/http"

	"canepanion-server/internal/push"
	http_helper "canepanion-server/pkg/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(db *gorm.DB, notifier push.Notifier) *Handler {
	repo := NewRepository(db)
	service := NewService(repo, notifier)
	return &Handler{service: service}
}

func (h *Handler) Submit(c *gin.Context) {
	req, err := http_helper.BindJSON[SubmitTelemetryRequest](c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response, err := h.service.SubmitTelemetry(c.Param("deviceId"), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	status := http.StatusOK
	if response.HasRejectedItems() {
		status = http.StatusMultiStatus
	}
	c.JSON(status, response)
}
