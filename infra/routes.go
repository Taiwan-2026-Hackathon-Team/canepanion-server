package infra

import (
	"canepanion-server/internal/audio"
	"canepanion-server/internal/auth"
	devicecontrol "canepanion-server/internal/device_controls"
	"canepanion-server/internal/devices"
	firmwareauth "canepanion-server/internal/firmware_auth"
	firmwareupdates "canepanion-server/internal/firmware_updates"
	"canepanion-server/internal/push"
	"canepanion-server/internal/telemetry"

	"canepanion-server/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, DB *gorm.DB, notifier push.Notifier) {
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "🩼 Canepanion Server is running"})
	})

	v1 := r.Group("/api/v1")

	// Baseline Feature Routes
	registerAuth(v1, DB)
	registerDevices(v1, DB)

	// Firmware–Cloud API Routes
	firmware := v1.Group("/firmware")
	registerFirmwareAuth(firmware, DB)
	registerFirmwareTelemetry(firmware, DB, notifier)
	registerFirmwareAudio(firmware, DB)
	// registerFirmwareControl(firmware, DB)
	// registerFirmwareUpdates(firmware, DB)
}

func registerAuth(r *gin.RouterGroup, DB *gorm.DB) {
	auth := auth.NewHandler(DB)

	authGrp := r.Group("/auth")
	{
		authGrp.POST("/register", auth.SignUp)
		authGrp.POST("/login", auth.LogIn)
		authGrp.POST("/logout", auth.LogOut)
		authGrp.GET("/me", middleware.JWTAuthMiddleware(), auth.GetCurrentUser)
	}
}

func registerDevices(r *gin.RouterGroup, DB *gorm.DB) {
	handler := devices.NewHandler(DB)

	r.POST("/devices", middleware.JWTAuthMiddleware(), handler.AddDevice)
}

func registerFirmwareAuth(r *gin.RouterGroup, DB *gorm.DB) {
	handler := firmwareauth.NewHandler(DB)

	r.POST("/devices/activate", handler.ActivateDevice)
	r.POST("/session", handler.CreateSession)
	r.POST("/devices/:deviceId/heartbeat", middleware.DeviceAuthMiddleware(), handler.Heartbeat)
}

func registerFirmwareTelemetry(r *gin.RouterGroup, DB *gorm.DB, notifier push.Notifier) {
	handler := telemetry.NewHandler(DB, notifier)

	r.POST("/devices/:deviceId/telemetry", middleware.DeviceAuthMiddleware(), handler.Submit)
}

func registerFirmwareAudio(r *gin.RouterGroup, DB *gorm.DB) {
	handler := audio.NewHandler(DB)

	audioGrp := r.Group("/devices/:deviceId/audio")
	audioGrp.Use(middleware.DeviceAuthMiddleware())
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
