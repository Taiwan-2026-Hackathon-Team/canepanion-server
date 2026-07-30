package firmwareauth

import (
	"strings"
	"time"

	"canepanion-server/models"
	appErr "canepanion-server/pkg/errors"
	"canepanion-server/pkg/utils"
)

const (
	maxHeartbeatClockSkew = 5 * time.Minute
	nextHeartbeatSeconds  = 60
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ActivateDevice(req *ActivateRequest) (*ActivateResponse, error) {
	deviceID, err := utils.ParseId(req.DeviceID)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}

	device, err := s.repo.FindDeviceByID(deviceID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find device", err)
	}
	if device == nil {
		return nil, appErr.NewNotFound("Device not found", nil)
	}

	now := time.Now().UTC()
	firmwareVersion := req.FirmwareVersion
	device.Status = models.DeviceStatusOnline
	credential, err := utils.GenerateDeviceCredential()
	if err != nil {
		return nil, appErr.NewInternal("Failed to generate device credential", err)
	}
	credentialHash, err := utils.HashPassword(credential)
	if err != nil {
		return nil, appErr.NewInternal("Failed to secure device credential", err)
	}

	device.BatteryLevel = req.BatteryLevel
	device.FirmwareVersion = &firmwareVersion
	device.CredentialHash = &credentialHash
	device.LastSeenAt = now

	if err := s.repo.UpdateActivation(device); err != nil {
		return nil, appErr.NewInternal("Failed to activate device", err)
	}

	return &ActivateResponse{
		DeviceID:        device.ID,
		Status:          device.Status,
		BatteryLevel:    device.BatteryLevel,
		FirmwareVersion: firmwareVersion,
		LastSeenAt:      device.LastSeenAt,
		ServerTime:      now,
		Credential:      credential,
	}, nil
}

func (s *Service) CreateSession(req *CreateSessionRequest) (*CreateSessionResponse, error) {
	deviceID, err := utils.ParseId(req.DeviceID)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}

	device, err := s.repo.FindDeviceByID(deviceID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find device", err)
	}
	if device == nil || device.CredentialHash == nil {
		return nil, appErr.NewUnauthorized("Invalid device credentials", nil)
	}

	if err := utils.ValidatePassword(*device.CredentialHash, req.Credential); err != nil {
		return nil, appErr.NewUnauthorized("Invalid device credentials", nil)
	}

	accessToken, expiresAt, err := utils.GenerateDeviceToken(device.ID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to generate device token", err)
	}

	return &CreateSessionResponse{
		DeviceID:    device.ID,
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(utils.DeviceTokenTTL.Seconds()),
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *Service) RecordHeartbeat(deviceIDValue string, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}

	if strings.TrimSpace(req.MessageID) == "" {
		return nil, appErr.NewBadRequest("Invalid messageId", nil)
	}
	if req.RecordedAt.IsZero() {
		return nil, appErr.NewBadRequest("recordedAt is required and must be an RFC3339 timestamp", nil)
	}
	if req.BatteryLevel == nil || *req.BatteryLevel < 0 || *req.BatteryLevel > 100 {
		return nil, appErr.NewBadRequest("batteryLevel must be between 0 and 100", nil)
	}

	firmwareVersion := strings.TrimSpace(req.FirmwareVersion)
	if firmwareVersion == "" || len(firmwareVersion) > 25 {
		return nil, appErr.NewBadRequest("firmwareVersion is required and must not exceed 25 characters", nil)
	}
	if req.Status != models.DeviceStatusOnline {
		return nil, appErr.NewBadRequest("heartbeat status must be ONLINE", nil)
	}

	now := time.Now().UTC().Truncate(time.Second)
	if req.RecordedAt.After(now.Add(maxHeartbeatClockSkew)) {
		return nil, appErr.NewBadRequest("recordedAt cannot be more than 5 minutes in the future", nil)
	}

	updated, err := s.repo.UpdateHeartbeat(deviceID, *req.BatteryLevel, firmwareVersion, now)
	if err != nil {
		return nil, appErr.NewInternal("Failed to record device heartbeat", err)
	}
	if !updated {
		return nil, appErr.NewNotFound("Device not found", nil)
	}

	return &HeartbeatResponse{
		ServerTime:           now,
		NextHeartbeatSeconds: nextHeartbeatSeconds,
	}, nil
}
