package firmwareupdates

import (
	"strings"
	"time"

	"canepanion-server/models"
	appErr "canepanion-server/pkg/errors"
	"canepanion-server/pkg/utils"

	"github.com/google/uuid"
)

const maxFirmwareReportClockSkew = 5 * time.Minute

type repository interface {
	FindDeviceByID(deviceID uuid.UUID) (*models.Devices, error)
	FindLatestRelease(hardwareVersion string, now time.Time) (*models.FirmwareReleases, error)
	FindReleaseByID(releaseID uuid.UUID) (*models.FirmwareReleases, error)
	SaveInstallationReport(report *models.FirmwareInstallations, installedVersion string) (*models.FirmwareInstallations, bool, error)
}

type Service struct {
	repo repository
	now  func() time.Time
}

func NewService(repo repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) GetLatest(deviceIDValue string) (*GetLatestResponse, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}

	device, err := s.repo.FindDeviceByID(deviceID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find device", err)
	}
	if device == nil {
		return nil, appErr.NewNotFound("Device not found", nil)
	}

	hardwareVersion := deviceHardwareVersion(device)
	if hardwareVersion == "" {
		return nil, appErr.NewBadRequest("Device hardware version is not configured", nil)
	}

	release, err := s.repo.FindLatestRelease(hardwareVersion, s.now().UTC())
	if err != nil {
		return nil, appErr.NewInternal("Failed to find compatible firmware release", err)
	}
	if release == nil {
		return nil, appErr.NewNotFound("Compatible firmware release not found", nil)
	}

	return &GetLatestResponse{
		ReleaseID: release.ID, Version: release.Version, HardwareVersion: release.HardwareVersion,
		DownloadURL: release.DownloadURL, FileSizeBytes: release.FileSizeBytes, SHA256: release.SHA256,
		Signature: release.Signature, Mandatory: release.Mandatory, PublishedAt: release.PublishedAt,
	}, nil
}

func (s *Service) ReportFirmwareUpdate(deviceIDValue string, req *ReportRequest) (*ReportResponse, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}
	if req == nil {
		return nil, appErr.NewBadRequest("Firmware installation report is required", nil)
	}
	messageID := strings.TrimSpace(req.MessageID)
	if messageID == "" || len(messageID) > 100 {
		return nil, appErr.NewBadRequest("messageId is required and must not exceed 100 characters", nil)
	}
	releaseID, err := utils.ParseId(req.ReleaseID)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid release ID", err)
	}
	if !validFirmwareInstallationStatus(req.Status) {
		return nil, appErr.NewBadRequest("Invalid firmware installation status", nil)
	}
	if req.ReportedAt.IsZero() {
		return nil, appErr.NewBadRequest("reportedAt is required and must be an RFC3339 timestamp", nil)
	}
	now := s.now().UTC().Truncate(time.Second)
	if req.ReportedAt.After(now.Add(maxFirmwareReportClockSkew)) {
		return nil, appErr.NewBadRequest("reportedAt cannot be more than 5 minutes in the future", nil)
	}

	var errorCode, errorMessage *string
	if req.Status == models.FirmwareInstallationStatusFailed {
		if req.Error == nil || strings.TrimSpace(req.Error.Code) == "" || strings.TrimSpace(req.Error.Message) == "" {
			return nil, appErr.NewBadRequest("error code and message are required when status is FAILED", nil)
		}
		code := strings.TrimSpace(req.Error.Code)
		message := strings.TrimSpace(req.Error.Message)
		errorCode, errorMessage = &code, &message
	} else if req.Error != nil {
		return nil, appErr.NewBadRequest("error is only allowed when status is FAILED", nil)
	}

	device, err := s.repo.FindDeviceByID(deviceID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find device", err)
	}
	if device == nil {
		return nil, appErr.NewNotFound("Device not found", nil)
	}
	release, err := s.repo.FindReleaseByID(releaseID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find firmware release", err)
	}
	if release == nil {
		return nil, appErr.NewNotFound("Firmware release not found", nil)
	}
	if deviceHardwareVersion(device) == "" {
		return nil, appErr.NewBadRequest("Device hardware version is not configured", nil)
	}
	if deviceHardwareVersion(device) != strings.TrimSpace(release.HardwareVersion) {
		return nil, appErr.NewBadRequest("Firmware release is not compatible with device hardware", nil)
	}

	report := &models.FirmwareInstallations{
		ID: utils.GenerateUUID(), DeviceID: deviceID, ReleaseID: releaseID, MessageID: messageID,
		Status: req.Status, ErrorCode: errorCode, ErrorMessage: errorMessage, ReportedAt: req.ReportedAt.UTC(),
	}
	stored, duplicate, err := s.repo.SaveInstallationReport(report, release.Version)
	if err != nil {
		return nil, appErr.NewInternal("Failed to save firmware installation report", err)
	}

	return &ReportResponse{
		InstallationID: stored.ID, MessageID: stored.MessageID, ReleaseID: stored.ReleaseID,
		Status: stored.Status, ReportedAt: stored.ReportedAt, Duplicate: duplicate, ServerTime: now,
	}, nil
}

func deviceHardwareVersion(device *models.Devices) string {
	if device == nil || device.HardwareVersion == nil {
		return ""
	}
	return strings.TrimSpace(*device.HardwareVersion)
}

func validFirmwareInstallationStatus(status models.FirmwareInstallationStatus) bool {
	switch status {
	case models.FirmwareInstallationStatusDownloading, models.FirmwareInstallationStatusDownloaded,
		models.FirmwareInstallationStatusVerifying, models.FirmwareInstallationStatusVerified,
		models.FirmwareInstallationStatusInstalling, models.FirmwareInstallationStatusInstalled,
		models.FirmwareInstallationStatusRolledBack, models.FirmwareInstallationStatusFailed:
		return true
	default:
		return false
	}
}
