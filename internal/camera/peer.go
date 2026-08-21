package camera

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"canepanion-server/pkg/utils"

	"github.com/google/uuid"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

const disconnectedGrace = 15 * time.Second

const pliCoalesceWindow = 500 * time.Millisecond

type peerKind int

const (
	peerKindPublisher peerKind = iota
	peerKindViewer
)

type remoteTrackOnce struct {
	track atomic.Pointer[webrtc.TrackRemote]
	ready chan struct{}
	once  sync.Once
}

func newRemoteTrackOnce() remoteTrackOnce {
	return remoteTrackOnce{ready: make(chan struct{})}
}

func (r *remoteTrackOnce) set(track *webrtc.TrackRemote) {
	r.track.Store(track)
	r.once.Do(func() { close(r.ready) })
}

type publisherPeer struct {
	id     uuid.UUID
	pc     *webrtc.PeerConnection
	lease  relayLease
	remote remoteTrackOnce
	closed chan struct{}
}

type viewerPeer struct {
	id        uuid.UUID
	principal uuid.UUID
	pc        *webrtc.PeerConnection
	sender    *webrtc.RTPSender
}

func (h *Hub) negotiatePublisher(ctx context.Context, offer publisherOffer) (*publisherPeer, localAnswer, error) {
	pc, err := h.api.NewPeerConnection(webrtc.Configuration{ICEServers: h.iceServers})
	if err != nil {
		return nil, localAnswer{}, fmt.Errorf("create publisher peer connection: %w", err)
	}

	peer := &publisherPeer{
		id:     utils.GenerateUUID(),
		pc:     pc,
		remote: newRemoteTrackOnce(),
		closed: make(chan struct{}),
	}

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		go drainReceiverRTCP(receiver)
		peer.remote.set(track)
	})

	answer, err := negotiateNonTrickle(ctx, pc, offer.rawSDP, h.negotiationTimeout)
	if err != nil {
		_ = pc.Close()
		return nil, localAnswer{}, err
	}
	return peer, answer, nil
}

// AddTrack must run before SetRemoteDescription: pion matches the offer's
// recvonly section against GetTransceivers() at that call, not later.
func (h *Hub) negotiateViewer(
	ctx context.Context,
	principal uuid.UUID,
	offer viewerOffer,
	relay *h264Relay,
) (*viewerPeer, localAnswer, error) {
	pc, err := h.api.NewPeerConnection(webrtc.Configuration{ICEServers: h.iceServers})
	if err != nil {
		return nil, localAnswer{}, fmt.Errorf("create viewer peer connection: %w", err)
	}

	sender, err := pc.AddTrack(relay.track)
	if err != nil {
		_ = pc.Close()
		return nil, localAnswer{}, fmt.Errorf("attach relay track: %w", err)
	}

	peer := &viewerPeer{id: utils.GenerateUUID(), principal: principal, pc: pc, sender: sender}

	answer, err := negotiateNonTrickle(ctx, pc, offer.rawSDP, h.negotiationTimeout)
	if err != nil {
		_ = pc.Close()
		return nil, localAnswer{}, err
	}
	return peer, answer, nil
}

func negotiateNonTrickle(ctx context.Context, pc *webrtc.PeerConnection, offerSDP string, timeout time.Duration) (localAnswer, error) {
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offerSDP}); err != nil {
		return localAnswer{}, fmt.Errorf("set remote description: %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return localAnswer{}, fmt.Errorf("create answer: %w", err)
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		return localAnswer{}, fmt.Errorf("set local description: %w", err)
	}

	negotiationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-gatherComplete:
	case <-negotiationCtx.Done():
		return localAnswer{}, errNegotiationTime
	}

	return localAnswer{sdp: pc.LocalDescription().SDP}, nil
}

func watchConnectionState(session *deviceSession, kind peerKind, id uuid.UUID, pc *webrtc.PeerConnection, closed chan struct{}) {
	var mu sync.Mutex
	var disconnectedTimer *time.Timer
	var once sync.Once

	end := func() {
		once.Do(func() {
			mu.Lock()
			if disconnectedTimer != nil {
				disconnectedTimer.Stop()
			}
			mu.Unlock()
			if closed != nil {
				close(closed)
			}
			_ = session.submit(peerEndedCmd{kind: kind, id: id})
			_ = pc.Close()
		})
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			end()
		case webrtc.PeerConnectionStateDisconnected:
			mu.Lock()
			disconnectedTimer = time.AfterFunc(disconnectedGrace, func() {
				if pc.ConnectionState() == webrtc.PeerConnectionStateDisconnected {
					end()
				}
			})
			mu.Unlock()
		case webrtc.PeerConnectionStateConnected:
			mu.Lock()
			if disconnectedTimer != nil {
				disconnectedTimer.Stop()
			}
			mu.Unlock()
		}
	})
}

func forwardRTP(track *webrtc.TrackRemote, lease relayLease) {
	for {
		packet, _, err := track.ReadRTP()
		if err != nil {
			return
		}
		_ = lease.write(packet)
	}
}

func drainReceiverRTCP(receiver *webrtc.RTPReceiver) {
	for {
		if _, _, err := receiver.ReadRTCP(); err != nil {
			return
		}
	}
}

// ReadRTCP is the only path that drains Pion's interceptor chain (NACK and friends).
func drainSenderRTCP(sender *webrtc.RTPSender, session *deviceSession) {
	for {
		packets, _, err := sender.ReadRTCP()
		if err != nil {
			return
		}
		for _, pkt := range packets {
			switch pkt.(type) {
			case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
				_ = session.submit(requestKeyframeCmd{})
			}
		}
	}
}
