# Firmware–Cloud API

This document summarizes the suggested web API endpoints for communication
between Canepanion firmware and the cloud server.

## Conventions

- Base path: `/api/v1/firmware`
- Content type: `application/json`, except file upload endpoints
- Authentication: device access token in `Authorization: Bearer <token>`
- Timestamps: UTC in RFC 3339 format
- Firmware-generated writes should include a unique `messageId` so retries do
  not create duplicate records.

## Priority 1: Core communication

These endpoints form the minimum useful firmware–cloud integration.

| Method | Endpoint | Authentication | Purpose | Related model |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/firmware/devices/activate` | Activation code | Register or provision a physical device and issue device credentials. | `devices` |
| `POST` | `/api/v1/firmware/session` | Device credentials | Exchange device credentials for a short-lived access token. | `devices` |
| `POST` | `/api/v1/firmware/devices/{deviceId}/heartbeat` | Device token | Report connectivity, battery level, firmware version, and device health. | `devices` |
| `POST` | `/api/v1/firmware/devices/{deviceId}/events` | Device token | Upload a sensor event such as a fall, obstacle, SOS, low battery, startup, or device error. | `sensor_events`, `alerts` |
| `POST` | `/api/v1/firmware/devices/{deviceId}/locations` | Device token | Upload one or more stored location samples. | `locations` |
| `POST` | `/api/v1/firmware/devices/{deviceId}/audio` | Device token | Upload a small audio recording and its metadata. | `audio` |

## Priority 2: Cloud-to-device control

These endpoints allow the cloud to configure devices and send actions to them.

| Method | Endpoint | Authentication | Purpose | Related model |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/firmware/devices/{deviceId}/config` | Device token | Download the current cloud-managed device configuration. Supports `ETag` and `If-None-Match`. | New `device_configurations` model |
| `GET` | `/api/v1/firmware/devices/{deviceId}/commands` | Device token | Poll for pending commands, optionally using a cursor. | New `device_commands` model |
| `POST` | `/api/v1/firmware/devices/{deviceId}/commands/{commandId}/ack` | Device token | Report that a command was received, completed, or failed. | New `device_commands` model |

Suggested command types include:

- `PLAY_MESSAGE`
- `REQUEST_LOCATION`
- `START_AUDIO_CAPTURE`
- `UPDATE_CONFIG`
- `REBOOT`
- `FIRMWARE_UPDATE`

## Priority 3: Audio object-storage flow

Use this flow when recordings are too large to send through the application
server.

| Method | Endpoint | Authentication | Purpose | Related model |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/firmware/devices/{deviceId}/audio/uploads` | Device token | Create an audio record and obtain a temporary object-storage upload URL. | `audio` |
| `PUT` | Temporary object-storage URL | Signed URL | Upload audio directly to object storage. | External storage |
| `POST` | `/api/v1/firmware/devices/{deviceId}/audio/{audioId}/complete` | Device token | Confirm that the upload finished and start processing. | `audio` |

## Priority 4: Firmware updates

| Method | Endpoint | Authentication | Purpose | Related model |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/firmware/devices/{deviceId}/firmware/latest` | Device token | Check for a firmware release compatible with the device hardware. | New `firmware_releases` model |
| `POST` | `/api/v1/firmware/devices/{deviceId}/firmware/report` | Device token | Report download, verification, installation, rollback, or failure status. | New `firmware_installations` model |

Firmware release responses should provide the version, file size, download URL,
SHA-256 digest, cryptographic signature, and whether the update is mandatory.

## Recommended implementation order

| Order | Capability | Reason |
| --- | --- | --- |
| 1 | Device activation and authentication | Establishes a secure identity for every request. |
| 2 | Heartbeat | Confirms basic two-way communication and device health. |
| 3 | Sensor events and alert creation | Enables fall and SOS safety functionality. |
| 4 | Batched location upload | Preserves samples through intermittent connectivity. |
| 5 | Audio upload | Enables voice interaction and processing. |
| 6 | Configuration synchronization | Allows behavior to be adjusted without reflashing firmware. |
| 7 | Commands and acknowledgements | Enables reliable cloud-to-device actions. |
| 8 | Firmware updates | Adds controlled remote software delivery after the core protocol is stable. |

## Required API behavior

| Requirement | Recommendation |
| --- | --- |
| Device authorization | A device token may access only the matching `deviceId`. Do not use a user JWT as device identity. |
| Retry safety | Store or uniquely constrain `messageId` and return the original result for repeated requests. |
| Offline operation | Accept batched, device-timestamped records after connectivity returns. |
| Time synchronization | Include `serverTime` in activation, session, and heartbeat responses. |
| Validation | Reject unknown enum values and invalid coordinates, timestamps, or payload sizes. |
| Versioning | Keep the version in the URL and optionally accept a firmware protocol-version header. |
| Error format | Return a stable error code, readable message, and retryable flag. |
| Transport | Require HTTPS in every deployed environment. |
| Large files | Prefer signed object-storage uploads instead of proxying large audio through Gin. |

## Suggested model additions

The existing schema covers telemetry, locations, audio, and alerts. The
following additions support reliable firmware communication:

| Model or field | Purpose |
| --- | --- |
| `devices.serial_number` | Stable manufacturing identity used during activation. |
| `devices.hardware_version` | Determines configuration and firmware compatibility. |
| `devices.protocol_version` | Tracks the API protocol understood by the firmware. |
| `devices.credential_hash` | Stores a non-reversible device credential representation. |
| `devices.configuration_version` | Allows efficient configuration synchronization. |
| `device_configurations` | Stores cloud-managed settings for each device. |
| `device_commands` | Stores commands, expiration, delivery, and completion state. |
| Unique `message_id` fields | Prevent duplicate ingestion during firmware retries. |
| `firmware_releases` | Stores signed firmware release metadata. |
| `firmware_installations` | Tracks update status for each device. |
