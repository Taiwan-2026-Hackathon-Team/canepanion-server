package middleware

import (
	"net/http"
	"strings"

	"canepanion-server/pkg/utils"

	"github.com/gin-gonic/gin"
)

func DeviceAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		const bearerPrefix = "Bearer "

		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing device bearer token"})
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing device bearer token"})
			return
		}

		claims, err := utils.ParseDeviceToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired device token"})
			return
		}

		pathDeviceID := c.Param("deviceId")
		if pathDeviceID == "" || claims.DeviceID != pathDeviceID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Device token cannot access this device"})
			return
		}

		c.Set("deviceID", claims.DeviceID)
		c.Set("deviceClaims", claims)
		c.Next()
	}
}
