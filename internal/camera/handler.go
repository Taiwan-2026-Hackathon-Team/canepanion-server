package camera

import (
	"errors"
	"mime"
	"net/http"
	"strings"

	appErr "canepanion-server/pkg/errors"
	http_helper "canepanion-server/pkg/http"

	"github.com/gin-gonic/gin"
)

const sdpMediaType = "application/sdp"

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreatePublication(c *gin.Context) {
	if err := checkSDPRequest(c); err != nil {
		_ = c.Error(err)
		return
	}

	offer, err := decodePublisherOffer(c.Request.Body)
	if err != nil {
		_ = c.Error(mapSDPError(err))
		return
	}

	pub, err := h.service.Publish(c.Request.Context(), c.Param("deviceId"), offer)
	if err != nil {
		_ = c.Error(err)
		return
	}

	location := "/api/v1/firmware/devices/" + c.Param("deviceId") + "/camera/publications/" + pub.id.String()
	writeSDP(c, http.StatusCreated, location, pub.answer)
}

func (h *Handler) CreateViewer(c *gin.Context) {
	if err := checkSDPRequest(c); err != nil {
		_ = c.Error(err)
		return
	}

	userID, err := http_helper.ExtractUserIDFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	offer, err := decodeViewerOffer(c.Request.Body)
	if err != nil {
		_ = c.Error(mapSDPError(err))
		return
	}

	view, err := h.service.View(c.Request.Context(), userID, c.Param("deviceId"), offer)
	if err != nil {
		_ = c.Error(err)
		return
	}

	location := "/api/v1/devices/" + c.Param("deviceId") + "/camera/viewers/" + view.id.String()
	writeSDP(c, http.StatusCreated, location, view.answer)
}

func (h *Handler) DeletePublication(c *gin.Context) {
	err := h.service.StopPublication(c.Request.Context(), c.Param("deviceId"), c.Param("publicationId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) DeleteViewer(c *gin.Context) {
	userID, err := http_helper.ExtractUserIDFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.service.StopViewer(c.Request.Context(), userID, c.Param("deviceId"), c.Param("viewerId")); err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) GetStatus(c *gin.Context) {
	userID, err := http_helper.ExtractUserIDFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response, err := h.service.Status(userID, c.Param("deviceId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// RejectPatch answers the WHIP/WHEP publication and viewer resources: this
// profile has no trickle ICE restart, so clients must POST a new offer.
func (h *Handler) RejectPatch(c *gin.Context) {
	c.Header("Allow", "DELETE")
	_ = c.Error(appErr.NewMethodNotAllowed("PATCH is not supported; POST a new offer to restart negotiation", nil))
}

func writeSDP(c *gin.Context, status int, location string, answer localAnswer) {
	c.Header("Location", location)
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(status, sdpMediaType, []byte(answer.sdp))
}

func checkSDPRequest(c *gin.Context) error {
	if err := requireSDPContentType(c); err != nil {
		return err
	}
	return requireSDPAccept(c)
}

func requireSDPContentType(c *gin.Context) error {
	mediaType, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != sdpMediaType {
		return appErr.NewUnsupportedMediaType("Content-Type must be application/sdp", err)
	}
	for key := range params {
		if key != "charset" {
			return appErr.NewUnsupportedMediaType("Content-Type must be application/sdp", nil)
		}
	}
	return nil
}

func requireSDPAccept(c *gin.Context) error {
	accept := c.GetHeader("Accept")
	if accept == "" {
		return nil
	}
	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if part == sdpMediaType || part == "*/*" {
			return nil
		}
	}
	return appErr.NewNotAcceptable("Accept must include application/sdp", nil)
}

func mapSDPError(err error) error {
	switch {
	case errors.Is(err, errSDPTooLarge):
		return appErr.NewContentTooLarge("Camera SDP exceeds the size limit", err)
	case errors.Is(err, errSDPSyntax):
		return appErr.NewBadRequest("Camera SDP is not valid", err)
	case errors.Is(err, errUnsupportedSDP):
		return appErr.NewUnprocessableEntity("Unsupported camera SDP", err)
	default:
		return appErr.NewBadRequest("Unable to read camera SDP", err)
	}
}
