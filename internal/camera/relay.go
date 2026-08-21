package camera

import (
	"sync"
	"sync/atomic"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

type h264Relay struct {
	track          *webrtc.TrackLocalStaticRTP
	fastGeneration atomic.Uint64

	mu                  sync.Mutex
	generation          uint64
	nextSequence        uint16
	sourceStarted       bool
	sourceTimestampBase uint32
	outputTimestampBase uint32
	haveOutputTimestamp bool
	lastOutputTimestamp uint32
}

type relayLease struct {
	relay      *h264Relay
	generation uint64
}

func newH264Relay(streamID string) (*h264Relay, error) {
	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000},
		"camera",
		streamID,
	)
	if err != nil {
		return nil, err
	}
	return &h264Relay{track: track}, nil
}

func (r *h264Relay) activate() relayLease {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.generation++
	r.sourceStarted = false
	r.fastGeneration.Store(r.generation)
	return relayLease{relay: r, generation: r.generation}
}

// Pion rewrites SSRC and payload type per viewer binding in writeRTP.
func (l relayLease) write(packet *rtp.Packet) error {
	r := l.relay
	if r.fastGeneration.Load() != l.generation {
		return errStaleGeneration
	}

	r.mu.Lock()
	if r.generation != l.generation {
		r.mu.Unlock()
		return errStaleGeneration
	}

	out := *packet
	out.Extension = false
	out.Extensions = nil
	out.Padding = false
	out.PaddingSize = 0

	if !r.sourceStarted {
		r.sourceStarted = true
		r.sourceTimestampBase = packet.Timestamp
		if r.haveOutputTimestamp {
			r.outputTimestampBase = r.lastOutputTimestamp + 1
		} else {
			r.outputTimestampBase = packet.Timestamp
		}
	}
	out.Timestamp = r.outputTimestampBase + (packet.Timestamp - r.sourceTimestampBase)
	out.SequenceNumber = r.nextSequence

	r.nextSequence++
	r.lastOutputTimestamp = out.Timestamp
	r.haveOutputTimestamp = true
	r.mu.Unlock()

	return r.track.WriteRTP(&out)
}
