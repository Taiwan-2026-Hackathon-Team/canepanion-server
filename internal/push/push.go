// Package push delivers guardian-facing alerts to phones.
//
// The wire format is fixed by the guardian app's push contract
// (cane-panion-client/docs/push-contract.md). The app is push-payload driven:
// everything the fall-alert screen renders comes from the Data map below, so
// changing a key here breaks the app silently. In particular the app drops any
// payload whose type is not exactly "fall_sos" or whose lat/lon do not parse
// as finite numbers.
package push

import (
	"context"
	"time"
)

// TopicSOS is the FCM topic every guardian phone subscribes to at startup.
// Matches SOS_TOPIC in the client's src/constants.ts.
const TopicSOS = "sos-alerts"

// FallAlert is everything the guardian app needs to render a fall.
type FallAlert struct {
	// EventID must be unique and stable per fall. The app dedupes on it, and
	// the firmware retries a fall up to 3x, so this must not change between
	// retries of the same event.
	EventID string

	// DeviceName is the human-facing cane identifier (e.g. "cane-panion-01"),
	// not the internal UUID: the app shows it in the notification body.
	DeviceName string

	Latitude  float64
	Longitude float64

	// CreatedAt is the server's ingest time. The cane has no RTC, so ingest
	// time is the event time.
	CreatedAt time.Time
}

// Notifier delivers fall alerts to guardian phones.
type Notifier interface {
	// NotifyFall delivers one alert. It is safe to call on a nil or disabled
	// notifier, which reports success without sending.
	NotifyFall(ctx context.Context, alert FallAlert) error

	// Enabled reports whether deliveries actually leave the process.
	Enabled() bool
}
