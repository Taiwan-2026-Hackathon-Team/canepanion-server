package infra

import (
	"canepanion-server/internal/audio"
	"canepanion-server/internal/auth"
	"canepanion-server/internal/camera"
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

func RegisterRoutes(r *gin.Engine, DB *gorm.DB, notifier push.Notifier, voiceJob *audio.VoiceJob, cameraHub *camera.Hub) {
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "🩼 Canepanion Server is running"})
	})

	v1 := r.Group("/api/v1")

	// Baseline Feature Routes
	registerAuth(v1, DB)
	registerDevices(v1, DB)

	// cameraHandler is shared by the app-side WHEP routes below and the
	// firmware-side WHIP routes further down; both sit on the one
	// process-wide Hub.
	cameraHandler := camera.NewHandler(camera.NewService(devicecontrol.NewRepository(DB), cameraHub))
	registerCamera(v1, cameraHandler)

	// Firmware–Cloud API Routes
	firmware := v1.Group("/firmware")
	registerFirmwareAuth(firmware, DB)
	registerFirmwareTelemetry(firmware, DB, notifier)
	registerFirmwareAudio(firmware, DB, voiceJob)
	registerFirmwareControl(firmware, DB)
	registerFirmwareUpdates(firmware, DB)
	registerFirmwareCamera(firmware, cameraHandler)
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
	deviceHandler := devices.NewHandler(DB)
	commandHandler := devicecontrol.NewHandler(DB)

	deviceGrp := r.Group("/devices", middleware.JWTAuthMiddleware())
	{
		deviceGrp.POST("", deviceHandler.AddDevice)
		deviceGrp.POST("/:deviceId/commands", commandHandler.CreateCommand)
	}
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

func registerFirmwareAudio(r *gin.RouterGroup, DB *gorm.DB, voiceJob *audio.VoiceJob) {
	handler := audio.NewHandler(DB, voiceJob)

	audioGrp := r.Group("/devices/:deviceId/audio")
	audioGrp.Use(middleware.DeviceAuthMiddleware())
	{
		audioGrp.POST("/uploads", handler.CreateUpload)
		audioGrp.POST("/:audioId/complete", handler.CompleteUpload)
		audioGrp.GET("/:audioId", handler.GetAudio)
	}
}

func registerFirmwareControl(r *gin.RouterGroup, DB *gorm.DB) {
	handler := devicecontrol.NewHandler(DB)

	deviceGrp := r.Group("/devices/:deviceId", middleware.DeviceAuthMiddleware())
	{
		deviceGrp.GET("/config", handler.GetConfig)
		deviceGrp.GET("/commands", handler.ListCommands)
		deviceGrp.POST("/commands/:commandId/track", handler.TrackCommand)
	}
}

func registerFirmwareUpdates(r *gin.RouterGroup, DB *gorm.DB) {
	handler := firmwareupdates.NewHandler(DB)

	firmwareGrp := r.Group("/devices/:deviceId/firmware", middleware.DeviceAuthMiddleware())
	{
		firmwareGrp.GET("/latest", handler.GetLatest)
		firmwareGrp.POST("/report", handler.ReportFirmwareUpdate)
	}
}

// registerCamera wires the guardian-app WHEP endpoints.
func registerCamera(r *gin.RouterGroup, handler *camera.Handler) {
	deviceGrp := r.Group("/devices/:deviceId/camera", middleware.JWTAuthMiddleware())
	{
		deviceGrp.GET("", handler.GetStatus)
		deviceGrp.POST("/viewers", handler.CreateViewer)
		deviceGrp.DELETE("/viewers/:viewerId", handler.DeleteViewer)
		deviceGrp.PATCH("/viewers/:viewerId", handler.RejectPatch)
	}
}

// registerFirmwareCamera wires the cane's WHIP publication endpoints.
func registerFirmwareCamera(r *gin.RouterGroup, handler *camera.Handler) {
	deviceGrp := r.Group("/devices/:deviceId/camera", middleware.DeviceAuthMiddleware())
	{
		deviceGrp.POST("/publications", handler.CreatePublication)
		deviceGrp.DELETE("/publications/:publicationId", handler.DeletePublication)
		deviceGrp.PATCH("/publications/:publicationId", handler.RejectPatch)
	}
}
