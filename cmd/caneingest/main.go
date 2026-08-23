// Command caneingest bridges the cane's UDP camera stream into the WebRTC relay.
//
//	cane --UDP slices--> caneingest --raw frames--> ffmpeg --H264--> WHIP --> relay
//
// It runs as its own process rather than inside the Gin server so that frames
// never pass through the HTTP handlers, matching the principle in docs/api.md
// that large media should not be proxied through the application server.
//
// The firmware needs no changes to be ingested: it streams to whoever last sent
// it a datagram, so this plays the same role tools/cam_view.py does on the
// bench. Only one viewer can watch at a time.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"canepanion-server/internal/caneingest"

	"github.com/joho/godotenv"
)

// stallTimeout is how long the canvas may go unchanged before the publication
// is torn down.
const stallTimeout = 10 * time.Second

type config struct {
	caneAddr      string
	deviceID      string
	credential    string
	serverURL     string
	fps           int
	pixelFormat   string
	stunURLs      []string
	debugPNGPath  string
	debugInterval time.Duration
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Optional: the server keeps its own .env, and reusing it means the device
	// credential can live in the same place as everything else.
	_ = godotenv.Load()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("caneingest: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, cfg); err != nil && ctx.Err() == nil {
		log.Fatalf("caneingest: %v", err)
	}
	log.Print("caneingest: stopped")
}

func loadConfig() (config, error) {
	cfg := config{
		serverURL:     envOr("CANE_SERVER_URL", "http://localhost:8080"),
		pixelFormat:   envOr("CANE_PIXEL_FORMAT", "rgb565le"),
		debugPNGPath:  os.Getenv("CANE_DEBUG_PNG"),
		debugInterval: 10 * time.Second,
	}

	var missing []string
	for _, required := range []struct {
		key   string
		field *string
	}{
		{"CANE_ADDR", &cfg.caneAddr},
		{"CANE_DEVICE_ID", &cfg.deviceID},
		{"CANE_DEVICE_CREDENTIAL", &cfg.credential},
	} {
		*required.field = os.Getenv(required.key)
		if *required.field == "" {
			missing = append(missing, required.key)
		}
	}
	if len(missing) > 0 {
		return cfg, missingEnvError(missing)
	}

	fps, err := strconv.Atoi(envOr("CANE_OUT_FPS", "15"))
	if err != nil || fps < 1 || fps > 60 {
		return cfg, errInvalid("CANE_OUT_FPS must be an integer between 1 and 60")
	}
	cfg.fps = fps

	if raw := os.Getenv("CANE_STUN_URLS"); raw != "" {
		for _, u := range strings.Split(raw, ",") {
			if u = strings.TrimSpace(u); u != "" {
				cfg.stunURLs = append(cfg.stunURLs, u)
			}
		}
	}

	cfg.serverURL = strings.TrimRight(cfg.serverURL, "/")
	return cfg, nil
}

func run(ctx context.Context, cfg config) error {
	log.Printf("caneingest: announcing to cane at %s", cfg.caneAddr)
	viewer, err := caneingest.DialViewer(cfg.caneAddr)
	if err != nil {
		return err
	}
	defer viewer.Close()

	stop := make(chan struct{})
	defer close(stop)
	go viewer.Receive(stop)
	go viewer.ReportLoop(stop)

	reasm := viewer.Reassembler()

	// Wait for the stream before starting the encoder: the frame size comes off
	// the wire, so there is nothing to guess or configure.
	if err := awaitFirstFrame(ctx, reasm); err != nil {
		return err
	}

	// Republish on failure. The cane keeps streaming throughout, so a dropped
	// publication costs a gap rather than a restart.
	backoff := time.Second
	for ctx.Err() == nil {
		err := publishOnce(ctx, cfg, reasm)
		if ctx.Err() != nil {
			return nil
		}
		log.Printf("caneingest: publication ended: %v; retrying in %s", err, backoff)

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}

		// Only hold a publication open while the cane is actually sending.
		if err := awaitSlices(ctx, reasm); err != nil {
			return nil
		}
		backoff = time.Second
	}
	return nil
}

// awaitSlices blocks until the cane's slice counter advances again.
func awaitSlices(ctx context.Context, reasm *caneingest.Reassembler) error {
	start, _ := reasm.Counters()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	warn := time.After(15 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-warn:
			log.Print("caneingest: still waiting for the cane to resume sending")
		case <-ticker.C:
			if now, _ := reasm.Counters(); now > start {
				log.Print("caneingest: cane is sending again")
				return nil
			}
		}
	}
}

func awaitFirstFrame(ctx context.Context, reasm *caneingest.Reassembler) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	warn := time.After(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-warn:
			log.Print("caneingest: no slices yet -- is the cane powered, on this network, " +
				"and not already streaming to another viewer?")
		case <-ticker.C:
			if w, h, size, ok := reasm.Dimensions(); ok {
				log.Printf("caneingest: cane is streaming %dx%d (%d bytes/frame)", w, h, size)
				return nil
			}
		}
	}
}

func publishOnce(ctx context.Context, cfg config, reasm *caneingest.Reassembler) error {
	width, height, frameSize, ok := reasm.Dimensions()
	if !ok {
		return errNoStream
	}

	token, err := caneingest.DeviceToken(ctx, cfg.serverURL, cfg.deviceID, cfg.credential)
	if err != nil {
		return err
	}

	endpoint := cfg.serverURL + "/api/v1/firmware/devices/" + cfg.deviceID + "/camera/publications"
	publisher, err := caneingest.Publish(ctx, endpoint, token, cfg.stunURLs)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = publisher.Close(closeCtx)
	}()
	log.Printf("caneingest: publishing to %s", endpoint)

	encoder, err := caneingest.StartEncoder(int(width), int(height), cfg.fps, cfg.pixelFormat)
	if err != nil {
		return err
	}
	defer encoder.Close()

	pumpDone := make(chan error, 1)
	go func() {
		pumpDone <- caneingest.PumpEncoderToTrack(
			encoder.Output(), publisher, time.Second/time.Duration(cfg.fps))
	}()

	return feedFrames(ctx, cfg, reasm, encoder, publisher, pumpDone, int(width), int(height), frameSize)
}

// feedFrames paces the canvas into the encoder.
//
// It sends on a fixed tick regardless of what has arrived, because the encoder
// needs a steady cadence and the canvas always holds a whole frame's worth of
// bytes. When slices are lost the repeated regions simply cost nothing to
// encode.
func feedFrames(
	ctx context.Context,
	cfg config,
	reasm *caneingest.Reassembler,
	encoder *caneingest.Encoder,
	publisher *caneingest.Publisher,
	pumpDone <-chan error,
	width, height, frameSize int,
) error {
	frame := make([]byte, frameSize)
	ticker := time.NewTicker(time.Second / time.Duration(cfg.fps))
	defer ticker.Stop()

	statsTicker := time.NewTicker(5 * time.Second)
	defer statsTicker.Stop()

	debugTicker := time.NewTicker(cfg.debugInterval)
	defer debugTicker.Stop()

	var lastSlices, lastFrames uint64
	lastActivity := time.Now()
	bigEndian := strings.EqualFold(cfg.pixelFormat, "rgb565be")

	for {
		select {
		case <-ctx.Done():
			return nil

		case err := <-pumpDone:
			if err == nil {
				return errEncoderClosed
			}
			return err

		case <-publisher.Failed():
			return errPeerFailed

		case <-ticker.C:
			if _, _, size, ok := reasm.Dimensions(); ok && size != frameSize {
				return errFrameSizeChanged
			}
			_, _, n, ok := reasm.Snapshot(frame)
			if !ok {
				continue
			}
			if err := encoder.WriteFrame(frame[:n]); err != nil {
				return err
			}

		case <-statsTicker.C:
			slices, frames := reasm.Counters()
			log.Printf("caneingest: +%d slices, +%d whole frames in 5s (%.1f fps delivered)",
				slices-lastSlices, frames-lastFrames, float64(frames-lastFrames)/5.0)

			// A canvas nobody is updating is not a live camera. Publishing it
			// anyway would leave the relay reporting LIVE while a guardian
			// watches a still picture of the past, which is worse than showing
			// nothing -- so end the publication and let the caller wait for the
			// cane to come back.
			if slices != lastSlices {
				lastActivity = time.Now()
			} else if time.Since(lastActivity) > stallTimeout {
				return errStreamStalled
			}
			lastSlices, lastFrames = slices, frames

		case <-debugTicker.C:
			if cfg.debugPNGPath == "" {
				continue
			}
			if _, _, n, ok := reasm.Snapshot(frame); ok {
				if err := caneingest.WriteCanvasPNG(cfg.debugPNGPath, frame[:n], width, height, bigEndian); err != nil {
					log.Printf("caneingest: debug png: %v", err)
					continue
				}
				if r, err := caneingest.Roughness(frame[:n], width, height, bigEndian); err == nil {
					log.Printf("caneingest: wrote %s (roughness %.2f)", cfg.debugPNGPath, r)
				}
			}
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
