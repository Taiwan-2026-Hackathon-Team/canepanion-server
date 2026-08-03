package audio

import (
	"fmt"

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

// MarkAudioFailed transitions PROCESSING → FAILED only.
func (r *Repository) MarkAudioFailed(deviceID, audioID uuid.UUID) error {
	return r.db.Model(&models.Audio{}).
		Where("id = ? AND device_id = ? AND status = ?", audioID, deviceID, models.AudioStatusProcessing).
		Update("status", models.AudioStatusFailed).Error
}

// MarkAudioCompleted transitions PROCESSING → COMPLETED only.
func (r *Repository) MarkAudioCompleted(deviceID, audioID uuid.UUID) error {
	return r.db.Model(&models.Audio{}).
		Where("id = ? AND device_id = ? AND status = ?", audioID, deviceID, models.AudioStatusProcessing).
		Update("status", models.AudioStatusCompleted).Error
}

// CreateReplyAndCompleteUser inserts the assistant reply and marks the user
// clip COMPLETED in one transaction.
func (r *Repository) CreateReplyAndCompleteUser(deviceID, userAudioID uuid.UUID, reply *models.Audio) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(reply).Error; err != nil {
			return err
		}
		result := tx.Model(&models.Audio{}).
			Where("id = ? AND device_id = ? AND status = ?", userAudioID, deviceID, models.AudioStatusProcessing).
			Update("status", models.AudioStatusCompleted)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("user clip is not PROCESSING")
		}
		return nil
	})
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
