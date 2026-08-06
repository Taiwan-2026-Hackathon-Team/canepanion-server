package devicecontrol

import (
	"net/http"
	"strings"

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

func (h *Handler) GetConfig(c *gin.Context) {
	response, etag, err := h.service.GetDeviceConfig(c.Param("deviceId"))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Header("ETag", etag)
	c.Header("Cache-Control", "no-cache")
	if matchesETag(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}

	c.JSON(http.StatusOK, response)
}

func matchesETag(ifNoneMatch, currentETag string) bool {
	for candidate := range strings.SplitSeq(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return true
		}
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == currentETag {
			return true
		}
	}
	return false
}

func (h *Handler) ListCommands(c *gin.Context) {
	query, err := http_helper.BindQuery[ListCommandsQuery](c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response, err := h.service.ListCommands(c.Param("deviceId"), query)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) AcknowledgeCommand(c *gin.Context) {
	notImplemented(c, "acknowledge device command")
}

func notImplemented(c *gin.Context, operation string) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": operation + " is not implemented"})
}
