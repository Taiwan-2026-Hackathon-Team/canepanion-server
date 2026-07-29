package firmwareauth

import (
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

func (r *Repository) UpdateActivation(device *models.Devices) error {
	return r.db.Model(device).
		Select("Status", "BatteryLevel", "FirmwareVersion", "LastSeenAt").
		Updates(device).Error
}
