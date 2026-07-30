package telemetry

import (
	"encoding/json"
	"strings"
	"time"

	"canepanion-server/models"
	appErr "canepanion-server/pkg/errors"
	"canepanion-server/pkg/utils"

	"github.com/google/uuid"
)

const (
	maxBatchEvents    = 100
	maxBatchLocations = 100
	maxClockSkew      = 5 * time.Minute
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SubmitTelemetry(deviceIDValue string, req *SubmitTelemetryRequest) (*SubmitTelemetryResponse, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}

	device, err := s.repo.FindDeviceByID(deviceID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find device", err)
	}
	if device == nil {
		return nil, appErr.NewNotFound("Device not found", nil)
	}
	if strings.TrimSpace(req.MessageID) == "" {
		return nil, appErr.NewBadRequest("Invalid messageId", nil)
	}

	if len(req.Events) == 0 && len(req.Locations) == 0 {
		return nil, appErr.NewBadRequest("Telemetry batch must include at least one event or location", nil)
	}
	if len(req.Events) > maxBatchEvents || len(req.Locations) > maxBatchLocations {
		return nil, appErr.NewBadRequest("Telemetry batch exceeds the item limit", nil)
	}

	now := time.Now().UTC().Truncate(time.Second)
	if req.SentAt.IsZero() {
		return nil, appErr.NewBadRequest("sentAt is required and must be an RFC3339 timestamp", nil)
	}
	if req.SentAt.After(now.Add(maxClockSkew)) {
		return nil, appErr.NewBadRequest("sentAt cannot be more than 5 minutes in the future", nil)
	}

	existingResponse, err := s.repo.FindBatchResponse(deviceID, req.MessageID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to check telemetry batch", err)
	}
	if existingResponse != nil {
		response, err := decodeTelemetryResponse(existingResponse)
		if err != nil {
			return nil, appErr.NewInternal("Failed to restore telemetry batch response", err)
		}
		return response, nil
	}

	response := &SubmitTelemetryResponse{
		MessageID:  req.MessageID,
		Status:     SubmitTelemetryStatusAccepted,
		ServerTime: now,
	}

	for i := range req.Events {
		eventRequest := &req.Events[i]
		eventResult, nestedLocationResult := s.storeEvent(deviceID, eventRequest, now)
		if eventResult.Status == ItemStatusRejected ||
			(nestedLocationResult != nil && nestedLocationResult.Status == ItemStatusRejected) {
			response.Status = SubmitTelemetryStatusPartiallyAccepted
		}
	}

	for i := range req.Locations {
		locationResult := s.storeLocation(deviceID, &req.Locations[i], now)
		if locationResult.Status == ItemStatusRejected {
			response.Status = SubmitTelemetryStatusPartiallyAccepted
		}
	}

	encodedResponse, err := json.Marshal(response)
	if err != nil {
		return nil, appErr.NewInternal("Failed to encode telemetry response", err)
	}

	persistedResponse, err := s.repo.SaveBatchResponse(&models.IngestionBatches{
		ID:        utils.GenerateUUID(),
		DeviceID:  deviceID,
		MessageID: req.MessageID,
		Response:  encodedResponse,
	})
	if err != nil {
		return nil, appErr.NewInternal("Failed to save telemetry batch", err)
	}

	finalResponse, err := decodeTelemetryResponse(persistedResponse)
	if err != nil {
		return nil, appErr.NewInternal("Failed to restore saved telemetry response", err)
	}
	return finalResponse, nil
}

func decodeTelemetryResponse(data json.RawMessage) (*SubmitTelemetryResponse, error) {
	var stored struct {
		MessageID  string                `json:"messageId"`
		Status     SubmitTelemetryStatus `json:"status"`
		ServerTime time.Time             `json:"serverTime"`
		Events     []EventResult         `json:"events"`
		Locations  []LocationResult      `json:"locations"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}

	status := stored.Status
	if status == "" {
		status = SubmitTelemetryStatusAccepted
		for _, event := range stored.Events {
			if event.Status == ItemStatusRejected {
				status = SubmitTelemetryStatusPartiallyAccepted
				break
			}
		}
		if status == SubmitTelemetryStatusAccepted {
			for _, location := range stored.Locations {
				if location.Status == ItemStatusRejected {
					status = SubmitTelemetryStatusPartiallyAccepted
					break
				}
			}
		}
	}

	return &SubmitTelemetryResponse{
		MessageID:  stored.MessageID,
		Status:     status,
		ServerTime: stored.ServerTime,
	}, nil
}

func (s *Service) storeEvent(deviceID uuid.UUID, req *EventRequest, now time.Time) (EventResult, *LocationResult) {
	result := EventResult{EventID: req.EventID, Status: ItemStatusRejected}
	if validationError := validateEvent(req, now); validationError != nil {
		result.Error = validationError
		if req.Location != nil {
			return result, &LocationResult{
				LocationID: req.Location.LocationID,
				Status:     ItemStatusRejected,
				Error:      &ItemError{Code: "PARENT_EVENT_REJECTED", Message: "Location was not stored because its event was rejected"},
			}
		}
		return result, nil
	}

	var location *models.Locations
	if req.Location != nil {
		if validationError := validateLocation(req.Location, now); validationError != nil {
			result.Error = &ItemError{Code: "INVALID_EVENT_LOCATION", Message: validationError.Message}
			return result, &LocationResult{
				LocationID: req.Location.LocationID,
				Status:     ItemStatusRejected,
				Error:      validationError,
			}
		}
		location = newLocationModel(deviceID, req.Location)
	}

	event := &models.SensorEvents{
		ID:              utils.GenerateUUID(),
		DeviceID:        deviceID,
		ExternalEventID: req.EventID,
		EventType:       req.EventType,
		Severity:        req.Severity,
		EventData:       req.EventData,
		RecordedAt:      req.RecordedAt.UTC(),
	}
	alert := newAlertForEvent(deviceID, event, req)

	storedEvent, storedLocation, storedAlert, eventDuplicate, locationDuplicate, err := s.repo.StoreEvent(event, location, alert)
	if err != nil {
		result.Error = &ItemError{Code: "STORAGE_ERROR", Message: "Failed to store sensor event"}
		var locationResult *LocationResult
		if req.Location != nil {
			locationResult = &LocationResult{
				LocationID: req.Location.LocationID,
				Status:     ItemStatusRejected,
				Error:      &ItemError{Code: "STORAGE_ERROR", Message: "Failed to store event location"},
			}
		}
		return result, locationResult
	}

	result.Status = ItemStatusStored
	if eventDuplicate {
		result.Status = ItemStatusDuplicate
	}
	result.SensorEventID = uuidPointer(storedEvent.ID)
	if storedAlert != nil {
		result.AlertID = uuidPointer(storedAlert.ID)
	}

	var locationResult *LocationResult
	if req.Location != nil {
		locationResult = &LocationResult{LocationID: req.Location.LocationID, Status: ItemStatusStored}
		if locationDuplicate || eventDuplicate {
			locationResult.Status = ItemStatusDuplicate
		}
		if storedLocation != nil {
			locationResult.LocationRecordID = uuidPointer(storedLocation.ID)
		}
	}
	return result, locationResult
}

func (s *Service) storeLocation(deviceID uuid.UUID, req *LocationRequest, now time.Time) LocationResult {
	result := LocationResult{LocationID: req.LocationID, Status: ItemStatusRejected}
	if validationError := validateLocation(req, now); validationError != nil {
		result.Error = validationError
		return result
	}

	stored, duplicate, err := s.repo.StoreLocation(newLocationModel(deviceID, req))
	if err != nil {
		result.Error = &ItemError{Code: "STORAGE_ERROR", Message: "Failed to store location"}
		return result
	}

	result.Status = ItemStatusStored
	if duplicate {
		result.Status = ItemStatusDuplicate
	}
	result.LocationRecordID = uuidPointer(stored.ID)
	return result
}

func validateEvent(req *EventRequest, now time.Time) *ItemError {
	if strings.TrimSpace(req.EventID) == "" || len(req.EventID) > 100 {
		return &ItemError{Code: "INVALID_EVENT_ID", Message: "eventId is required and must not exceed 100 characters"}
	}
	if !validEventType(req.EventType) {
		return &ItemError{Code: "INVALID_EVENT_TYPE", Message: "Unknown eventType"}
	}
	if !validSeverity(req.Severity) {
		return &ItemError{Code: "INVALID_SEVERITY", Message: "Unknown severity"}
	}
	if req.RecordedAt.IsZero() || req.RecordedAt.After(now.Add(maxClockSkew)) {
		return &ItemError{Code: "INVALID_TIMESTAMP", Message: "recordedAt is missing or too far in the future"}
	}
	var eventData map[string]any
	if len(req.EventData) == 0 || json.Unmarshal(req.EventData, &eventData) != nil || eventData == nil {
		return &ItemError{Code: "INVALID_EVENT_DATA", Message: "eventData must be a JSON object"}
	}
	return nil
}

func validateLocation(req *LocationRequest, now time.Time) *ItemError {
	if strings.TrimSpace(req.LocationID) == "" || len(req.LocationID) > 100 {
		return &ItemError{Code: "INVALID_LOCATION_ID", Message: "locationId is required and must not exceed 100 characters"}
	}
	if req.Latitude < -90 || req.Latitude > 90 || req.Longitude < -180 || req.Longitude > 180 {
		return &ItemError{Code: "INVALID_COORDINATES", Message: "Latitude or longitude is outside its valid range"}
	}
	if req.AccuracyMeters < 0 {
		return &ItemError{Code: "INVALID_ACCURACY", Message: "accuracyMeters cannot be negative"}
	}
	if req.RecordedAt.IsZero() || req.RecordedAt.After(now.Add(maxClockSkew)) {
		return &ItemError{Code: "INVALID_TIMESTAMP", Message: "recordedAt is missing or too far in the future"}
	}
	return nil
}

func validEventType(value models.SensorEventType) bool {
	switch value {
	case models.SensorEventTypeFallDetected,
		models.SensorEventTypeObstacleDetected,
		models.SensorEventTypeSOSTriggered,
		models.SensorEventTypeLowBattery,
		models.SensorEventTypeDeviceStarted,
		models.SensorEventTypeDeviceError:
		return true
	default:
		return false
	}
}

func validSeverity(value models.EventSeverity) bool {
	switch value {
	case models.EventSeverityInfo, models.EventSeverityWarning, models.EventSeverityCritical:
		return true
	default:
		return false
	}
}

func newLocationModel(deviceID uuid.UUID, req *LocationRequest) *models.Locations {
	return &models.Locations{
		ID:                 utils.GenerateUUID(),
		DeviceID:           deviceID,
		ExternalLocationID: req.LocationID,
		Latitude:           req.Latitude,
		Longitude:          req.Longitude,
		AccuracyMeters:     req.AccuracyMeters,
		RecordedAt:         req.RecordedAt.UTC(),
	}
}

func newAlertForEvent(deviceID uuid.UUID, event *models.SensorEvents, req *EventRequest) *models.Alerts {
	alert := &models.Alerts{
		ID:       utils.GenerateUUID(),
		DeviceID: deviceID,
		Status:   models.AlertStatusActive,
	}

	switch {
	case req.EventType == models.SensorEventTypeFallDetected && req.Severity == models.EventSeverityCritical:
		alert.AlertType = models.AlertTypeFall
		alert.Message = "Critical fall detected"
	case req.EventType == models.SensorEventTypeSOSTriggered:
		alert.AlertType = models.AlertTypeSOS
		alert.Message = "SOS triggered"
	case req.EventType == models.SensorEventTypeLowBattery &&
		(req.Severity == models.EventSeverityWarning || req.Severity == models.EventSeverityCritical):
		alert.AlertType = models.AlertTypeLowBattery
		alert.Message = "Device reported low battery"
	default:
		return nil
	}

	alert.SensorEventID = event.ID
	return alert
}

func uuidPointer(value uuid.UUID) *uuid.UUID {
	return &value
}
