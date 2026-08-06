package models

type Role string

const (
	RoleAdmin    Role = "ADMIN"
	RoleCaneUser Role = "CANE_USER"
	RoleGuardian Role = "GUARDIAN"
)

type DeviceStatus string

const (
	DeviceStatusOnline   DeviceStatus = "ONLINE"
	DeviceStatusOffline  DeviceStatus = "OFFLINE"
	DeviceStatusInactive DeviceStatus = "INACTIVE"
)

type DeviceCommandType string

const (
	DeviceCommandTypePlayMessage       DeviceCommandType = "PLAY_MESSAGE"
	DeviceCommandTypeRequestLocation   DeviceCommandType = "REQUEST_LOCATION"
	DeviceCommandTypeStartAudioCapture DeviceCommandType = "START_AUDIO_CAPTURE"
	DeviceCommandTypeUpdateConfig      DeviceCommandType = "UPDATE_CONFIG"
	DeviceCommandTypeReboot            DeviceCommandType = "REBOOT"
	DeviceCommandTypeFirmwareUpdate    DeviceCommandType = "FIRMWARE_UPDATE"
)

type DeviceCommandStatus string

const (
	DeviceCommandStatusPending   DeviceCommandStatus = "PENDING"
	DeviceCommandStatusReceived  DeviceCommandStatus = "RECEIVED"
	DeviceCommandStatusCompleted DeviceCommandStatus = "COMPLETED"
	DeviceCommandStatusFailed    DeviceCommandStatus = "FAILED"
)

type AudioDirection string

const (
	AudioDirectionUserToAssistant AudioDirection = "USER_TO_ASSISTANT"
	AudioDirectionAssistantToUser AudioDirection = "ASSISTANT_TO_USER"
)

type AudioStatus string

const (
	AudioStatusUploaded   AudioStatus = "UPLOADED"
	AudioStatusProcessing AudioStatus = "PROCESSING"
	AudioStatusCompleted  AudioStatus = "COMPLETED"
	AudioStatusFailed     AudioStatus = "FAILED"
)

type SensorEventType string

const (
	SensorEventTypeFallDetected     SensorEventType = "FALL_DETECTED"
	SensorEventTypeObstacleDetected SensorEventType = "OBSTACLE_DETECTED"
	SensorEventTypeSOSTriggered     SensorEventType = "SOS_TRIGGERED"
	SensorEventTypeLowBattery       SensorEventType = "LOW_BATTERY"
	SensorEventTypeDeviceStarted    SensorEventType = "DEVICE_STARTED"
	SensorEventTypeDeviceError      SensorEventType = "DEVICE_ERROR"
)

type EventSeverity string

const (
	EventSeverityInfo     EventSeverity = "INFO"
	EventSeverityWarning  EventSeverity = "WARNING"
	EventSeverityCritical EventSeverity = "CRITICAL"
)

type AlertType string

const (
	AlertTypeFall          AlertType = "FALL"
	AlertTypeSOS           AlertType = "SOS"
	AlertTypeLowBattery    AlertType = "LOW_BATTERY"
	AlertTypeDeviceOffline AlertType = "DEVICE_OFFLINE"
)

type AlertStatus string

const (
	AlertStatusActive       AlertStatus = "ACTIVE"
	AlertStatusAcknowledged AlertStatus = "ACKNOWLEDGED"
	AlertStatusResolved     AlertStatus = "RESOLVED"
)
