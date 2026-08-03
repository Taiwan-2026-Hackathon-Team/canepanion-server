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

func (r *Repository) MarkAudioFailed(deviceID, audioID uuid.UUID) error {
	return r.db.Model(&models.Audio{}).
		Where("id = ? AND device_id = ?", audioID, deviceID).
		Update("status", models.AudioStatusFailed).Error
}

func (r *Repository) MarkAudioCompleted(deviceID, audioID uuid.UUID) error {
	return r.db.Model(&models.Audio{}).
		Where("id = ? AND device_id = ?", audioID, deviceID).
		Update("status", models.AudioStatusCompleted).Error
}

func (r *Repository) UpdateTranscript(deviceID, audioID uuid.UUID, transcript string) error {
	return r.db.Model(&models.Audio{}).
		Where("id = ? AND device_id = ?", audioID, deviceID).
		Update("transcript", transcript).Error
}

func (r *Repository) FindReplyByParentID(deviceID, parentAudioID uuid.UUID) (*models.Audio, error) {
	var audio models.Audio
	err := r.db.Where(
		"device_id = ? AND parent_audio_id = ? AND direction = ?",
		deviceID,
		parentAudioID,
		models.AudioDirectionAssistantToUser,
	).First(&audio).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &audio, nil
}
