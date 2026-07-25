package main

import (
	"canepanion-server/config"
	"canepanion-server/infra"
)

func main() {
	config.LoadEnvVariables()

	// add --migrate in running Go if it needs db migration
	config.HandleMigrationFlag()

	infra.ConnectDb()

	infra.RunGin(config.CORS())
}