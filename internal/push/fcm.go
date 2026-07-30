package push

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// sendTimeout caps a single FCM round trip. Fall delivery is time-critical and
// the caller runs it off the ingest path, so failing fast beats hanging.
const sendTimeout = 10 * time.Second

// FCMNotifier publishes to Firebase Cloud Messaging (HTTP v1).
type FCMNotifier struct {
	client *messaging.Client
	topic  string
}

// NewFCMNotifier builds a notifier from the environment:
//
//	FIREBASE_CREDENTIALS_JSON  service account JSON, inline (checked first;
//	                           preferred on hosts that only offer env vars)
//	FIREBASE_CREDENTIALS_FILE  path to a service account JSON file
//	FCM_TOPIC                  topic override; defaults to "sos-alerts"
//
// With neither credential set it returns a disabled notifier and no error, so
// the server still boots for teammates who have not been given the service
// account. Those deliveries are logged and dropped.
func NewFCMNotifier(ctx context.Context) (Notifier, error) {
	var creds option.ClientOption

	switch {
	case strings.TrimSpace(os.Getenv("FIREBASE_CREDENTIALS_JSON")) != "":
		creds = option.WithCredentialsJSON([]byte(os.Getenv("FIREBASE_CREDENTIALS_JSON")))
	case strings.TrimSpace(os.Getenv("FIREBASE_CREDENTIALS_FILE")) != "":
		path := strings.TrimSpace(os.Getenv("FIREBASE_CREDENTIALS_FILE"))
		// A checked-in .env points at the service account, but not everyone
		// has downloaded it. Degrade to disabled rather than refusing to boot:
		// the rest of the API stays usable, and the log says exactly why.
		if _, err := os.Stat(path); err != nil {
			log.Printf("push: FIREBASE_CREDENTIALS_FILE %q is unreadable (%v)", path, err)
			log.Println("push: fall alerts will NOT reach guardian phones")
			return Disabled{}, nil
		}
		creds = option.WithCredentialsFile(path)
	default:
		log.Println("push: no Firebase credentials configured; fall alerts will NOT reach guardian phones")
		log.Println("push: set FIREBASE_CREDENTIALS_FILE or FIREBASE_CREDENTIALS_JSON to enable delivery")
		return Disabled{}, nil
	}

	app, err := firebase.NewApp(ctx, nil, creds)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase messaging: %w", err)
	}

	topic := strings.TrimSpace(os.Getenv("FCM_TOPIC"))
	if topic == "" {
		topic = TopicSOS
	}

	log.Printf("push: FCM enabled, publishing fall alerts to topic %q", topic)
	return &FCMNotifier{client: client, topic: topic}, nil
}

func (n *FCMNotifier) Enabled() bool { return true }

func (n *FCMNotifier) NotifyFall(ctx context.Context, alert FallAlert) error {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	body := fmt.Sprintf("%s reported a fall", alert.DeviceName)

	message := &messaging.Message{
		Topic: n.topic,
		// All values must be strings; the app parses lat/lon with Number().
		Data: map[string]string{
			"type":      "fall_sos",
			"eventId":   alert.EventID,
			"deviceId":  alert.DeviceName,
			"lat":       formatCoord(alert.Latitude),
			"lon":       formatCoord(alert.Longitude),
			"createdAt": alert.CreatedAt.UTC().Format(time.RFC3339),
		},
		// Deliberately data-only on Android: the app renders its own
		// alarm-style notification on a high-importance channel with a
		// full-screen intent. Adding a Notification block here would make the
		// OS draw a default-priority one instead and break tap-routing when
		// the app has been quit.
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		// iOS will not reliably wake a quit app for data-only pushes, so the
		// APNs alert block is required. time-sensitive breaks through Focus
		// modes; bypassing the silent switch needs a Critical Alerts
		// entitlement from Apple, which we do not have yet.
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority":  "10",
				"apns-push-type": "alert",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: "FALL DETECTED",
						Body:  body,
					},
					Sound:            "default",
					ContentAvailable: true,
					CustomData: map[string]interface{}{
						"interruption-level": "time-sensitive",
					},
				},
			},
		},
	}

	id, err := n.client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("send fall alert %s: %w", alert.EventID, err)
	}

	log.Printf("push: delivered fall alert %s for %s (fcm id %s)", alert.EventID, alert.DeviceName, id)
	return nil
}

// formatCoord renders a coordinate at the 6 decimal places (~0.1 m) the push
// contract uses. strconv avoids the exponent notation %v would produce for
// small magnitudes, which Number() would still parse but which reads badly in
// logs and in the Firebase console.
func formatCoord(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

// Disabled stands in when no credentials are configured. It keeps the ingest
// path identical whether or not push is wired up.
type Disabled struct{}

func (Disabled) Enabled() bool { return false }

func (Disabled) NotifyFall(_ context.Context, alert FallAlert) error {
	log.Printf("push: DISABLED, dropping fall alert %s for %s at %s,%s",
		alert.EventID, alert.DeviceName,
		formatCoord(alert.Latitude), formatCoord(alert.Longitude))
	return nil
}
