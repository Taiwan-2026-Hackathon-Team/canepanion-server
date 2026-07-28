package models

import (
	"time"

	"github.com/google/uuid"
)

type Alerts struct {
	ID            uuid.UUID   `gorm:"primaryKey;type:uuid" json:"id"`
	DeviceID      uuid.UUID   `gorm:"type:uuid;not null;index" json:"deviceId"`
	SensorEventID uuid.UUID   `gorm:"type:uuid;not null;index" json:"sensorEventId"`
	AlertType     AlertType   `gorm:"type:varchar(25);not null" json:"alertType"`
	Message       string      `gorm:"type:text;not null" json:"message"`
	Status        AlertStatus `gorm:"type:varchar(20);not null;default:ACTIVE" json:"status"`
	CreatedAt     time.Time   `gorm:"autoCreateTime" json:"createdAt"`
	ResolvedAt    *time.Time  `json:"resolvedAt"`

	Device      Devices      `gorm:"foreignKey:DeviceID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	SensorEvent SensorEvents `gorm:"foreignKey:SensorEventID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}
