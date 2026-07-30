package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SensorEvents struct {
	ID              uuid.UUID       `gorm:"primaryKey;type:uuid" json:"id"`
	DeviceID        uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:idx_sensor_events_device_external;index" json:"deviceId"`
	ExternalEventID string          `gorm:"column:external_event_id;type:varchar(100);uniqueIndex:idx_sensor_events_device_external" json:"externalEventId"`
	LocationID      *uuid.UUID      `gorm:"type:uuid;index" json:"locationId"`
	EventType       SensorEventType `gorm:"type:varchar(30);not null" json:"eventType"`
	Severity        EventSeverity   `gorm:"type:varchar(20);not null" json:"severity"`
	EventData       json.RawMessage `gorm:"type:jsonb;not null" json:"eventData"`
	RecordedAt      time.Time       `gorm:"not null;index" json:"recordedAt"`

	Device   Devices    `gorm:"foreignKey:DeviceID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Location *Locations `gorm:"foreignKey:LocationID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
}
