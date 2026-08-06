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
	FindPendingCommands(deviceID uuid.UUID, cursor *commandCursor, limit int, now time.Time) ([]models.DeviceCommands, bool, error)
}

type Service struct {
	repo repository
	now  func() time.Time
}

func NewService(repo repository) *Service {
	return &Service{repo: repo, now: time.Now}
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
