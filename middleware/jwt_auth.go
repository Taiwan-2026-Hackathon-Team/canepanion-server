package middleware

import (
	appErr "canepanion-server/pkg/errors"

	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		const bearerPrefix = "Bearer "

		var jwt_secret = []byte(os.Getenv("JWT_SECRET"))
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			appError := appErr.NewUnauthorized("Unauthorized. Missing bearer token", nil)
			c.JSON(appError.Code, gin.H{"error": appError.Message})
			c.Abort()
			return
		}

		tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
		if tokenStr == "" {
			appError := appErr.NewUnauthorized("Unauthorized. Missing bearer token", nil)
			c.JSON(appError.Code, gin.H{"error": appError.Message})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwt_secret, nil
		})

		if err != nil || !token.Valid {
			appError := appErr.NewUnauthorized("Invalid or token has expired", err)
			c.JSON(appError.Code, gin.H{"error": appError.Message})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			appError := appErr.NewUnauthorized("Invalid token claims", nil)
			c.JSON(appError.Code, gin.H{"error": appError.Message})
			c.Abort()
			return
		}

		userId, exists := claims["userId"]
		if !exists {
			appError := appErr.NewUnauthorized("Missing userId at the token claims", nil)
			c.JSON(appError.Code, gin.H{"error": appError.Message})
			c.Abort()
			return
		}

		c.Set("userID", userId)
		c.Set("claims", claims)
		c.Next()
	}

}
