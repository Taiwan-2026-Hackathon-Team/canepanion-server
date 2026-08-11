package devicecontrol

import (
	"encoding/json"
	"time"

	"canepanion-server/models"

	"github.com/google/uuid"
)

type DeviceConfiguration struct {
	DeviceID      uuid.UUID
	Version       uint64
	Configuration json.RawMessage
}

type GetConfigResponse struct {
	DeviceID      uuid.UUID       `json:"deviceId"`
	Version       uint64          `json:"version"`
	Configuration json.RawMessage `json:"configuration"`
}

type ListCommandsQuery struct {
	Cursor string `form:"cursor" binding:"omitempty"`
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=100"`
}

type DeviceCommandResponse struct {
	CommandID uuid.UUID                `json:"commandId"`
	Type      models.DeviceCommandType `json:"type"`
	Payload   json.RawMessage          `json:"payload"`
	CreatedAt time.Time                `json:"createdAt"`
	ExpiresAt time.Time                `json:"expiresAt"`
}

type ListCommandsResponse struct {
	Commands   []DeviceCommandResponse `json:"commands"`
	NextCursor string                  `json:"nextCursor,omitempty"`
}

type CreateCommandRequest struct {
	Type      models.DeviceCommandType `json:"type" binding:"required"`
	Payload   json.RawMessage          `json:"payload"`
	ExpiresAt time.Time                `json:"expiresAt" binding:"required"`
}

type CreateCommandResponse struct {
	CommandID uuid.UUID                  `json:"commandId"`
	DeviceID  uuid.UUID                  `json:"deviceId"`
	Type      models.DeviceCommandType   `json:"type"`
	Payload   json.RawMessage            `json:"payload"`
	Status    models.DeviceCommandStatus `json:"status"`
	ExpiresAt time.Time                  `json:"expiresAt"`
	CreatedAt time.Time                  `json:"createdAt"`
}

type TrackCommandRequest struct {
	Status models.DeviceCommandStatus `json:"status" binding:"required"`
}

type TrackCommandResponse struct {
	CommandID uuid.UUID                  `json:"commandId"`
	Status    models.DeviceCommandStatus `json:"status"`
	UpdatedAt time.Time                  `json:"updatedAt"`
}
