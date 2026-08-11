package models

import (
	"time"

	"github.com/google/uuid"
)

type FirmwareReleases struct {
	ID              uuid.UUID `gorm:"primaryKey;type:uuid" json:"id"`
	Version         string    `gorm:"type:varchar(25);not null;uniqueIndex:idx_firmware_release_hardware_version,priority:2" json:"version"`
	HardwareVersion string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_firmware_release_hardware_version,priority:1;index" json:"hardwareVersion"`
	DownloadURL     string    `gorm:"type:text;not null" json:"downloadUrl"`
	FileSizeBytes   int64     `gorm:"not null" json:"fileSizeBytes"`
	SHA256          string    `gorm:"column:sha256;type:char(64);not null" json:"sha256"`
	Signature       string    `gorm:"type:text;not null" json:"signature"`
	Mandatory       bool      `gorm:"not null;default:false" json:"mandatory"`
	PublishedAt     time.Time `gorm:"not null;index" json:"publishedAt"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}
