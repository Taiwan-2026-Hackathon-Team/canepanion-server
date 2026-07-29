package devices

import (
	"canepanion-server/models"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateDevice(device *models.Devices) error {
	return r.db.Create(device).Error
}
