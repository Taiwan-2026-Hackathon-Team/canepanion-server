// Package camera implements a non-trickle WHIP/WHEP relay so one cane
// publishes H264 over WebRTC and up to a few guardian viewers watch it. The
// HTTP boundary (sdp.go, handler.go) is the only place that touches raw SDP
// bytes; everything past it operates on the opaque, pre-validated types
// below, per boundary-discipline.
package camera

import (
	"errors"

	"github.com/google/uuid"
)

// maxViewersPerDevice bounds fan-out per device relay. WAITING/LIVE state
// and the 429 response for an over-limit viewer both key off this constant.
const maxViewersPerDevice = 4

// h264Profile is the negotiated codec shape a publisher or viewer offer
// committed to. Constructing one requires passing the sdp.go boundary checks,
// so hub/session/relay code trusts packetization mode and clock rate.
type h264Profile struct {
	profileLevelID    string
	packetizationMode uint8
	clockRate         uint32
}

// publisherOffer and viewerOffer can only be built by decodePublisherOffer
// and decodeViewerOffer. Their fields stay private so the rest of the
// package may trust direction, media count, ICE, DTLS, rtcp-mux, and codec
// invariants without re-checking them.
type publisherOffer struct {
	rawSDP string
	codec  h264Profile
}

type viewerOffer struct {
	rawSDP string
	codec  h264Profile
}

// localAnswer is the SDP the hub generated after ICE gathering completed.
// Only the HTTP handler serializes it into a response body.
type localAnswer struct {
	sdp string
}

type publication struct {
	id     uuid.UUID
	answer localAnswer
}

type viewerHandle struct {
	id     uuid.UUID
	answer localAnswer
}

// sessionState is the state machine the GET /camera endpoint reports.
// LIVE means a publisher is attached; WAITING means viewers exist without
// one; OFFLINE means the device has no session at all.
type sessionState string

const (
	sessionStateOffline sessionState = "OFFLINE"
	sessionStateWaiting sessionState = "WAITING"
	sessionStateLive    sessionState = "LIVE"
)

type sessionStatus struct {
	state       sessionState
	viewerCount int
}

var (
	errHubClosed       = errors.New("camera hub closed")
	errSessionClosed   = errors.New("device session closed")
	errUnsupportedSDP  = errors.New("unsupported camera SDP")
	errSDPTooLarge     = errors.New("camera SDP exceeds size limit")
	errSDPSyntax       = errors.New("camera SDP is not valid")
	errNotViewerOwner  = errors.New("viewer belongs to another user")
	errTooManyViewers  = errors.New("device already has the maximum number of viewers")
	errNegotiationTime = errors.New("camera negotiation timed out")
	errStaleGeneration = errors.New("camera relay generation is stale")
)
