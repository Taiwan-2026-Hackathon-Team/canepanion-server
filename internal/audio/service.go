package audio

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"canepanion-server/models"
	appErr "canepanion-server/pkg/errors"
	"canepanion-server/pkg/utils"

	"github.com/google/uuid"
)

const maxAudioFileSize int64 = 25 << 20

type audioRepository interface {
	FindDeviceByID(deviceID uuid.UUID) (*models.Devices, error)
	CreateAudio(audio *models.Audio) error
}

type audioUploader interface {
	Upload(
		ctx context.Context,
		file multipart.File,
		fileHeader *multipart.FileHeader,
		folder string,
	) (secureURL string, publicID string, err error)
	Delete(ctx context.Context, publicID string) error
}

type cloudinaryUploader struct{}

func (cloudinaryUploader) Upload(
	ctx context.Context,
	file multipart.File,
	fileHeader *multipart.FileHeader,
	folder string,
) (string, string, error) {
	return utils.UploadAudio(ctx, file, fileHeader, folder)
}

func (cloudinaryUploader) Delete(ctx context.Context, publicID string) error {
	return utils.DeleteAudio(ctx, publicID)
}

type Service struct {
	repo     audioRepository
	uploader audioUploader
}

func NewService(repo *Repository) *Service {
	return newService(repo, cloudinaryUploader{})
}

func newService(repo audioRepository, uploader audioUploader) *Service {
	return &Service{repo: repo, uploader: uploader}
}

func (s *Service) CreateUpload(
	ctx context.Context,
	deviceIDValue string,
	req *CreateUploadRequest,
	file multipart.File,
	fileHeader *multipart.FileHeader,
) (*CreateUploadResponse, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}

	if req == nil || !validDirection(req.Direction) {
		return nil, appErr.NewBadRequest("Invalid audio direction", nil)
	}
	if file == nil || fileHeader == nil {
		return nil, appErr.NewBadRequest("Audio file is required", nil)
	}
	if fileHeader.Size <= 0 {
		return nil, appErr.NewBadRequest("Audio file cannot be empty", nil)
	}
	if fileHeader.Size > maxAudioFileSize {
		return nil, appErr.NewBadRequest("Audio file exceeds the 25 MB limit", nil)
	}

	contentType := strings.ToLower(strings.TrimSpace(fileHeader.Header.Get("Content-Type")))
	if !strings.HasPrefix(contentType, "audio/") {
		return nil, appErr.NewBadRequest("Uploaded file must have an audio content type", nil)
	}

	device, err := s.repo.FindDeviceByID(deviceID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find device", err)
	}
	if device == nil {
		return nil, appErr.NewNotFound("Device not found", nil)
	}

	audioID := utils.GenerateUUID()
	uploadHeader := *fileHeader
	uploadHeader.Filename = audioID.String() + strings.ToLower(filepath.Ext(fileHeader.Filename))
	folder := fmt.Sprintf("canepanion/devices/%s/audio", deviceID)

	secureURL, publicID, err := s.uploader.Upload(ctx, file, &uploadHeader, folder)
	if err != nil {
		return nil, appErr.NewInternal("Failed to upload audio", err)
	}

	audio := &models.Audio{
		ID:         audioID,
		DeviceID:   deviceID,
		Direction:  req.Direction,
		StorageKey: publicID,
		Status:     models.AudioStatusUploaded,
	}
	if err := s.repo.CreateAudio(audio); err != nil {
		_ = s.uploader.Delete(context.WithoutCancel(ctx), publicID)
		return nil, appErr.NewInternal("Failed to save audio metadata", err)
	}

	return &CreateUploadResponse{
		AudioID:    audio.ID,
		DeviceID:   audio.DeviceID,
		Direction:  audio.Direction,
		AudioURL:   secureURL,
		StorageKey: audio.StorageKey,
		Status:     audio.Status,
		CreatedAt:  audio.CreatedAt,
	}, nil
}

func validDirection(direction models.AudioDirection) bool {
	switch direction {
	case models.AudioDirectionUserToAssistant, models.AudioDirectionAssistantToUser:
		return true
	default:
		return false
	}
}
