package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SensorEvents struct {
	ID         uuid.UUID       `gorm:"primaryKey;type:uuid" json:"id"`
	DeviceID   uuid.UUID       `gorm:"type:uuid;not null;index" json:"deviceId"`
	EventType  SensorEventType `gorm:"type:varchar(30);not null" json:"eventType"`
	Severity   EventSeverity   `gorm:"type:varchar(20);not null" json:"severity"`
	EventData  json.RawMessage `gorm:"type:jsonb;not null" json:"eventData"`
	RecordedAt time.Time       `gorm:"not null;index" json:"recordedAt"`

	Device Devices `gorm:"foreignKey:DeviceID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}
