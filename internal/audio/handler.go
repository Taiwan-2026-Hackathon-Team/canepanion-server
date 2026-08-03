package audio

import (
	"net/http"

	appErr "canepanion-server/pkg/errors"
	http_helper "canepanion-server/pkg/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(db *gorm.DB, voiceJob *VoiceJob) *Handler {
	repo := NewRepository(db)
	if voiceJob == nil {
		voiceJob = &VoiceJob{}
	}
	voiceJob.bind(repo)
	service := NewService(repo, voiceJob)
	return &Handler{service: service}
}

func (h *Handler) CreateUpload(c *gin.Context) {
	req, err := http_helper.BindFormJSON[CreateUploadRequest](c, "metadata")
	if err != nil {
		_ = c.Error(err)
		return
	}

	file, fileHeader, err := c.Request.FormFile("audio")
	if err != nil {
		_ = c.Error(appErr.NewBadRequest("Audio file is required", err))
		return
	}
	defer file.Close()

	response, err := h.service.CreateUpload(c.Request.Context(), c.Param("deviceId"), req, file, fileHeader)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

func (h *Handler) CompleteUpload(c *gin.Context) {
	response, err := h.service.CompleteUpload(c.Param("deviceId"), c.Param("audioId"))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) GetAudio(c *gin.Context) {
	response, err := h.service.GetAudio(c.Param("deviceId"), c.Param("audioId"))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}
