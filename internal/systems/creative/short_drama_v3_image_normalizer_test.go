package creative

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestRenderShortDramaCoverPNGProducesExactCanvas(t *testing.T) {
	t.Parallel()

	source := image.NewRGBA(image.Rect(0, 0, 1536, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1536; x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 80, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	output, err := renderShortDramaCoverPNG(bytes.NewReader(encoded.Bytes()), 1280, 546)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _, err := image.Decode(bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 1280 || decoded.Bounds().Dy() != 546 {
		t.Fatalf("normalized bounds = %v", decoded.Bounds())
	}
}
