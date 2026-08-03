package audio

import (
	"os"
	"strings"
)

// googleCredentialsConfigured reports whether GOOGLE_APPLICATION_CREDENTIALS
// points at a readable file (ADC shared by Speech, TTS, and Vertex Gemini).
func googleCredentialsConfigured() bool {
	path := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
