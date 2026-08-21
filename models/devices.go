package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Devices struct {
	ID                   uuid.UUID       `gorm:"primaryKey;type:uuid" json:"id"`
	OwnerUserID          uuid.UUID       `gorm:"column:owner_user_id;type:uuid;not null;index" json:"ownerUserId"`
	GuardianUserID       uuid.UUID       `gorm:"column:guardian_user_id;type:uuid;not null;index" json:"guardianUserId"`
	Name                 string          `gorm:"type:varchar(100);not null" json:"name"`
	Status               DeviceStatus    `gorm:"type:varchar(25);not null;default:ONLINE" json:"status"`
	BatteryLevel         int32           `gorm:"type:integer" json:"batteryLevel"`
	FirmwareVersion      *string         `gorm:"type:varchar(25)" json:"firmwareVersion"`
	HardwareVersion      *string         `gorm:"type:varchar(50);index" json:"hardwareVersion"`
	Configuration        json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"configuration"`
	ConfigurationVersion uint64          `gorm:"not null;default:1" json:"configurationVersion"`
	LastSeenAt           time.Time       `json:"lastSeenAt"`
	CredentialHash       *string         `gorm:"column:credential_hash;type:text" json:"-"`
	CreatedAt            time.Time       `gorm:"autoCreateTime" json:"createdAt"`

	Owner    Users `gorm:"foreignKey:OwnerUserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Guardian Users `gorm:"foreignKey:GuardianUserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

func (d Devices) IsOwnerOrGuardian(userID uuid.UUID) bool {
	return userID == d.OwnerUserID || userID == d.GuardianUserID
}
