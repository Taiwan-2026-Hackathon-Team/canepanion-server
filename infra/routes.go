package infra

import (
	"github.com/gin-gonic/gin"

	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, DB *gorm.DB) {
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "🩼 Canepanion Server is running"})
	})

	v1 := r.Group("/api/v1")

	// Baseline Feature Routes
	registerDevices(v1, DB)

	// Firmware–Cloud API Routes
	firmware := v1.Group("/firmware")
	registerFirmwareAuth(firmware, DB)
	registerFirmwareTelemetry(firmware, DB)
	registerFirmwareAudio(firmware, DB)
	registerFirmwareControl(firmware, DB)
	registerFirmwareUpdates(firmware, DB)
}

func registerDevices(r *gin.RouterGroup, DB *gorm.DB) {
	// devices := device.NewHandler(DB)

	{
		r.POST("/devices")
	}
}

func registerFirmwareAuth(r *gin.RouterGroup, DB *gorm.DB) {
	// firmwareAuth := firmwareauth.NewHandler(DB)

	r.POST("/devices/activate")
	r.POST("/session")
	r.POST("/devices/:deviceId/heartbeat")
}

func registerFirmwareTelemetry(r *gin.RouterGroup, DB *gorm.DB) {
	// telemetry := telemetry.NewHandler(DB)

	r.POST("/devices/:deviceId/telemetry")
}

func registerFirmwareAudio(r *gin.RouterGroup, DB *gorm.DB) {
	// audio := audio.NewHandler(DB)

	audioGrp := r.Group("/devices/:deviceId/audio")
	{
		audioGrp.POST("/uploads")
		audioGrp.POST("/:audioId/complete")
	}
}

func registerFirmwareControl(r *gin.RouterGroup, DB *gorm.DB) {
	// control := devicecontrol.NewHandler(DB)

	deviceGrp := r.Group("/devices/:deviceId")
	{
		deviceGrp.GET("/config")
		deviceGrp.GET("/commands")
		deviceGrp.POST("/commands/:commandId/ack")
	}
}

func registerFirmwareUpdates(r *gin.RouterGroup, DB *gorm.DB) {
	// updates := firmwareupdates.NewHandler(DB)

	firmwareGrp := r.Group("/devices/:deviceId/firmware")
	{
		firmwareGrp.GET("/latest")
		firmwareGrp.POST("/report")
	}
}
