package camera

import (
	"errors"
	"strings"
	"testing"
)

func baseOfferLines(direction string) []string {
	return []string{
		"v=0",
		"o=- 123456 2 IN IP4 127.0.0.1",
		"s=-",
		"t=0 0",
		"a=group:BUNDLE 0",
		"m=video 9 UDP/TLS/RTP/SAVPF 96",
		"c=IN IP4 0.0.0.0",
		"a=rtcp-mux",
		"a=ice-ufrag:abcd",
		"a=ice-pwd:abcdefghijklmnopqrstuvwx",
		"a=fingerprint:sha-256 00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF",
		"a=setup:actpass",
		"a=mid:0",
		"a=" + direction,
		"a=rtpmap:96 H264/90000",
		"a=fmtp:96 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		"a=candidate:1 1 udp 2130706431 127.0.0.1 12345 typ host",
	}
}

func joinSDP(lines []string) string {
	return strings.Join(lines, "\r\n") + "\r\n"
}

func withoutLinePrefix(lines []string, prefix string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if !strings.HasPrefix(l, prefix) {
			out = append(out, l)
		}
	}
	return out
}

func replaceLinePrefix(lines []string, prefix, replacement string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			out = append(out, replacement)
		} else {
			out = append(out, l)
		}
	}
	return out
}

func TestDecodePublisherOffer_AcceptsMinimalValidOffer(t *testing.T) {
	offer, err := decodePublisherOffer(strings.NewReader(joinSDP(baseOfferLines("sendonly"))))
	if err != nil {
		t.Fatalf("decodePublisherOffer() = %v, want nil error", err)
	}
	if offer.codec.profileLevelID != "42e01f" {
		t.Errorf("profileLevelID = %q, want 42e01f", offer.codec.profileLevelID)
	}
	if offer.codec.packetizationMode != 1 {
		t.Errorf("packetizationMode = %d, want 1", offer.codec.packetizationMode)
	}
	if offer.codec.clockRate != 90000 {
		t.Errorf("clockRate = %d, want 90000", offer.codec.clockRate)
	}
}

func TestDecodeViewerOffer_AcceptsMinimalValidOffer(t *testing.T) {
	offer, err := decodeViewerOffer(strings.NewReader(joinSDP(baseOfferLines("recvonly"))))
	if err != nil {
		t.Fatalf("decodeViewerOffer() = %v, want nil error", err)
	}
	if offer.codec.profileLevelID != "42e01f" {
		t.Errorf("profileLevelID = %q, want 42e01f", offer.codec.profileLevelID)
	}
}

func TestDecodePublisherOffer_RejectsRecvonlyDirection(t *testing.T) {
	_, err := decodePublisherOffer(strings.NewReader(joinSDP(baseOfferLines("recvonly"))))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodeViewerOffer_RejectsSendonlyDirection(t *testing.T) {
	_, err := decodeViewerOffer(strings.NewReader(joinSDP(baseOfferLines("sendonly"))))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodeViewerOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsAudioSection(t *testing.T) {
	lines := baseOfferLines("sendonly")
	lines = append(lines, []string{
		"m=audio 9 UDP/TLS/RTP/SAVPF 111",
		"c=IN IP4 0.0.0.0",
		"a=rtcp-mux",
		"a=ice-ufrag:abcd",
		"a=ice-pwd:abcdefghijklmnopqrstuvwx",
		"a=fingerprint:sha-256 00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF",
		"a=setup:actpass",
		"a=mid:1",
		"a=sendonly",
		"a=rtpmap:111 opus/48000/2",
	}...)

	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsExtraActiveVideoSection(t *testing.T) {
	lines := baseOfferLines("sendonly")
	lines = append(lines, []string{
		"m=video 9 UDP/TLS/RTP/SAVPF 96",
		"c=IN IP4 0.0.0.0",
		"a=rtcp-mux",
		"a=ice-ufrag:abcd",
		"a=ice-pwd:abcdefghijklmnopqrstuvwx",
		"a=fingerprint:sha-256 00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF",
		"a=setup:actpass",
		"a=mid:1",
		"a=sendonly",
		"a=rtpmap:96 H264/90000",
		"a=fmtp:96 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		"a=candidate:1 1 udp 2130706431 127.0.0.1 12345 typ host",
	}...)

	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsMissingRTCPMux(t *testing.T) {
	lines := withoutLinePrefix(baseOfferLines("sendonly"), "a=rtcp-mux")
	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsMissingICECredentials(t *testing.T) {
	lines := withoutLinePrefix(baseOfferLines("sendonly"), "a=ice-ufrag:")
	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsMissingFingerprint(t *testing.T) {
	lines := withoutLinePrefix(baseOfferLines("sendonly"), "a=fingerprint:")
	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsNonActpassSetup(t *testing.T) {
	lines := replaceLinePrefix(baseOfferLines("sendonly"), "a=setup:", "a=setup:active")
	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsMissingCandidate(t *testing.T) {
	lines := withoutLinePrefix(baseOfferLines("sendonly"), "a=candidate:")
	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_AcceptsSessionLevelCandidate(t *testing.T) {
	lines := withoutLinePrefix(baseOfferLines("sendonly"), "a=candidate:")
	var out []string
	for _, l := range lines {
		out = append(out, l)
		if l == "t=0 0" {
			out = append(out, "a=candidate:1 1 udp 2130706431 127.0.0.1 12345 typ host")
		}
	}
	if _, err := decodePublisherOffer(strings.NewReader(joinSDP(out))); err != nil {
		t.Fatalf("session-level ICE candidate should be accepted: %v", err)
	}
}

func TestDecodePublisherOffer_RejectsNonConstrainedBaselineProfile(t *testing.T) {
	lines := replaceLinePrefix(baseOfferLines("sendonly"), "a=fmtp:",
		"a=fmtp:96 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=64001f")
	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsPacketizationModeZero(t *testing.T) {
	lines := replaceLinePrefix(baseOfferLines("sendonly"), "a=fmtp:",
		"a=fmtp:96 level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42e01f")
	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsSimulcastRid(t *testing.T) {
	lines := baseOfferLines("sendonly")
	lines = append(lines, "a=rid:hi send")
	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsMissingH264Codec(t *testing.T) {
	lines := withoutLinePrefix(baseOfferLines("sendonly"), "a=rtpmap:")
	_, err := decodePublisherOffer(strings.NewReader(joinSDP(lines)))
	if !errors.Is(err, errUnsupportedSDP) {
		t.Fatalf("decodePublisherOffer() = %v, want errUnsupportedSDP", err)
	}
}

func TestDecodePublisherOffer_RejectsSyntaxError(t *testing.T) {
	_, err := decodePublisherOffer(strings.NewReader("this is not sdp at all"))
	if !errors.Is(err, errSDPSyntax) {
		t.Fatalf("decodePublisherOffer() = %v, want errSDPSyntax", err)
	}
}

func TestDecodePublisherOffer_RejectsEmptyBody(t *testing.T) {
	_, err := decodePublisherOffer(strings.NewReader(""))
	if !errors.Is(err, errSDPSyntax) {
		t.Fatalf("decodePublisherOffer() = %v, want errSDPSyntax", err)
	}
}

func TestDecodePublisherOffer_RejectsOversizedBody(t *testing.T) {
	oversized := strings.Repeat("a", maxSDPBytes+1)
	_, err := decodePublisherOffer(strings.NewReader(oversized))
	if !errors.Is(err, errSDPTooLarge) {
		t.Fatalf("decodePublisherOffer() = %v, want errSDPTooLarge", err)
	}
}
