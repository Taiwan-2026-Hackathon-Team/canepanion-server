package caneingest

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// DecodeRGB565 expands a raw RGB565 canvas into an image.
//
// bigEndian mirrors tools/cam_view.py's --swap: which way round the two bytes
// go was ambiguous in practice, and getting it wrong shows up as blue and
// orange trading places.
func DecodeRGB565(canvas []byte, width, height int, bigEndian bool) (*image.RGBA, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("caneingest: bad dimensions %dx%d", width, height)
	}
	if len(canvas) < width*height*2 {
		return nil, fmt.Errorf("caneingest: canvas has %d bytes, need %d", len(canvas), width*height*2)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 2
			var v uint16
			if bigEndian {
				v = binary.BigEndian.Uint16(canvas[i : i+2])
			} else {
				v = binary.LittleEndian.Uint16(canvas[i : i+2])
			}
			r := uint8((v >> 11) & 0x1F)
			g := uint8((v >> 5) & 0x3F)
			b := uint8(v & 0x1F)
			// Replicate the high bits into the low ones so full-scale stays full.
			img.Set(x, y, color.RGBA{
				R: r<<3 | r>>2,
				G: g<<2 | g>>4,
				B: b<<3 | b>>2,
				A: 255,
			})
		}
	}
	return img, nil
}

// WriteCanvasPNG dumps the canvas for eyeballing.
//
// This exists because the counters cannot see the failure that matters most:
// frames captured, slices sent and delivered can all read perfectly healthy
// while the picture is rainbow noise. Only looking at a frame catches that.
func WriteCanvasPNG(path string, canvas []byte, width, height int, bigEndian bool) error {
	img, err := DecodeRGB565(canvas, width, height, bigEndian)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// Roughness is the mean absolute luma difference between horizontally adjacent
// pixels -- a cheap automated stand-in for looking at the picture.
//
// A real scene sits low because neighbouring pixels correlate; corruption
// destroys that correlation and drives the figure up by roughly an order of
// magnitude. Compare against a known-good sample from the same camera rather
// than against an absolute threshold, since scene detail moves it too.
func Roughness(canvas []byte, width, height int, bigEndian bool) (float64, error) {
	img, err := DecodeRGB565(canvas, width, height, bigEndian)
	if err != nil {
		return 0, err
	}

	var total float64
	var count int
	for y := 0; y < height; y++ {
		for x := 1; x < width; x++ {
			total += math.Abs(luma(img.RGBAAt(x, y)) - luma(img.RGBAAt(x-1, y)))
			count++
		}
	}
	if count == 0 {
		return 0, nil
	}
	return total / float64(count), nil
}

func luma(c color.RGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
}
