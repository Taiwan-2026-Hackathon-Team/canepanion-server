package telemetry

import (
	"encoding/json"
	"time"

	"canepanion-server/models"

	"github.com/google/uuid"
)

type ItemStatus string
type SubmitTelemetryStatus string

const (
	ItemStatusStored    ItemStatus = "STORED"
	ItemStatusDuplicate ItemStatus = "DUPLICATE"
	ItemStatusRejected  ItemStatus = "REJECTED"

	SubmitTelemetryStatusAccepted          SubmitTelemetryStatus = "ACCEPTED"
	SubmitTelemetryStatusPartiallyAccepted SubmitTelemetryStatus = "PARTIALLY_ACCEPTED"
)

type SubmitTelemetryRequest struct {
	MessageID string            `json:"messageId" binding:"required,max=100"`
	SentAt    time.Time         `json:"sentAt" binding:"required"`
	Events    []EventRequest    `json:"events"`
	Locations []LocationRequest `json:"locations"`
}

type EventRequest struct {
	EventID    string                 `json:"eventId"`
	EventType  models.SensorEventType `json:"eventType"`
	Severity   models.EventSeverity   `json:"severity"`
	RecordedAt time.Time              `json:"recordedAt"`
	EventData  json.RawMessage        `json:"eventData"`
	Location   *LocationRequest       `json:"location,omitempty"`
}

type LocationRequest struct {
	LocationID     string    `json:"locationId"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	AccuracyMeters float64   `json:"accuracyMeters"`
	RecordedAt     time.Time `json:"recordedAt"`
}

type SubmitTelemetryResponse struct {
	MessageID  string                `json:"messageId"`
	Status     SubmitTelemetryStatus `json:"status"`
	ServerTime time.Time             `json:"serverTime"`
}

type EventResult struct {
	EventID       string     `json:"eventId"`
	Status        ItemStatus `json:"status"`
	SensorEventID *uuid.UUID `json:"sensorEventId,omitempty"`
	AlertID       *uuid.UUID `json:"alertId,omitempty"`
	Error         *ItemError `json:"error,omitempty"`
}

type LocationResult struct {
	LocationID       string     `json:"locationId"`
	Status           ItemStatus `json:"status"`
	LocationRecordID *uuid.UUID `json:"locationRecordId,omitempty"`
	Error            *ItemError `json:"error,omitempty"`
}

type ItemError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r *SubmitTelemetryResponse) HasRejectedItems() bool {
	return r.Status == SubmitTelemetryStatusPartiallyAccepted
}
