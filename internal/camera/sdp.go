package camera

import (
	"fmt"
	"io"
	"strings"

	"github.com/pion/sdp/v3"
)

const maxSDPBytes = 64 << 10

func decodePublisherOffer(r io.Reader) (publisherOffer, error) {
	rawSDP, profile, err := decodeOffer(r, sdp.DirectionSendOnly)
	if err != nil {
		return publisherOffer{}, err
	}
	return publisherOffer{rawSDP: rawSDP, codec: profile}, nil
}

func decodeViewerOffer(r io.Reader) (viewerOffer, error) {
	rawSDP, profile, err := decodeOffer(r, sdp.DirectionRecvOnly)
	if err != nil {
		return viewerOffer{}, err
	}
	return viewerOffer{rawSDP: rawSDP, codec: profile}, nil
}

func decodeOffer(r io.Reader, want sdp.Direction) (string, h264Profile, error) {
	raw, err := readCapped(r)
	if err != nil {
		return "", h264Profile{}, err
	}

	var parsed sdp.SessionDescription
	if err := parsed.Unmarshal(raw); err != nil {
		return "", h264Profile{}, fmt.Errorf("%w: %v", errSDPSyntax, err)
	}

	video, err := singleActiveVideoSection(&parsed, want)
	if err != nil {
		return "", h264Profile{}, err
	}

	profile, err := negotiatedH264Profile(video)
	if err != nil {
		return "", h264Profile{}, err
	}

	if err := requireNonTrickleTransport(&parsed, video); err != nil {
		return "", h264Profile{}, err
	}

	return string(raw), profile, nil
}

func readCapped(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSDPBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errSDPSyntax, err)
	}
	if len(data) > maxSDPBytes {
		return nil, errSDPTooLarge
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty body", errSDPSyntax)
	}
	return data, nil
}

func singleActiveVideoSection(parsed *sdp.SessionDescription, want sdp.Direction) (*sdp.MediaDescription, error) {
	var video *sdp.MediaDescription
	for _, m := range parsed.MediaDescriptions {
		if m.MediaName.Port.Value == 0 {
			continue
		}
		switch m.MediaName.Media {
		case "video":
			if video != nil {
				return nil, fmt.Errorf("%w: more than one active video section", errUnsupportedSDP)
			}
			video = m
		default:
			return nil, fmt.Errorf("%w: %q media sections are not supported", errUnsupportedSDP, m.MediaName.Media)
		}
	}
	if video == nil {
		return nil, fmt.Errorf("%w: missing an active video section", errUnsupportedSDP)
	}

	if _, ok := video.Attribute("simulcast"); ok {
		return nil, fmt.Errorf("%w: simulcast is not supported", errUnsupportedSDP)
	}
	for _, a := range video.Attributes {
		if a.Key == "rid" {
			return nil, fmt.Errorf("%w: simulcast rid is not supported", errUnsupportedSDP)
		}
	}

	direction := mediaDirection(video)
	if direction != want {
		return nil, fmt.Errorf("%w: video section must be %s", errUnsupportedSDP, want)
	}

	return video, nil
}

func mediaDirection(m *sdp.MediaDescription) sdp.Direction {
	for _, a := range m.Attributes {
		if d, err := sdp.NewDirection(a.Key); err == nil {
			return d
		}
	}
	return sdp.Direction(0)
}

func negotiatedH264Profile(m *sdp.MediaDescription) (h264Profile, error) {
	payloadType := ""
	for _, a := range m.Attributes {
		if a.Key != "rtpmap" {
			continue
		}
		fields := strings.Fields(a.Value)
		if len(fields) != 2 || !strings.EqualFold(fields[1], "H264/90000") {
			continue
		}
		payloadType = fields[0]
		break
	}
	if payloadType == "" {
		return h264Profile{}, fmt.Errorf("%w: no H264/90000 rtpmap", errUnsupportedSDP)
	}

	fmtpLine, ok := fmtpFor(m, payloadType)
	if !ok {
		return h264Profile{}, fmt.Errorf("%w: missing fmtp for the H264 payload type", errUnsupportedSDP)
	}

	params := parseFmtp(fmtpLine)
	if params["packetization-mode"] != "1" {
		return h264Profile{}, fmt.Errorf("%w: packetization-mode must be 1", errUnsupportedSDP)
	}

	profileLevelID := params["profile-level-id"]
	if !isConstrainedBaseline(profileLevelID) {
		return h264Profile{}, fmt.Errorf("%w: profile-level-id %q is not H264 constrained baseline", errUnsupportedSDP, profileLevelID)
	}

	return h264Profile{profileLevelID: profileLevelID, packetizationMode: 1, clockRate: 90000}, nil
}

func fmtpFor(m *sdp.MediaDescription, payloadType string) (string, bool) {
	for _, a := range m.Attributes {
		if a.Key != "fmtp" {
			continue
		}
		parts := strings.SplitN(a.Value, " ", 2)
		if len(parts) != 2 || parts[0] != payloadType {
			continue
		}
		return parts[1], true
	}
	return "", false
}

func parseFmtp(line string) map[string]string {
	params := map[string]string{}
	for _, part := range strings.Split(line, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		params[strings.ToLower(strings.TrimSpace(kv[0]))] = strings.TrimSpace(kv[1])
	}
	return params
}

func isConstrainedBaseline(profileLevelID string) bool {
	return len(profileLevelID) == 6 && strings.HasPrefix(strings.ToLower(profileLevelID), "42e0")
}

func requireNonTrickleTransport(parsed *sdp.SessionDescription, m *sdp.MediaDescription) error {
	if _, ok := mediaOrSessionAttribute(parsed, m, "rtcp-mux"); !ok {
		return fmt.Errorf("%w: rtcp-mux is required", errUnsupportedSDP)
	}
	if _, ok := mediaOrSessionAttribute(parsed, m, "ice-ufrag"); !ok {
		return fmt.Errorf("%w: ice-ufrag is required", errUnsupportedSDP)
	}
	if _, ok := mediaOrSessionAttribute(parsed, m, "ice-pwd"); !ok {
		return fmt.Errorf("%w: ice-pwd is required", errUnsupportedSDP)
	}
	if _, ok := mediaOrSessionAttribute(parsed, m, "fingerprint"); !ok {
		return fmt.Errorf("%w: a DTLS fingerprint is required", errUnsupportedSDP)
	}
	if setup, ok := mediaOrSessionAttribute(parsed, m, "setup"); !ok || setup != "actpass" {
		return fmt.Errorf("%w: setup must be actpass", errUnsupportedSDP)
	}

	if !hasICECandidate(parsed, m) {
		return fmt.Errorf("%w: at least one inline ICE candidate is required", errUnsupportedSDP)
	}
	return nil
}

func hasICECandidate(parsed *sdp.SessionDescription, m *sdp.MediaDescription) bool {
	for _, a := range m.Attributes {
		if a.Key == "candidate" {
			return true
		}
	}
	for _, a := range parsed.Attributes {
		if a.Key == "candidate" {
			return true
		}
	}
	return false
}

func mediaOrSessionAttribute(parsed *sdp.SessionDescription, m *sdp.MediaDescription, key string) (string, bool) {
	if v, ok := m.Attribute(key); ok {
		return v, true
	}
	return parsed.Attribute(key)
}
