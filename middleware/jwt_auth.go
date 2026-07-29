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
		authHeader := c.GetHeader("Authorization")

		var jwt_secret = []byte(os.Getenv("JWT_SECRET"))
		var tokenStr string

		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			cookieToken, err := c.Cookie("jwt")
			if err != nil {
				appError := appErr.NewUnauthorized("Unauthorized. Missing token at the cookie or header", err)
				c.JSON(appError.Code, gin.H{"error": appError.Message})
				c.Abort()
				return
			}

			tokenStr = cookieToken
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
