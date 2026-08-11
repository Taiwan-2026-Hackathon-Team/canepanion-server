package firmwareupdates

import (
	"time"

	"canepanion-server/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindDeviceByID(deviceID uuid.UUID) (*models.Devices, error) {
	var device models.Devices
	if err := r.db.First(&device, "id = ?", deviceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

func (r *Repository) FindLatestRelease(hardwareVersion string, now time.Time) (*models.FirmwareReleases, error) {
	var release models.FirmwareReleases
	err := r.db.Where("hardware_version = ? AND published_at <= ?", hardwareVersion, now).
		Order("published_at DESC").
		Order("created_at DESC").
		First(&release).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &release, nil
}

func (r *Repository) FindReleaseByID(releaseID uuid.UUID) (*models.FirmwareReleases, error) {
	var release models.FirmwareReleases
	if err := r.db.First(&release, "id = ?", releaseID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &release, nil
}

func (r *Repository) SaveInstallationReport(report *models.FirmwareInstallations, installedVersion string) (*models.FirmwareInstallations, bool, error) {
	var stored *models.FirmwareInstallations
	duplicate := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "device_id"}, {Name: "message_id"}},
			DoNothing: true,
		}).Create(report)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			duplicate = true
			var existing models.FirmwareInstallations
			if err := tx.Where("device_id = ? AND message_id = ?", report.DeviceID, report.MessageID).First(&existing).Error; err != nil {
				return err
			}
			stored = &existing
			return nil
		}

		stored = report
		if report.Status == models.FirmwareInstallationStatusInstalled {
			return tx.Model(&models.Devices{}).Where("id = ?", report.DeviceID).Update("firmware_version", installedVersion).Error
		}
		return nil
	})
	return stored, duplicate, err
}
