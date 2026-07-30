package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type IngestionBatches struct {
	ID        uuid.UUID       `gorm:"primaryKey;type:uuid" json:"id"`
	DeviceID  uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:idx_ingestion_batches_device_message;index" json:"deviceId"`
	MessageID string          `gorm:"column:message_id;type:varchar(100);not null;uniqueIndex:idx_ingestion_batches_device_message" json:"messageId"`
	Response  json.RawMessage `gorm:"type:jsonb;not null" json:"response"`
	CreatedAt time.Time       `gorm:"autoCreateTime" json:"createdAt"`

	Device Devices `gorm:"foreignKey:DeviceID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}
