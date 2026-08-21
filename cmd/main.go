package main

import (
	"context"
	"log"

	"canepanion-server/config"
	"canepanion-server/infra"
	"canepanion-server/internal/audio"
	"canepanion-server/internal/camera"
	"canepanion-server/internal/push"
)

func main() {
	config.LoadEnvVariables()

	// add --migrate in running Go if it needs db migration
	config.HandleMigrationFlag()

	infra.ConnectDb()

	// Guardian push. Missing credentials yield a disabled notifier rather than
	// an error, so the server still runs for anyone without the Firebase
	// service account; only a malformed credential is fatal.
	notifier, err := push.NewFCMNotifier(context.Background())
	if err != nil {
		log.Fatalf("Failed to initialize push notifications: %v", err)
	}

	// Voice pipeline (STT → Gemini → TTS). Missing Google ADC disables the
	// job without blocking boot; malformed credentials are fatal.
	voiceJob, err := audio.NewVoiceJob(context.Background())
	if err != nil {
		log.Fatalf("Failed to initialize voice pipeline: %v", err)
	}

	cameraHub, err := camera.NewHubFromEnv()
	if err != nil {
		log.Fatalf("Failed to initialize camera relay: %v", err)
	}
	defer func() { _ = cameraHub.Close() }()

	infra.RunGin(config.CORS(), notifier, voiceJob, cameraHub)
}
