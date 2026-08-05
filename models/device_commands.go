package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type DeviceCommands struct {
	ID        uuid.UUID           `gorm:"primaryKey;type:uuid" json:"id"`
	DeviceID  uuid.UUID           `gorm:"type:uuid;not null;index:idx_device_commands_pending,priority:1" json:"deviceId"`
	Type      DeviceCommandType   `gorm:"type:varchar(50);not null" json:"type"`
	Payload   json.RawMessage     `gorm:"type:jsonb;not null;default:'{}'" json:"payload"`
	Status    DeviceCommandStatus `gorm:"type:varchar(25);not null;default:PENDING;index:idx_device_commands_pending,priority:2" json:"status"`
	ExpiresAt time.Time           `gorm:"not null;index:idx_device_commands_pending,priority:3" json:"expiresAt"`
	CreatedAt time.Time           `gorm:"autoCreateTime;index:idx_device_commands_pending,priority:4" json:"createdAt"`
	UpdatedAt time.Time           `gorm:"autoUpdateTime" json:"updatedAt"`

	Device Devices `gorm:"foreignKey:DeviceID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}
