// Package caneingest terminates the cane's UDP camera protocol so the frames
// can be re-published to the WebRTC relay.
//
// The cane firmware (app/src/main.c in cane-panion-firmware) sends each frame
// as ~28 datagrams and never retransmits. Every datagram carries the full
// descriptor, so slices can be absorbed in any order and a lost one costs
// pixels rather than synchronisation.
package caneingest

import (
	"encoding/binary"
	"sync"
)

// HeaderSize is the wire size of the firmware's slice_header: a 4-byte magic
// followed by frame, width, height, fourcc, offset, total and len, packed
// little-endian with no padding.
const HeaderSize = 26

// SliceMagic marks a camera slice. The firmware writes it into every datagram.
const SliceMagic = "CAMU"

// keepFrames is how many frames newer than an incomplete one to keep waiting
// for. A slice that never arrives must not leave its frame in the map forever.
const keepFrames = 4

// Header describes one slice of one frame.
type Header struct {
	Frame  uint32
	Width  uint16
	Height uint16
	FourCC uint32
	Offset uint32
	Total  uint32
	Len    uint16
}

// ParseSlice validates one datagram and splits it into header and payload.
// The payload aliases pkt; copy it if it must outlive the read buffer.
func ParseSlice(pkt []byte) (Header, []byte, bool) {
	if len(pkt) < HeaderSize {
		return Header{}, nil, false
	}
	if string(pkt[0:4]) != SliceMagic {
		return Header{}, nil, false
	}

	h := Header{
		Frame:  binary.LittleEndian.Uint32(pkt[4:8]),
		Width:  binary.LittleEndian.Uint16(pkt[8:10]),
		Height: binary.LittleEndian.Uint16(pkt[10:12]),
		FourCC: binary.LittleEndian.Uint32(pkt[12:16]),
		Offset: binary.LittleEndian.Uint32(pkt[16:20]),
		Total:  binary.LittleEndian.Uint32(pkt[20:24]),
		Len:    binary.LittleEndian.Uint16(pkt[24:26]),
	}

	if h.Total == 0 {
		return Header{}, nil, false
	}
	// Widened to uint64 so a hostile or corrupt offset cannot wrap past Total.
	if uint64(h.Offset)+uint64(h.Len) > uint64(h.Total) {
		return Header{}, nil, false
	}
	if len(pkt) < HeaderSize+int(h.Len) {
		return Header{}, nil, false
	}

	return h, pkt[HeaderSize : HeaderSize+int(h.Len)], true
}

type partial struct {
	buf []byte
	// seen records which byte offsets have arrived. Accumulating h.Len
	// instead would let a re-delivered slice count twice and declare a frame
	// whole while it still has a hole -- which inflates the figure the cane's
	// rate controller trusts, so it must be exact rather than approximate.
	seen   map[uint32]struct{}
	filled uint32
	width  uint16
	height uint16
}

// Reassembler rebuilds frames from slices.
//
// It keeps two views deliberately. The canvas is a persistent picture painted
// slice by slice and never cleared, because the radio link loses a large and
// variable share of slices -- insisting on whole frames would leave almost
// nothing to show, whereas painting into a canvas turns a lost slice into a
// stale band. The per-frame buffers exist only to count frames that arrived
// genuinely whole, which is the number the firmware's rate controller
// hill-climbs on and the only figure that must not be flattered.
//
// Safe for one reader goroutine feeding while another snapshots.
type Reassembler struct {
	mu        sync.Mutex
	canvas    []byte
	width     uint16
	height    uint16
	pending   map[uint32]*partial
	newest    uint32
	haveFrame bool
	slices    uint64
	completed uint64
}

// NewReassembler returns a Reassembler with no canvas yet; the first slice
// sizes it.
func NewReassembler() *Reassembler {
	return &Reassembler{pending: make(map[uint32]*partial)}
}

// Feed absorbs one datagram and reports whether it completed a whole frame.
// Datagrams that are malformed, truncated or not camera slices are ignored.
func (r *Reassembler) Feed(pkt []byte) bool {
	h, payload, ok := ParseSlice(pkt)
	if !ok {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.canvas == nil || len(r.canvas) != int(h.Total) {
		r.canvas = make([]byte, h.Total)
	}
	copy(r.canvas[h.Offset:], payload)
	r.width, r.height = h.Width, h.Height
	r.slices++

	entry := r.pending[h.Frame]
	if entry == nil {
		entry = &partial{
			buf:    make([]byte, h.Total),
			seen:   make(map[uint32]struct{}),
			width:  h.Width,
			height: h.Height,
		}
		r.pending[h.Frame] = entry
	}
	copy(entry.buf[h.Offset:], payload)
	if _, duplicate := entry.seen[h.Offset]; !duplicate {
		entry.seen[h.Offset] = struct{}{}
		entry.filled += uint32(h.Len)
	}

	if !r.haveFrame || h.Frame > r.newest {
		r.newest = h.Frame
		r.haveFrame = true
	}
	r.evictLocked()

	if entry.filled >= h.Total {
		delete(r.pending, h.Frame)
		r.completed++
		return true
	}
	return false
}

// evictLocked drops frames too old to still be completed by a late slice.
func (r *Reassembler) evictLocked() {
	if r.newest < keepFrames {
		return
	}
	cutoff := r.newest - keepFrames
	for frame := range r.pending {
		if frame < cutoff {
			delete(r.pending, frame)
		}
	}
}

// Snapshot copies the current canvas into dst, returning the frame dimensions
// and how many bytes were written. ok is false until the first slice lands, or
// if dst is too small to hold a whole frame.
func (r *Reassembler) Snapshot(dst []byte) (width, height uint16, n int, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.canvas == nil || len(dst) < len(r.canvas) {
		return 0, 0, 0, false
	}
	return r.width, r.height, copy(dst, r.canvas), true
}

// FrameSize reports the byte length of a whole frame, or 0 before the first
// slice arrives.
func (r *Reassembler) FrameSize() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.canvas)
}

// Dimensions reports the current frame geometry and byte size, or ok=false
// before the first slice arrives. The size comes off the wire, so it can change
// if the cane restarts at a different resolution.
func (r *Reassembler) Dimensions() (width, height uint16, size int, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.canvas == nil || r.width == 0 || r.height == 0 {
		return 0, 0, 0, false
	}
	return r.width, r.height, len(r.canvas), true
}

// Counters returns cumulative slices absorbed and whole frames completed. The
// firmware wants deltas, so the caller keeps the previous reading.
func (r *Reassembler) Counters() (slices, frames uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.slices, r.completed
}
