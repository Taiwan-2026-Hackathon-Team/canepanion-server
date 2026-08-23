package caneingest

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func makeSlice(frame uint32, width, height uint16, offset, total uint32, payload []byte) []byte {
	pkt := make([]byte, HeaderSize+len(payload))
	copy(pkt[0:4], SliceMagic)
	binary.LittleEndian.PutUint32(pkt[4:8], frame)
	binary.LittleEndian.PutUint16(pkt[8:10], width)
	binary.LittleEndian.PutUint16(pkt[10:12], height)
	binary.LittleEndian.PutUint32(pkt[12:16], 0x50424752) // "RGBP"
	binary.LittleEndian.PutUint32(pkt[16:20], offset)
	binary.LittleEndian.PutUint32(pkt[20:24], total)
	binary.LittleEndian.PutUint16(pkt[24:26], uint16(len(payload)))
	copy(pkt[HeaderSize:], payload)
	return pkt
}

// The firmware packs slice_header with __packed and no padding. If this size
// ever drifts, every offset in ParseSlice is wrong and the picture is garbage.
func TestHeaderSizeMatchesFirmware(t *testing.T) {
	if HeaderSize != 26 {
		t.Fatalf("HeaderSize = %d, want 26 (4+4+2+2+4+4+4+2)", HeaderSize)
	}
}

// Golden bytes lifted from the firmware's little-endian layout, so a field
// reordering fails loudly rather than silently shifting the image.
func TestParseSliceGoldenBytes(t *testing.T) {
	pkt := []byte{
		'C', 'A', 'M', 'U', // magic
		0x07, 0x00, 0x00, 0x00, // frame = 7
		0xA0, 0x00, // width = 160
		0x78, 0x00, // height = 120
		0x52, 0x47, 0x42, 0x50, // fourcc
		0x10, 0x00, 0x00, 0x00, // offset = 16
		0x00, 0x96, 0x00, 0x00, // total = 38400
		0x02, 0x00, // len = 2
		0xAB, 0xCD, // payload
	}

	h, payload, ok := ParseSlice(pkt)
	if !ok {
		t.Fatal("ParseSlice() rejected a valid golden packet")
	}
	if h.Frame != 7 || h.Width != 160 || h.Height != 120 {
		t.Errorf("Frame/Width/Height = %d/%d/%d, want 7/160/120", h.Frame, h.Width, h.Height)
	}
	if h.Offset != 16 || h.Total != 38400 || h.Len != 2 {
		t.Errorf("Offset/Total/Len = %d/%d/%d, want 16/38400/2", h.Offset, h.Total, h.Len)
	}
	if !bytes.Equal(payload, []byte{0xAB, 0xCD}) {
		t.Errorf("payload = % x, want ab cd", payload)
	}
}

func TestParseSliceRejectsMalformed(t *testing.T) {
	valid := makeSlice(1, 160, 120, 0, 8, []byte{1, 2, 3, 4})

	cases := map[string][]byte{
		"empty":            {},
		"shorter than hdr": valid[:HeaderSize-1],
		"bad magic": func() []byte {
			p := append([]byte(nil), valid...)
			copy(p[0:4], "XXXX")
			return p
		}(),
		"total zero": makeSlice(1, 160, 120, 0, 0, []byte{1, 2}),
		"offset past total": func() []byte {
			p := append([]byte(nil), valid...)
			binary.LittleEndian.PutUint32(p[16:20], 7) // offset 7 + len 4 > total 8
			return p
		}(),
		"offset overflows uint32": func() []byte {
			p := append([]byte(nil), valid...)
			binary.LittleEndian.PutUint32(p[16:20], 0xFFFFFFFF)
			return p
		}(),
		"payload truncated": valid[:HeaderSize+2],
	}

	for name, pkt := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, ok := ParseSlice(pkt); ok {
				t.Errorf("ParseSlice(%s) = ok, want rejected", name)
			}
		})
	}
}

func TestReassemblerCompletesWholeFrame(t *testing.T) {
	r := NewReassembler()

	if r.Feed(makeSlice(1, 4, 1, 0, 8, []byte{1, 2, 3, 4})) {
		t.Fatal("frame reported complete after only half its slices")
	}
	if !r.Feed(makeSlice(1, 4, 1, 4, 8, []byte{5, 6, 7, 8})) {
		t.Fatal("frame did not complete when its last slice arrived")
	}

	dst := make([]byte, 8)
	w, h, n, ok := r.Snapshot(dst)
	if !ok || n != 8 || w != 4 || h != 1 {
		t.Fatalf("Snapshot() = %d/%d/%d/%v, want 4/1/8/true", w, h, n, ok)
	}
	if !bytes.Equal(dst, []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Errorf("canvas = % x, want 01..08", dst)
	}
}

// Slices arrive interleaved, not in address order -- the firmware sends every
// 4th slice per pass so losses spread instead of wiping one contiguous band.
func TestReassemblerAcceptsOutOfOrderSlices(t *testing.T) {
	r := NewReassembler()

	r.Feed(makeSlice(1, 4, 1, 4, 8, []byte{5, 6, 7, 8}))
	if !r.Feed(makeSlice(1, 4, 1, 0, 8, []byte{1, 2, 3, 4})) {
		t.Fatal("out-of-order slices did not complete the frame")
	}

	dst := make([]byte, 8)
	r.Snapshot(dst)
	if !bytes.Equal(dst, []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Errorf("canvas = % x, want 01..08", dst)
	}
}

// A lost slice must leave a stale band from the previous frame rather than a
// hole, and must not be counted as a delivered frame.
func TestReassemblerPaintsCanvasThroughLoss(t *testing.T) {
	r := NewReassembler()

	r.Feed(makeSlice(1, 4, 1, 0, 8, []byte{1, 1, 1, 1}))
	r.Feed(makeSlice(1, 4, 1, 4, 8, []byte{2, 2, 2, 2}))

	// Frame 2: only the first half arrives.
	if r.Feed(makeSlice(2, 4, 1, 0, 8, []byte{9, 9, 9, 9})) {
		t.Fatal("incomplete frame 2 reported as complete")
	}

	dst := make([]byte, 8)
	r.Snapshot(dst)
	want := []byte{9, 9, 9, 9, 2, 2, 2, 2} // new top, stale bottom
	if !bytes.Equal(dst, want) {
		t.Errorf("canvas = % x, want % x", dst, want)
	}

	if _, frames := r.Counters(); frames != 1 {
		t.Errorf("completed frames = %d, want 1 (frame 2 was never whole)", frames)
	}
}

func TestReassemblerEvictsStaleIncompleteFrames(t *testing.T) {
	r := NewReassembler()

	// Frame 1 never completes.
	r.Feed(makeSlice(1, 4, 1, 0, 8, []byte{1, 2, 3, 4}))

	// Push the newest frame well past the keep window.
	for frame := uint32(2); frame <= 2+keepFrames+1; frame++ {
		r.Feed(makeSlice(frame, 4, 1, 0, 8, []byte{0, 0, 0, 0}))
	}

	r.mu.Lock()
	_, stillPending := r.pending[1]
	r.mu.Unlock()
	if stillPending {
		t.Error("frame 1 still pending after the keep window passed")
	}

	// Its missing slice arriving late must not resurrect it as a whole frame.
	if r.Feed(makeSlice(1, 4, 1, 4, 8, []byte{5, 6, 7, 8})) {
		t.Error("an evicted frame completed on a late slice")
	}
}

func TestReassemblerCountersTrackSlicesAndFrames(t *testing.T) {
	r := NewReassembler()

	r.Feed(makeSlice(1, 4, 1, 0, 8, []byte{1, 2, 3, 4}))
	r.Feed(makeSlice(1, 4, 1, 4, 8, []byte{5, 6, 7, 8}))
	r.Feed(makeSlice(2, 4, 1, 0, 8, []byte{1, 2, 3, 4}))
	r.Feed([]byte("garbage")) // must not count

	slices, frames := r.Counters()
	if slices != 3 || frames != 1 {
		t.Errorf("Counters() = %d slices / %d frames, want 3/1", slices, frames)
	}
}

func TestSnapshotBeforeFirstSliceAndUndersizedDst(t *testing.T) {
	r := NewReassembler()

	if _, _, _, ok := r.Snapshot(make([]byte, 8)); ok {
		t.Error("Snapshot() succeeded before any slice arrived")
	}
	if got := r.FrameSize(); got != 0 {
		t.Errorf("FrameSize() = %d, want 0 before the first slice", got)
	}

	r.Feed(makeSlice(1, 4, 1, 0, 8, []byte{1, 2, 3, 4}))

	if got := r.FrameSize(); got != 8 {
		t.Errorf("FrameSize() = %d, want 8", got)
	}
	if _, _, _, ok := r.Snapshot(make([]byte, 4)); ok {
		t.Error("Snapshot() wrote into a dst too small for a whole frame")
	}
}

// The canvas is resized when the cane changes resolution mid-session.
func TestReassemblerResizesCanvasOnNewFrameSize(t *testing.T) {
	r := NewReassembler()

	r.Feed(makeSlice(1, 4, 1, 0, 8, []byte{1, 2, 3, 4, 5, 6, 7, 8}))
	r.Feed(makeSlice(2, 2, 1, 0, 4, []byte{9, 9, 9, 9}))

	if got := r.FrameSize(); got != 4 {
		t.Fatalf("FrameSize() = %d, want 4 after the frame size changed", got)
	}
	dst := make([]byte, 4)
	w, h, _, _ := r.Snapshot(dst)
	if w != 2 || h != 1 {
		t.Errorf("dimensions = %dx%d, want 2x1", w, h)
	}
}

func TestDimensionsTracksTheWire(t *testing.T) {
	r := NewReassembler()

	if _, _, _, ok := r.Dimensions(); ok {
		t.Error("Dimensions() reported geometry before any slice arrived")
	}

	r.Feed(makeSlice(1, 160, 120, 0, 38400, []byte{1, 2, 3, 4}))
	w, h, size, ok := r.Dimensions()
	if !ok || w != 160 || h != 120 || size != 38400 {
		t.Fatalf("Dimensions() = %d/%d/%d/%v, want 160/120/38400/true", w, h, size, ok)
	}

	// A cane that restarts at a different resolution must be visible here,
	// because the encoder is started for one geometry and cannot change it.
	r.Feed(makeSlice(2, 320, 240, 0, 153600, []byte{9, 9}))
	w, h, size, ok = r.Dimensions()
	if !ok || w != 320 || h != 240 || size != 153600 {
		t.Fatalf("Dimensions() = %d/%d/%d/%v, want 320/240/153600/true", w, h, size, ok)
	}
}

// A re-delivered slice must not be able to complete a frame that is still
// missing a piece. The count it feeds back drives the cane's rate controller,
// so an over-count makes the link look better than it is.
func TestDuplicateSliceCannotCompleteAHoledFrame(t *testing.T) {
	r := NewReassembler()

	// Frame needs offsets 0 and 4. Send 0 three times; 4 never arrives.
	for i := 0; i < 3; i++ {
		if r.Feed(makeSlice(1, 4, 1, 0, 8, []byte{1, 2, 3, 4})) {
			t.Fatalf("duplicate slice %d completed a frame with a hole", i)
		}
	}

	if _, frames := r.Counters(); frames != 0 {
		t.Errorf("completed frames = %d, want 0", frames)
	}

	// The genuinely missing slice completes it exactly once.
	if !r.Feed(makeSlice(1, 4, 1, 4, 8, []byte{5, 6, 7, 8})) {
		t.Fatal("frame did not complete when its last slice finally arrived")
	}
	if _, frames := r.Counters(); frames != 1 {
		t.Errorf("completed frames = %d, want 1", frames)
	}
}
