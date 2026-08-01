package devicecontrol

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
