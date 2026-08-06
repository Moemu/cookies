package creative

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	stdDraw "image/draw"
	"image/png"
	"io"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type ImageTextRenderer struct {
	FontBytes      []byte
	FontRef        string
	ExpectedSHA256 string
}

type RenderedImage struct {
	Content     *bytes.Reader
	Bytes       []byte
	SizeBytes   int64
	ContentHash string
	Width       int
	Height      int
}

func (r ImageTextRenderer) Ready() error {
	if len(r.FontBytes) == 0 || strings.TrimSpace(r.FontRef) == "" {
		return fmt.Errorf("image-text renderer font is not configured")
	}
	sum := sha256.Sum256(r.FontBytes)
	actual := hex.EncodeToString(sum[:])
	expected := strings.ToLower(strings.TrimSpace(r.ExpectedSHA256))
	if len(expected) != 64 {
		return fmt.Errorf("image-text renderer font checksum is required")
	}
	if actual != expected {
		return fmt.Errorf("image-text renderer font checksum mismatch")
	}
	if !strings.HasSuffix(r.FontRef, "@sha256:"+actual) {
		return fmt.Errorf("image-text renderer font_ref must include the verified checksum")
	}
	if _, err := parseImageTextFont(r.FontBytes); err != nil {
		return fmt.Errorf("parse image-text renderer font: %w", err)
	}
	return nil
}

func (r ImageTextRenderer) Render(base io.Reader, spec ImageRenderSpec) (RenderedImage, error) {
	if err := r.Ready(); err != nil {
		return RenderedImage{}, err
	}
	if base == nil || spec.ContractVersion != ImageRenderSpecV1Contract ||
		spec.SourceWidth != ImageTextSourceWidth || spec.SourceHeight != ImageTextSourceHeight ||
		spec.FinalWidth != ImageTextFinalWidth || spec.FinalHeight != ImageTextFinalHeight ||
		spec.OutputFormat != "png" ||
		(spec.RendererVersion != ImageRendererV1 && spec.RendererVersion != ImageRendererV2) ||
		spec.FontRef != r.FontRef || strings.TrimSpace(spec.OverlayCopy) == "" {
		return RenderedImage{}, fmt.Errorf("image render spec is invalid")
	}
	source, _, err := image.Decode(base)
	if err != nil {
		return RenderedImage{}, fmt.Errorf("decode image base: %w", err)
	}
	bounds := source.Bounds()
	if bounds.Dx() < ImageTextSourceWidth || bounds.Dy() < ImageTextSourceHeight {
		return RenderedImage{}, fmt.Errorf("image base is smaller than the frozen source profile")
	}
	crop := centerCropThreeByFour(bounds)
	canvas := image.NewRGBA(image.Rect(0, 0, ImageTextFinalWidth, ImageTextFinalHeight))
	xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), source, crop, stdDraw.Src, nil)

	parsedFont, err := parseImageTextFont(r.FontBytes)
	if err != nil {
		return RenderedImage{}, err
	}
	fontSize, maxLines, box := layoutMetrics(spec.LayoutPreset)
	if spec.RendererVersion == ImageRendererV2 {
		fontSize, maxLines, box = modernLayoutMetrics(spec.LayoutPreset)
	}
	face, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size: fontSize, DPI: 72, Hinting: font.HintingFull,
	})
	if err != nil {
		return RenderedImage{}, fmt.Errorf("create image font face: %w", err)
	}
	defer face.Close()

	textInset := 96
	if spec.RendererVersion == ImageRendererV2 {
		textInset = 56
	}
	lines, err := wrapText(face, spec.OverlayCopy, box.Dx()-textInset, maxLines)
	if err != nil {
		return RenderedImage{}, err
	}
	if spec.RendererVersion == ImageRendererV2 {
		drawModernOverlay(canvas, face, fontSize, lines, box, spec.LayoutPreset)
	} else {
		stdDraw.Draw(canvas, box, &image.Uniform{C: color.RGBA{R: 12, G: 17, B: 24, A: 190}}, image.Point{}, stdDraw.Over)
		drawer := font.Drawer{Dst: canvas, Src: image.White, Face: face}
		lineHeight := int(fontSize * 1.35)
		totalHeight := len(lines) * lineHeight
		y := box.Min.Y + (box.Dy()-totalHeight)/2 + int(fontSize)
		for _, line := range lines {
			width := drawer.MeasureString(line).Ceil()
			x := box.Min.X + (box.Dx()-width)/2
			drawer.Dot = fixed.P(x, y)
			drawer.DrawString(line)
			y += lineHeight
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return RenderedImage{}, fmt.Errorf("encode rendered image: %w", err)
	}
	data := output.Bytes()
	sum := sha256.Sum256(data)
	return RenderedImage{
		Content: bytes.NewReader(data), Bytes: append([]byte{}, data...), SizeBytes: int64(len(data)),
		ContentHash: hex.EncodeToString(sum[:]), Width: ImageTextFinalWidth, Height: ImageTextFinalHeight,
	}, nil
}

func centerCropThreeByFour(bounds image.Rectangle) image.Rectangle {
	width, height := bounds.Dx(), bounds.Dy()
	targetHeight := width * 4 / 3
	if targetHeight <= height {
		top := bounds.Min.Y + (height-targetHeight)/2
		return image.Rect(bounds.Min.X, top, bounds.Max.X, top+targetHeight)
	}
	targetWidth := height * 3 / 4
	left := bounds.Min.X + (width-targetWidth)/2
	return image.Rect(left, bounds.Min.Y, left+targetWidth, bounds.Max.Y)
}

func layoutMetrics(preset string) (float64, int, image.Rectangle) {
	switch preset {
	case "proof_lower_left_v1":
		return 54, 4, image.Rect(72, 930, 1008, 1348)
	case "cta_bottom_v1":
		return 58, 3, image.Rect(72, 1030, 1008, 1368)
	default:
		return 72, 2, image.Rect(72, 470, 1008, 930)
	}
}

func modernLayoutMetrics(preset string) (float64, int, image.Rectangle) {
	switch preset {
	case "proof_lower_left_v1":
		return 58, 4, image.Rect(82, 990, 950, 1328)
	case "cta_bottom_v1":
		return 62, 3, image.Rect(82, 1020, 998, 1338)
	default:
		return 74, 3, image.Rect(82, 875, 998, 1318)
	}
}

func drawModernOverlay(canvas *image.RGBA, face font.Face, fontSize float64, lines []string, box image.Rectangle, preset string) {
	gradientStart := 700
	if preset == "proof_lower_left_v1" {
		gradientStart = 820
	} else if preset == "cta_bottom_v1" {
		gradientStart = 880
	}
	drawVerticalScrim(canvas, gradientStart, canvas.Bounds().Max.Y, 8, 210)

	lineHeight := int(fontSize * 1.3)
	totalHeight := len(lines) * lineHeight
	y := box.Min.Y + (box.Dy()-totalHeight)/2 + int(fontSize)
	textX := box.Min.X + 28
	accentTop := y - int(fontSize)
	accentBottom := y + (len(lines)-1)*lineHeight + 8
	stdDraw.Draw(canvas, image.Rect(box.Min.X, accentTop, box.Min.X+8, accentBottom),
		&image.Uniform{C: color.RGBA{R: 54, G: 132, B: 246, A: 255}}, image.Point{}, stdDraw.Over)
	shadow := font.Drawer{Dst: canvas, Src: &image.Uniform{C: color.RGBA{A: 185}}, Face: face}
	drawer := font.Drawer{Dst: canvas, Src: image.White, Face: face}
	for _, line := range lines {
		shadow.Dot = fixed.P(textX+2, y+3)
		shadow.DrawString(line)
		drawer.Dot = fixed.P(textX, y)
		drawer.DrawString(line)
		y += lineHeight
	}
}

func drawVerticalScrim(canvas *image.RGBA, startY, endY int, startAlpha, endAlpha uint8) {
	if startY < canvas.Bounds().Min.Y {
		startY = canvas.Bounds().Min.Y
	}
	if endY > canvas.Bounds().Max.Y {
		endY = canvas.Bounds().Max.Y
	}
	if endY <= startY {
		return
	}
	span := endY - startY
	for y := startY; y < endY; y++ {
		alpha := int(startAlpha) + (int(endAlpha)-int(startAlpha))*(y-startY)/span
		stdDraw.Draw(canvas, image.Rect(canvas.Bounds().Min.X, y, canvas.Bounds().Max.X, y+1),
			&image.Uniform{C: color.RGBA{R: 7, G: 12, B: 20, A: uint8(alpha)}}, image.Point{}, stdDraw.Over)
	}
}

func wrapText(face font.Face, value string, maxWidth, maxLines int) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("overlay copy is required")
	}
	drawer := font.Drawer{Face: face}
	lines := []string{}
	current := ""
	for _, token := range []rune(value) {
		if token != '\n' && !strings.ContainsRune(" \t\r", token) {
			if _, ok := face.GlyphAdvance(token); !ok {
				return nil, fmt.Errorf("configured font does not contain required overlay glyph %q", token)
			}
		}
		candidate := current + string(token)
		if token == '\n' || drawer.MeasureString(candidate).Ceil() > maxWidth {
			if strings.TrimSpace(current) == "" {
				return nil, fmt.Errorf("overlay copy contains a glyph wider than the safe area")
			}
			lines = append(lines, strings.TrimSpace(current))
			current = ""
			if token != '\n' {
				current = string(token)
			}
			continue
		}
		current = candidate
	}
	if strings.TrimSpace(current) != "" {
		lines = append(lines, strings.TrimSpace(current))
	}
	if len(lines) == 0 || len(lines) > maxLines {
		return nil, fmt.Errorf("overlay copy exceeds the selected layout preset")
	}
	return lines, nil
}

func parseImageTextFont(data []byte) (*opentype.Font, error) {
	parsed, err := opentype.Parse(data)
	if err == nil {
		return parsed, nil
	}
	collection, collectionErr := opentype.ParseCollection(data)
	if collectionErr != nil {
		return nil, err
	}
	if collection.NumFonts() < 1 {
		return nil, fmt.Errorf("font collection is empty")
	}
	return collection.Font(0)
}

func NewImageRenderSpec(slot ImagePlanItem, fontRef string) (ImageRenderSpec, error) {
	value := ImageRenderSpec{
		ContractVersion: ImageRenderSpecV1Contract, LayoutPreset: slot.LayoutPreset,
		OverlayCopy: slot.OverlayCopy, SourceWidth: ImageTextSourceWidth, SourceHeight: ImageTextSourceHeight,
		FinalWidth: ImageTextFinalWidth, FinalHeight: ImageTextFinalHeight, OutputFormat: "png",
		FontRef: fontRef, RendererVersion: ImageRendererV2,
	}
	hash, err := contractHashWithoutRenderContent(value)
	if err != nil {
		return ImageRenderSpec{}, err
	}
	value.ContentHash = hash
	return value, nil
}

func contractHashWithoutRenderContent(value ImageRenderSpec) (string, error) {
	value.ContentHash = ""
	hash, err := contract.NewContentHash(value)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
