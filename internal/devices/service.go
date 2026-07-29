package devices

import (
	"canepanion-server/models"
	appErr "canepanion-server/pkg/errors"
	"canepanion-server/pkg/utils"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) AddDevice(ownerUserID string, req *CreateDeviceRequest) (*CreateDeviceResponse, error) {
	ownerID, err := utils.ParseId(ownerUserID)
	if err != nil {
		return nil, appErr.NewUnauthorized("Invalid user ID in token", err)
	}

	guardianID, err := utils.ParseId(req.GuardianUserID)
	if err != nil {
		return nil, appErr.NewBadRequest("Invalid guardian user ID", err)
	}

	device := &models.Devices{
		ID:             utils.GenerateUUID(),
		OwnerUserID:    ownerID,
		GuardianUserID: guardianID,
		Name:           req.Name,
	}

	if err := s.repo.CreateDevice(device); err != nil {
		return nil, appErr.NewInternal("Failed to create device", err)
	}

	return newCreateDeviceResponse(device), nil
}
