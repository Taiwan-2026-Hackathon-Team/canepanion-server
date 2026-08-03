package audio

import (
	"context"
	"fmt"
	"log"
)

// Transcriber turns user audio bytes into text (STT).
type Transcriber interface {
	Recognize(ctx context.Context, content []byte, languageCode string, sampleRateHz int32) (string, error)
}

// Replier turns a transcript into assistant reply text.
type Replier interface {
	GenerateReply(ctx context.Context, transcript string) (string, error)
}

// Speaker turns reply text into spoken audio bytes (TTS).
type Speaker interface {
	Synthesize(ctx context.Context, text, appLang string) ([]byte, error)
}

// DisabledTranscriber stands in when the voice pipeline has no ADC.
type DisabledTranscriber struct{}

func (DisabledTranscriber) Recognize(_ context.Context, _ []byte, _ string, _ int32) (string, error) {
	log.Println("voice: DISABLED, dropping STT request")
	return "", fmt.Errorf("speech is disabled")
}

// DisabledReplier stands in when the voice pipeline has no ADC.
type DisabledReplier struct{}

func (DisabledReplier) GenerateReply(_ context.Context, _ string) (string, error) {
	log.Println("voice: DISABLED, dropping Gemini request")
	return "", fmt.Errorf("gemini is disabled")
}

// DisabledSpeaker stands in when the voice pipeline has no ADC.
type DisabledSpeaker struct{}

func (DisabledSpeaker) Synthesize(_ context.Context, _, _ string) ([]byte, error) {
	log.Println("voice: DISABLED, dropping TTS request")
	return nil, fmt.Errorf("tts is disabled")
}
