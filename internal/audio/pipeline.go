package audio

import (
	"context"
	"fmt"
	"log"
	"time"

	"canepanion-server/models"
	"canepanion-server/pkg/gemini"
	"canepanion-server/pkg/speech"
	"canepanion-server/pkg/tts"
	"canepanion-server/pkg/utils"

	"github.com/google/uuid"
)

const voiceJobTimeout = 90 * time.Second

// Pipeline holds optional Google clients for the STT → Gemini → TTS job.
// When any client is nil the pipeline is disabled (FCM-style degrade).
type Pipeline struct {
	Speech *speech.Client
	Gemini *gemini.Client
	TTS    *tts.Client
}

// NewPipeline builds Google clients from ADC. Missing credentials yield a
// disabled pipeline and no error so the server still boots.
func NewPipeline(ctx context.Context) (*Pipeline, error) {
	if !speech.CredentialsConfigured() {
		path := "GOOGLE_APPLICATION_CREDENTIALS"
		log.Printf("voice: %s is unset or unreadable", path)
		log.Println("voice: voice pipeline disabled; upload/complete still accept audio")
		return &Pipeline{}, nil
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
		_ = geminiClient.Close()
		return nil, fmt.Errorf("initialize tts client: %w", err)
	}

	log.Println("voice: pipeline enabled (Speech + Vertex Gemini + TTS)")
	return &Pipeline{
		Speech: speechClient,
		Gemini: geminiClient,
		TTS:    ttsClient,
	}, nil
}

// Enabled reports whether all pipeline clients are available.
func (p *Pipeline) Enabled() bool {
	return p != nil && p.Speech != nil && p.Gemini != nil && p.TTS != nil
}

// startVoiceJob runs the pipeline in the background. When the pipeline is
// disabled it marks the clip FAILED synchronously and returns false.
func (s *Service) startVoiceJob(deviceID, audioID uuid.UUID) bool {
	if s.pipeline == nil || !s.pipeline.Enabled() {
		log.Printf("voice: pipeline disabled; marking audio %s FAILED", audioID)
		if err := s.repo.MarkAudioFailed(deviceID, audioID); err != nil {
			log.Printf("voice: failed to mark audio %s FAILED: %v", audioID, err)
		}
		return false
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), voiceJobTimeout)
		defer cancel()
		if err := s.runVoicePipeline(ctx, deviceID, audioID); err != nil {
			log.Printf("voice: pipeline failed for audio %s: %v", audioID, err)
			if markErr := s.repo.MarkAudioFailed(deviceID, audioID); markErr != nil {
				log.Printf("voice: failed to mark audio %s FAILED: %v", audioID, markErr)
			}
		}
	}()
	return true
}

func (s *Service) runVoicePipeline(ctx context.Context, deviceID, audioID uuid.UUID) error {
	audio, err := s.repo.FindAudioByID(deviceID, audioID)
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

	existing, err := s.repo.FindReplyByParentID(deviceID, audioID)
	if err != nil {
		return fmt.Errorf("lookup existing reply: %w", err)
	}
	if existing != nil && existing.StorageKey != "" {
		if err := s.repo.MarkAudioCompleted(deviceID, audioID); err != nil {
			return fmt.Errorf("mark completed after existing reply: %w", err)
		}
		return nil
	}

	deliveryURL, err := utils.AudioDeliveryURL(audio.StorageKey)
	if err != nil {
		return fmt.Errorf("build delivery URL: %w", err)
	}

	transcript, err := s.pipeline.Speech.RecognizeOpus(
		ctx,
		deliveryURL,
		speech.DefaultLanguageCode(),
		speech.DefaultSampleRateHz(),
	)
	if err != nil {
		return fmt.Errorf("stt: %w", err)
	}
	if err := s.repo.UpdateTranscript(deviceID, audioID, transcript); err != nil {
		return fmt.Errorf("save transcript: %w", err)
	}

	replyText, err := s.pipeline.Gemini.GenerateReply(ctx, transcript)
	if err != nil {
		return fmt.Errorf("gemini: %w", err)
	}

	appLang := tts.DetectAppLang(replyText)
	mp3, err := s.pipeline.TTS.Synthesize(ctx, replyText, appLang)
	if err != nil {
		return fmt.Errorf("tts: %w", err)
	}

	replyID := utils.GenerateUUID()
	folder := fmt.Sprintf("canepanion/devices/%s/audio", deviceID)
	filename := replyID.String() + ".mp3"
	_, publicID, err := utils.UploadAudioBytes(ctx, mp3, filename, folder)
	if err != nil {
		return fmt.Errorf("upload reply audio: %w", err)
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
	if err := s.repo.CreateAudio(reply); err != nil {
		_ = utils.DeleteAudio(context.WithoutCancel(ctx), publicID)
		// Unique parent_audio_id: another worker may have finished first.
		existing, lookupErr := s.repo.FindReplyByParentID(deviceID, audioID)
		if lookupErr == nil && existing != nil {
			if err := s.repo.MarkAudioCompleted(deviceID, audioID); err != nil {
				return fmt.Errorf("mark completed after concurrent reply: %w", err)
			}
			return nil
		}
		return fmt.Errorf("save reply audio: %w", err)
	}

	if err := s.repo.MarkAudioCompleted(deviceID, audioID); err != nil {
		return fmt.Errorf("mark user clip completed: %w", err)
	}
	return nil
}
