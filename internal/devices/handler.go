package devices

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

func (h *Handler) AddDevice(c *gin.Context) {
	req, err := http_helper.BindJSON[CreateDeviceRequest](c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	ownerUserID, err := http_helper.ExtractUserIDFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response, err := h.service.AddDevice(ownerUserID, req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, response)
}
