package models

import (
	"time"

	"github.com/google/uuid"
)

type Devices struct {
	ID              uuid.UUID    `gorm:"primaryKey;type:uuid" json:"id"`
	OwnerUserID     uuid.UUID    `gorm:"column:owner_user_id;type:uuid;not null;index" json:"ownerUserId"`
	GuardianUserID  uuid.UUID    `gorm:"column:guardian_user_id;type:uuid;not null;index" json:"guardianUserId"`
	Name            string       `gorm:"type:varchar(100);not null" json:"name"`
	Status          DeviceStatus `gorm:"type:varchar(25);not null;default:ONLINE" json:"status"`
	BatteryLevel    int32        `gorm:"type:integer" json:"batteryLevel"`
	FirmwareVersion *string      `gorm:"type:varchar(25)" json:"firmwareVersion"`
	LastSeenAt      time.Time    `json:"lastSeenAt"`
	CreatedAt       time.Time    `gorm:"autoCreateTime" json:"createdAt"`

	Owner    Users `gorm:"foreignKey:OwnerUserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Guardian Users `gorm:"foreignKey:GuardianUserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}
