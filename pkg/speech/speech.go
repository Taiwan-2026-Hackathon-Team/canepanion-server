package speech

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	speechapi "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
)

const maxSyncRecognizeBytes = 10 << 20

// Client wraps Cloud Speech-to-Text synchronous Recognize.
type Client struct {
	inner *speechapi.Client
}

// CredentialsConfigured reports whether GOOGLE_APPLICATION_CREDENTIALS points
// at a readable file (ADC for Speech / TTS / Vertex).
func CredentialsConfigured() bool {
	path := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
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

// RecognizeOpus downloads audioURL and runs sync Recognize as Ogg Opus.
func (c *Client) RecognizeOpus(ctx context.Context, audioURL, languageCode string, sampleRateHz int32) (string, error) {
	if c == nil || c.inner == nil {
		return "", fmt.Errorf("speech client is not configured")
	}
	if languageCode == "" {
		languageCode = DefaultLanguageCode()
	}
	if sampleRateHz == 0 {
		sampleRateHz = DefaultSampleRateHz()
	}

	body, err := downloadBytes(ctx, audioURL, maxSyncRecognizeBytes)
	if err != nil {
		return "", err
	}

	resp, err := c.inner.Recognize(ctx, &speechpb.RecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:        speechpb.RecognitionConfig_OGG_OPUS,
			SampleRateHertz: sampleRateHz,
			LanguageCode:    languageCode,
		},
		Audio: &speechpb.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Content{Content: body},
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

func downloadBytes(ctx context.Context, url string, maxBytes int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download audio: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download audio %s: %s", url, res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, int64(maxBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("read audio body: %w", err)
	}
	if len(body) > maxBytes {
		return nil, fmt.Errorf("audio exceeds %d byte sync Recognize limit", maxBytes)
	}
	return body, nil
}
