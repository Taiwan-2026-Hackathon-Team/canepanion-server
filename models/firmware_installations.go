package models

import (
	"time"

	"github.com/google/uuid"
)

type FirmwareInstallations struct {
	ID           uuid.UUID                  `gorm:"primaryKey;type:uuid" json:"id"`
	DeviceID     uuid.UUID                  `gorm:"type:uuid;not null;uniqueIndex:idx_firmware_installation_device_message;index:idx_firmware_installation_timeline,priority:1" json:"deviceId"`
	ReleaseID    uuid.UUID                  `gorm:"type:uuid;not null;index" json:"releaseId"`
	MessageID    string                     `gorm:"type:varchar(100);not null;uniqueIndex:idx_firmware_installation_device_message" json:"messageId"`
	Status       FirmwareInstallationStatus `gorm:"type:varchar(25);not null;index:idx_firmware_installation_timeline,priority:2" json:"status"`
	ErrorCode    *string                    `gorm:"type:varchar(50)" json:"errorCode,omitempty"`
	ErrorMessage *string                    `gorm:"type:varchar(500)" json:"errorMessage,omitempty"`
	ReportedAt   time.Time                  `gorm:"not null;index:idx_firmware_installation_timeline,priority:3" json:"reportedAt"`
	CreatedAt    time.Time                  `gorm:"autoCreateTime" json:"createdAt"`

	Device  Devices          `gorm:"foreignKey:DeviceID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Release FirmwareReleases `gorm:"foreignKey:ReleaseID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
}
