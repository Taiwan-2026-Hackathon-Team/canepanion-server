import type { ArchitectureData } from './components/ArchitectureMap'
import type { ArchEdge, ArchFlow, ArchNode, Group } from './core/types'
import type { Archetype, ArchetypeParams } from './core/archetypes'
import { deriveArchetype, deriveHeight, deriveSize, packLayout } from './core/layout'
import type { Footprint } from './core/iso'
import { MEASURED, UNCLAIMED } from './measured.generated'

export const GROUPS: readonly Group[] = [
  { id: 'entry', label: 'Entry & boot' },
  { id: 'platform', label: 'Server platform' },
  { id: 'guardian', label: 'Guardian app' },
  { id: 'firmware', label: 'Cane firmware' },
  { id: 'outside', label: 'Outside world' },
]

export const INTRO = {
  title: 'CanePanion cloud server',
  lede:
    'A Gin API that sits between smart-cane firmware, guardian phones, and the ' +
    'safety services they depend on.',
  whatItDoes:
    'Authenticates guardians and devices, ingests telemetry and voice clips from ' +
    'firmware, turns safety events into guardian alerts, runs an assistant ' +
    'voice pipeline when a user clip is marked complete, and relays live cane ' +
    'camera over WebRTC.',
  howItsBuilt:
    'Each domain is a handler → service → repository stack wired from ' +
    'infra/routes.go. Optional integrations (FCM, Google Speech/Gemini/TTS, ' +
    'Cloudinary) degrade gracefully when credentials are missing so local dev ' +
    'still boots. The camera Hub always starts; a bad TURN config aborts boot ' +
    'rather than disabling the relay.',
}

type NodeDraft = Omit<
  ArchNode,
  'archetype' | 'params' | 'footprint' | 'height' | 'count' | 'loc'
> & { footprint?: Footprint; archetype?: Archetype; params?: ArchetypeParams }

const DRAFTS: NodeDraft[] = [
  {
    id: 'boot',
    code: 'BT',
    name: 'Boot',
    role: 'the process entry',
    group: 'entry',
    whatItDoes:
      'Loads environment, opens the database, and wires optional subsystems ' +
      'before the HTTP server listens.',
    howItsBuilt:
      'Push and voice clients are constructed here with the same degrade pattern: ' +
      'missing credentials yield a disabled implementation, malformed ones abort ' +
      'startup.',
    files: ['cmd/main.go'],
    stack: ['godotenv'],
  },
  {
    id: 'config',
    code: 'CF',
    name: 'Config',
    role: 'the env loader',
    group: 'entry',
    whatItDoes:
      'Reads .env, applies CORS policy, and exposes a migration flag for schema ' +
      'changes.',
    howItsBuilt:
      'Migration is opt-in via a CLI flag rather than running on every boot, so ' +
      'production deploys do not surprise-migrate.',
    files: ['config/loadEnv.go', 'config/cors.go', 'config/migration.go'],
  },
  {
    id: 'infra',
    code: 'IN',
    name: 'Infra',
    role: 'the HTTP shell',
    group: 'platform',
    whatItDoes:
      'Owns the Gin engine, PostgreSQL connection, and the route registration ' +
      'table that mounts every handler group.',
    howItsBuilt:
      'Routes are split into register* helpers so firmware paths stay under ' +
      '/api/v1/firmware without duplicating middleware setup.',
    files: ['infra/run-gin.go', 'infra/routes.go', 'infra/connectDb.go', 'infra/syncDb.go'],
    stack: ['Gin', 'GORM', 'PostgreSQL'],
  },
  {
    id: 'middleware',
    code: 'MW',
    name: 'Middleware',
    role: 'the request gates',
    group: 'platform',
    whatItDoes:
      'JWT auth for guardian routes, device bearer auth for firmware routes, and ' +
      'a centralized JSON error envelope.',
    howItsBuilt:
      'Device auth binds the path deviceId to token claims before any handler ' +
      'runs, so a stolen token for device A cannot touch device B.',
    files: [
      'middleware/jwt_auth.go',
      'middleware/device_auth.go',
      'middleware/error_handler.go',
    ],
  },
  {
    id: 'models',
    code: 'MD',
    name: 'Models',
    role: 'the shared schema',
    group: 'platform',
    whatItDoes:
      'GORM structs and enums for users, devices, sensor events, alerts, audio ' +
      'rows, and ingestion batches.',
    howItsBuilt:
      'Audio carries direction, status, parent link, transcript, and response ' +
      'text so the voice job can resume mid-pipeline without orphan rows.',
    files: ['models/audio.go', 'models/devices.go', 'models/alerts.go', 'models/sensor_events.go'],
  },
  {
    id: 'pkg-errors',
    code: 'ER',
    name: 'App errors',
    role: 'the error vocabulary',
    group: 'platform',
    whatItDoes:
      'Typed HTTP errors (400/401/404/500) that handlers push onto Gin context ' +
      'for the error middleware to render.',
    howItsBuilt:
      'Services return domain errors; handlers never construct status codes ' +
      'directly.',
    files: ['pkg/errors/errors.go'],
  },
  {
    id: 'pkg-http',
    code: 'HT',
    name: 'HTTP helpers',
    role: 'the binding layer',
    group: 'platform',
    whatItDoes:
      'Form+JSON binding for multipart uploads and JWT user-id extraction for ' +
      'guardian routes.',
    howItsBuilt:
      'BindFormJSON keeps firmware multipart contracts in one place instead of ' +
      'repeating c.ShouldBind in every handler.',
    files: ['pkg/http/bindjson.go', 'pkg/http/extract_userid.go'],
  },
  {
    id: 'pkg-utils',
    code: 'UT',
    name: 'Shared utils',
    role: 'the crypto & media helpers',
    group: 'platform',
    whatItDoes:
      'Password hashing, JWT and device tokens, UUID parsing, and Cloudinary ' +
      'upload/download for audio bytes.',
    howItsBuilt:
      'Cloudinary helpers are shared by the upload path and the voice job reply ' +
      'upload so storage layout stays consistent.',
    files: [
      'pkg/utils/jwt.go',
      'pkg/utils/device_token.go',
      'pkg/utils/cloudinary.go',
      'pkg/utils/bcrypt.go',
    ],
    stack: ['Cloudinary', 'JWT', 'bcrypt'],
  },
  {
    id: 'auth',
    code: 'AU',
    name: 'Guardian auth',
    role: 'the session gate',
    group: 'guardian',
    whatItDoes:
      'Register, login, logout, and /me for guardian and cane-user accounts. ' +
      'Successful auth sets an HTTP-only session cookie.',
    howItsBuilt:
      'JWT lives in a cookie rather than a response body so browser clients get ' +
      'session semantics without storing tokens in JS.',
    files: ['internal/auth/handler.go', 'internal/auth/service.go', 'internal/auth/repo.go'],
  },
  {
    id: 'devices',
    code: 'DV',
    name: 'Device registry',
    role: 'the pairing record',
    group: 'guardian',
    whatItDoes:
      'Lets an authenticated guardian register a cane device record that ' +
      'firmware later activates.',
    howItsBuilt:
      'Registration is guardian-side only; firmware activation attaches ' +
      'credentials separately under firmware-auth.',
    files: ['internal/devices/handler.go', 'internal/devices/service.go', 'internal/devices/repo.go'],
  },
  {
    id: 'firmware-auth',
    code: 'FA',
    name: 'Firmware auth',
    role: 'the device session',
    group: 'firmware',
    whatItDoes:
      'Activates a registered device, mints a short-lived bearer token, and ' +
      'records heartbeats that keep last-seen and battery fresh.',
    howItsBuilt:
      'Activation rotates a bcrypt-hashed device credential; session exchanges ' +
      'that credential for a JWT scoped to one deviceId.',
    files: [
      'internal/firmware_auth/handler.go',
      'internal/firmware_auth/service.go',
      'internal/firmware_auth/repo.go',
    ],
  },
  {
    id: 'telemetry',
    code: 'TM',
    name: 'Telemetry ingest',
    role: 'the safety inbox',
    group: 'firmware',
    whatItDoes:
      'Accepts batched sensor events and locations from firmware, dedupes by ' +
      'messageId, and raises alerts for falls and SOS.',
    howItsBuilt:
      'Fall pushes run in a goroutine off the ingest path so firmware retries ' +
      'get a fast 200; FCM failure is logged, never returned to the cane.',
    files: [
      'internal/telemetry/handler.go',
      'internal/telemetry/service.go',
      'internal/telemetry/repo.go',
    ],
    stack: ['FCM via push'],
  },
  {
    id: 'audio',
    code: 'AD',
    name: 'Audio & voice job',
    role: 'the voice pipeline hub',
    group: 'firmware',
    whatItDoes:
      'Firmware uploads user clips to Cloudinary, calls complete to claim ' +
      'processing, and polls GET until a reply MP3 is ready. A background ' +
      '[[VoiceJob]] runs STT → Gemini → TTS for USER_TO_ASSISTANT clips.',
    howItsBuilt:
      'Complete uses ClaimProcessing for idempotency; VoiceJob binds repository ' +
      'adapters at handler construction and resumes from partial reply rows ' +
      'instead of creating duplicates.',
    files: [
      'internal/audio/handler.go',
      'internal/audio/service.go',
      'internal/audio/voice_job.go',
      'internal/audio/adapters.go',
      'internal/audio/repo.go',
    ],
    stack: ['Cloudinary', 'Google Speech', 'Vertex Gemini', 'Google TTS'],
  },
  {
    id: 'device-controls',
    code: 'DC',
    name: 'Device config',
    role: 'the settings pull',
    group: 'firmware',
    whatItDoes:
      'Returns the latest configuration blob and version for a device so ' +
      'firmware can sync cloud-side settings.',
    howItsBuilt:
      'ETag is derived from the config version string so firmware can skip ' +
      're-downloading unchanged settings.',
    files: [
      'internal/device_controls/handler.go',
      'internal/device_controls/service.go',
      'internal/device_controls/repo.go',
    ],
  },
  {
    id: 'firmware-updates',
    code: 'FU',
    name: 'Firmware OTA',
    role: 'the update stub',
    group: 'firmware',
    whatItDoes:
      'Handlers for latest-firmware lookup and install reports — wired in code ' +
      'but not yet mounted in routes.',
    howItsBuilt:
      'Kept as a full domain stack so OTA can be switched on by uncommenting ' +
      'one register call in routes.go.',
    files: [
      'internal/firmware_updates/handler.go',
      'internal/firmware_updates/service.go',
      'internal/firmware_updates/repo.go',
    ],
  },
  {
    id: 'camera',
    code: 'CM',
    name: 'Camera relay',
    role: 'the live video bridge',
    group: 'firmware',
    whatItDoes:
      'Takes an H264 [[WHIP]] offer from the cane and a [[WHEP]] offer from a ' +
      'guardian, then copies RTP between them so the phone can watch the cane ' +
      'camera.',
    howItsBuilt:
      'Sessions live in process memory, one goroutine per device, with a ' +
      'generation-stamped H264 relay so a replaced publisher cannot leak packets ' +
      'into the old lease. The SDP answer waits for local ICE gathering to ' +
      'finish, and PATCH is rejected because an ICE restart is a new POST.',
    files: [
      'internal/camera/handler.go',
      'internal/camera/hub.go',
      'internal/camera/session.go',
      'internal/camera/relay.go',
      'internal/camera/peer.go',
      'internal/camera/sdp.go',
    ],
    stack: ['Pion WebRTC', 'WHIP', 'WHEP'],
    // Ten files of similar size would derive as a fin-row. Camera is one
    // relay, not a collection of siblings, so the stack is the honest shape.
    archetype: 'slab-stack',
    params: { levels: 3 },
  },
  {
    id: 'push',
    code: 'PU',
    name: 'Push notifier',
    role: 'the guardian pager',
    group: 'platform',
    whatItDoes:
      'Sends fall/SOS payloads to a Firebase topic when telemetry creates a ' +
      'critical alert.',
    howItsBuilt:
      'Same degrade pattern as voice: no Firebase creds → Disabled notifier, ' +
      'server still runs.',
    files: ['internal/push/fcm.go', 'internal/push/push.go'],
    stack: ['Firebase Cloud Messaging'],
  },
  {
    id: 'pkg-speech',
    code: 'ST',
    name: 'Speech STT',
    role: 'the transcriber',
    group: 'platform',
    whatItDoes:
      'Sync recognize on user audio bytes downloaded from Cloudinary, with a ' +
      'size cap matching Google sync limits.',
    howItsBuilt:
      'Wrapped behind Transcriber so VoiceJob can swap in DisabledTranscriber ' +
      'when ADC is missing.',
    files: ['pkg/speech/speech.go'],
    stack: ['Google Cloud Speech'],
  },
  {
    id: 'pkg-gemini',
    code: 'GM',
    name: 'Gemini reply',
    role: 'the assistant brain',
    group: 'platform',
    whatItDoes:
      'Turns a transcript string into assistant reply text via Vertex Gemini.',
    howItsBuilt:
      'Replier interface keeps the model client out of VoiceJob orchestration ' +
      'logic.',
    files: ['pkg/gemini/gemini.go'],
    stack: ['Vertex Gemini'],
  },
  {
    id: 'pkg-tts',
    code: 'TS',
    name: 'Text-to-speech',
    role: 'the voice synthesizer',
    group: 'platform',
    whatItDoes:
      'Synthesizes reply text to MP3 bytes, picking voice language from reply ' +
      'content.',
    howItsBuilt:
      'Speaker interface mirrors STT and Gemini so all three Google clients ' +
      'share the same disable-at-boot pattern.',
    files: ['pkg/tts/tts.go'],
    stack: ['Google Cloud Text-to-Speech'],
  },
  {
    id: 'postgres',
    code: 'PG',
    name: 'PostgreSQL',
    role: 'the system of record',
    group: 'outside',
    whatItDoes:
      'Stores users, devices, events, alerts, audio metadata, and ingestion ' +
      'batch responses.',
    howItsBuilt: 'Accessed only through GORM repositories — no raw SQL in handlers.',
    files: ['infra/connectDb.go'],
    stack: ['PostgreSQL', 'GORM'],
  },
  {
    id: 'cloudinary',
    code: 'CL',
    name: 'Cloudinary',
    role: 'the audio store',
    group: 'outside',
    whatItDoes:
      'Holds uploaded cane audio and synthesized reply MP3s; the server keeps ' +
      'public IDs in Postgres.',
    howItsBuilt:
      'Upload on ingest, byte download for STT, and reply upload all go through ' +
      'pkg/utils/cloudinary.go.',
    files: ['pkg/utils/cloudinary.go'],
    stack: ['Cloudinary'],
  },
  {
    id: 'firebase',
    code: 'FB',
    name: 'Firebase FCM',
    role: 'the alert fan-out',
    group: 'outside',
    whatItDoes:
      'Delivers fall/SOS notifications to guardian app subscribers on a topic.',
    howItsBuilt: 'HTTP v1 via service account JSON from env.',
    files: ['internal/push/fcm.go'],
    stack: ['Firebase Cloud Messaging'],
  },
  {
    id: 'google-voice',
    code: 'GV',
    name: 'Google AI APIs',
    role: 'the speech stack',
    group: 'outside',
    whatItDoes:
      'Speech-to-text, Gemini text generation, and text-to-speech for the voice ' +
      'pipeline.',
    howItsBuilt:
      'All three clients authenticate with Application Default Credentials from ' +
      'GOOGLE_APPLICATION_CREDENTIALS.',
    files: ['pkg/speech/speech.go', 'pkg/gemini/gemini.go', 'pkg/tts/tts.go'],
    stack: ['Google Cloud Speech', 'Vertex Gemini', 'Google TTS'],
  },
  {
    id: 'ice',
    code: 'IC',
    name: 'STUN & TURN',
    role: 'the ICE path',
    group: 'outside',
    whatItDoes:
      'Gives STUN and optional TURN URLs to every Pion peer so the cane and ' +
      'the phone can punch through NAT.',
    howItsBuilt:
      'Google STUN is the default. TURN URLs need a username and credential or ' +
      'boot fails, and firmware plus the guardian app have to use the same ICE ' +
      'servers as the relay.',
    files: ['internal/camera/hub.go'],
    stack: ['STUN', 'TURN'],
  },
]

const EDGES: ArchEdge[] = [
  {
    id: 'boot-infra',
    from: 'boot',
    to: 'infra',
    kind: 'call',
    label: 'RunGin',
    flowIds: [
      'voice-reply',
      'fall-alert',
      'device-online',
      'guardian-login',
      'pull-config',
      'watch-camera',
    ],
  },
  {
    id: 'boot-push',
    from: 'boot',
    to: 'push',
    kind: 'support',
    label: 'FCM notifier',
    flowIds: ['fall-alert'],
  },
  {
    id: 'boot-audio-job',
    from: 'boot',
    to: 'audio',
    kind: 'support',
    label: 'VoiceJob wiring',
    flowIds: ['voice-reply'],
  },
  {
    id: 'boot-camera',
    from: 'boot',
    to: 'camera',
    kind: 'support',
    label: 'Hub from env',
    flowIds: ['watch-camera'],
  },
  {
    id: 'infra-routes-auth',
    from: 'infra',
    to: 'auth',
    kind: 'call',
    label: 'registerAuth',
    flowIds: ['guardian-login'],
  },
  {
    id: 'infra-routes-firmware-auth',
    from: 'infra',
    to: 'firmware-auth',
    kind: 'call',
    label: 'registerFirmwareAuth',
    flowIds: ['device-online'],
  },
  {
    id: 'infra-routes-telemetry',
    from: 'infra',
    to: 'telemetry',
    kind: 'call',
    label: 'registerFirmwareTelemetry',
    flowIds: ['fall-alert'],
  },
  {
    id: 'infra-routes-audio',
    from: 'infra',
    to: 'audio',
    kind: 'call',
    label: 'registerFirmwareAudio',
    flowIds: ['voice-reply'],
  },
  {
    id: 'infra-routes-controls',
    from: 'infra',
    to: 'device-controls',
    kind: 'call',
    label: 'registerFirmwareControl',
    flowIds: ['pull-config'],
  },
  {
    id: 'infra-routes-firmware-camera',
    from: 'infra',
    to: 'camera',
    kind: 'call',
    label: 'registerFirmwareCamera',
    flowIds: ['watch-camera'],
  },
  {
    id: 'infra-routes-camera',
    from: 'infra',
    to: 'camera',
    kind: 'call',
    label: 'registerCamera',
    flowIds: ['watch-camera'],
  },
  {
    id: 'infra-middleware',
    from: 'infra',
    to: 'middleware',
    kind: 'support',
    label: 'Use() chain',
    flowIds: [
      'voice-reply',
      'fall-alert',
      'device-online',
      'pull-config',
      'guardian-login',
      'watch-camera',
    ],
  },
  {
    id: 'middleware-firmware-auth',
    from: 'middleware',
    to: 'firmware-auth',
    kind: 'call',
    label: 'DeviceAuthMiddleware',
    flowIds: ['voice-reply', 'fall-alert', 'pull-config', 'watch-camera'],
  },
  {
    id: 'middleware-guardian-auth',
    from: 'middleware',
    to: 'auth',
    kind: 'call',
    label: 'JWTAuthMiddleware',
    flowIds: ['guardian-login', 'watch-camera'],
  },
  {
    id: 'auth-utils',
    from: 'auth',
    to: 'pkg-utils',
    kind: 'call',
    label: 'JWT & bcrypt',
    flowIds: ['guardian-login'],
  },
  {
    id: 'auth-postgres',
    from: 'auth',
    to: 'postgres',
    kind: 'data',
    label: 'user rows',
    flowIds: ['guardian-login'],
  },
  {
    id: 'firmware-auth-utils',
    from: 'firmware-auth',
    to: 'pkg-utils',
    kind: 'call',
    label: 'device token',
    flowIds: ['device-online'],
  },
  {
    id: 'firmware-auth-postgres',
    from: 'firmware-auth',
    to: 'postgres',
    kind: 'data',
    label: 'device credentials',
    flowIds: ['device-online'],
  },
  {
    id: 'telemetry-push',
    from: 'telemetry',
    to: 'push',
    kind: 'call',
    label: 'NotifyFall',
    flowIds: ['fall-alert'],
  },
  {
    id: 'telemetry-postgres',
    from: 'telemetry',
    to: 'postgres',
    kind: 'data',
    label: 'events & alerts',
    flowIds: ['fall-alert'],
  },
  {
    id: 'push-firebase',
    from: 'push',
    to: 'firebase',
    kind: 'call',
    label: 'FCM send',
    flowIds: ['fall-alert'],
  },
  {
    id: 'audio-cloudinary-upload',
    from: 'audio',
    to: 'cloudinary',
    kind: 'call',
    label: 'upload clip',
    flowIds: ['voice-reply'],
  },
  {
    id: 'audio-voicejob-start',
    from: 'audio',
    to: 'audio',
    kind: 'call',
    label: 'VoiceJob.Start',
    flowIds: ['voice-reply'],
  },
  {
    id: 'audio-postgres',
    from: 'audio',
    to: 'postgres',
    kind: 'data',
    label: 'audio metadata',
    flowIds: ['voice-reply'],
  },
  {
    id: 'voicejob-cloudinary-download',
    from: 'audio',
    to: 'cloudinary',
    kind: 'call',
    label: 'download bytes',
    flowIds: ['voice-reply'],
  },
  {
    id: 'voicejob-speech',
    from: 'audio',
    to: 'pkg-speech',
    kind: 'call',
    label: 'Recognize',
    flowIds: ['voice-reply'],
  },
  {
    id: 'speech-google',
    from: 'pkg-speech',
    to: 'google-voice',
    kind: 'call',
    label: 'STT API',
    flowIds: ['voice-reply'],
  },
  {
    id: 'voicejob-gemini',
    from: 'audio',
    to: 'pkg-gemini',
    kind: 'call',
    label: 'GenerateReply',
    flowIds: ['voice-reply'],
  },
  {
    id: 'gemini-google',
    from: 'pkg-gemini',
    to: 'google-voice',
    kind: 'call',
    label: 'Vertex call',
    flowIds: ['voice-reply'],
  },
  {
    id: 'voicejob-tts',
    from: 'audio',
    to: 'pkg-tts',
    kind: 'call',
    label: 'Synthesize',
    flowIds: ['voice-reply'],
  },
  {
    id: 'tts-google',
    from: 'pkg-tts',
    to: 'google-voice',
    kind: 'call',
    label: 'TTS API',
    flowIds: ['voice-reply'],
  },
  {
    id: 'voicejob-cloudinary-reply',
    from: 'audio',
    to: 'cloudinary',
    kind: 'call',
    label: 'upload reply MP3',
    flowIds: ['voice-reply'],
  },
  {
    id: 'controls-postgres',
    from: 'device-controls',
    to: 'postgres',
    kind: 'data',
    label: 'config row',
    flowIds: ['pull-config'],
  },
  {
    id: 'audio-disabled-fail',
    from: 'audio',
    to: 'postgres',
    kind: 'retry',
    label: 'MarkAudioFailed',
    flowIds: ['voice-reply'],
  },
  {
    id: 'camera-ownership',
    from: 'camera',
    to: 'device-controls',
    kind: 'call',
    label: 'FindDeviceByID',
    flowIds: ['watch-camera'],
  },
  {
    id: 'camera-ice-publish',
    from: 'camera',
    to: 'ice',
    kind: 'call',
    label: 'publisher ICE',
    flowIds: ['watch-camera'],
  },
  {
    id: 'camera-ice-view',
    from: 'camera',
    to: 'ice',
    kind: 'call',
    label: 'viewer ICE',
    flowIds: ['watch-camera'],
  },
  {
    id: 'camera-relay',
    from: 'camera',
    to: 'camera',
    kind: 'call',
    label: 'forward RTP',
    flowIds: ['watch-camera'],
  },
]

const FLOWS: ArchFlow[] = [
  {
    id: 'voice-reply',
    name: 'Voice reply',
    payload: 'user audio clip',
    summary:
      'Firmware uploads a USER_TO_ASSISTANT clip, completes it, and polls until ' +
      'the server finishes STT → Gemini → TTS and stores a reply MP3.',
    route: [
      'infra-routes-audio',
      'middleware-firmware-auth',
      'audio-cloudinary-upload',
      'audio-postgres',
      'audio-voicejob-start',
      'voicejob-cloudinary-download',
      'voicejob-speech',
      'speech-google',
      'voicejob-gemini',
      'gemini-google',
      'voicejob-tts',
      'tts-google',
      'voicejob-cloudinary-reply',
      'audio-postgres',
    ],
  },
  {
    id: 'fall-alert',
    name: 'Fall alert',
    payload: 'sensor event batch',
    summary:
      'A critical fall event is stored, an alert row is created, and guardians ' +
      'are paged via FCM without blocking the firmware ingest response.',
    route: [
      'infra-routes-telemetry',
      'middleware-firmware-auth',
      'telemetry-postgres',
      'telemetry-push',
      'push-firebase',
    ],
  },
  {
    id: 'device-online',
    name: 'Device online',
    payload: 'device credential',
    summary:
      'Firmware activates with a guardian-registered device ID, then exchanges ' +
      'its credential for a bearer token used on all subsequent calls.',
    route: [
      'infra-routes-firmware-auth',
      'firmware-auth-postgres',
      'firmware-auth-utils',
    ],
  },
  {
    id: 'guardian-login',
    name: 'Guardian login',
    payload: 'session cookie',
    summary:
      'A guardian signs in and receives a JWT in an HTTP-only cookie for ' +
      'browser session access.',
    route: ['infra-routes-auth', 'auth-utils', 'auth-postgres'],
  },
  {
    id: 'pull-config',
    name: 'Pull config',
    payload: 'config version',
    summary:
      'Authenticated firmware reads its latest configuration blob and version ' +
      'for cloud-side settings sync.',
    route: [
      'infra-routes-controls',
      'middleware-firmware-auth',
      'controls-postgres',
    ],
  },
  {
    id: 'watch-camera',
    name: 'Watch camera',
    payload: 'H264 RTP',
    summary:
      'After the cane picks up a START_CAMERA_STREAM command, firmware posts a ' +
      'WHIP offer and a guardian posts a WHEP offer. The server copies H264 RTP ' +
      'between the two Pion peers.',
    route: [
      'infra-routes-firmware-camera',
      'middleware-firmware-auth',
      'camera-ice-publish',
      'infra-routes-camera',
      'middleware-guardian-auth',
      'camera-ownership',
      'camera-ice-view',
      'camera-relay',
    ],
  },
]

function shapeOf(draft: NodeDraft, measure: { count: number; loc: number }) {
  const derived = deriveArchetype(measure)
  const archetype = draft.archetype ?? derived.archetype
  const params = draft.params ?? derived.params
  return { archetype, params }
}

function buildNodes(): ArchNode[] {
  const inputs = DRAFTS.map((draft) => {
    const measure = MEASURED[draft.id] ?? { count: 0, loc: 0 }
    const { archetype, params } = shapeOf(draft, measure)
    const size = deriveSize(archetype, params, measure)
    return { item: draft, group: draft.group, size }
  })
  const footprints = packLayout(inputs, GROUPS.map((g) => g.id))

  return DRAFTS.map((draft) => {
    const measure = MEASURED[draft.id] ?? { count: 0, loc: 0 }
    const { archetype, params } = shapeOf(draft, measure)
    const footprint = draft.footprint ?? footprints.get(draft)
    if (!footprint) throw new Error(`no footprint for ${draft.id}`)

    return {
      ...draft,
      archetype,
      params,
      height: deriveHeight(measure),
      footprint,
      count: measure.count,
      loc: measure.loc,
    }
  })
}

export const ARCHITECTURE: ArchitectureData = {
  groups: GROUPS,
  nodes: buildNodes(),
  edges: EDGES,
  flows: FLOWS,
  intro: INTRO,
  unmapped: UNCLAIMED,
  repo: 'canepanion-server',
}
