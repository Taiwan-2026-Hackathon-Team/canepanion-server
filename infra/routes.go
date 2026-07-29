package infra

import (
	"canepanion-server/internal/audio"
	devicecontrol "canepanion-server/internal/device_controls"
	"canepanion-server/internal/devices"
	firmwareauth "canepanion-server/internal/firmware_auth"
	firmwareupdates "canepanion-server/internal/firmware_updates"
	"canepanion-server/internal/telemetry"

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
	handler := devices.NewHandler(DB)

	r.POST("/devices", handler.Create)
}

func registerFirmwareAuth(r *gin.RouterGroup, DB *gorm.DB) {
	handler := firmwareauth.NewHandler(DB)

	r.POST("/devices/activate", handler.Activate)
	r.POST("/session", handler.CreateSession)
	r.POST("/devices/:deviceId/heartbeat", handler.Heartbeat)
}

func registerFirmwareTelemetry(r *gin.RouterGroup, DB *gorm.DB) {
	handler := telemetry.NewHandler(DB)

	r.POST("/devices/:deviceId/telemetry", handler.Submit)
}

func registerFirmwareAudio(r *gin.RouterGroup, DB *gorm.DB) {
	handler := audio.NewHandler(DB)

	audioGrp := r.Group("/devices/:deviceId/audio")
	{
		audioGrp.POST("/uploads", handler.CreateUpload)
		audioGrp.POST("/:audioId/complete", handler.CompleteUpload)
	}
}

func registerFirmwareControl(r *gin.RouterGroup, DB *gorm.DB) {
	handler := devicecontrol.NewHandler(DB)

	deviceGrp := r.Group("/devices/:deviceId")
	{
		deviceGrp.GET("/config", handler.GetConfig)
		deviceGrp.GET("/commands", handler.ListCommands)
		deviceGrp.POST("/commands/:commandId/ack", handler.AcknowledgeCommand)
	}
}

func registerFirmwareUpdates(r *gin.RouterGroup, DB *gorm.DB) {
	handler := firmwareupdates.NewHandler(DB)

	firmwareGrp := r.Group("/devices/:deviceId/firmware")
	{
		firmwareGrp.GET("/latest", handler.GetLatest)
		firmwareGrp.POST("/report", handler.Report)
	}
}
