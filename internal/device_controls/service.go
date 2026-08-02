package devicecontrol

import (
	"fmt"

	appErr "canepanion-server/pkg/errors"
	"canepanion-server/pkg/utils"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
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
