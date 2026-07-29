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
	device.BatteryLevel = req.BatteryLevel
	device.FirmwareVersion = &firmwareVersion
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
	}, nil
}
