package telemetry

import (
	"encoding/json"

	"canepanion-server/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindDeviceByID(deviceID uuid.UUID) (*models.Devices, error) {
	var device models.Devices
	if err := r.db.First(&device, "id = ?", deviceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

func (r *Repository) FindBatchResponse(deviceID uuid.UUID, messageID string) (json.RawMessage, error) {
	var batch models.IngestionBatches
	if err := r.db.Where("device_id = ? AND message_id = ?", deviceID, messageID).First(&batch).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return batch.Response, nil
}

func (r *Repository) SaveBatchResponse(batch *models.IngestionBatches) (json.RawMessage, error) {
	result := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}, {Name: "message_id"}},
		DoNothing: true,
	}).Create(batch)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 1 {
		return batch.Response, nil
	}
	return r.FindBatchResponse(batch.DeviceID, batch.MessageID)
}

func (r *Repository) StoreLocation(location *models.Locations) (*models.Locations, bool, error) {
	var stored *models.Locations
	var duplicate bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		stored, duplicate, err = upsertLocation(tx, location)
		return err
	})
	return stored, duplicate, err
}

func (r *Repository) StoreEvent(
	event *models.SensorEvents,
	location *models.Locations,
	alert *models.Alerts,
) (*models.SensorEvents, *models.Locations, *models.Alerts, bool, bool, error) {
	var storedEvent *models.SensorEvents
	var storedLocation *models.Locations
	var storedAlert *models.Alerts
	var eventDuplicate bool
	var locationDuplicate bool

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing models.SensorEvents
		err := tx.Where(
			"device_id = ? AND external_event_id = ?",
			event.DeviceID,
			event.ExternalEventID,
		).First(&existing).Error
		if err == nil {
			storedEvent = &existing
			eventDuplicate = true
			if existing.LocationID != nil {
				var existingLocation models.Locations
				if err := tx.First(&existingLocation, "id = ?", *existing.LocationID).Error; err != nil {
					return err
				}
				storedLocation = &existingLocation
				locationDuplicate = true
			}
			var existingAlert models.Alerts
			if err := tx.Where("sensor_event_id = ?", existing.ID).First(&existingAlert).Error; err == nil {
				storedAlert = &existingAlert
			} else if err != gorm.ErrRecordNotFound {
				return err
			}
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}

		if location != nil {
			storedLocation, locationDuplicate, err = upsertLocation(tx, location)
			if err != nil {
				return err
			}
			event.LocationID = &storedLocation.ID
		}

		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "device_id"}, {Name: "external_event_id"}},
			DoNothing: true,
		}).Create(event)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			eventDuplicate = true
			var raced models.SensorEvents
			if err := tx.Where("device_id = ? AND external_event_id = ?", event.DeviceID, event.ExternalEventID).First(&raced).Error; err != nil {
				return err
			}
			storedEvent = &raced
			if raced.LocationID != nil {
				var racedLocation models.Locations
				if err := tx.First(&racedLocation, "id = ?", *raced.LocationID).Error; err != nil {
					return err
				}
				storedLocation = &racedLocation
				locationDuplicate = true
			}
			var racedAlert models.Alerts
			if err := tx.Where("sensor_event_id = ?", raced.ID).First(&racedAlert).Error; err == nil {
				storedAlert = &racedAlert
			} else if err != gorm.ErrRecordNotFound {
				return err
			}
			return nil
		}

		storedEvent = event
		if alert != nil {
			alert.DeviceID = event.DeviceID
			alert.SensorEventID = event.ID
			if err := tx.Create(alert).Error; err != nil {
				return err
			}
			storedAlert = alert
		}
		return nil
	})

	return storedEvent, storedLocation, storedAlert, eventDuplicate, locationDuplicate, err
}

func upsertLocation(tx *gorm.DB, location *models.Locations) (*models.Locations, bool, error) {
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}, {Name: "external_location_id"}},
		DoNothing: true,
	}).Create(location)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return location, false, nil
	}

	var existing models.Locations
	if err := tx.Where(
		"device_id = ? AND external_location_id = ?",
		location.DeviceID,
		location.ExternalLocationID,
	).First(&existing).Error; err != nil {
		return nil, false, err
	}
	return &existing, true, nil
}
