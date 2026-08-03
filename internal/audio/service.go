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
	FindAudioByID(deviceID, audioID uuid.UUID) (*models.Audio, error)
	ClaimProcessing(deviceID, audioID uuid.UUID) (*models.Audio, bool, error)
	MarkAudioFailed(deviceID, audioID uuid.UUID) error
	MarkAudioCompleted(deviceID, audioID uuid.UUID) error
	CreateReplyAndCompleteUser(deviceID, userAudioID uuid.UUID, reply *models.Audio) error
	UpdateTranscript(deviceID, audioID uuid.UUID, transcript string) error
	FindReplyByParentID(deviceID, parentAudioID uuid.UUID) (*models.Audio, error)
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
	voiceJob *VoiceJob
}

func NewService(repo *Repository, voiceJob *VoiceJob) *Service {
	return newService(repo, cloudinaryUploader{}, voiceJob)
}

func newService(repo audioRepository, uploader audioUploader, voiceJob *VoiceJob) *Service {
	return &Service{repo: repo, uploader: uploader, voiceJob: voiceJob}
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
	if err := validateParentLink(audio); err != nil {
		_ = s.uploader.Delete(context.WithoutCancel(ctx), publicID)
		return nil, appErr.NewBadRequest(err.Error(), err)
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

func (s *Service) CompleteUpload(deviceIDValue, audioIDValue string) (*CompleteUploadResponse, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}
	audioID, err := utils.ParseId(audioIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid audio ID", err)
	}

	audio, claimed, err := s.repo.ClaimProcessing(deviceID, audioID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to complete audio upload", err)
	}
	if audio == nil {
		return nil, appErr.NewNotFound("Audio recording not found", nil)
	}

	switch audio.Status {
	case models.AudioStatusProcessing, models.AudioStatusCompleted:
		// Idempotent: claim winner or safe firmware retry.
	case models.AudioStatusFailed:
		return nil, appErr.NewBadRequest("Failed audio recording cannot be completed", nil)
	default:
		return nil, appErr.NewBadRequest("Audio recording cannot be completed from its current status", nil)
	}

	if claimed && audio.Direction == models.AudioDirectionUserToAssistant {
		if s.voiceJob == nil || !s.voiceJob.Start(deviceID, audioID) {
			audio.Status = models.AudioStatusFailed
		}
	}

	return &CompleteUploadResponse{
		AudioID: audio.ID,
		Status:  audio.Status,
	}, nil
}

func (s *Service) GetAudio(deviceIDValue, audioIDValue string) (*GetAudioResponse, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}
	audioID, err := utils.ParseId(audioIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid audio ID", err)
	}

	audio, err := s.repo.FindAudioByID(deviceID, audioID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find audio recording", err)
	}
	if audio == nil {
		return nil, appErr.NewNotFound("Audio recording not found", nil)
	}

	resp := &GetAudioResponse{
		AudioID: audio.ID,
		Status:  audio.Status,
	}
	if audio.Status != models.AudioStatusCompleted {
		return resp, nil
	}

	reply, err := s.repo.FindReplyByParentID(deviceID, audioID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find reply audio", err)
	}
	if reply == nil || reply.StorageKey == "" {
		return resp, nil
	}

	url, err := utils.AudioDeliveryURL(reply.StorageKey)
	if err != nil {
		return nil, appErr.NewInternal("Failed to build reply audio URL", err)
	}
	resp.ReplyAudioURL = url
	return resp, nil
}

func validDirection(direction models.AudioDirection) bool {
	switch direction {
	case models.AudioDirectionUserToAssistant, models.AudioDirectionAssistantToUser:
		return true
	default:
		return false
	}
}

func validateParentLink(audio *models.Audio) error {
	// parent_audio_id is non-null only on ASSISTANT_TO_USER (pipeline replies).
	// Firmware may still upload ASSISTANT_TO_USER clips without a parent.
	if audio.ParentAudioID != nil && audio.Direction != models.AudioDirectionAssistantToUser {
		return fmt.Errorf("parentAudioId is only allowed on ASSISTANT_TO_USER recordings")
	}
	return nil
}
