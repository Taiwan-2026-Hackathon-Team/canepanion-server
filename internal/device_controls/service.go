package devicecontrol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"canepanion-server/models"
	appErr "canepanion-server/pkg/errors"
	"canepanion-server/pkg/utils"

	"github.com/google/uuid"
)

const (
	defaultCommandLimit = 50
	maxCommandLimit     = 100
)

type repository interface {
	FindConfigurationByDeviceID(deviceID uuid.UUID) (*DeviceConfiguration, error)
	FindDeviceByID(deviceID uuid.UUID) (*models.Devices, error)
	CreateCommand(command *models.DeviceCommands) error
	FindPendingCommands(deviceID uuid.UUID, cursor *commandCursor, limit int, now time.Time) ([]models.DeviceCommands, bool, error)
	FindCommandByID(deviceID, commandID uuid.UUID) (*models.DeviceCommands, error)
	UpdateCommandStatus(deviceID, commandID uuid.UUID, currentStatus, nextStatus models.DeviceCommandStatus) (bool, error)
}

type Service struct {
	repo repository
	now  func() time.Time
}

func NewService(repo repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) CreateCommand(userIDValue, deviceIDValue string, req *CreateCommandRequest) (*CreateCommandResponse, error) {
	userID, err := utils.ParseId(userIDValue)
	if err != nil {
		return nil, appErr.NewUnauthorized("Invalid user ID in token", err)
	}
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}
	if req == nil || !isValidCommandType(req.Type) {
		return nil, appErr.NewBadRequest("Invalid command type", nil)
	}
	if req.ExpiresAt.IsZero() || !req.ExpiresAt.After(s.now().UTC()) {
		return nil, appErr.NewBadRequest("expiresAt must be in the future", nil)
	}

	payload := req.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	} else if !json.Valid(payload) {
		return nil, appErr.NewBadRequest("payload must be valid JSON", nil)
	}

	device, err := s.repo.FindDeviceByID(deviceID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find device", err)
	}
	if device == nil {
		return nil, appErr.NewNotFound("Device not found", nil)
	}
	if !device.IsOwnerOrGuardian(userID) {
		return nil, appErr.NewForbidden("User cannot create commands for this device", nil)
	}

	command := &models.DeviceCommands{
		ID:        utils.GenerateUUID(),
		DeviceID:  deviceID,
		Type:      req.Type,
		Payload:   payload,
		Status:    models.DeviceCommandStatusPending,
		ExpiresAt: req.ExpiresAt.UTC(),
	}
	if err := s.repo.CreateCommand(command); err != nil {
		return nil, appErr.NewInternal("Failed to create device command", err)
	}

	return &CreateCommandResponse{
		CommandID: command.ID,
		DeviceID:  command.DeviceID,
		Type:      command.Type,
		Payload:   command.Payload,
		Status:    command.Status,
		ExpiresAt: command.ExpiresAt,
		CreatedAt: command.CreatedAt,
	}, nil
}

func isValidCommandType(commandType models.DeviceCommandType) bool {
	switch commandType {
	case models.DeviceCommandTypePlayMessage,
		models.DeviceCommandTypeRequestLocation,
		models.DeviceCommandTypeStartAudioCapture,
		models.DeviceCommandTypeUpdateConfig,
		models.DeviceCommandTypeReboot,
		models.DeviceCommandTypeFirmwareUpdate,
		models.DeviceCommandTypeStartCameraStream,
		models.DeviceCommandTypeStopCameraStream:
		return true
	default:
		return false
	}
}

func (s *Service) GetDeviceConfig(deviceIDValue string) (*GetConfigResponse, string, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, "", appErr.NewBadRequest("Invalid device ID", err)
	}

	config, err := s.repo.FindConfigurationByDeviceID(deviceID)
	if err != nil {
		return nil, "", appErr.NewInternal("Failed to retrieve device configuration", err)
	}
	if config == nil {
		return nil, "", appErr.NewNotFound("Device configuration not found", nil)
	}

	response := &GetConfigResponse{
		DeviceID:      config.DeviceID,
		Version:       config.Version,
		Configuration: config.Configuration,
	}
	return response, fmt.Sprintf(`"%d"`, config.Version), nil
}

type encodedCommandCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        uuid.UUID `json:"id"`
}

func (s *Service) ListCommands(deviceIDValue string, query *ListCommandsQuery) (*ListCommandsResponse, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}
	if query == nil {
		query = &ListCommandsQuery{}
	}

	limit := query.Limit
	if limit == 0 {
		limit = defaultCommandLimit
	}
	if limit < 1 || limit > maxCommandLimit {
		return nil, appErr.NewBadRequest("limit must be between 1 and 100", nil)
	}

	cursor, err := decodeCommandCursor(query.Cursor)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid command cursor", err)
	}

	commands, deviceExists, err := s.repo.FindPendingCommands(deviceID, cursor, limit+1, s.now().UTC())
	if err != nil {
		return nil, appErr.NewInternal("Failed to retrieve device commands", err)
	}
	if !deviceExists {
		return nil, appErr.NewNotFound("Device not found", nil)
	}
	if len(commands) > limit {
		commands = commands[:limit]
	}

	response := &ListCommandsResponse{
		Commands: make([]DeviceCommandResponse, 0, len(commands)),
	}
	for _, command := range commands {
		response.Commands = append(response.Commands, DeviceCommandResponse{
			CommandID: command.ID,
			Type:      command.Type,
			Payload:   command.Payload,
			CreatedAt: command.CreatedAt,
			ExpiresAt: command.ExpiresAt,
		})
	}

	if len(commands) > 0 {
		last := commands[len(commands)-1]
		response.NextCursor, err = encodeCommandCursor(last.CreatedAt, last.ID)
		if err != nil {
			return nil, appErr.NewInternal("Failed to create command cursor", err)
		}
	}

	return response, nil
}

func (s *Service) TrackCommand(deviceIDValue, commandIDValue string, req *TrackCommandRequest) (*TrackCommandResponse, error) {
	deviceID, err := utils.ParseId(deviceIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid device ID", err)
	}
	commandID, err := utils.ParseId(commandIDValue)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid command ID", err)
	}
	if req == nil || !isTrackableCommandStatus(req.Status) {
		return nil, appErr.NewBadRequest("status must be RECEIVED, COMPLETED, or FAILED", nil)
	}

	command, err := s.repo.FindCommandByID(deviceID, commandID)
	if err != nil {
		return nil, appErr.NewInternal("Failed to find device command", err)
	}
	if command == nil {
		return nil, appErr.NewNotFound("Device command not found", nil)
	}
	if command.Status != req.Status {
		if !canTransitionCommand(command.Status, req.Status) {
			return nil, appErr.NewBadRequest(fmt.Sprintf("command cannot transition from %s to %s", command.Status, req.Status), nil)
		}
		updated, err := s.repo.UpdateCommandStatus(deviceID, commandID, command.Status, req.Status)
		if err != nil {
			return nil, appErr.NewInternal("Failed to update device command status", err)
		}
		command, err = s.repo.FindCommandByID(deviceID, commandID)
		if err != nil {
			return nil, appErr.NewInternal("Failed to reload device command", err)
		}
		if command == nil {
			return nil, appErr.NewNotFound("Device command not found", nil)
		}
		if !updated && command.Status != req.Status {
			return nil, appErr.NewBadRequest(fmt.Sprintf("command cannot transition from %s to %s", command.Status, req.Status), nil)
		}
	}

	return &TrackCommandResponse{CommandID: command.ID, Status: command.Status, UpdatedAt: command.UpdatedAt}, nil
}

func isTrackableCommandStatus(status models.DeviceCommandStatus) bool {
	switch status {
	case models.DeviceCommandStatusReceived, models.DeviceCommandStatusCompleted, models.DeviceCommandStatusFailed:
		return true
	default:
		return false
	}
}

func canTransitionCommand(current, next models.DeviceCommandStatus) bool {
	switch current {
	case models.DeviceCommandStatusPending:
		return isTrackableCommandStatus(next)
	case models.DeviceCommandStatusReceived:
		return next == models.DeviceCommandStatusCompleted || next == models.DeviceCommandStatusFailed
	default:
		return false
	}
}

func encodeCommandCursor(createdAt time.Time, id uuid.UUID) (string, error) {
	value, err := json.Marshal(encodedCommandCursor{CreatedAt: createdAt.UTC(), ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func decodeCommandCursor(value string) (*commandCursor, error) {
	if value == "" {
		return nil, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}

	var cursor encodedCommandCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return nil, err
	}
	if cursor.CreatedAt.IsZero() || cursor.ID == uuid.Nil {
		return nil, fmt.Errorf("cursor is missing required fields")
	}

	return &commandCursor{CreatedAt: cursor.CreatedAt.UTC(), ID: cursor.ID}, nil
}
