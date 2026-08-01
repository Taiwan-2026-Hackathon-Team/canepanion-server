package devicecontrol

import (
	"encoding/json"

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

type ListCommandsQuery struct{}
type ListCommandsResponse struct{}

type AcknowledgeCommandRequest struct{}
type AcknowledgeCommandResponse struct{}
