package tts

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
)

// Client wraps Cloud Text-to-Speech SynthesizeSpeech.
type Client struct {
	inner *texttospeech.Client
}

// NewClient builds a TTS client via Application Default Credentials.
func NewClient(ctx context.Context) (*Client, error) {
	inner, err := texttospeech.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create tts client: %w", err)
	}
	return &Client{inner: inner}, nil
}

func (c *Client) Close() error {
	if c == nil || c.inner == nil {
		return nil
	}
	return c.inner.Close()
}

// Synthesize returns MP3 bytes for text in the given app language
// (zh-TW or en-US). Empty appLang defaults to zh-TW.
func (c *Client) Synthesize(ctx context.Context, text, appLang string) ([]byte, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("tts client is not configured")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("tts input text is empty")
	}

	langCode, voiceName := voiceForAppLang(appLang)
	resp, err := c.inner.SynthesizeSpeech(ctx, &texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: langCode,
			Name:         voiceName,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: texttospeechpb.AudioEncoding_MP3,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("synthesize speech: %w", err)
	}
	if len(resp.AudioContent) == 0 {
		return nil, fmt.Errorf("tts returned empty audio")
	}
	return resp.AudioContent, nil
}

func voiceForAppLang(appLang string) (languageCode, voiceName string) {
	switch strings.TrimSpace(appLang) {
	case "en-US", "en":
		return "en-US", "en-US-Neural2-C"
	default:
		return "cmn-TW", "cmn-TW-Wavenet-A"
	}
}

// DetectAppLang picks zh-TW when text contains CJK ideographs, else en-US.
func DetectAppLang(text string) string {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return "zh-TW"
		}
	}
	return "en-US"
}
