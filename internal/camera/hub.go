package camera

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

const (
	defaultSTUNURL            = "stun:stun.l.google.com:19302"
	defaultNegotiationTimeout = 10 * time.Second
	sessionCreateAttempts     = 3
	h264PayloadType           = 96
)

type Hub struct {
	sessions sync.Map

	api                *webrtc.API
	iceServers         []webrtc.ICEServer
	negotiationTimeout time.Duration
	closed             atomic.Bool
}

func NewHubFromEnv() (*Hub, error) {
	iceServers, err := iceServersFromEnv()
	if err != nil {
		return nil, err
	}
	timeout, err := negotiationTimeoutFromEnv()
	if err != nil {
		return nil, err
	}

	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000},
		PayloadType:        h264PayloadType,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("register H264 codec: %w", err)
	}

	interceptorRegistry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		return nil, fmt.Errorf("register default interceptors: %w", err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithInterceptorRegistry(interceptorRegistry))

	return &Hub{api: api, iceServers: iceServers, negotiationTimeout: timeout}, nil
}

func iceServersFromEnv() ([]webrtc.ICEServer, error) {
	var servers []webrtc.ICEServer

	if stunURLs := splitEnvList("WEBRTC_STUN_URLS", defaultSTUNURL); len(stunURLs) > 0 {
		servers = append(servers, webrtc.ICEServer{URLs: stunURLs})
	}

	turnURLs := splitEnvList("WEBRTC_TURN_URLS", "")
	if len(turnURLs) == 0 {
		return servers, nil
	}

	username := os.Getenv("WEBRTC_TURN_USERNAME")
	credential := os.Getenv("WEBRTC_TURN_CREDENTIAL")
	if username == "" || credential == "" {
		return nil, fmt.Errorf("WEBRTC_TURN_USERNAME and WEBRTC_TURN_CREDENTIAL are required when WEBRTC_TURN_URLS is set")
	}
	return append(servers, webrtc.ICEServer{URLs: turnURLs, Username: username, Credential: credential}), nil
}

func splitEnvList(key, fallback string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		raw = fallback
	}
	if raw == "" {
		return nil
	}

	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func negotiationTimeoutFromEnv() (time.Duration, error) {
	raw := os.Getenv("WEBRTC_NEGOTIATION_TIMEOUT")
	if raw == "" {
		return defaultNegotiationTimeout, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid WEBRTC_NEGOTIATION_TIMEOUT: %w", err)
	}
	return d, nil
}

func asDeviceSession(v any) *deviceSession {
	session, ok := v.(*deviceSession)
	if !ok {
		panic("camera hub stored a non-session value")
	}
	return session
}

func (h *Hub) loadedSession(deviceID uuid.UUID) (*deviceSession, bool) {
	existing, ok := h.sessions.Load(deviceID)
	if !ok {
		return nil, false
	}
	return asDeviceSession(existing), true
}

func (h *Hub) Close() error {
	h.closed.Store(true)

	var sessions []*deviceSession
	h.sessions.Range(func(_, v any) bool {
		sessions = append(sessions, asDeviceSession(v))
		return true
	})
	for _, s := range sessions {
		s.closeAll()
	}
	for _, s := range sessions {
		<-s.done
	}
	return nil
}

func (h *Hub) getOrCreateSession(deviceID uuid.UUID) (*deviceSession, error) {
	if session, ok := h.loadedSession(deviceID); ok {
		return session, nil
	}

	relay, err := newH264Relay(deviceID.String())
	if err != nil {
		return nil, fmt.Errorf("create device relay: %w", err)
	}
	session := newDeviceSession(deviceID, relay, func(s *deviceSession) {
		h.sessions.CompareAndDelete(deviceID, s)
	})

	actual, loaded := h.sessions.LoadOrStore(deviceID, session)
	if loaded {
		session.closeAll()
		return asDeviceSession(actual), nil
	}
	return session, nil
}

func (h *Hub) publish(ctx context.Context, deviceID uuid.UUID, offer publisherOffer) (publication, error) {
	if h.closed.Load() {
		return publication{}, errHubClosed
	}

	peer, answer, err := h.negotiatePublisher(ctx, offer)
	if err != nil {
		return publication{}, err
	}

	var lastErr error
	for attempt := 0; attempt < sessionCreateAttempts; attempt++ {
		session, err := h.getOrCreateSession(deviceID)
		if err != nil {
			_ = peer.pc.Close()
			return publication{}, err
		}

		lease := session.relay.activate()
		peer.lease = lease

		reply := make(chan replacePublisherResult, 1)
		if err := session.submit(replacePublisherCmd{peer: peer, reply: reply}); err != nil {
			if errors.Is(err, errSessionClosed) {
				lastErr = err
				continue
			}
			_ = peer.pc.Close()
			return publication{}, err
		}

		watchConnectionState(session, peerKindPublisher, peer.id, peer.pc, peer.closed)
		go func() {
			select {
			case <-peer.remote.ready:
				forwardRTP(peer.remote.track.Load(), lease)
			case <-peer.closed:
			}
		}()

		if result := <-reply; result.old != nil {
			_ = result.old.pc.Close()
		}
		return publication{id: peer.id, answer: answer}, nil
	}

	_ = peer.pc.Close()
	return publication{}, lastErr
}

func (h *Hub) view(ctx context.Context, deviceID, principal uuid.UUID, offer viewerOffer) (viewerHandle, error) {
	if h.closed.Load() {
		return viewerHandle{}, errHubClosed
	}

	var lastErr error
	for attempt := 0; attempt < sessionCreateAttempts; attempt++ {
		session, err := h.getOrCreateSession(deviceID)
		if err != nil {
			return viewerHandle{}, err
		}

		peer, answer, err := h.negotiateViewer(ctx, principal, offer, session.relay)
		if err != nil {
			return viewerHandle{}, err
		}

		reply := make(chan error, 1)
		if err := session.submit(addViewerCmd{peer: peer, reply: reply}); err != nil {
			_ = peer.pc.Close()
			if errors.Is(err, errSessionClosed) {
				lastErr = err
				continue
			}
			return viewerHandle{}, err
		}
		if err := <-reply; err != nil {
			_ = peer.pc.Close()
			return viewerHandle{}, err
		}

		watchConnectionState(session, peerKindViewer, peer.id, peer.pc, nil)
		go drainSenderRTCP(peer.sender, session)

		return viewerHandle{id: peer.id, answer: answer}, nil
	}
	return viewerHandle{}, lastErr
}

func (h *Hub) stopPublication(_ context.Context, deviceID, publicationID uuid.UUID) error {
	session, ok := h.loadedSession(deviceID)
	if !ok {
		return nil
	}

	reply := make(chan *publisherPeer, 1)
	if err := session.submit(removePublisherCmd{id: publicationID, reply: reply}); err != nil {
		return nil
	}
	if removed := <-reply; removed != nil {
		_ = removed.pc.Close()
	}
	return nil
}

func (h *Hub) stopViewer(_ context.Context, deviceID, principal, viewerID uuid.UUID) error {
	session, ok := h.loadedSession(deviceID)
	if !ok {
		return nil
	}

	reply := make(chan removeViewerResult, 1)
	if err := session.submit(removeViewerCmd{id: viewerID, principal: principal, reply: reply}); err != nil {
		return nil
	}
	result := <-reply
	if result.err != nil {
		return result.err
	}
	if result.peer != nil {
		_ = result.peer.pc.Close()
	}
	return nil
}

func (h *Hub) status(deviceID uuid.UUID) sessionStatus {
	session, ok := h.loadedSession(deviceID)
	if !ok {
		return sessionStatus{state: sessionStateOffline}
	}

	reply := make(chan sessionStatus, 1)
	if err := session.submit(getStatusCmd{reply: reply}); err != nil {
		return sessionStatus{state: sessionStateOffline}
	}
	return <-reply
}
