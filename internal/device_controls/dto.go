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

type AcknowledgeCommandRequest struct{}
type AcknowledgeCommandResponse struct{}
