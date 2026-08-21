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
| 5 | `POST` | `/api/v1/firmware/devices/{deviceId}/audio/uploads` | Upload a voice clip (multipart) to Cloudinary and store metadata. | `audio` |
| 6 | `POST` | `/api/v1/firmware/devices/{deviceId}/audio/{audioId}/complete` | Mark the clip `PROCESSING` and start the STT → Gemini → TTS background job. | `audio` |
| 7 | `GET` | `/api/v1/firmware/devices/{deviceId}/audio/{audioId}` | Short-poll user-clip status; when `COMPLETED`, includes `replyAudioUrl`. | `audio` |

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

The firmware sends audio to the API as multipart form data. The API validates
the device and metadata, uploads the file to Cloudinary, and stores its public
ID as `storage_key`. The upload API accepts files up to **25 MB**.

For the **voice reply pipeline** (`USER_TO_ASSISTANT` → complete → poll), keep
user clips at or under **~10 MB**. Sync Speech-to-Text rejects larger content;
oversized clips that upload successfully still become `FAILED` after complete.
Record **Ogg Opus at 16000 Hz** with Content-Type `audio/opus`. The server STT
always uses `STT_SAMPLE_RATE_HZ` or **16000**; it does not read the file’s rate.
STT language defaults to `zh-TW` (`VOICE_LANGUAGE`). Empty or unrecognized
speech fails the job (`FAILED`).

`POST /api/v1/firmware/devices/{deviceId}/audio/uploads`

```bash
curl -X POST \
  -H "Authorization: Bearer <device-token>" \
  -F 'metadata={"direction":"USER_TO_ASSISTANT"};type=application/json' \
  -F 'audio=@sample.opus;type=audio/opus' \
  http://localhost:8080/api/v1/firmware/devices/<device-id>/audio/uploads
```

```json
{
  "audioId": "40fb49ee-64fb-4a66-a3fd-c89fcdc097e1",
  "deviceId": "6fdce032-90da-4a27-938b-9b5c367121f4",
  "direction": "USER_TO_ASSISTANT",
  "audioUrl": "https://res.cloudinary.com/example/video/upload/v1/canepanion/devices/.../audio/40fb49ee.opus",
  "storageKey": "canepanion/devices/6fdce032-90da-4a27-938b-9b5c367121f4/audio/40fb49ee-64fb-4a66-a3fd-c89fcdc097e1",
  "status": "UPLOADED",
  "createdAt": "2026-07-28T08:32:43Z"
}
```

The `metadata` part accepts `USER_TO_ASSISTANT` or `ASSISTANT_TO_USER`. The
`audio` part must use an `audio/*` content type. Device token TTL is
**15 minutes** (`POST /api/v1/firmware/session` to refresh). Path `{deviceId}`
must match the token’s device (**403** on mismatch); missing/invalid/expired
token returns **401**.

### Complete an audio upload

`POST /api/v1/firmware/devices/{deviceId}/audio/{audioId}/complete`

The endpoint requires a matching device token and no request body. It
transitions an `UPLOADED` recording to `PROCESSING` and starts the voice
pipeline background job (STT → Vertex Gemini → TTS → reply upload). The HTTP
response is **200** with body `{ audioId, status }` only — no `replyAudioUrl`.
Firmware must read the body `status`, not assume success from HTTP 200 alone.

```json
{
  "audioId": "40fb49ee-64fb-4a66-a3fd-c89fcdc097e1",
  "status": "PROCESSING"
}
```

| Body `status` | Firmware action |
| --- | --- |
| `PROCESSING` | Start polling GET below. |
| `COMPLETED` | Rare / idempotent. Complete has no URL — poll once or GET for `replyAudioUrl`. |
| `FAILED` | Hard stop. Do **not** poll. Start a new upload (new `audioId`) for another turn. |

Idempotent retries: if the clip is already `PROCESSING` or `COMPLETED`, the
server does not start a second job. Completing a clip that is already `FAILED`
returns **400** — not safe to retry; upload a new clip. If GET later shows
`UPLOADED`, complete was never applied — call complete or abort; do not play.

### Poll audio processing status

`GET /api/v1/firmware/devices/{deviceId}/audio/{audioId}`

`{audioId}` is always the **user clip** id (`USER_TO_ASSISTANT`). Requires a
matching device token. Firmware should poll every **2 seconds** and give up
after about **90 seconds** (aligned with the server voice job timeout),
starting only after complete returns `PROCESSING`.

While processing:

```json
{
  "audioId": "40fb49ee-64fb-4a66-a3fd-c89fcdc097e1",
  "status": "PROCESSING"
}
```

When the spoken reply is ready:

```json
{
  "audioId": "40fb49ee-64fb-4a66-a3fd-c89fcdc097e1",
  "status": "COMPLETED",
  "replyAudioUrl": "https://res.cloudinary.com/example/video/upload/v1/canepanion/devices/.../audio/reply.mp3"
}
```

On hard failure (detail is logged server-side only):

```json
{
  "audioId": "40fb49ee-64fb-4a66-a3fd-c89fcdc097e1",
  "status": "FAILED"
}
```

Play only when `status` is `COMPLETED` **and** `replyAudioUrl` is present and
non-empty. If `COMPLETED` without a URL, treat as failure and stop — do not
poll forever. `replyAudioUrl` is omitted unless a reply file exists. This
endpoint does not return transcript or assistant text for MVP.

## Priority 2: Cloud-to-device control

These endpoints allow the cloud to configure devices and send actions to them.

| Method | Endpoint | Authentication | Purpose | Related model |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/firmware/devices/{deviceId}/config` | Device token | Download the current cloud-managed device configuration. Supports `ETag` and `If-None-Match`. | New `device_configurations` model |
| `GET` | `/api/v1/firmware/devices/{deviceId}/commands` | Device token | Poll for pending commands, optionally using a cursor. | New `device_commands` model |
| `POST` | `/api/v1/firmware/devices/{deviceId}/commands/{commandId}/track` | Device token | Report that a command was received, completed, or failed. | New `device_commands` model |

Suggested command types include:

- `PLAY_MESSAGE`
- `REQUEST_LOCATION`
- `START_AUDIO_CAPTURE`
- `UPDATE_CONFIG`
- `REBOOT`
- `FIRMWARE_UPDATE`
- `START_CAMERA_STREAM`
- `STOP_CAMERA_STREAM`

## Priority 3: Firmware updates

| Method | Endpoint | Authentication | Purpose | Related model |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/firmware/devices/{deviceId}/firmware/latest` | Device token | Get the latest published release compatible with the device hardware. | `firmware_releases` |
| `POST` | `/api/v1/firmware/devices/{deviceId}/firmware/report` | Device token | Report download, verification, installation, rollback, or failure. | `firmware_installations`, and `devices` for `INSTALLED` |

### Get the latest compatible firmware

`GET /api/v1/firmware/devices/{deviceId}/firmware/latest`

The device token must match the path `deviceId`. The server reads
`devices.hardware_version`, selects releases with an exact hardware-version
match and `published_at` not later than server time, and returns the most
recently published release.

```json
{
  "releaseId": "9e6047f1-03ea-486b-9278-27a69e970ca1",
  "version": "1.4.2",
  "hardwareVersion": "HW-1",
  "downloadUrl": "https://example.com/firmware/HW-1/1.4.2.bin",
  "fileSizeBytes": 8388608,
  "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "signature": "base64-or-armored-signature",
  "mandatory": false,
  "publishedAt": "2026-08-11T08:00:00Z"
}
```

The endpoint returns `400` when hardware version is not configured, `404` when
the device or compatible release does not exist, `401` for an invalid device
token, and `403` when the token belongs to another device.

### Report firmware update progress

`POST /api/v1/firmware/devices/{deviceId}/firmware/report`

Each lifecycle event uses a unique `messageId`. `reportedAt` is the device event
time and cannot be more than five minutes in the future relative to server
time.

```json
{
  "messageId": "01K2FA4Q6G8M3V7R2Y51N9P0KD",
  "releaseId": "9e6047f1-03ea-486b-9278-27a69e970ca1",
  "status": "INSTALLED",
  "reportedAt": "2026-08-11T08:32:10Z"
}
```

Supported statuses are `DOWNLOADING`, `DOWNLOADED`, `VERIFYING`, `VERIFIED`,
`INSTALLING`, `INSTALLED`, `ROLLED_BACK`, and `FAILED`. A `FAILED` report must
include `error`; `error` is rejected for every other status:

```json
{
  "messageId": "01K2FA4Q6G8M3V7R2Y51N9P0KE",
  "releaseId": "9e6047f1-03ea-486b-9278-27a69e970ca1",
  "status": "FAILED",
  "reportedAt": "2026-08-11T08:32:10Z",
  "error": {
    "code": "SIGNATURE_INVALID",
    "message": "Firmware signature verification failed"
  }
}
```

The release must exist and its hardware version must match the device. A
successful report returns `200 OK`:

```json
{
  "installationId": "62cacbc3-9bb8-4ed5-8993-a6cb811f76d5",
  "messageId": "01K2FA4Q6G8M3V7R2Y51N9P0KD",
  "releaseId": "9e6047f1-03ea-486b-9278-27a69e970ca1",
  "status": "INSTALLED",
  "reportedAt": "2026-08-11T08:32:10Z",
  "duplicate": false,
  "serverTime": "2026-08-11T08:32:11Z"
}
```

Reports are idempotent by `(deviceId, messageId)`. A retry returns the original
event with `duplicate: true`, even if retry fields differ. A new `INSTALLED`
report updates `devices.firmware_version` to the release version in the same
database transaction.

## Priority 4: Cane camera live feed

A guardian starts the cane's camera by creating a `START_CAMERA_STREAM`
command through the existing command API (`POST
/api/v1/devices/{deviceId}/commands`, above). Once the cane sees that
command on its next poll, media setup happens over WebRTC, not the command
channel: the cane publishes one H264 track and the app views it through a
non-trickle [WHIP](https://www.ietf.org/archive/id/draft-ietf-wish-whip-09.html)
/[WHEP](https://www.ietf.org/archive/id/draft-ietf-wish-whep-01.html)
profile. The cane and the app each open a WebRTC peer connection to this
server. The server relays RTP between them. Video does not travel as an
HTTP body.

| Method | Endpoint | Authentication | Purpose |
| --- | --- | --- | --- |
| `POST` | `/api/v1/firmware/devices/{deviceId}/camera/publications` | Device token | Publish (or replace) the cane's H264 track via a WHIP offer. |
| `DELETE` | `/api/v1/firmware/devices/{deviceId}/camera/publications/{publicationId}` | Device token | Stop publishing. Idempotent. |
| `PATCH` | `/api/v1/firmware/devices/{deviceId}/camera/publications/{publicationId}` | Device token | Always `405`; ICE restart is not supported. |
| `POST` | `/api/v1/devices/{deviceId}/camera/viewers` | User JWT (owner or guardian) | Create a viewer via a WHEP offer. Succeeds even if the cane has not published yet. |
| `DELETE` | `/api/v1/devices/{deviceId}/camera/viewers/{viewerId}` | User JWT (owner or guardian, and the viewer's creator) | Stop viewing. Idempotent. |
| `PATCH` | `/api/v1/devices/{deviceId}/camera/viewers/{viewerId}` | User JWT | Always `405`; ICE restart is not supported. |
| `GET` | `/api/v1/devices/{deviceId}/camera` | User JWT (owner or guardian) | Read the current session state and viewer count. |

This is a non-trickle profile: every offer must gather all of its ICE
candidates locally and inline them in the SDP body before POSTing, because
there is no PATCH to trickle additional candidates afterward. A client that
needs to restart ICE POSTs a brand-new offer to the same collection
endpoint instead.

### Publish (or replace) the camera track

`POST /api/v1/firmware/devices/{deviceId}/camera/publications`

- `Content-Type: application/sdp` is required; the only allowed parameter is
  `charset`.
- `Accept`, if present, must include `application/sdp` or `*/*` (`406` otherwise).
- The body is an SDP offer, at most 65,536 bytes, with exactly one active
  `sendonly` video section: H264 Constrained Baseline
  (`profile-level-id` starting `42e0`), `packetization-mode=1`, `rtcp-mux`,
  ICE credentials and at least one inline candidate, and a DTLS fingerprint
  with `setup:actpass`. Audio, data channels, simulcast, and any extra
  active media section are rejected with `422`.

A successful response is `201 Created` with `Content-Type: application/sdp`,
`Location: /api/v1/firmware/devices/{deviceId}/camera/publications/{publicationId}`,
`Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, and the SDP
answer (including gathered server candidates) as the body. The answer is
returned once local ICE gathering finishes, not once the peers actually
connect; gathering has a 10-second budget (`WEBRTC_NEGOTIATION_TIMEOUT`,
`504` on timeout). The new publication immediately becomes the device's one
active publisher; any previous publication is closed. Existing viewers keep
their own peer connection and simply start receiving RTP from the new
source — no viewer renegotiates.

### Stop publishing

`DELETE /api/v1/firmware/devices/{deviceId}/camera/publications/{publicationId}`

Returns `204 No Content`, including when the ID is unknown, already closed,
or was superseded by a later publication (stopping a stale ID never affects
the current one).

### Create a viewer

`POST /api/v1/devices/{deviceId}/camera/viewers`

Same `Content-Type`/`Accept`/size/gathering rules as publishing, but the
offer's one active video section must be `recvonly`. A device with no active
publisher still returns `201`; the viewer attaches to the device's stable
relay track and starts receiving RTP whenever a cane connects. A device may
have at most 4 concurrent viewers; the 5th attempt returns `429`.

A successful response is `201 Created` with
`Location: /api/v1/devices/{deviceId}/camera/viewers/{viewerId}` and the SDP
answer as the body, using the same headers as publication creation.

### Stop viewing

`DELETE /api/v1/devices/{deviceId}/camera/viewers/{viewerId}`

Returns `204 No Content`, including repeat calls and already-closed viewers.
A viewer may only be deleted by the same user who created it; the other
owner/guardian gets `403` rather than being able to guess the ID.

### Get camera status

`GET /api/v1/devices/{deviceId}/camera`

```json
{
  "state": "LIVE",
  "viewerCount": 2
}
```

`state` is `OFFLINE` (no session for this device), `WAITING` (viewers
connected, no publisher yet), or `LIVE` (a publisher is attached).

### Errors

| Status | Condition |
| --- | --- |
| `400 Bad Request` | Invalid path UUID or syntactically invalid SDP |
| `401 Unauthorized` | Missing, invalid, or expired bearer token |
| `403 Forbidden` | Device token/path mismatch, caller is not owner or guardian, or the viewer belongs to another user |
| `404 Not Found` | The device does not exist |
| `405 Method Not Allowed` | `PATCH` on a publication or viewer resource (`Allow: DELETE`) |
| `406 Not Acceptable` | `Accept` excludes `application/sdp` |
| `413 Content Too Large` | SDP body exceeds 65,536 bytes |
| `415 Unsupported Media Type` | Request `Content-Type` is not `application/sdp` |
| `422 Unprocessable Content` | SDP parses but violates the H264/direction/ICE/DTLS/non-trickle profile |
| `429 Too Many Requests` | The device already has 4 viewers |
| `504 Gateway Timeout` | Local ICE gathering did not finish within `WEBRTC_NEGOTIATION_TIMEOUT` |

### Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `WEBRTC_STUN_URLS` | `stun:stun.l.google.com:19302` | Comma-separated STUN server URLs handed to every peer connection. |
| `WEBRTC_TURN_URLS` | *(none)* | Comma-separated TURN server URLs; optional. |
| `WEBRTC_TURN_USERNAME`, `WEBRTC_TURN_CREDENTIAL` | *(none)* | Required together when `WEBRTC_TURN_URLS` is set. |
| `WEBRTC_NEGOTIATION_TIMEOUT` | `10s` | Local ICE gathering budget for both WHIP and WHEP offers. |

Firmware and the guardian app must use equivalent ICE configuration to
create a compatible offer. Because sessions live in server process memory,
a deployment with multiple Gin replicas needs device-ID session affinity;
plain round-robin routing would split a device's publisher and viewers
across replicas that cannot see each other's relay.

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
| 8 | Camera WHIP/WHEP relay | Lets a guardian watch the cane camera after the command poll and media path are both in place. |

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
