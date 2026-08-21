package camera

import (
	"time"

	"github.com/google/uuid"
	"github.com/pion/rtcp"
)

// deviceSession is one device's publisher and viewer lifecycle actor. Only
// its run goroutine reads or writes the publisher pointer and viewer map,
// so replacement and removal never race per separate-before-serializing
// shared state: every caller reaches this state through a channel, not a
// shared lock.
type deviceSession struct {
	id       uuid.UUID
	relay    *h264Relay
	commands chan sessionCommand
	done     chan struct{}
}

type sessionCommand interface{}

type replacePublisherCmd struct {
	peer  *publisherPeer
	reply chan replacePublisherResult
}

type replacePublisherResult struct {
	old *publisherPeer
}

type removePublisherCmd struct {
	id    uuid.UUID
	reply chan *publisherPeer
}

type addViewerCmd struct {
	peer  *viewerPeer
	reply chan error
}

type removeViewerCmd struct {
	id        uuid.UUID
	principal uuid.UUID
	reply     chan removeViewerResult
}

type removeViewerResult struct {
	peer *viewerPeer
	err  error
}

type peerEndedCmd struct {
	kind peerKind
	id   uuid.UUID
}

type getStatusCmd struct {
	reply chan sessionStatus
}

type requestKeyframeCmd struct{}

type closeAllCmd struct{}

func newDeviceSession(id uuid.UUID, relay *h264Relay, onIdle func(*deviceSession)) *deviceSession {
	s := &deviceSession{
		id:       id,
		relay:    relay,
		commands: make(chan sessionCommand),
		done:     make(chan struct{}),
	}
	go s.run(onIdle)
	return s
}

// submit hands a command to the actor. If the actor already exited, the
// caller gets errSessionClosed and is expected to look up (or create) a
// fresh session rather than block forever on a receiver that is gone.
func (s *deviceSession) submit(cmd sessionCommand) error {
	select {
	case s.commands <- cmd:
		return nil
	case <-s.done:
		return errSessionClosed
	}
}

func (s *deviceSession) closeAll() {
	_ = s.submit(closeAllCmd{})
}

func (s *deviceSession) run(onIdle func(*deviceSession)) { //nolint:gocognit
	defer close(s.done)

	var publisher *publisherPeer
	viewers := map[uuid.UUID]*viewerPeer{}
	var lastPLI time.Time

	for cmd := range s.commands {
		switch c := cmd.(type) {
		case replacePublisherCmd:
			old := publisher
			publisher = c.peer
			c.reply <- replacePublisherResult{old: old}

		case removePublisherCmd:
			var removed *publisherPeer
			if publisher != nil && publisher.id == c.id {
				removed = publisher
				publisher = nil
			}
			c.reply <- removed

		case addViewerCmd:
			if len(viewers) >= maxViewersPerDevice {
				c.reply <- errTooManyViewers
				continue
			}
			viewers[c.peer.id] = c.peer
			c.reply <- nil

		case removeViewerCmd:
			peer, ok := viewers[c.id]
			if !ok {
				c.reply <- removeViewerResult{}
				continue
			}
			if peer.principal != c.principal {
				c.reply <- removeViewerResult{err: errNotViewerOwner}
				continue
			}
			delete(viewers, c.id)
			c.reply <- removeViewerResult{peer: peer}

		case peerEndedCmd:
			switch c.kind {
			case peerKindPublisher:
				if publisher != nil && publisher.id == c.id {
					publisher = nil
				}
			case peerKindViewer:
				delete(viewers, c.id)
			}

		case requestKeyframeCmd:
			if publisher != nil && time.Since(lastPLI) >= pliCoalesceWindow {
				if ssrc, ok := publisherSSRC(publisher); ok {
					lastPLI = time.Now()
					_ = publisher.pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: ssrc}})
				}
			}
			continue

		case getStatusCmd:
			c.reply <- sessionStatus{state: deriveSessionState(publisher, len(viewers)), viewerCount: len(viewers)}
			continue

		case closeAllCmd:
			for _, v := range viewers {
				_ = v.pc.Close()
			}
			viewers = map[uuid.UUID]*viewerPeer{}
			if publisher != nil {
				_ = publisher.pc.Close()
				publisher = nil
			}
		}

		if publisher == nil && len(viewers) == 0 {
			onIdle(s)
			return
		}
	}
}

func publisherSSRC(p *publisherPeer) (uint32, bool) {
	track := p.remoteTrack.Load()
	if track == nil {
		return 0, false
	}
	return uint32(track.SSRC()), true
}

func deriveSessionState(publisher *publisherPeer, viewerCount int) sessionState {
	switch {
	case publisher != nil:
		return sessionStateLive
	case viewerCount > 0:
		return sessionStateWaiting
	default:
		return sessionStateOffline
	}
}
