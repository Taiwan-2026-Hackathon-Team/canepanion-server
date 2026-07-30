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

func NewHandler(db *gorm.DB) *Handler {
	repo := NewRepository(db)
	service := NewService(repo)
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
	notImplemented(c, "complete audio upload")
}

func notImplemented(c *gin.Context, operation string) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": operation + " is not implemented"})
}
