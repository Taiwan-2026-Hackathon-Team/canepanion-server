package camera

import (
	"sync"
	"sync/atomic"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// h264Relay is the one stable local track a device session offers to every
// viewer. Viewers bind to it before a publisher exists; a publisher
// replacement changes the relay's active generation, not the viewer tracks,
// so nobody renegotiates when the cane reconnects.
type h264Relay struct {
	track *webrtc.TrackLocalStaticRTP

	// fastGeneration lets a stale publisher's write bail out without taking
	// mu. mu remains the authority: every write re-checks under lock, so the
	// two checks can never both pass for a generation activate() already
	// superseded.
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

// relayLease is the capability a publisher peer writes through. It carries
// the generation it was issued for, so a replaced publisher's in-flight RTP
// is silently dropped instead of corrupting the live stream.
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

// activate starts a fresh generation for a newly negotiated publisher.
func (r *h264Relay) activate() relayLease {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.generation++
	r.sourceStarted = false
	r.fastGeneration.Store(r.generation)
	return relayLease{relay: r, generation: r.generation}
}

// write rewrites sequence numbers and timestamps so a replacement publisher
// does not look like a new RTP source to already-connected viewers, strips
// header extensions and padding that were never negotiated with viewers, and
// forwards through the shared local track. Pion applies each viewer
// binding's own negotiated SSRC and payload type during that write.
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
