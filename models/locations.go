package models

import (
	"time"

	"github.com/google/uuid"
)

type Locations struct {
	ID                 uuid.UUID `gorm:"primaryKey;type:uuid" json:"id"`
	DeviceID           uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_locations_device_external;index" json:"deviceId"`
	ExternalLocationID string    `gorm:"column:external_location_id;type:varchar(100);uniqueIndex:idx_locations_device_external" json:"externalLocationId"`
	Latitude           float64   `gorm:"type:double precision;not null" json:"latitude"`
	Longitude          float64   `gorm:"type:double precision;not null" json:"longitude"`
	AccuracyMeters     float64   `gorm:"type:double precision;not null" json:"accuracyMeters"`
	RecordedAt         time.Time `gorm:"not null;index" json:"recordedAt"`

	Device Devices `gorm:"foreignKey:DeviceID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}
