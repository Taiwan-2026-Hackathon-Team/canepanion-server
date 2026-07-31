package audio

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

func (r *Repository) CreateAudio(audio *models.Audio) error {
	return r.db.Create(audio).Error
}

func (r *Repository) FindAudioByID(deviceID, audioID uuid.UUID) (*models.Audio, error) {
	var audio models.Audio
	if err := r.db.Where("id = ? AND device_id = ?", audioID, deviceID).First(&audio).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &audio, nil
}

func (r *Repository) MarkAudioProcessing(deviceID, audioID uuid.UUID) (bool, error) {
	result := r.db.Model(&models.Audio{}).
		Where("id = ? AND device_id = ? AND status = ?", audioID, deviceID, models.AudioStatusUploaded).
		Update("status", models.AudioStatusProcessing)
	return result.RowsAffected == 1, result.Error
}
