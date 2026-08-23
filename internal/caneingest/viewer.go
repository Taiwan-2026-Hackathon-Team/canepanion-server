package caneingest

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"sync"
	"syscall"
	"time"
)

// reportSize is the wire size of the firmware's viewer_report: "CAMR" plus two
// little-endian uint32 counters.
const reportSize = 12

// reportMagic marks a viewer report. The firmware treats any datagram as proof
// a viewer is still watching, but only one carrying this magic steers its rate.
const reportMagic = "CAMR"

// reportInterval matches what the firmware's rate controller expects. Its own
// viewer timeout is 5s, so this also serves as the keepalive.
const reportInterval = time.Second

// logInterval bounds how often the repeating network complaints are printed.
const logInterval = 10 * time.Second

// Viewer plays the role tools/cam_view.py plays on the bench: it announces
// itself to the cane, absorbs the slice stream, and reports back once a second.
//
// The report is not optional. The firmware hill-climbs its send rate on whole
// frames delivered per second, so a viewer that stays silent leaves it pinned
// at its compiled-in default instead of finding what the link can carry.
type Viewer struct {
	conn  *net.UDPConn
	reasm *Reassembler

	logMu      sync.Mutex
	lastLogged time.Time
}

// DialViewer connects to the cane and announces this viewer, which is what
// makes the firmware start sending. The cane streams to whoever spoke most
// recently, so only one viewer should run at a time.
func DialViewer(caneAddr string) (*Viewer, error) {
	addr, err := net.ResolveUDPAddr("udp4", caneAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, err
	}

	v := &Viewer{conn: conn, reasm: NewReassembler()}
	if err := v.sendReport(0, 0); err != nil {
		conn.Close()
		return nil, err
	}
	return v, nil
}

// Reassembler exposes the canvas being painted by the receive loop.
func (v *Viewer) Reassembler() *Reassembler { return v.reasm }

// Close releases the socket.
func (v *Viewer) Close() error { return v.conn.Close() }

func (v *Viewer) sendReport(slices, frames uint32) error {
	buf := make([]byte, reportSize)
	copy(buf[0:4], reportMagic)
	binary.LittleEndian.PutUint32(buf[4:8], slices)
	binary.LittleEndian.PutUint32(buf[8:12], frames)
	_, err := v.conn.Write(buf)
	return err
}

// Receive absorbs slices until the context-bound stop channel closes. A frame
// is 28 datagrams at 160x120, so the buffer only needs to hold one datagram.
func (v *Viewer) Receive(stop <-chan struct{}) {
	buf := make([]byte, 2048)
	for {
		select {
		case <-stop:
			return
		default:
		}

		// Bounded so a silent cane cannot park this goroutine forever; the
		// deadline expiring is normal and not worth logging.
		_ = v.conn.SetReadDeadline(time.Now().Add(time.Second))
		n, err := v.conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			select {
			case <-stop:
				return
			default:
			}
			// Read errors here describe the cane, not this socket, so none of
			// them are grounds to stop listening. A connected UDP socket
			// reports ICMP port-unreachable as ECONNREFUSED on the next read,
			// which is simply what a cane that rebooted or dropped off the
			// network looks like. Returning on it once left the service deaf
			// for good while still publishing a frozen canvas -- healthy by
			// every counter, and wrong.
			if !errors.Is(err, syscall.ECONNREFUSED) || v.shouldLog() {
				log.Printf("caneingest: udp read: %v (still listening)", err)
			}
			continue
		}
		v.reasm.Feed(buf[:n])
	}
}

// ReportLoop keeps the cane sending and feeds its rate controller. The counters
// are deltas since the previous report, which is what the firmware subtracts
// against.
func (v *Viewer) ReportLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	var lastSlices, lastFrames uint64
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			slices, frames := v.reasm.Counters()
			if err := v.sendReport(uint32(slices-lastSlices), uint32(frames-lastFrames)); err != nil {
				select {
				case <-stop:
					return
				default:
				}
				// As in Receive: a cane that is not listening right now is not
				// a reason to stop announcing to it. The next report is what
				// re-registers this viewer when the cane comes back.
				if v.shouldLog() {
					log.Printf("caneingest: report: %v (still announcing)", err)
				}
				continue
			}
			lastSlices, lastFrames = slices, frames
		}
	}
}

// shouldLog rate-limits the repeating network complaints. A cane that stays
// down would otherwise emit one line per second indefinitely.
func (v *Viewer) shouldLog() bool {
	v.logMu.Lock()
	defer v.logMu.Unlock()

	if time.Since(v.lastLogged) < logInterval {
		return false
	}
	v.lastLogged = time.Now()
	return true
}
