package caneingest

import (
	"errors"
	"io"
	"time"

	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/h264reader"
)

var annexBStartCode = []byte{0x00, 0x00, 0x00, 0x01}

// PumpEncoderToTrack reads the encoder's Annex-B output and writes one sample
// per access unit.
//
// Parameter sets and SEI are held back and prepended to the next coded slice
// rather than sent as samples of their own, so each sample is a complete,
// independently decodable access unit and the frame duration lands on the
// picture it belongs to. The encoder is configured for one slice per frame
// (see StartEncoder), so one coded-slice NAL is one frame.
func PumpEncoderToTrack(r io.Reader, pub *Publisher, frameDuration time.Duration) error {
	reader, err := h264reader.NewReader(r)
	if err != nil {
		return err
	}

	var pending []byte
	for {
		nal, err := reader.NextNAL()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		switch nal.UnitType {
		case h264reader.NalUnitTypeSPS,
			h264reader.NalUnitTypePPS,
			h264reader.NalUnitTypeSEI,
			h264reader.NalUnitTypeAUD:
			pending = append(pending, annexBStartCode...)
			pending = append(pending, nal.Data...)
			continue
		}

		accessUnit := make([]byte, 0, len(pending)+len(annexBStartCode)+len(nal.Data))
		accessUnit = append(accessUnit, pending...)
		accessUnit = append(accessUnit, annexBStartCode...)
		accessUnit = append(accessUnit, nal.Data...)
		pending = nil

		if err := pub.WriteSample(media.Sample{Data: accessUnit, Duration: frameDuration}); err != nil {
			return err
		}
	}
}
