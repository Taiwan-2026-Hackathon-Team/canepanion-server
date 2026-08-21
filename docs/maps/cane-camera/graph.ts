import type { ArchitectureData } from './components/ArchitectureMap'
import type { ArchEdge, ArchFlow, ArchNode, Group } from './core/types'
import type { Archetype, ArchetypeParams } from './core/archetypes'
import { deriveArchetype, deriveHeight, deriveSize, packLayout, type LayoutInput } from './core/layout'
import type { Footprint } from './core/iso'
import { MEASURED, UNCLAIMED } from './measured.generated'

export const GROUPS: readonly Group[] = [
  { id: 'cane', label: 'The cane' },
  { id: 'notes', label: 'Hello notes' },
  { id: 'switchboard', label: 'The switchboard' },
  { id: 'pipe', label: 'The video pipe' },
  { id: 'phone', label: 'The phone' },
  { id: 'outside', label: 'Outside world' },
]

export const INTRO = {
  title: 'Live camera: cane, server, and phone',
  lede:
    'The cane never calls the phone directly: each books a call with the server ' +
    'switchboard, which copies live pictures across an internal video pipe.',
  whatItDoes:
    'Orchestrates live video streaming from the smart cane to the guardian mobile ' +
    'app. The phone triggers camera capture, both sides exchange hello and reply ' +
    'notes over HTTP to book their calls, and pictures flow live over WebRTC sockets ' +
    'through an in-memory video pipe.',
  howItsBuilt:
    'The server answers two live calls, one from the cane and one from the phone, ' +
    'and copies pictures through one internal video pipe. Hello notes go over HTTP. ' +
    'Pictures do not.',
}

type NodeDraft = Omit<
  ArchNode,
  'archetype' | 'params' | 'footprint' | 'height' | 'count' | 'loc'
> & { footprint?: Footprint; archetype?: Archetype; params?: ArchetypeParams; height?: number }

const DRAFTS: NodeDraft[] = [
  {
    id: 'cane-firmware',
    code: 'CF',
    name: 'Cane firmware',
    role: 'the camera broadcaster',
    group: 'cane',
    archetype: 'low-slab',
    height: 1,
    whatItDoes:
      'Captures camera frames on the physical cane and sends them over a ' +
      '[[live call]] once instructed.',
    howItsBuilt:
      'Runs its own WebRTC client independently; it polls this server over HTTP ' +
      'for commands, but streams live frames directly over media sockets.',
    files: [],
    stack: ['WebRTC client'],
  },
  {
    id: 'cane-auth',
    code: 'DA',
    name: 'Device auth',
    role: 'the device gatekeeper',
    group: 'cane',
    whatItDoes:
      'Verifies the cane bearer token on incoming HTTP requests and ensures ' +
      'the device ID matches the URL path.',
    howItsBuilt:
      'Runs before camera or command handlers, rejecting expired tokens or ' +
      'mismatched device IDs at the perimeter.',
    files: ['middleware/device_auth.go'],
    stack: ['Gin middleware', 'device token'],
  },
  {
    id: 'routes-entry',
    code: 'RD',
    name: 'Route dispatch',
    role: 'the HTTP dispatcher',
    group: 'notes',
    whatItDoes:
      'Mounts REST endpoints for firmware and mobile apps, wiring authentication ' +
      'gates to their corresponding handlers.',
    howItsBuilt:
      'Separates firmware routes from guardian routes so device tokens and user ' +
      'logins never share validation rules.',
    files: ['infra/routes.go', 'cmd/main.go'],
    stack: ['Gin Engine', 'GORM'],
  },
  {
    id: 'camera-handler',
    code: 'CG',
    name: 'Camera gateway',
    role: 'the HTTP booking desk',
    group: 'notes',
    whatItDoes:
      'Receives [[hello notes]] over HTTP POST, asks the switchboard for answers, ' +
      'and returns reply notes with location headers.',
    howItsBuilt:
      'Treats HTTP purely as a booking exchange; pictures never travel through ' +
      'these request or response bodies.',
    files: ['internal/camera/handler.go'],
    stack: ['Gin', 'HTTP SDP'],
  },
  {
    id: 'sdp-decoder',
    code: 'NV',
    name: 'Note validator',
    role: 'the offer inspector',
    group: 'notes',
    whatItDoes:
      'Inspects incoming [[hello notes]] to verify video codec settings and stream ' +
      'directions match server expectations.',
    howItsBuilt:
      'Enforces single-video rules and non-trickle candidate bundling so media ' +
      'negotiation succeeds in one round trip.',
    files: ['internal/camera/sdp.go', 'internal/camera/sdp_test.go'],
    stack: ['Pion SDP'],
  },
  {
    id: 'camera-service',
    code: 'CP',
    name: 'Camera permissions',
    role: 'the guardian access gate',
    group: 'notes',
    whatItDoes:
      'Checks whether a user is registered as an owner or guardian before letting ' +
      'them view a cane camera stream.',
    howItsBuilt:
      'Sits between HTTP handlers and the switchboard to keep database lookups and ' +
      'access rules out of real-time loops.',
    files: ['internal/camera/service.go'],
    stack: ['GORM lookup', 'UUID'],
  },
  {
    id: 'command-queue',
    code: 'CD',
    name: 'Command dispatch',
    role: 'the wake-up channel',
    group: 'notes',
    whatItDoes:
      'Queues commands like [[START_CAMERA_STREAM]] from the phone so the cane ' +
      'can poll and turn on its camera.',
    howItsBuilt:
      'Persisted in PostgreSQL with expiration timestamps and pagination cursors, ' +
      'completely separate from the real-time media path.',
    files: [
      'internal/device_controls/handler.go',
      'internal/device_controls/service.go',
      'internal/device_controls/repo.go',
      'internal/device_controls/dto.go',
      'models/enum.go',
    ],
    stack: ['PostgreSQL', 'GORM', 'JSON payload'],
  },
  {
    id: 'camera-hub',
    code: 'SB',
    name: 'The switchboard',
    role: 'the central switchboard',
    group: 'switchboard',
    whatItDoes:
      'Coordinates live calls across active canes, spinning up session rooms and ' +
      'answering connection offers with [[Pion WebRTC]].',
    howItsBuilt:
      'Operates as a dual-sided exchange where neither cane nor phone talk directly, ' +
      'keeping all session coordination in memory.',
    files: ['internal/camera/hub.go', 'internal/camera/hub_test.go'],
    stack: ['Pion WebRTC', 'sync.Map'],
  },
  {
    id: 'peer-negotiator',
    code: 'CN',
    name: 'Call negotiator',
    role: 'the media negotiator',
    group: 'switchboard',
    whatItDoes:
      'Creates media connection instances and completes the handshake, attaching ' +
      'the video pipe before answering viewers.',
    howItsBuilt:
      'Gathers network candidates locally until negotiation finishes in a single ' +
      'answer, avoiding trickle candidate chatter.',
    files: ['internal/camera/peer.go'],
    stack: ['Pion PeerConnection', 'RTCP'],
  },
  {
    id: 'camera-types',
    code: 'SM',
    name: 'Session models',
    role: 'the state definitions',
    group: 'switchboard',
    whatItDoes:
      'Defines session states, viewer descriptors, publication handles, and domain ' +
      'errors for the switchboard.',
    howItsBuilt:
      'Decouples error handling and state models from low-level media engine ' +
      'dependencies.',
    files: ['internal/camera/types.go'],
    stack: ['google/uuid', 'Go types'],
  },
  {
    id: 'session-actor',
    code: 'SC',
    name: 'Session coordinator',
    role: 'the device room actor',
    group: 'pipe',
    whatItDoes:
      'Serializes state changes for a cane in a dedicated loop, tracking the active ' +
      'broadcaster and attached viewers.',
    howItsBuilt:
      'Runs as a goroutine processing channel commands sequentially, eliminating ' +
      'lock contention and race conditions when watchers join.',
    files: ['internal/camera/session.go'],
    stack: ['Go channels', 'goroutines'],
  },
  {
    id: 'shared-pipe',
    code: 'VP',
    name: 'Shared video pipe',
    role: 'the internal video pipe',
    group: 'pipe',
    whatItDoes:
      'Holds one shared [[video pipe]] inside the server that the cane writes ' +
      'pictures onto and watching phones read from.',
    howItsBuilt:
      'Rewrites sequence numbers and timestamps under an active lease so viewers ' +
      'experience seamless playback across cane reconnects.',
    files: ['internal/camera/relay.go'],
    stack: ['Pion RTP', 'H.264 track'],
  },
  {
    id: 'jwt-auth',
    code: 'GA',
    name: 'Guardian auth',
    role: 'the user authenticator',
    group: 'phone',
    whatItDoes:
      'Validates JSON Web Tokens on user requests to ensure caller identity before ' +
      'any camera view is booked.',
    howItsBuilt:
      'Decodes token claims and injects the user ID into the request context for ' +
      'downstream permission checks.',
    files: ['middleware/jwt_auth.go'],
    stack: ['golang-jwt/jwt', 'Gin middleware'],
  },
  {
    id: 'guardian-app',
    code: 'GP',
    name: 'Guardian phone',
    role: 'the viewer client',
    group: 'phone',
    archetype: 'low-slab',
    height: 1,
    whatItDoes:
      'Displays live video to the caregiver after booking a viewing call and ' +
      'connecting to the server.',
    howItsBuilt:
      'Runs its own WebRTC client; it requests camera activation via HTTP and ' +
      'receives video frames over a dedicated viewer call.',
    files: [],
    stack: ['WebRTC client'],
  },
  {
    id: 'ice-servers',
    code: 'ST',
    name: 'STUN & TURN',
    role: 'the NAT traversal helper',
    group: 'outside',
    archetype: 'low-slab',
    height: 1,
    whatItDoes:
      'Helps cane and phone discover public network addresses and relays packets ' +
      'when cell networks block direct UDP.',
    howItsBuilt:
      'Configured via environment variables on server boot and passed into every ' +
      'connection handshake.',
    files: [],
    stack: ['STUN', 'TURN'],
  },
]

const layoutInputs: LayoutInput<NodeDraft>[] = DRAFTS.map((draft) => {
  const measure = MEASURED[draft.id] ?? { count: draft.files.length, loc: 0 }
  const { archetype, params } = draft.archetype
    ? { archetype: draft.archetype, params: draft.params }
    : deriveArchetype(measure)
  const size = deriveSize(archetype, params, measure)
  return { item: draft, group: draft.group, size }
})

const footprints = packLayout(
  layoutInputs,
  GROUPS.map((g) => g.id),
)

export const NODES: readonly ArchNode[] = DRAFTS.map((draft) => {
  const measure = MEASURED[draft.id] ?? { count: draft.files.length, loc: 0 }
  const { archetype, params } = draft.archetype
    ? { archetype: draft.archetype, params: draft.params }
    : deriveArchetype(measure)
  const height = draft.height ?? (draft.archetype === 'low-slab' ? 1 : deriveHeight(measure))
  const footprint = draft.footprint ?? footprints.get(draft)!
  return {
    ...draft,
    archetype,
    params,
    height,
    footprint,
    count: measure.count,
    loc: measure.loc,
  }
})

export const EDGES: readonly ArchEdge[] = [
  // Flow 1: Ask the cane to start
  {
    id: 'phone-post-cmd',
    from: 'guardian-app',
    to: 'jwt-auth',
    kind: 'call',
    label: 'POST /devices/:id/commands',
    flowIds: ['ask-start'],
  },
  {
    id: 'jwt-to-routes-cmd',
    from: 'jwt-auth',
    to: 'routes-entry',
    kind: 'call',
    label: 'JWTAuthMiddleware',
    flowIds: ['ask-start'],
  },
  {
    id: 'routes-to-create-cmd',
    from: 'routes-entry',
    to: 'command-queue',
    kind: 'call',
    label: 'commandHandler.CreateCommand',
    flowIds: ['ask-start'],
  },
  {
    id: 'cane-poll-cmd',
    from: 'cane-firmware',
    to: 'cane-auth',
    kind: 'call',
    label: 'GET /devices/:id/commands',
    flowIds: ['ask-start'],
  },
  {
    id: 'cane-auth-to-routes-cmd',
    from: 'cane-auth',
    to: 'routes-entry',
    kind: 'call',
    label: 'DeviceAuthMiddleware',
    flowIds: ['ask-start'],
  },
  {
    id: 'routes-to-list-cmd',
    from: 'routes-entry',
    to: 'command-queue',
    kind: 'call',
    label: 'commandHandler.ListCommands',
    flowIds: ['ask-start'],
  },
  {
    id: 'cmd-to-cane',
    from: 'command-queue',
    to: 'cane-firmware',
    kind: 'data',
    label: 'START_CAMERA_STREAM command',
    flowIds: ['ask-start'],
  },

  // Flow 2: Book the cane's call
  {
    id: 'cane-post-offer',
    from: 'cane-firmware',
    to: 'cane-auth',
    kind: 'call',
    label: 'POST /camera/publications',
    flowIds: ['book-cane'],
  },
  {
    id: 'cane-auth-to-routes-pub',
    from: 'cane-auth',
    to: 'routes-entry',
    kind: 'call',
    label: 'registerFirmwareCamera',
    flowIds: ['book-cane'],
  },
  {
    id: 'routes-to-handler-pub',
    from: 'routes-entry',
    to: 'camera-handler',
    kind: 'call',
    label: 'handler.CreatePublication',
    flowIds: ['book-cane'],
  },
  {
    id: 'handler-decode-pub',
    from: 'camera-handler',
    to: 'sdp-decoder',
    kind: 'call',
    label: 'decodePublisherOffer',
    flowIds: ['book-cane'],
  },
  {
    id: 'handler-to-service-pub',
    from: 'camera-handler',
    to: 'camera-service',
    kind: 'call',
    label: 'service.Publish',
    flowIds: ['book-cane'],
  },
  {
    id: 'service-to-hub-pub',
    from: 'camera-service',
    to: 'camera-hub',
    kind: 'call',
    label: 'hub.publish',
    flowIds: ['book-cane'],
  },
  {
    id: 'hub-negotiate-pub',
    from: 'camera-hub',
    to: 'peer-negotiator',
    kind: 'call',
    label: 'negotiatePublisher',
    flowIds: ['book-cane'],
  },
  {
    id: 'hub-to-session-pub',
    from: 'camera-hub',
    to: 'session-actor',
    kind: 'call',
    label: 'getOrCreateSession & replacePublisherCmd',
    flowIds: ['book-cane'],
  },
  {
    id: 'session-activate-lease',
    from: 'session-actor',
    to: 'shared-pipe',
    kind: 'call',
    label: 'relay.activate()',
    flowIds: ['book-cane'],
  },
  {
    id: 'handler-reply-pub',
    from: 'camera-handler',
    to: 'cane-firmware',
    kind: 'data',
    label: '201 Created with reply note',
    flowIds: ['book-cane'],
  },

  // Flow 3: Book the phone's call
  {
    id: 'phone-post-offer',
    from: 'guardian-app',
    to: 'jwt-auth',
    kind: 'call',
    label: 'POST /camera/viewers',
    flowIds: ['book-phone'],
  },
  {
    id: 'jwt-to-routes-view',
    from: 'jwt-auth',
    to: 'routes-entry',
    kind: 'call',
    label: 'registerCamera',
    flowIds: ['book-phone'],
  },
  {
    id: 'routes-to-handler-view',
    from: 'routes-entry',
    to: 'camera-handler',
    kind: 'call',
    label: 'handler.CreateViewer',
    flowIds: ['book-phone'],
  },
  {
    id: 'handler-decode-view',
    from: 'camera-handler',
    to: 'sdp-decoder',
    kind: 'call',
    label: 'decodeViewerOffer',
    flowIds: ['book-phone'],
  },
  {
    id: 'handler-to-service-view',
    from: 'camera-handler',
    to: 'camera-service',
    kind: 'call',
    label: 'service.View',
    flowIds: ['book-phone'],
  },
  {
    id: 'service-to-hub-view',
    from: 'camera-service',
    to: 'camera-hub',
    kind: 'call',
    label: 'hub.view (after IsOwnerOrGuardian)',
    flowIds: ['book-phone'],
  },
  {
    id: 'hub-to-session-view',
    from: 'camera-hub',
    to: 'session-actor',
    kind: 'call',
    label: 'getOrCreateSession & addViewerCmd',
    flowIds: ['book-phone'],
  },
  {
    id: 'hub-negotiate-view',
    from: 'camera-hub',
    to: 'peer-negotiator',
    kind: 'call',
    label: 'negotiateViewer',
    flowIds: ['book-phone'],
  },
  {
    id: 'peer-attach-pipe',
    from: 'peer-negotiator',
    to: 'shared-pipe',
    kind: 'call',
    label: 'pc.AddTrack(relay.track)',
    flowIds: ['book-phone'],
  },
  {
    id: 'handler-reply-view',
    from: 'camera-handler',
    to: 'guardian-app',
    kind: 'data',
    label: '201 Created with reply note',
    flowIds: ['book-phone'],
  },

  // Flow 4: Watch live
  {
    id: 'cane-live-call',
    from: 'cane-firmware',
    to: 'peer-negotiator',
    kind: 'data',
    label: 'live call (Pion OnTrack / RTP)',
    flowIds: ['watch-live'],
  },
  {
    id: 'peer-forward-pipe',
    from: 'peer-negotiator',
    to: 'shared-pipe',
    kind: 'data',
    label: 'forwardRTP -> lease.write()',
    flowIds: ['watch-live'],
  },
  {
    id: 'pipe-to-phone-live',
    from: 'shared-pipe',
    to: 'guardian-app',
    kind: 'data',
    label: 'live call (WriteRTP to viewer)',
    flowIds: ['watch-live'],
  },

  // Support edges (Outside world / types)
  {
    id: 'ice-to-hub',
    from: 'ice-servers',
    to: 'camera-hub',
    kind: 'support',
    label: 'WEBRTC_STUN_URLS / WEBRTC_TURN_*',
    flowIds: [],
  },
  {
    id: 'ice-to-cane',
    from: 'ice-servers',
    to: 'cane-firmware',
    kind: 'support',
    label: 'NAT traversal',
    flowIds: [],
  },
  {
    id: 'ice-to-phone',
    from: 'ice-servers',
    to: 'guardian-app',
    kind: 'support',
    label: 'NAT traversal',
    flowIds: [],
  },
  {
    id: 'types-to-session',
    from: 'camera-types',
    to: 'session-actor',
    kind: 'support',
    label: 'sessionState enums & errors',
    flowIds: [],
  },
]

export const FLOWS: readonly ArchFlow[] = [
  {
    id: 'ask-start',
    name: 'Ask the cane to start',
    payload: 'start command',
    summary:
      'The guardian phone posts a START_CAMERA_STREAM command, which the cane picks up on its next poll.',
    route: [
      'phone-post-cmd',
      'jwt-to-routes-cmd',
      'routes-to-create-cmd',
      'cane-poll-cmd',
      'cane-auth-to-routes-cmd',
      'routes-to-list-cmd',
      'cmd-to-cane',
    ],
  },
  {
    id: 'book-cane',
    name: "Book the cane's call",
    payload: 'hello note',
    summary:
      'The cane sends its video offer over HTTP POST, and the server configures its publisher session and returns the answer.',
    route: [
      'cane-post-offer',
      'cane-auth-to-routes-pub',
      'routes-to-handler-pub',
      'handler-decode-pub',
      'handler-to-service-pub',
      'service-to-hub-pub',
      'hub-negotiate-pub',
      'hub-to-session-pub',
      'session-activate-lease',
      'handler-reply-pub',
    ],
  },
  {
    id: 'book-phone',
    name: "Book the phone's call",
    payload: 'hello note',
    summary:
      'The phone requests to view the stream over HTTP POST; the server verifies guardian permissions, attaches the video pipe, and returns the answer.',
    route: [
      'phone-post-offer',
      'jwt-to-routes-view',
      'routes-to-handler-view',
      'handler-decode-view',
      'handler-to-service-view',
      'service-to-hub-view',
      'hub-to-session-view',
      'hub-negotiate-view',
      'peer-attach-pipe',
      'handler-reply-view',
    ],
  },
  {
    id: 'watch-live',
    name: 'Watch live',
    payload: 'live pictures',
    summary:
      'Live pictures travel from cane camera over WebRTC media sockets onto the server video pipe, which copies frames to the phone.',
    route: [
      'cane-live-call',
      'peer-forward-pipe',
      'pipe-to-phone-live',
    ],
  },
]

export const ARCHITECTURE: ArchitectureData = {
  groups: GROUPS,
  nodes: NODES,
  edges: EDGES,
  flows: FLOWS,
  intro: INTRO,
  unmapped: UNCLAIMED,
  repo: 'canepanion-server',
}
