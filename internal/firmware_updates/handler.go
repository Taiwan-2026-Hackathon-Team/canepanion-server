package firmwareupdates

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

func (h *Handler) GetLatest(c *gin.Context) {
	response, err := h.service.GetLatest(c.Param("deviceId"))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) ReportFirmwareUpdate(c *gin.Context) {
	req, err := http_helper.BindJSON[ReportRequest](c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response, err := h.service.ReportFirmwareUpdate(c.Param("deviceId"), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}
