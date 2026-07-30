package audio

import (
	"time"

	"canepanion-server/models"

	"github.com/google/uuid"
)

type CreateUploadRequest struct {
	Direction models.AudioDirection `json:"direction"`
}

type CreateUploadResponse struct {
	AudioID    uuid.UUID             `json:"audioId"`
	DeviceID   uuid.UUID             `json:"deviceId"`
	Direction  models.AudioDirection `json:"direction"`
	AudioURL   string                `json:"audioUrl"`
	StorageKey string                `json:"storageKey"`
	Status     models.AudioStatus    `json:"status"`
	CreatedAt  time.Time             `json:"createdAt"`
}

type CompleteUploadRequest struct{}
type CompleteUploadResponse struct{}
