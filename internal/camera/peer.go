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

// disconnectedGrace is how long a peer may sit in the ICE "disconnected"
// state before it is treated as ended. A short network blip should not tear
// down the session.
const disconnectedGrace = 15 * time.Second

// pliCoalesceWindow rate-limits keyframe requests forwarded to the
// publisher. Several viewers reporting loss around the same time collapse
// into one PLI instead of a storm.
const pliCoalesceWindow = 500 * time.Millisecond

type peerKind int

const (
	peerKindPublisher peerKind = iota
	peerKindViewer
)

type publisherPeer struct {
	id    uuid.UUID
	pc    *webrtc.PeerConnection
	lease relayLease

	// remoteTrack is published through trackReady (closed exactly once) so
	// both the RTP-forwarding goroutine and requestKeyframeCmd's SSRC lookup
	// observe it safely without racing each other for a single value.
	remoteTrack  atomic.Pointer[webrtc.TrackRemote]
	trackReady   chan struct{}
	trackReadyOK sync.Once

	// closed is signaled when the connection ends, so a publisher that
	// never sends media does not leak the goroutine waiting on trackReady.
	closed chan struct{}
}

type viewerPeer struct {
	id        uuid.UUID
	principal uuid.UUID
	pc        *webrtc.PeerConnection
	sender    *webrtc.RTPSender
}

// negotiatePublisher creates the WHIP peer connection, applies the offer,
// and answers after local ICE gathering completes. It does not touch any
// session or relay; the caller wires the remote track to a relay lease once
// it knows which device session owns this publisher.
func (h *Hub) negotiatePublisher(ctx context.Context, offer publisherOffer) (*publisherPeer, localAnswer, error) {
	pc, err := h.api.NewPeerConnection(webrtc.Configuration{ICEServers: h.iceServers})
	if err != nil {
		return nil, localAnswer{}, fmt.Errorf("create publisher peer connection: %w", err)
	}

	peer := &publisherPeer{
		id:         utils.GenerateUUID(),
		pc:         pc,
		trackReady: make(chan struct{}),
		closed:     make(chan struct{}),
	}

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		go drainReceiverRTCP(receiver)
		peer.remoteTrack.Store(track)
		peer.trackReadyOK.Do(func() { close(peer.trackReady) })
	})

	answer, err := negotiateNonTrickle(ctx, pc, offer.rawSDP, h.negotiationTimeout)
	if err != nil {
		_ = pc.Close()
		return nil, localAnswer{}, err
	}
	return peer, answer, nil
}

// negotiateViewer creates the WHEP peer connection bound to the session's
// stable relay track, then applies the offer and answers after local ICE
// gathering completes. The relay track is attached before SetRemoteDescription
// so Pion matches it to the offer's recvonly video section.
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

// watchConnectionState posts a resource-ID-qualified peerEndedCmd when the
// peer fails or closes outright, or when it stays disconnected past the
// grace period. Stale callbacks from a replaced peer carry the old ID, so
// the session actor's compare-before-remove keeps them harmless.
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

// drainSenderRTCP pumps a viewer's RTPSender so Pion's interceptors process
// retransmission feedback, and coalesces any PLI/FIR into a keyframe request
// forwarded to the device's current publisher.
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
