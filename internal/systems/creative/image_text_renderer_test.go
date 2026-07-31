package creative

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestImageTextRendererProducesDeterministicThreeByFourPNG(t *testing.T) {
	t.Parallel()
	var base bytes.Buffer
	source := image.NewRGBA(image.Rect(0, 0, ImageTextSourceWidth, ImageTextSourceHeight))
	for y := 0; y < ImageTextSourceHeight; y++ {
		for x := 0; x < ImageTextSourceWidth; x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 90, A: 255})
		}
	}
	if err := png.Encode(&base, source); err != nil {
		t.Fatalf("encode base image: %v", err)
	}
	sum := sha256.Sum256(goregular.TTF)
	checksum := hex.EncodeToString(sum[:])
	renderer := ImageTextRenderer{
		FontBytes: goregular.TTF, FontRef: "goregular-test@sha256:" + checksum,
		ExpectedSHA256: checksum,
	}
	spec, err := NewImageRenderSpec(ImagePlanItem{
		Order: 1, Role: string(ImageTextRoleCover), Purpose: "cover",
		VisualBrief: "editorial product image", Caption: "cover", OverlayCopy: "VISIBLE PRECISION",
		LayoutPreset: "cover_center_v1",
	}, renderer.FontRef)
	if err != nil {
		t.Fatalf("NewImageRenderSpec() error = %v", err)
	}
	first, err := renderer.Render(bytes.NewReader(base.Bytes()), spec)
	if err != nil {
		t.Fatalf("first Render() error = %v", err)
	}
	second, err := renderer.Render(bytes.NewReader(base.Bytes()), spec)
	if err != nil {
		t.Fatalf("second Render() error = %v", err)
	}
	if first.ContentHash != second.ContentHash || !bytes.Equal(first.Bytes, second.Bytes) {
		t.Fatal("renderer output is not deterministic for identical input")
	}
	decoded, format, err := image.Decode(bytes.NewReader(first.Bytes))
	if err != nil {
		t.Fatalf("decode rendered output: %v", err)
	}
	if format != "png" || decoded.Bounds().Dx() != ImageTextFinalWidth ||
		decoded.Bounds().Dy() != ImageTextFinalHeight {
		t.Fatalf("rendered output = %s %v, want png %dx%d", format, decoded.Bounds(), ImageTextFinalWidth, ImageTextFinalHeight)
	}
}

func TestImageTextRendererRejectsUnpinnedFont(t *testing.T) {
	t.Parallel()
	renderer := ImageTextRenderer{
		FontBytes: goregular.TTF, FontRef: "goregular-test",
		ExpectedSHA256: "not-the-font-checksum",
	}
	if err := renderer.Ready(); err == nil {
		t.Fatal("Ready() accepted a mismatched font checksum")
	}
}

func TestImageTextRendererRejectsMissingFontChecksum(t *testing.T) {
	t.Parallel()
	renderer := ImageTextRenderer{FontBytes: goregular.TTF, FontRef: "goregular-test"}
	if err := renderer.Ready(); err == nil {
		t.Fatal("Ready() accepted a font without a checksum")
	}
}
