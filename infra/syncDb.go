package infra

import (
	"canepanion-server/models"
	"log"
)

func SyncDatabase() {
	log.Println("Syncing declared database schema...")

	err := DB.AutoMigrate(
		&models.Users{},
		&models.Devices{},
		&models.Locations{},
		&models.Audio{},
		&models.SensorEvents{},
		&models.Alerts{},
		&models.Notifications{},
		&models.IngestionBatches{},
		&models.DeviceCommands{},
	)
	if err != nil {
		log.Fatalf("failed to auto-migrate database schema: %v", err)
	}

	log.Println("Database migration completed successfully")
}
