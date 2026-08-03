package audio

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"canepanion-server/models"
	"canepanion-server/pkg/gemini"
	"canepanion-server/pkg/speech"
	"canepanion-server/pkg/tts"
	"canepanion-server/pkg/utils"

	"github.com/google/uuid"
)

const voiceJobTimeout = 90 * time.Second

// VoiceJob owns background STT → Gemini → TTS orchestration and terminal
// status transitions for user clips. Missing Google ADC yields a disabled job
// so the server still boots (FCM-style degrade).
type VoiceJob struct {
	repo    audioRepository
	speech  Transcriber
	gemini  Replier
	tts     Speaker
	enabled bool
}

// NewVoiceJob builds Google clients from ADC. Missing credentials yield a
// disabled job and no error so the server still boots.
func NewVoiceJob(ctx context.Context) (*VoiceJob, error) {
	if !googleCredentialsConfigured() {
		log.Printf("voice: GOOGLE_APPLICATION_CREDENTIALS is unset or unreadable")
		log.Println("voice: voice pipeline disabled; upload/complete still accept audio")
		return &VoiceJob{
			speech:  DisabledTranscriber{},
			gemini:  DisabledReplier{},
			tts:     DisabledSpeaker{},
			enabled: false,
		}, nil
	}

	speechClient, err := speech.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize speech client: %w", err)
	}
	geminiClient, err := gemini.NewClient(ctx)
	if err != nil {
		_ = speechClient.Close()
		return nil, fmt.Errorf("initialize gemini client: %w", err)
	}
	ttsClient, err := tts.NewClient(ctx)
	if err != nil {
		_ = speechClient.Close()
		return nil, fmt.Errorf("initialize tts client: %w", err)
	}

	log.Println("voice: pipeline enabled (Speech + Vertex Gemini + TTS)")
	return &VoiceJob{
		speech:  speechClient,
		gemini:  geminiClient,
		tts:     ttsClient,
		enabled: true,
	}, nil
}

// bind attaches the audio repository used for job orchestration.
func (j *VoiceJob) bind(repo audioRepository) {
	if j == nil {
		return
	}
	j.repo = repo
}

// Enabled reports whether Google clients are wired for real delivery.
func (j *VoiceJob) Enabled() bool {
	return j != nil && j.enabled
}

// Start runs the pipeline in the background. When the job is disabled it marks
// the clip FAILED synchronously and returns false.
func (j *VoiceJob) Start(deviceID, audioID uuid.UUID) bool {
	if j == nil || j.repo == nil || !j.Enabled() {
		log.Printf("voice: pipeline disabled; marking audio %s FAILED", audioID)
		if j != nil && j.repo != nil {
			if err := j.repo.MarkAudioFailed(deviceID, audioID); err != nil {
				log.Printf("voice: failed to mark audio %s FAILED: %v", audioID, err)
			}
		}
		return false
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), voiceJobTimeout)
		defer cancel()
		if err := j.run(ctx, deviceID, audioID); err != nil {
			log.Printf("voice: pipeline failed for audio %s: %v", audioID, err)
			if markErr := j.repo.MarkAudioFailed(deviceID, audioID); markErr != nil {
				log.Printf("voice: failed to mark audio %s FAILED: %v", audioID, markErr)
			}
		}
	}()
	return true
}

func (j *VoiceJob) run(ctx context.Context, deviceID, audioID uuid.UUID) error {
	audio, err := j.repo.FindAudioByID(deviceID, audioID)
	if err != nil {
		return fmt.Errorf("load audio: %w", err)
	}
	if audio == nil {
		return fmt.Errorf("audio not found")
	}
	if audio.Direction != models.AudioDirectionUserToAssistant {
		return fmt.Errorf("voice pipeline only runs for USER_TO_ASSISTANT clips")
	}
	if audio.Status == models.AudioStatusCompleted {
		return nil
	}
	if audio.Status == models.AudioStatusFailed {
		return nil
	}

	existing, err := j.repo.FindReplyByParentID(deviceID, audioID)
	if err != nil {
		return fmt.Errorf("lookup existing reply: %w", err)
	}
	if existing != nil {
		return j.resumeOrFinishReply(ctx, deviceID, audioID, audio, existing)
	}

	transcript, err := j.ensureTranscript(ctx, deviceID, audioID, audio)
	if err != nil {
		return err
	}

	replyText, err := j.gemini.GenerateReply(ctx, transcript)
	if err != nil {
		return fmt.Errorf("gemini: %w", err)
	}

	replyID, publicID, err := j.synthesizeAndUpload(ctx, deviceID, uuid.Nil, replyText)
	if err != nil {
		return err
	}

	parentID := audioID
	reply := &models.Audio{
		ID:            replyID,
		DeviceID:      deviceID,
		Direction:     models.AudioDirectionAssistantToUser,
		StorageKey:    publicID,
		ResponseText:  &replyText,
		Status:        models.AudioStatusCompleted,
		ParentAudioID: &parentID,
	}
	if err := j.repo.CreateReplyAndCompleteUser(deviceID, audioID, reply); err != nil {
		_ = utils.DeleteAudio(context.WithoutCancel(ctx), publicID)
		// Unique parent_audio_id: another worker may have finished first.
		existing, lookupErr := j.repo.FindReplyByParentID(deviceID, audioID)
		if lookupErr == nil && existing != nil && existing.StorageKey != "" {
			if err := j.repo.MarkAudioCompleted(deviceID, audioID); err != nil {
				return fmt.Errorf("mark completed after concurrent reply: %w", err)
			}
			return nil
		}
		return fmt.Errorf("save reply audio: %w", err)
	}
	return nil
}

// resumeOrFinishReply short-circuits when a reply row already exists for the
// parent: finish COMPLETED if audio is present, otherwise resume from stored
// response_text (or STT/Gemini when still missing) without creating a second reply.
func (j *VoiceJob) resumeOrFinishReply(
	ctx context.Context,
	deviceID, audioID uuid.UUID,
	user *models.Audio,
	reply *models.Audio,
) error {
	if reply.StorageKey != "" {
		if err := j.repo.MarkAudioCompleted(deviceID, audioID); err != nil {
			return fmt.Errorf("mark completed after existing reply: %w", err)
		}
		return nil
	}

	replyText := strings.TrimSpace(derefString(reply.ResponseText))
	if replyText == "" {
		transcript, err := j.ensureTranscript(ctx, deviceID, audioID, user)
		if err != nil {
			return err
		}
		replyText, err = j.gemini.GenerateReply(ctx, transcript)
		if err != nil {
			return fmt.Errorf("gemini: %w", err)
		}
		if err := j.repo.UpdateReplyResponseText(deviceID, reply.ID, replyText); err != nil {
			return fmt.Errorf("save reply text: %w", err)
		}
	}

	_, publicID, err := j.synthesizeAndUpload(ctx, deviceID, reply.ID, replyText)
	if err != nil {
		return err
	}
	if err := j.repo.AttachReplyStorageAndCompleteUser(deviceID, audioID, reply.ID, publicID); err != nil {
		_ = utils.DeleteAudio(context.WithoutCancel(ctx), publicID)
		return fmt.Errorf("attach reply audio: %w", err)
	}
	return nil
}

func (j *VoiceJob) ensureTranscript(
	ctx context.Context,
	deviceID, audioID uuid.UUID,
	user *models.Audio,
) (string, error) {
	transcript := strings.TrimSpace(derefString(user.Transcript))
	if transcript != "" {
		return transcript, nil
	}

	content, err := utils.DownloadAudioBytes(ctx, user.StorageKey, speech.MaxSyncRecognizeBytes)
	if err != nil {
		return "", fmt.Errorf("download user audio: %w", err)
	}
	transcript, err = j.speech.Recognize(
		ctx,
		content,
		speech.DefaultLanguageCode(),
		speech.DefaultSampleRateHz(),
	)
	if err != nil {
		return "", fmt.Errorf("stt: %w", err)
	}
	if err := j.repo.UpdateTranscript(deviceID, audioID, transcript); err != nil {
		return "", fmt.Errorf("save transcript: %w", err)
	}
	return transcript, nil
}

// synthesizeAndUpload runs TTS and uploads MP3. When replyID is uuid.Nil a new
// id is generated for the Cloudinary filename; otherwise that id is reused.
func (j *VoiceJob) synthesizeAndUpload(
	ctx context.Context,
	deviceID, replyID uuid.UUID,
	replyText string,
) (uuid.UUID, string, error) {
	appLang := tts.DetectAppLang(replyText)
	mp3, err := j.tts.Synthesize(ctx, replyText, appLang)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("tts: %w", err)
	}

	if replyID == uuid.Nil {
		replyID = utils.GenerateUUID()
	}
	folder := fmt.Sprintf("canepanion/devices/%s/audio", deviceID)
	filename := replyID.String() + ".mp3"
	_, publicID, err := utils.UploadAudioBytes(ctx, mp3, filename, folder)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("upload reply audio: %w", err)
	}
	return replyID, publicID, nil
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
