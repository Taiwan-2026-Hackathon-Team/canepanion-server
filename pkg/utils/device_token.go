package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
)

const DeviceTokenTTL = 15 * time.Minute

func GenerateDeviceCredential() (string, error) {
	credentialBytes := make([]byte, 32)
	if _, err := rand.Read(credentialBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(credentialBytes), nil
}

func GenerateDeviceToken(deviceID uuid.UUID) (string, time.Time, error) {
	secret := os.Getenv("DEVICE_JWT_SECRET")
	if secret == "" {
		return "", time.Time{}, fmt.Errorf("DEVICE_JWT_SECRET is not configured")
	}

	now := time.Now().UTC()
	expiresAt := now.Add(DeviceTokenTTL)
	claims := jwt.MapClaims{
		"deviceId":  deviceID.String(),
		"tokenType": "device",
		"iss":       "canepanion-server",
		"aud":       "canepanion-firmware",
		"iat":       now.Unix(),
		"exp":       expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}
