package infra

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"canepanion-server/internal/audio"
	"canepanion-server/internal/push"
	"canepanion-server/middleware"
)

func RunGin(config cors.Config, notifier push.Notifier, voiceJob *audio.VoiceJob) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()

	r.Use(cors.New(config))

	if os.Getenv("GIN_MODE") == "release" {
		err := r.SetTrustedProxies([]string{"127.0.0.1"})
		if err != nil {
			log.Fatalf("Failed to set trusted proxies: %v", err)
		}
	}

	r.Use(middleware.ErrorHandler())

	RegisterRoutes(r, DB, notifier, voiceJob)

	err := r.Run("0.0.0.0:" + port)
	if err != nil {
		log.Fatal("Failed to run the server")
	}
}
