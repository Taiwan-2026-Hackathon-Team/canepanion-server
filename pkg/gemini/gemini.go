package gemini

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

// SystemPrompt is the approved Canepanion assistant instruction for MVP.
const SystemPrompt = `You are Canepanion, a warm and calm companion on a smart mobility cane.
Be helpful and clear. Sound friendly, not cold or overly formal.
Keep answers short enough to speak aloud: prefer 1–2 short sentences; never more than 3 short sentences.
Reply in the same language the user used (Traditional Chinese or English).
You are not a doctor, nurse, or emergency service. Do not diagnose, prescribe, or give medical treatment advice.
Do not help with illegal or harmful activities.
Do not claim to be a human caregiver or that you can send emergency responders.
If the user may be in danger or needs urgent help, briefly tell them to contact local emergency services.
You only receive the user's spoken words as text. Do not invent location, battery, or sensor data.`

// Client wraps Vertex Gemini GenerateContent.
type Client struct {
	inner *genai.Client
	model string
}

// NewClient builds a Vertex AI Gemini client via Application Default Credentials.
func NewClient(ctx context.Context) (*Client, error) {
	project := strings.TrimSpace(os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if project == "" {
		project = strings.TrimSpace(os.Getenv("GCP_PROJECT"))
	}
	location := strings.TrimSpace(os.Getenv("GOOGLE_CLOUD_LOCATION"))
	if location == "" {
		location = strings.TrimSpace(os.Getenv("VERTEX_LOCATION"))
	}
	if project == "" {
		project = "canepanion"
	}
	if location == "" {
		location = "asia-southeast1"
	}

	inner, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  project,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}

	model := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if model == "" {
		model = "gemini-2.5-flash"
	}

	return &Client{inner: inner, model: model}, nil
}

func (c *Client) Close() error {
	// genai.Client has no Close in current SDK; keep for symmetry.
	return nil
}

// GenerateReply runs system instruction + transcript and returns assistant text.
func (c *Client) GenerateReply(ctx context.Context, transcript string) (string, error) {
	if c == nil || c.inner == nil {
		return "", fmt.Errorf("gemini client is not configured")
	}
	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		return "", fmt.Errorf("gemini transcript is empty")
	}

	cfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: SystemPrompt}},
		},
	}

	resp, err := c.inner.Models.GenerateContent(ctx, c.model, genai.Text(transcript), cfg)
	if err != nil {
		return "", fmt.Errorf("gemini generate: %w", err)
	}
	out := strings.TrimSpace(resp.Text())
	if out == "" {
		return "", fmt.Errorf("empty Gemini response")
	}
	return out, nil
}
