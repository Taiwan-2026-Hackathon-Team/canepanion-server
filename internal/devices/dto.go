package devices

import (
	"time"

	"canepanion-server/models"

	"github.com/google/uuid"
)

type CreateDeviceRequest struct {
	GuardianUserID string `json:"guardianUserId" binding:"required,uuid"`
	Name           string `json:"name" binding:"required,max=100"`
}

type CreateDeviceResponse struct {
	ID              uuid.UUID           `json:"id"`
	OwnerUserID     uuid.UUID           `json:"ownerUserId"`
	GuardianUserID  uuid.UUID           `json:"guardianUserId"`
	Name            string              `json:"name"`
	Status          models.DeviceStatus `json:"status"`
	BatteryLevel    int32               `json:"batteryLevel"`
	FirmwareVersion *string             `json:"firmwareVersion"`
	LastSeenAt      time.Time           `json:"lastSeenAt"`
	CreatedAt       time.Time           `json:"createdAt"`
}

func newCreateDeviceResponse(device *models.Devices) *CreateDeviceResponse {
	return &CreateDeviceResponse{
		ID:              device.ID,
		OwnerUserID:     device.OwnerUserID,
		GuardianUserID:  device.GuardianUserID,
		Name:            device.Name,
		Status:          device.Status,
		BatteryLevel:    device.BatteryLevel,
		FirmwareVersion: device.FirmwareVersion,
		LastSeenAt:      device.LastSeenAt,
		CreatedAt:       device.CreatedAt,
	}
}
