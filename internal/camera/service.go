package camera

import (
	"context"
	"errors"

	"canepanion-server/models"
	appErr "canepanion-server/pkg/errors"
	"canepanion-server/pkg/utils"

	"github.com/google/uuid"
)

// deviceLookup is implemented by the existing internal/device_controls
// Repository, so camera authorization reuses the one device query the rest
// of the server already has.
type deviceLookup interface {
	FindDeviceByID(deviceID uuid.UUID) (*models.Devices, error)
}

type Service struct {
	devices deviceLookup
	hub     *Hub
}

func NewService(devices deviceLookup, hub *Hub) *Service {
	return &Service{devices: devices, hub: hub}
}

// CameraStatusResponse is the GET /camera body. State mirrors the session
// state machine the hub owns: OFFLINE, WAITING, or LIVE.
type CameraStatusResponse struct {
	State       sessionState `json:"state"`
	ViewerCount int          `json:"viewerCount"`
}

// Publish accepts a firmware WHIP offer. DeviceAuthMiddleware has already
// bound the bearer claim to deviceIDValue, so no additional device lookup
// happens here.
func (s *Service) Publish(ctx context.Context, deviceIDValue string, offer publisherOffer) (publication, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return publication{}, appErr.NewBadRequest("Invalid device ID", err)
	}

	pub, err := s.hub.publish(ctx, deviceID, offer)
	if err != nil {
		return publication{}, mapHubError(err)
	}
	return pub, nil
}

// View accepts a guardian-app WHEP offer once the caller is confirmed as the
// device's owner or guardian.
func (s *Service) View(ctx context.Context, userIDValue, deviceIDValue string, offer viewerOffer) (viewerHandle, error) {
	userID, deviceID, err := s.authorize(userIDValue, deviceIDValue)
	if err != nil {
		return viewerHandle{}, err
	}

	handle, err := s.hub.view(ctx, deviceID, userID, offer)
	if err != nil {
		return viewerHandle{}, mapHubError(err)
	}
	return handle, nil
}

// StopPublication tears down a publication. It performs no ownership check
// beyond DeviceAuthMiddleware's path binding, matching the idempotent DELETE
// contract: a stale or missing publicationID still succeeds.
func (s *Service) StopPublication(ctx context.Context, deviceIDValue, publicationIDValue string) error {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return appErr.NewBadRequest("Invalid device ID", err)
	}
	publicationID, err := utils.ParseId(publicationIDValue)
	if err != nil {
		return appErr.NewBadRequest("Invalid publication ID", err)
	}

	if err := s.hub.stopPublication(ctx, deviceID, publicationID); err != nil {
		return mapHubError(err)
	}
	return nil
}

// StopViewer tears down a viewer once the caller is confirmed as the
// device's owner or guardian. The hub additionally requires the viewer to
// have been created by this same principal.
func (s *Service) StopViewer(ctx context.Context, userIDValue, deviceIDValue, viewerIDValue string) error {
	userID, deviceID, err := s.authorize(userIDValue, deviceIDValue)
	if err != nil {
		return err
	}
	viewerID, err := utils.ParseId(viewerIDValue)
	if err != nil {
		return appErr.NewBadRequest("Invalid viewer ID", err)
	}

	if err := s.hub.stopViewer(ctx, deviceID, userID, viewerID); err != nil {
		return mapHubError(err)
	}
	return nil
}

// Status answers GET /camera once the caller is confirmed as the device's
// owner or guardian.
func (s *Service) Status(userIDValue, deviceIDValue string) (*CameraStatusResponse, error) {
	_, deviceID, err := s.authorize(userIDValue, deviceIDValue)
	if err != nil {
		return nil, err
	}

	status := s.hub.status(deviceID)
	return &CameraStatusResponse{State: status.state, ViewerCount: status.viewerCount}, nil
}

// authorize is the one place View, StopViewer, and Status parse identities
// and enforce IsOwnerOrGuardian, so the rule cannot drift between them.
func (s *Service) authorize(userIDValue, deviceIDValue string) (uuid.UUID, uuid.UUID, error) {
	userID, err := utils.ParseId(userIDValue)
	if err != nil {
		return uuid.Nil, uuid.Nil, appErr.NewUnauthorized("Invalid user ID in token", err)
	}
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return uuid.Nil, uuid.Nil, appErr.NewBadRequest("Invalid device ID", err)
	}

	device, err := s.devices.FindDeviceByID(deviceID)
	if err != nil {
		return uuid.Nil, uuid.Nil, appErr.NewInternal("Failed to find device", err)
	}
	if device == nil {
		return uuid.Nil, uuid.Nil, appErr.NewNotFound("Device not found", nil)
	}
	if !device.IsOwnerOrGuardian(userID) {
		return uuid.Nil, uuid.Nil, appErr.NewForbidden("User cannot access this device's camera", nil)
	}
	return userID, deviceID, nil
}

func mapHubError(err error) error {
	switch {
	case errors.Is(err, errHubClosed):
		return appErr.NewInternal("Camera relay is shutting down", err)
	case errors.Is(err, errNegotiationTime):
		return appErr.NewGatewayTimeout("Camera negotiation timed out", err)
	case errors.Is(err, errTooManyViewers):
		return appErr.NewTooManyRequests("Device already has the maximum number of viewers", err)
	case errors.Is(err, errNotViewerOwner):
		return appErr.NewForbidden("Viewer belongs to another user", err)
	case errors.Is(err, errSessionClosed):
		return appErr.NewInternal("Camera session churned; please retry", err)
	default:
		return appErr.NewInternal("Camera negotiation failed", err)
	}
}
