package firmwareupdates

import (
	"time"

	"canepanion-server/models"

	"github.com/google/uuid"
)

type GetLatestResponse struct {
	ReleaseID       uuid.UUID `json:"releaseId"`
	Version         string    `json:"version"`
	HardwareVersion string    `json:"hardwareVersion"`
	DownloadURL     string    `json:"downloadUrl"`
	FileSizeBytes   int64     `json:"fileSizeBytes"`
	SHA256          string    `json:"sha256"`
	Signature       string    `json:"signature"`
	Mandatory       bool      `json:"mandatory"`
	PublishedAt     time.Time `json:"publishedAt"`
}

type ReportError struct {
	Code    string `json:"code" binding:"required,max=50"`
	Message string `json:"message" binding:"required,max=500"`
}

type ReportRequest struct {
	MessageID  string                            `json:"messageId" binding:"required,max=100"`
	ReleaseID  string                            `json:"releaseId" binding:"required,uuid"`
	Status     models.FirmwareInstallationStatus `json:"status" binding:"required"`
	ReportedAt time.Time                         `json:"reportedAt" binding:"required"`
	Error      *ReportError                      `json:"error,omitempty"`
}

type ReportResponse struct {
	InstallationID uuid.UUID                         `json:"installationId"`
	MessageID      string                            `json:"messageId"`
	ReleaseID      uuid.UUID                         `json:"releaseId"`
	Status         models.FirmwareInstallationStatus `json:"status"`
	ReportedAt     time.Time                         `json:"reportedAt"`
	Duplicate      bool                              `json:"duplicate"`
	ServerTime     time.Time                         `json:"serverTime"`
}
