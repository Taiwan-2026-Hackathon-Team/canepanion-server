package caneingest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// h264FmtpLine advertises constrained baseline. The relay parses this line and
// requires packetization-mode=1 (internal/camera/sdp.go).
const h264FmtpLine = "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"

const h264PayloadType = 96

// Publisher is a WHIP client built on pion.
//
// It exists because FFmpeg's WHIP muxer cannot satisfy this relay: it offers
// a=setup:passive where actpass is required, and inlines no ICE candidates
// where at least one is required. Pion offers actpass and, once gathering
// completes, carries every candidate in the offer body -- which is exactly the
// non-trickle profile the relay documents.
type Publisher struct {
	pc       *webrtc.PeerConnection
	track    *webrtc.TrackLocalStaticSample
	resource string
	token    string
	client   *http.Client

	failOnce sync.Once
	failed   chan struct{}
}

// Publish negotiates a publication and returns once the offer/answer exchange
// has completed. Media can be written immediately; pion buffers until ICE and
// DTLS finish.
func Publish(ctx context.Context, endpoint, token string, stunURLs []string) (*Publisher, error) {
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: h264FmtpLine,
		},
		PayloadType: h264PayloadType,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("register H264 codec: %w", err)
	}

	// Without this pion may offer .local mDNS candidates, which the relay
	// accepts syntactically but cannot resolve to reach us.
	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)

	config := webrtc.Configuration{}
	if len(stunURLs) > 0 {
		config.ICEServers = []webrtc.ICEServer{{URLs: stunURLs}}
	}

	pc, err := api.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("new peer connection: %w", err)
	}

	p := &Publisher{
		pc:     pc,
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
		failed: make(chan struct{}),
	}

	p.track, err = webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: h264FmtpLine},
		"video", "cane",
	)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("new track: %w", err)
	}

	// AddTransceiverFromTrack rather than AddTrack: the relay requires the
	// video section to be exactly sendonly, and AddTrack would offer sendrecv.
	if _, err := pc.AddTransceiverFromTrack(p.track, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
	}); err != nil {
		pc.Close()
		return nil, fmt.Errorf("add transceiver: %w", err)
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("caneingest: peer connection %s", state)
		switch state {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			p.failOnce.Do(func() { close(p.failed) })
		case webrtc.PeerConnectionStateDisconnected:
			// Deliberately not fatal. ICE reports Disconnected for
			// ordinary blips and promotes it to Failed itself if the
			// link really is gone, so tearing down here would trade a
			// recoverable pause for a guaranteed reconnect.
			log.Print("caneingest: peer connection disconnected, waiting for recovery")
		}
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("create offer: %w", err)
	}

	// Gather before POSTing: the relay has no PATCH to trickle candidates in
	// afterwards, so anything not in this body can never be delivered.
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(offer); err != nil {
		pc.Close()
		return nil, fmt.Errorf("set local description: %w", err)
	}
	select {
	case <-gatherComplete:
	case <-ctx.Done():
		pc.Close()
		return nil, ctx.Err()
	}

	answer, resource, err := p.postOffer(ctx, endpoint, token, pc.LocalDescription().SDP)
	if err != nil {
		pc.Close()
		return nil, err
	}
	p.resource = resource

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answer,
	}); err != nil {
		pc.Close()
		return nil, fmt.Errorf("set remote description: %w", err)
	}

	return p, nil
}

func (p *Publisher) postOffer(ctx context.Context, endpoint, token, offerSDP string) (answer, resource string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(offerSDP))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/sdp")
	req.Header.Set("Accept", "application/sdp")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("post WHIP offer: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", "", fmt.Errorf("read WHIP answer: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("WHIP publish returned %s: %s", resp.Status, bytes.TrimSpace(body))
	}

	// The relay returns a path; resolve it so DELETE has an absolute URL.
	location := resp.Header.Get("Location")
	if location != "" {
		if base, perr := url.Parse(endpoint); perr == nil {
			if ref, rerr := url.Parse(location); rerr == nil {
				location = base.ResolveReference(ref).String()
			}
		}
	}
	return string(body), location, nil
}

// WriteSample sends one access unit.
func (p *Publisher) WriteSample(sample media.Sample) error {
	return p.track.WriteSample(sample)
}

// Failed closes when the peer connection can no longer carry media, so the
// caller can tear down and republish with a fresh token.
func (p *Publisher) Failed() <-chan struct{} { return p.failed }

// Close deletes the publication and shuts the peer connection down. Deleting is
// idempotent server-side, so a failure here is logged rather than propagated.
func (p *Publisher) Close(ctx context.Context) error {
	if p.resource != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.resource, nil)
		if err == nil {
			// The DELETE sits behind DeviceAuthMiddleware just as the POST
			// does. Without this header it 401s, the publication survives
			// until the relay times it out, and the camera keeps reporting
			// LIVE after this process has gone away.
			req.Header.Set("Authorization", "Bearer "+p.token)
			resp, derr := p.client.Do(req)
			if derr != nil {
				log.Printf("caneingest: delete publication: %v", derr)
			} else {
				if resp.StatusCode != http.StatusNoContent {
					log.Printf("caneingest: delete publication returned %s", resp.Status)
				}
				resp.Body.Close()
			}
		}
	}
	return p.pc.Close()
}
