package http

import (
	appErr "canepanion-server/pkg/errors"
	"canepanion-server/pkg/utils"

	"github.com/gin-gonic/gin"
)

func ExtractTokenFromRequest(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:], nil
	}
	tokenStr, err := c.Cookie("jwt")
	if err != nil {
		return "", appErr.NewNotFound("missing token in header and cookie", err)
	}
	return tokenStr, nil
}

func ExtractUserIDFromContext(c *gin.Context) (string, error) {
	tokenStr, err := ExtractTokenFromRequest(c)
	if err != nil {
		return "", err
	}

	claims, err := utils.ParseJWT(tokenStr)
	if err != nil {
		return "", err
	}

	if claims.UserID == "" {
		return "", appErr.NewUnauthorized("userId not found in the token", nil)
	}

	return claims.UserID, nil
}
