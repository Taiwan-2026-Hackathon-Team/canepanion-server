package caneingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeviceToken exchanges the device credential for a short-lived bearer token
// using the same endpoint the firmware uses (POST /api/v1/firmware/session).
//
// The token lives 15 minutes, so a long-running publication outlives it. That
// is fine: the relay only checks the token when the publication is created, so
// a fresh one is needed per (re)publish rather than on a refresh timer.
func DeviceToken(ctx context.Context, baseURL, deviceID, credential string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"deviceId":   deviceID,
		"credential": credential,
	})
	if err != nil {
		return "", err
	}

	endpoint := baseURL + "/api/v1/firmware/session"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("create device session: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("device session returned %s: %s", resp.Status, bytes.TrimSpace(payload))
	}

	var parsed struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", fmt.Errorf("decode device session: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", fmt.Errorf("device session returned no accessToken")
	}
	return parsed.AccessToken, nil
}
