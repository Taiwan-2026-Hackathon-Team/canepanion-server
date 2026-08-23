package main

import (
	"errors"
	"fmt"
	"strings"
)

var (
	errEncoderClosed = errors.New("encoder output ended")
	errPeerFailed    = errors.New("peer connection failed")
	errStreamStalled = errors.New("cane stopped sending")
	errNoStream      = errors.New("no slices on the wire yet")

	// The encoder is started for one geometry and cannot change it, so a cane
	// that restarts at a different resolution needs a fresh publication.
	errFrameSizeChanged = errors.New("cane changed frame size")
)

func errInvalid(msg string) error { return errors.New(msg) }

func missingEnvError(missing []string) error {
	return fmt.Errorf(`missing required environment: %s

Set them and retry, for example:

  export CANE_ADDR=192.168.1.50:5000            # the cane's IP, port 5000
  export CANE_DEVICE_ID=<device uuid>
  export CANE_DEVICE_CREDENTIAL=<from POST /api/v1/firmware/devices/activate>
  export CANE_SERVER_URL=http://localhost:8080  # optional
  make ingest`, strings.Join(missing, ", "))
}
