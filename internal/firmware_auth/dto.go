package firmwareauth

import (
	"time"

	"canepanion-server/models"

	"github.com/google/uuid"
)

type ActivateRequest struct {
	DeviceID        string `json:"deviceId" binding:"required,uuid"`
	BatteryLevel    int32  `json:"batteryLevel" binding:"min=0,max=100"`
	FirmwareVersion string `json:"firmwareVersion" binding:"required,max=25"`
}

type ActivateResponse struct {
	DeviceID        uuid.UUID           `json:"deviceId"`
	Status          models.DeviceStatus `json:"status"`
	BatteryLevel    int32               `json:"batteryLevel"`
	FirmwareVersion string              `json:"firmwareVersion"`
	LastSeenAt      time.Time           `json:"lastSeenAt"`
	ServerTime      time.Time           `json:"serverTime"`
	Credential      string              `json:"credential"`
}

type CreateSessionRequest struct {
	DeviceID   string `json:"deviceId" binding:"required,uuid"`
	Credential string `json:"credential" binding:"required"`
}

type CreateSessionResponse struct {
	DeviceID    uuid.UUID `json:"deviceId"`
	AccessToken string    `json:"accessToken"`
	TokenType   string    `json:"tokenType"`
	ExpiresIn   int64     `json:"expiresIn"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type HeartbeatRequest struct{}
type HeartbeatResponse struct{}
