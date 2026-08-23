package caneingest

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"
)

// Encoder wraps ffmpeg as a pure codec: raw frames in on stdin, Annex-B H.264
// out on stdout. It deliberately does not use ffmpeg's WHIP muxer -- that muxer
// offers a=setup:passive and inlines no ICE candidates, both of which the
// relay rejects (internal/camera/sdp.go), so the WebRTC side is pion's job.
type Encoder struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

// StartEncoder launches ffmpeg for a stream of the given size and rate.
//
// slices=1 with sliced-threads off keeps one slice NAL per frame, which is what
// lets the publisher treat each VCL NAL as exactly one sample. repeat-headers
// puts SPS/PPS ahead of every keyframe so a viewer joining mid-stream can
// decode without waiting for a fresh parameter set.
func StartEncoder(width, height, fps int, pixelFormat string) (*Encoder, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("caneingest: bad frame size %dx%d", width, height)
	}

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "rawvideo",
		"-pix_fmt", pixelFormat,
		"-s", strconv.Itoa(width) + "x" + strconv.Itoa(height),
		"-r", strconv.Itoa(fps),
		"-i", "pipe:0",
		"-c:v", "libx264",
		"-profile:v", "baseline",
		"-pix_fmt", "yuv420p",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-g", strconv.Itoa(fps),
		"-x264-params", "slices=1:sliced-threads=0:repeat-headers=1",
		"-f", "h264", "pipe:1",
	}

	cmd := exec.Command("ffmpeg", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("caneingest: start ffmpeg (is it installed?): %w", err)
	}
	go logLines(stderr, "ffmpeg: ")
	return &Encoder{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

// WriteFrame hands one raw frame to the encoder.
func (e *Encoder) WriteFrame(frame []byte) error {
	_, err := e.stdin.Write(frame)
	return err
}

// Output is the Annex-B H.264 elementary stream.
func (e *Encoder) Output() io.Reader { return e.stdout }

// Close stops the encoder. Closing stdin first lets ffmpeg flush rather than
// die mid-frame.
func (e *Encoder) Close() error {
	_ = e.stdin.Close()
	if e.cmd.Process != nil {
		_ = e.cmd.Process.Kill()
	}
	return e.cmd.Wait()
}
