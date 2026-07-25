package config

import (
	"canepanion-server/infra"
	"flag"
	"log"
	"os"
)

func HandleMigrationFlag() {
	migrateFlag := flag.Bool("migrate", false, "run db migration and exit")
	flag.Parse()

	if *migrateFlag {
		infra.ConnectDb()

		log.Println("Starting automigrate...")
		infra.SyncDatabase()

		log.Println("Migration complete... exiting...")
		os.Exit(0)
	} else {
		log.Println("Skipping db migation (flag was not set)")
	}
}
