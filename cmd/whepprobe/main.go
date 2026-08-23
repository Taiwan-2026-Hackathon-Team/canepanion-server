// Command whepprobe subscribes to a device's camera relay and reports what
// actually arrives.
//
// The camera status endpoint reports LIVE as soon as a publication exists,
// which says nothing about whether RTP is reaching the relay or being forwarded
// on. This attaches a real WHEP viewer and counts packets, so "the stream
// works" can be asserted rather than assumed -- without needing the app.
//
//	CANE_SERVER_URL=http://localhost:8080 \
//	CANE_DEVICE_ID=<uuid> CANE_USER_TOKEN=<jwt> \
//	go run ./cmd/whepprobe
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	serverURL := strings.TrimRight(envOr("CANE_SERVER_URL", "http://localhost:8080"), "/")
	deviceID := os.Getenv("CANE_DEVICE_ID")
	userToken := os.Getenv("CANE_USER_TOKEN")
	seconds, _ := strconv.Atoi(envOr("CANE_PROBE_SECONDS", "15"))

	if deviceID == "" || userToken == "" {
		log.Fatal("whepprobe: set CANE_DEVICE_ID and CANE_USER_TOKEN")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds+30)*time.Second)
	defer cancel()

	var packets, bytesIn atomic.Uint64
	firstPacket := make(chan struct{})
	var firstOnce atomic.Bool

	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)

	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		log.Fatalf("whepprobe: register codec: %v", err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithSettingEngine(settingEngine))
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		log.Fatalf("whepprobe: new peer connection: %v", err)
	}
	defer pc.Close()

	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}); err != nil {
		log.Fatalf("whepprobe: add transceiver: %v", err)
	}

	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		log.Printf("whepprobe: track %s, ssrc %d", track.Codec().MimeType, track.SSRC())
		for {
			pkt, _, err := track.ReadRTP()
			if err != nil {
				return
			}
			packets.Add(1)
			bytesIn.Add(uint64(len(pkt.Payload)))
			if firstOnce.CompareAndSwap(false, true) {
				close(firstPacket)
			}
		}
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("whepprobe: peer connection %s", s)
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		log.Fatalf("whepprobe: create offer: %v", err)
	}
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(offer); err != nil {
		log.Fatalf("whepprobe: set local description: %v", err)
	}
	<-gatherComplete

	endpoint := serverURL + "/api/v1/devices/" + deviceID + "/camera/viewers"
	answer, resource, err := post(ctx, endpoint, userToken, pc.LocalDescription().SDP)
	if err != nil {
		log.Fatalf("whepprobe: %v", err)
	}
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer, SDP: answer,
	}); err != nil {
		log.Fatalf("whepprobe: set remote description: %v", err)
	}
	log.Printf("whepprobe: subscribed, watching for %ds", seconds)

	select {
	case <-firstPacket:
		log.Print("whepprobe: first RTP packet received")
	case <-time.After(20 * time.Second):
		log.Print("whepprobe: NO RTP received within 20s")
	}

	time.Sleep(time.Duration(seconds) * time.Second)

	if resource != "" {
		req, _ := http.NewRequest(http.MethodDelete, resource, nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
		}
	}

	p, b := packets.Load(), bytesIn.Load()
	log.Printf("whepprobe: RESULT %d RTP packets, %d payload bytes, ~%.1f kB/s",
		p, b, float64(b)/float64(seconds)/1000.0)
	if p == 0 {
		os.Exit(1)
	}
}

func post(ctx context.Context, endpoint, token, offerSDP string) (answer, resource string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(offerSDP))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/sdp")
	req.Header.Set("Accept", "application/sdp")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("WHEP subscribe returned %s: %s", resp.Status, bytes.TrimSpace(body))
	}

	location := resp.Header.Get("Location")
	if location != "" {
		if base, e := url.Parse(endpoint); e == nil {
			if ref, e2 := url.Parse(location); e2 == nil {
				location = base.ResolveReference(ref).String()
			}
		}
	}
	return string(body), location, nil
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
