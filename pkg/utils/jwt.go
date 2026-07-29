package utils

import (
	"canepanion-server/models"
	appErr "canepanion-server/pkg/errors"

	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
)

type UserClaims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	jwt.StandardClaims
}

func GenerateJWT(user *models.Users) (string, error) {
	claims := jwt.MapClaims{
		"userId": user.ID.String(),
		"role":   user.Role,
		"exp":    time.Now().Add(time.Hour * 5).Unix(), //5 hour token expiration
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := os.Getenv("JWT_SECRET")

	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenStr, nil
}

func ParseJWT(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{},

		// Token Validation
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, appErr.NewUnauthorized("invalid signing method: %v",
					fmt.Errorf("unexpected alg: %v", token.Header["alg"]))
			}

			return []byte(os.Getenv("JWT_SECRET")), nil
		})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, appErr.NewUnauthorized("invalid token: %v", err)
}
