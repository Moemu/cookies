package creative

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type ConditionedFrameRole string

const (
	FrameRoleTail  ConditionedFrameRole = "tail_frame"
	FrameRoleStart ConditionedFrameRole = "start_frame"
)

type FrameAssetStore interface {
	OpenImage(context.Context, contract.OrganizationID, contract.ProjectID, contract.AssetVersionRef) (io.ReadCloser, string, error)
	SavePNG(context.Context, contract.OrganizationID, contract.ProjectID, string, ConditionedFrameRole, []byte) (contract.AssetVersionRef, error)
}

type FrameConditioningRequest struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	FramePlan      CreativeFramePlan
}

func (r FrameConditioningRequest) Validate() error {
	if strings.TrimSpace(string(r.OrganizationID)) == "" || strings.TrimSpace(string(r.ProjectID)) == "" {
		return fmt.Errorf("organization and project are required")
	}
	if r.FramePlan.ContractVersion != "creative-frame-plan/v1" ||
		strings.TrimSpace(r.FramePlan.TaskID) == "" ||
		r.FramePlan.Template.ID != CommerceWindowRevealTemplateID ||
		r.FramePlan.Template.Version != 1 ||
		r.FramePlan.ProductAsset.Validate() != nil ||
		r.FramePlan.WidthPixels != 720 ||
		r.FramePlan.HeightPixels != 1280 ||
		r.FramePlan.StartFrameKind != "frosted_overlay" ||
		r.FramePlan.TailFrameKind != "clear_product_reveal" {
		return fmt.Errorf("window reveal frame plan is incomplete")
	}
	return nil
}

type ConditionedFrames struct {
	StartFrame contract.AssetVersionRef `json:"start_frame"`
	TailFrame  contract.AssetVersionRef `json:"tail_frame"`
}

// FrameConditioner owns the deterministic window-reveal frame preparation.
// Asset storage remains behind FrameAssetStore, so callers only handle stable
// AssetVersionRefs.
type FrameConditioner struct {
	Assets FrameAssetStore
}

func (c FrameConditioner) Prepare(ctx context.Context, request FrameConditioningRequest) (ConditionedFrames, error) {
	if c.Assets == nil {
		return ConditionedFrames{}, fmt.Errorf("frame asset store is required")
	}
	if err := request.Validate(); err != nil {
		return ConditionedFrames{}, err
	}
	source, mimeType, err := c.Assets.OpenImage(ctx, request.OrganizationID, request.ProjectID, request.FramePlan.ProductAsset)
	if err != nil {
		return ConditionedFrames{}, fmt.Errorf("open product image: %w", err)
	}
	defer source.Close()
	if mimeType != "image/png" && mimeType != "image/jpeg" {
		return ConditionedFrames{}, fmt.Errorf("product image must be PNG or JPEG")
	}
	product, _, err := image.Decode(source)
	if err != nil {
		return ConditionedFrames{}, fmt.Errorf("decode product image: %w", err)
	}
	tail := composeClearTail(product, request.FramePlan.WidthPixels, request.FramePlan.HeightPixels)
	tailBytes, err := encodePNG(tail)
	if err != nil {
		return ConditionedFrames{}, err
	}
	tailRef, err := c.Assets.SavePNG(ctx, request.OrganizationID, request.ProjectID, request.FramePlan.TaskID, FrameRoleTail, tailBytes)
	if err != nil {
		return ConditionedFrames{}, fmt.Errorf("save clear tail frame: %w", err)
	}
	start := deriveFrostedStart(tail)
	startBytes, err := encodePNG(start)
	if err != nil {
		return ConditionedFrames{}, err
	}
	startRef, err := c.Assets.SavePNG(ctx, request.OrganizationID, request.ProjectID, request.FramePlan.TaskID, FrameRoleStart, startBytes)
	if err != nil {
		return ConditionedFrames{}, fmt.Errorf("save frosted start frame: %w", err)
	}
	return ConditionedFrames{StartFrame: startRef, TailFrame: tailRef}, nil
}

func composeClearTail(product image.Image, width, height int) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		progress := float64(y) / float64(height-1)
		base := uint8(238 - 28*progress)
		warm := uint8(222 - 18*progress)
		for x := 0; x < width; x++ {
			canvas.SetRGBA(x, y, color.RGBA{R: base, G: warm, B: 190, A: 255})
		}
	}
	sourceBounds := product.Bounds()
	sourceWidth, sourceHeight := sourceBounds.Dx(), sourceBounds.Dy()
	maxWidth, maxHeight := width*46/100, height*58/100
	targetWidth, targetHeight := sourceWidth, sourceHeight
	if targetWidth > maxWidth {
		targetHeight = targetHeight * maxWidth / targetWidth
		targetWidth = maxWidth
	}
	if targetHeight > maxHeight {
		targetWidth = targetWidth * maxHeight / targetHeight
		targetHeight = maxHeight
	}
	if targetWidth < 1 {
		targetWidth = 1
	}
	if targetHeight < 1 {
		targetHeight = 1
	}
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		sourceY := sourceBounds.Min.Y + y*sourceHeight/targetHeight
		for x := 0; x < targetWidth; x++ {
			sourceX := sourceBounds.Min.X + x*sourceWidth/targetWidth
			target.Set(x, y, product.At(sourceX, sourceY))
		}
	}
	x := (width - targetWidth) / 2
	y := (height-targetHeight)/2 + height/20
	draw.Draw(canvas, image.Rect(x, y, x+targetWidth, y+targetHeight), target, image.Point{}, draw.Over)
	return canvas
}

func deriveFrostedStart(tail *image.RGBA) *image.RGBA {
	start := image.NewRGBA(tail.Bounds())
	draw.Draw(start, start.Bounds(), tail, tail.Bounds().Min, draw.Src)
	bounds := start.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			noise := uint8((x*17 + y*31 + (x*y)%29) % 35)
			alpha := uint8(118) + noise
			frost := color.RGBA{R: 238, G: 241, B: 238, A: alpha}
			start.Set(x, y, blendRGBA(color.RGBAModel.Convert(start.At(x, y)).(color.RGBA), frost))
		}
	}
	return start
}

func blendRGBA(base, overlay color.RGBA) color.RGBA {
	alpha := uint32(overlay.A)
	inverse := uint32(255 - overlay.A)
	return color.RGBA{
		R: uint8((uint32(overlay.R)*alpha + uint32(base.R)*inverse) / 255),
		G: uint8((uint32(overlay.G)*alpha + uint32(base.G)*inverse) / 255),
		B: uint8((uint32(overlay.B)*alpha + uint32(base.B)*inverse) / 255),
		A: 255,
	}
}

func encodePNG(value image.Image) ([]byte, error) {
	var output bytes.Buffer
	if err := png.Encode(&output, value); err != nil {
		return nil, fmt.Errorf("encode conditioned frame: %w", err)
	}
	return output.Bytes(), nil
}
