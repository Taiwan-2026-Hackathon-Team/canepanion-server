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

## Priority 1: Safety telemetry ingestion

The first milestone should receive and durably store firmware telemetry. Alert
creation is a server responsibility: firmware reports a sensor event, and the
server atomically creates an alert when the event matches an alert rule.

| Order | Method | Endpoint | Purpose | Writes |
| --- | --- | --- | --- | --- |
| 1 | `POST` | `/api/v1/firmware/devices/activate` | Provision a physical device and issue device credentials. | `devices` |
| 2 | `POST` | `/api/v1/firmware/session` | Exchange device credentials for a short-lived device token. | — |
| 3 | `POST` | `/api/v1/firmware/devices/{deviceId}/telemetry` | Receive a batch of sensor events and location samples, including data buffered while offline. | `sensor_events`, `locations`, and conditionally `alerts` |
| 4 | `POST` | `/api/v1/firmware/devices/{deviceId}/heartbeat` | Update connectivity, battery, firmware version, and last-seen time. | `devices` |
| 5 | `POST` | `/api/v1/firmware/devices/{deviceId}/audio/uploads` | Register audio metadata and obtain a temporary object-storage upload URL. | `audio` |
| 6 | `POST` | `/api/v1/firmware/devices/{deviceId}/audio/{audioId}/complete` | Confirm the object upload and make the recording available for processing. | `audio` |

The combined telemetry endpoint is preferred over one request per reading
because a cane may reconnect with several buffered records. Individual event
and location routes can be added later if they are useful to other clients.
Although these are firmware-to-cloud writes, their responses carry server time
and acceptance state back to firmware; cloud-initiated actions are covered in
Priority 2.

### Submit telemetry

`POST /api/v1/firmware/devices/{deviceId}/telemetry`

```json
{
  "messageId": "01JAZA4Q6G8M3V7R2Y51N9P0KD",
  "sentAt": "2026-07-28T08:32:10Z",
  "events": [
    {
      "eventId": "01JAZA2P3G7YZQ0NQKFS4J8T1A",
      "eventType": "FALL_DETECTED",
      "severity": "CRITICAL",
      "recordedAt": "2026-07-28T08:31:42Z",
      "eventData": {
        "impactG": 3.8,
        "orientationChangeDegrees": 79,
        "confidence": 0.94
      },
      "location": {
        "locationId": "01JAZA2P4AHBRXD2G6VKWN8JSC",
        "latitude": 25.033,
        "longitude": 121.5654,
        "accuracyMeters": 8.2,
        "recordedAt": "2026-07-28T08:31:40Z"
      }
    }
  ],
  "locations": [
    {
      "locationId": "01JAZA35XE2PP7MTDKRN5CHVJ9",
      "latitude": 25.0331,
      "longitude": 121.5655,
      "accuracyMeters": 7.6,
      "recordedAt": "2026-07-28T08:31:55Z"
    }
  ]
}
```

`messageId` identifies the batch. `eventId` and `locationId` identify records
inside it and allow a partially accepted batch to be retried without creating
duplicates. These external identifiers require the schema additions described
below. A location nested in an event is stored as a normal `locations` row. To
query that location directly from the event, add the optional event-to-location
foreign key described under [Suggested model additions](#suggested-model-additions);
otherwise the server can retain the location ID in `event_data`.

The server returns `200 OK` when every item was accepted and `207 Multi-Status`
when items have mixed results:

```json
{
  "messageId": "01JAZA4Q6G8M3V7R2Y51N9P0KD",
  "serverTime": "2026-07-28T08:32:11Z",
  "events": [
    {
      "eventId": "01JAZA2P3G7YZQ0NQKFS4J8T1A",
      "status": "STORED",
      "sensorEventId": "eb2a4914-fb5c-41cc-aa30-91ff8897ee8d",
      "alertId": "9e6047f1-03ea-486b-9278-27a69e970ca1"
    }
  ],
  "locations": [
    {
      "locationId": "01JAZA35XE2PP7MTDKRN5CHVJ9",
      "status": "STORED",
      "locationRecordId": "62cacbc3-9bb8-4ed5-8993-a6cb811f76d5"
    }
  ]
}
```

Valid item statuses are `STORED`, `DUPLICATE`, and `REJECTED`. A rejected item
includes an `error` object. The server must commit a safety event and any alert
derived from it in the same database transaction.

### Alert creation rules

The firmware must not call a separate "create alert" endpoint. This prevents a
client from supplying arbitrary alert text or producing an alert without its
required `sensor_event_id`.

| Sensor event | Default server action |
| --- | --- |
| `FALL_DETECTED` with `CRITICAL` severity | Create an active `FALL` alert. |
| `SOS_TRIGGERED` | Create an active `SOS` alert. |
| `LOW_BATTERY` with `WARNING` or `CRITICAL` severity | Create an active `LOW_BATTERY` alert. |
| `OBSTACLE_DETECTED`, `DEVICE_STARTED`, `DEVICE_ERROR` | Store the event; do not create an alert under the current alert enum. |

Alert messages should be generated from server-owned templates. Alert rules
should be configurable later, but their output must remain limited to the
`AlertType` values defined by the schema.

### Heartbeat

`POST /api/v1/firmware/devices/{deviceId}/heartbeat`

```json
{
  "messageId": "01JAZA5WDWY1V7BXWQZY39B8KX",
  "recordedAt": "2026-07-28T08:32:30Z",
  "batteryLevel": 74,
  "firmwareVersion": "1.4.2",
  "status": "ONLINE"
}
```

The server updates `devices.battery_level`, `devices.firmware_version`,
`devices.status`, and `devices.last_seen_at`, then returns:

```json
{
  "serverTime": "2026-07-28T08:32:31Z",
  "nextHeartbeatSeconds": 60
}
```

### Store audio metadata

Audio bytes should be uploaded directly to object storage. The API creates the
metadata record and controls the `storage_key`; firmware must never choose an
arbitrary storage key.

`POST /api/v1/firmware/devices/{deviceId}/audio/uploads`

```json
{
  "messageId": "01JAZA6VWBWMCBVY7D6Z3Q8F1K",
  "direction": "USER_TO_ASSISTANT",
  "contentType": "audio/opus",
  "byteLength": 48192,
  "durationMs": 6200,
  "sha256": "95a5a4f4f77c26bc7dfd74bce683031bc76e24260b82e0c55a12f94d17b5a9d1",
  "recordedAt": "2026-07-28T08:32:42Z"
}
```

```json
{
  "audioId": "40fb49ee-64fb-4a66-a3fd-c89fcdc097e1",
  "uploadUrl": "https://object-storage.example/signed-upload",
  "expiresAt": "2026-07-28T08:47:43Z",
  "requiredHeaders": {
    "Content-Type": "audio/opus"
  }
}
```

After the upload, firmware calls
`POST /api/v1/firmware/devices/{deviceId}/audio/{audioId}/complete` with the
same `messageId` and checksum. The upload record begins in
`PENDING_UPLOAD`; after verifying the object, the server changes it to
`UPLOADED` and queues processing. `PENDING_UPLOAD` and the additional metadata
fields require the audio schema extension described below.

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

## Priority 3: Firmware updates

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
| 3 | Batched telemetry ingestion and alert derivation | Stores safety events and locations, preserves offline data, and enables fall and SOS alerts. |
| 4 | Audio metadata and signed upload | Stores recording metadata without routing large files through the application server. |
| 5 | Configuration synchronization | Allows behavior to be adjusted without reflashing firmware. |
| 6 | Commands and acknowledgements | Enables reliable cloud-to-device actions. |
| 7 | Firmware updates | Adds controlled remote software delivery after the core protocol is stable. |

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
| Unique `sensor_events.external_event_id` per device | Prevents duplicate event ingestion during firmware retries. |
| Nullable `sensor_events.location_id` | Links a safety event to its relevant `locations` row without duplicating coordinates in `event_data`. |
| Unique `locations.external_location_id` per device | Prevents duplicate location ingestion during firmware retries. |
| `ingestion_batches.message_id` per device | Records batch-level idempotency and the response returned for a retry. |
| `audio.message_id` | Prevents duplicate audio records during firmware retries. |
| `audio.content_type`, `byte_length`, `duration_ms`, `sha256`, `recorded_at` | Persists the audio metadata sent by firmware. |
| `PENDING_UPLOAD` audio status | Distinguishes an issued signed URL from an object that was successfully uploaded. |
| `firmware_releases` | Stores signed firmware release metadata. |
| `firmware_installations` | Tracks update status for each device. |
