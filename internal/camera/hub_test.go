package camera

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	hub, err := NewHubFromEnv()
	if err != nil {
		t.Fatalf("NewHubFromEnv: %v", err)
	}
	hub.iceServers = nil
	hub.negotiationTimeout = 5 * time.Second
	t.Cleanup(func() { _ = hub.Close() })
	return hub
}

func newClientTestAPI(t *testing.T) *webrtc.API {
	t.Helper()
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		t.Fatalf("register H264 codec: %v", err)
	}

	interceptorRegistry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		t.Fatalf("register default interceptors: %v", err)
	}

	return webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithInterceptorRegistry(interceptorRegistry))
}

func createOfferSDP(t *testing.T, pc *webrtc.PeerConnection) string {
	t.Helper()
	gatherComplete := webrtc.GatheringCompletePromise(pc)

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local description: %v", err)
	}

	select {
	case <-gatherComplete:
	case <-time.After(5 * time.Second):
		t.Fatalf("ICE gathering did not complete")
	}
	return pc.LocalDescription().SDP
}

func newPublisherClient(t *testing.T) (*webrtc.PeerConnection, *webrtc.TrackLocalStaticSample, string) {
	t.Helper()
	pc, err := newClientTestAPI(t).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("new publisher peer connection: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000}, "video", "publisher",
	)
	if err != nil {
		t.Fatalf("new local track: %v", err)
	}
	if _, err := pc.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
		t.Fatalf("add sendonly transceiver: %v", err)
	}

	return pc, track, createOfferSDP(t, pc)
}

func newViewerClient(t *testing.T) (pc *webrtc.PeerConnection, offerSDP string, rtpReceived <-chan struct{}) {
	t.Helper()
	pc, err := newClientTestAPI(t).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("new viewer peer connection: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	received := make(chan struct{}, 1)
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		go func() {
			for {
				if _, _, err := receiver.ReadRTCP(); err != nil {
					return
				}
			}
		}()
		go func() {
			if _, _, err := track.ReadRTP(); err == nil {
				select {
				case received <- struct{}{}:
				default:
				}
			}
		}()
	})

	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		t.Fatalf("add recvonly transceiver: %v", err)
	}

	return pc, createOfferSDP(t, pc), received
}

func waitConnected(t *testing.T, pc *webrtc.PeerConnection, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pc.ConnectionState() == webrtc.PeerConnectionStateConnected {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("peer connection did not reach connected state within %s (last state=%s)", timeout, pc.ConnectionState())
}

func startWritingSamples(t *testing.T, track *webrtc.TrackLocalStaticSample, stop <-chan struct{}) {
	t.Helper()
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		fakeNAL := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0xBB, 0xCC, 0xDD}
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_ = track.WriteSample(media.Sample{Data: fakeNAL, Duration: 20 * time.Millisecond})
			}
		}
	}()
}

func TestHub_PublishThenView_RelaysMedia(t *testing.T) {
	hub := newTestHub(t)
	ctx := context.Background()
	deviceID := uuid.New()

	pubPC, track, pubOfferSDP := newPublisherClient(t)
	pubOffer, err := decodePublisherOffer(strings.NewReader(pubOfferSDP))
	if err != nil {
		t.Fatalf("decodePublisherOffer: %v", err)
	}

	pub, err := hub.publish(ctx, deviceID, pubOffer)
	if err != nil {
		t.Fatalf("hub.publish: %v", err)
	}
	if err := pubPC.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: pub.answer.sdp}); err != nil {
		t.Fatalf("publisher SetRemoteDescription: %v", err)
	}
	waitConnected(t, pubPC, 10*time.Second)

	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	startWritingSamples(t, track, stop)

	viewPC, viewOfferSDP, rtpReceived := newViewerClient(t)
	viewOffer, err := decodeViewerOffer(strings.NewReader(viewOfferSDP))
	if err != nil {
		t.Fatalf("decodeViewerOffer: %v", err)
	}

	principal := uuid.New()
	handle, err := hub.view(ctx, deviceID, principal, viewOffer)
	if err != nil {
		t.Fatalf("hub.view: %v", err)
	}
	if err := viewPC.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: handle.answer.sdp}); err != nil {
		t.Fatalf("viewer SetRemoteDescription: %v", err)
	}
	waitConnected(t, viewPC, 10*time.Second)

	select {
	case <-rtpReceived:
	case <-time.After(10 * time.Second):
		t.Fatal("viewer never received a relayed RTP packet")
	}

	status := hub.status(deviceID)
	if status.state != sessionStateLive {
		t.Fatalf("expected LIVE, got %s", status.state)
	}
	if status.viewerCount != 1 {
		t.Fatalf("expected 1 viewer, got %d", status.viewerCount)
	}

	if err := hub.stopViewer(ctx, deviceID, principal, handle.id); err != nil {
		t.Fatalf("stopViewer: %v", err)
	}
	if err := hub.stopPublication(ctx, deviceID, pub.id); err != nil {
		t.Fatalf("stopPublication: %v", err)
	}
}

func TestHub_ReplacePublisher_SwapsCurrentPublication(t *testing.T) {
	hub := newTestHub(t)
	ctx := context.Background()
	deviceID := uuid.New()

	_, _, firstOfferSDP := newPublisherClient(t)
	firstOffer, err := decodePublisherOffer(strings.NewReader(firstOfferSDP))
	if err != nil {
		t.Fatalf("decodePublisherOffer: %v", err)
	}
	firstPub, err := hub.publish(ctx, deviceID, firstOffer)
	if err != nil {
		t.Fatalf("hub.publish (first): %v", err)
	}

	_, _, secondOfferSDP := newPublisherClient(t)
	secondOffer, err := decodePublisherOffer(strings.NewReader(secondOfferSDP))
	if err != nil {
		t.Fatalf("decodePublisherOffer: %v", err)
	}
	secondPub, err := hub.publish(ctx, deviceID, secondOffer)
	if err != nil {
		t.Fatalf("hub.publish (second): %v", err)
	}

	if firstPub.id == secondPub.id {
		t.Fatalf("expected distinct publication IDs")
	}

	if err := hub.stopPublication(ctx, deviceID, firstPub.id); err != nil {
		t.Fatalf("stopPublication(stale first): %v", err)
	}
	if status := hub.status(deviceID); status.state != sessionStateLive {
		t.Fatalf("stale publication ID must not end the current publication, got %s", status.state)
	}

	if err := hub.stopPublication(ctx, deviceID, secondPub.id); err != nil {
		t.Fatalf("stopPublication(current second): %v", err)
	}
	if status := hub.status(deviceID); status.state != sessionStateOffline {
		t.Fatalf("expected OFFLINE after stopping the current publication, got %s", status.state)
	}
}

func TestHub_StopPublicationIsIdempotent(t *testing.T) {
	hub := newTestHub(t)
	ctx := context.Background()
	deviceID := uuid.New()

	_, _, offerSDP := newPublisherClient(t)
	offer, err := decodePublisherOffer(strings.NewReader(offerSDP))
	if err != nil {
		t.Fatalf("decodePublisherOffer: %v", err)
	}
	pub, err := hub.publish(ctx, deviceID, offer)
	if err != nil {
		t.Fatalf("hub.publish: %v", err)
	}

	if err := hub.stopPublication(ctx, deviceID, pub.id); err != nil {
		t.Fatalf("first stopPublication: %v", err)
	}
	if err := hub.stopPublication(ctx, deviceID, pub.id); err != nil {
		t.Fatalf("second stopPublication should be idempotent: %v", err)
	}
	if err := hub.stopPublication(ctx, uuid.New(), uuid.New()); err != nil {
		t.Fatalf("stopPublication on an unknown device should be idempotent: %v", err)
	}

	if status := hub.status(deviceID); status.state != sessionStateOffline {
		t.Fatalf("expected OFFLINE, got %s", status.state)
	}
}

func TestHub_TooManyViewersRejected(t *testing.T) {
	hub := newTestHub(t)
	ctx := context.Background()
	deviceID := uuid.New()

	_, _, pubOfferSDP := newPublisherClient(t)
	pubOffer, err := decodePublisherOffer(strings.NewReader(pubOfferSDP))
	if err != nil {
		t.Fatalf("decodePublisherOffer: %v", err)
	}
	if _, err := hub.publish(ctx, deviceID, pubOffer); err != nil {
		t.Fatalf("hub.publish: %v", err)
	}

	for i := 0; i < maxViewersPerDevice; i++ {
		_, offerSDP, _ := newViewerClient(t)
		offer, err := decodeViewerOffer(strings.NewReader(offerSDP))
		if err != nil {
			t.Fatalf("decodeViewerOffer %d: %v", i, err)
		}
		if _, err := hub.view(ctx, deviceID, uuid.New(), offer); err != nil {
			t.Fatalf("hub.view %d: %v", i, err)
		}
	}

	_, offerSDP, _ := newViewerClient(t)
	offer, err := decodeViewerOffer(strings.NewReader(offerSDP))
	if err != nil {
		t.Fatalf("decodeViewerOffer (over limit): %v", err)
	}
	if _, err := hub.view(ctx, deviceID, uuid.New(), offer); !errors.Is(err, errTooManyViewers) {
		t.Fatalf("expected errTooManyViewers, got %v", err)
	}
}

func TestHub_StatusOfflineForUnknownDevice(t *testing.T) {
	hub := newTestHub(t)
	status := hub.status(uuid.New())
	if status.state != sessionStateOffline || status.viewerCount != 0 {
		t.Fatalf("expected OFFLINE/0, got %+v", status)
	}
}
