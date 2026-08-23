package caneingest

import (
	"bufio"
	"io"
	"log"
)

// logLines forwards a subprocess's stderr into the service log one line at a
// time, so ffmpeg's complaints are attributable rather than interleaved noise.
// A pipe hands over arbitrary chunks, so the lines have to be reassembled
// rather than assumed to arrive whole.
func logLines(r io.Reader, prefix string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			log.Printf("%s%s", prefix, line)
		}
	}
}
