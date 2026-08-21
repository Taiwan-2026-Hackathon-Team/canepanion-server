package camera

import (
	"errors"

	"github.com/google/uuid"
)

const maxViewersPerDevice = 4

type h264Profile struct {
	profileLevelID    string
	packetizationMode uint8
	clockRate         uint32
}

type publisherOffer struct {
	rawSDP string
	codec  h264Profile
}

type viewerOffer struct {
	rawSDP string
	codec  h264Profile
}

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
