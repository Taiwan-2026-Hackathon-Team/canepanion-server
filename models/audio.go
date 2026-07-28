package models

import (
	"time"

	"github.com/google/uuid"
)

type Audio struct {
	ID           uuid.UUID      `gorm:"primaryKey;type:uuid" json:"id"`
	DeviceID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"deviceId"`
	Direction    AudioDirection `gorm:"type:varchar(30);not null" json:"direction"`
	StorageKey   string         `gorm:"type:text;not null" json:"storageKey"`
	Transcript   *string        `gorm:"type:text" json:"transcript"`
	ResponseText *string        `gorm:"type:text" json:"responseText"`
	Status       AudioStatus    `gorm:"type:varchar(20);not null;default:UPLOADED" json:"status"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"createdAt"`

	Device Devices `gorm:"foreignKey:DeviceID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}
