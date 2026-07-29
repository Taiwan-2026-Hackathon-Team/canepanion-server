package firmwareauth

import (
	"time"

	"canepanion-server/models"
	appErr "canepanion-server/pkg/errors"
	"canepanion-server/pkg/utils"
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
