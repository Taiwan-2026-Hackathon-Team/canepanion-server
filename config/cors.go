package config

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
)

const defaultAllowedOrigin = "http://localhost:8081"

func CORS() cors.Config {
	return cors.Config{
		AllowOrigins:     allowedOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Location", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           4 * time.Hour,
	}
}

// allowedOrigins reads CORS_ALLOWED_ORIGINS as a comma-separated list,
// falling back to the local dev origin so existing deployments keep working.
func allowedOrigins() []string {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		return []string{defaultAllowedOrigin}
	}

	origins := make([]string, 0)
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return []string{defaultAllowedOrigin}
	}
	return origins
}
