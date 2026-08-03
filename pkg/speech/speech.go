package speech

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	speechapi "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
)

// MaxSyncRecognizeBytes is the Cloud Speech sync Recognize inline limit (~10 MB).
const MaxSyncRecognizeBytes = 10 << 20

// Client wraps Cloud Speech-to-Text synchronous Recognize.
type Client struct {
	inner *speechapi.Client
}

// NewClient builds a Speech client via Application Default Credentials.
func NewClient(ctx context.Context) (*Client, error) {
	inner, err := speechapi.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create speech client: %w", err)
	}
	return &Client{inner: inner}, nil
}

func (c *Client) Close() error {
	if c == nil || c.inner == nil {
		return nil
	}
	return c.inner.Close()
}

// Recognize runs sync Recognize on Ogg Opus audio content.
func (c *Client) Recognize(ctx context.Context, content []byte, languageCode string, sampleRateHz int32) (string, error) {
	if c == nil || c.inner == nil {
		return "", fmt.Errorf("speech client is not configured")
	}
	if len(content) == 0 {
		return "", fmt.Errorf("speech content is empty")
	}
	if len(content) > MaxSyncRecognizeBytes {
		return "", fmt.Errorf("audio exceeds %d byte sync Recognize limit", MaxSyncRecognizeBytes)
	}
	if languageCode == "" {
		languageCode = DefaultLanguageCode()
	}
	if sampleRateHz == 0 {
		sampleRateHz = DefaultSampleRateHz()
	}

	resp, err := c.inner.Recognize(ctx, &speechpb.RecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:        speechpb.RecognitionConfig_OGG_OPUS,
			SampleRateHertz: sampleRateHz,
			LanguageCode:    languageCode,
		},
		Audio: &speechpb.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Content{Content: content},
		},
	})
	if err != nil {
		return "", fmt.Errorf("speech recognize: %w", err)
	}

	var b strings.Builder
	for _, result := range resp.Results {
		if len(result.Alternatives) == 0 {
			continue
		}
		b.WriteString(result.Alternatives[0].Transcript)
	}
	transcript := strings.TrimSpace(b.String())
	if transcript == "" {
		return "", fmt.Errorf("speech recognize returned empty transcript")
	}
	return transcript, nil
}

// DefaultLanguageCode returns VOICE_LANGUAGE or zh-TW.
func DefaultLanguageCode() string {
	if v := strings.TrimSpace(os.Getenv("VOICE_LANGUAGE")); v != "" {
		return v
	}
	return "zh-TW"
}

// DefaultSampleRateHz returns STT_SAMPLE_RATE_HZ or 16000.
func DefaultSampleRateHz() int32 {
	raw := strings.TrimSpace(os.Getenv("STT_SAMPLE_RATE_HZ"))
	if raw == "" {
		return 16000
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 16000
	}
	return int32(n)
}
