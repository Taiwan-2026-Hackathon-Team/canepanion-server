package devicecontrol

import (
	"time"

	"canepanion-server/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type commandCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (r *Repository) FindConfigurationByDeviceID(deviceID uuid.UUID) (*DeviceConfiguration, error) {
	var config DeviceConfiguration
	err := r.db.Model(&models.Devices{}).
		Select("id AS device_id, configuration_version AS version, configuration").
		Where("id = ?", deviceID).
		Take(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &config, nil
}

func (r *Repository) FindDeviceByID(deviceID uuid.UUID) (*models.Devices, error) {
	var device models.Devices
	err := r.db.First(&device, "id = ?", deviceID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

func (r *Repository) CreateCommand(command *models.DeviceCommands) error {
	return r.db.Create(command).Error
}

func (r *Repository) FindPendingCommands(
	deviceID uuid.UUID,
	cursor *commandCursor,
	limit int,
	now time.Time,
) ([]models.DeviceCommands, bool, error) {
	var deviceCount int64
	if err := r.db.Model(&models.Devices{}).Where("id = ?", deviceID).Count(&deviceCount).Error; err != nil {
		return nil, false, err
	}
	if deviceCount == 0 {
		return nil, false, nil
	}

	query := r.db.Model(&models.DeviceCommands{}).
		Where("device_id = ? AND status = ? AND expires_at > ?", deviceID, models.DeviceCommandStatusPending, now)
	if cursor != nil {
		query = query.Where(
			"created_at > ? OR (created_at = ? AND id > ?)",
			cursor.CreatedAt, cursor.CreatedAt, cursor.ID,
		)
	}

	var commands []models.DeviceCommands
	err := query.Order("created_at ASC").Order("id ASC").Limit(limit).Find(&commands).Error
	if err != nil {
		return nil, true, err
	}

	return commands, true, nil
}

func (r *Repository) FindCommandByID(deviceID, commandID uuid.UUID) (*models.DeviceCommands, error) {
	var command models.DeviceCommands
	err := r.db.Where("id = ? AND device_id = ?", commandID, deviceID).First(&command).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &command, nil
}

func (r *Repository) UpdateCommandStatus(
	deviceID, commandID uuid.UUID,
	currentStatus, nextStatus models.DeviceCommandStatus,
) (bool, error) {
	result := r.db.Model(&models.DeviceCommands{}).
		Where("id = ? AND device_id = ? AND status = ?", commandID, deviceID, currentStatus).
		Update("status", nextStatus)
	return result.RowsAffected == 1, result.Error
}
