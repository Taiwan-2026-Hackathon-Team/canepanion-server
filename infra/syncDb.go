package infra

import (
	"log"
	// "canepanion-server/models"
)

func SyncDatabase() {
	log.Println("Syncing declared database schema...")

	err := DB.AutoMigrate(
		// TODO: add models here after being declared on models/*
		// &models.Users{},
	)
	if err != nil {
		log.Fatalf("failed to auto-migrate database schema: %v", err)
	}

	log.Println("Database migration completed successfully")
}
