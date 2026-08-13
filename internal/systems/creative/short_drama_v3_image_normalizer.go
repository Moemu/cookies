package creative

import (
	"bytes"
	"context"
	"fmt"
	"image"
	stdDraw "image/draw"
	"image/png"
	"io"

	"github.com/shikanon/cookies/internal/platform/contract"
	xdraw "golang.org/x/image/draw"
)

func renderShortDramaCoverPNG(source io.Reader, width, height int) ([]byte, error) {
	if source == nil || width < 2 || height < 2 || width%2 != 0 || height%2 != 0 {
		return nil, fmt.Errorf("short drama image normalization dimensions are invalid")
	}
	decoded, _, err := image.Decode(source)
	if err != nil {
		return nil, fmt.Errorf("decode short drama first frame: %w", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() < 2 || bounds.Dy() < 2 {
		return nil, fmt.Errorf("short drama first frame is empty")
	}
	crop := coverCropBounds(bounds, width, height)
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), decoded, crop, stdDraw.Src, nil)
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, fmt.Errorf("encode normalized short drama first frame: %w", err)
	}
	return output.Bytes(), nil
}

func renderShortDramaBoardPNG(source io.Reader, width, height int) ([]byte, error) {
	if source == nil || width < 2 || height < 2 || width%2 != 0 || height%2 != 0 {
		return nil, fmt.Errorf("short drama reference board dimensions are invalid")
	}
	decoded, _, err := image.Decode(source)
	if err != nil {
		return nil, fmt.Errorf("decode short drama reference board: %w", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() < 2 || bounds.Dy() < 2 {
		return nil, fmt.Errorf("short drama reference board is empty")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	stdDraw.Draw(canvas, canvas.Bounds(), image.Black, image.Point{}, stdDraw.Src)
	scale := min(float64(width)/float64(bounds.Dx()), float64(height)/float64(bounds.Dy()))
	drawWidth := max(2, int(float64(bounds.Dx())*scale))
	drawHeight := max(2, int(float64(bounds.Dy())*scale))
	destination := image.Rect((width-drawWidth)/2, (height-drawHeight)/2, (width+drawWidth)/2, (height+drawHeight)/2)
	xdraw.CatmullRom.Scale(canvas, destination, decoded, bounds, stdDraw.Src, nil)
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, fmt.Errorf("encode normalized short drama reference board: %w", err)
	}
	return output.Bytes(), nil
}

func (s Service) normalizeShortDramaReferenceBoardCandidate(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	candidate ShortDramaReferenceBoardCandidate,
	board ShortDramaBoardCanvas,
) (ShortDramaReferenceBoardCandidate, error) {
	if candidate.Asset == nil || s.ImageBaseAssets == nil || s.RenderedImages == nil {
		return candidate, fmt.Errorf("short drama reference-board normalization capability is unavailable")
	}
	reader, err := s.ImageBaseAssets.OpenImage(ctx, actor, projectID, candidate.Asset.AssetVersion)
	if err != nil {
		return candidate, err
	}
	defer reader.Close()
	content, err := renderShortDramaBoardPNG(reader, board.Width, board.Height)
	if err != nil {
		return candidate, err
	}
	requestContext := contract.RequestContext{RequestID: "short-drama-board-" + candidate.ID, TraceID: taskID, Actor: actor}
	asset, err := s.RenderedImages.IngestRenderedImage(
		ctx, requestContext, projectID, candidate.ID+"-model-reference", bytes.NewReader(content), int64(len(content)),
		board.Width, board.Height, []contract.AssetVersionRef{candidate.Asset.AssetVersion}, nil,
	)
	if err != nil {
		return candidate, err
	}
	candidate.ModelReferenceAsset = &asset
	return candidate, nil
}

func coverCropBounds(bounds image.Rectangle, targetWidth, targetHeight int) image.Rectangle {
	sourceWidth, sourceHeight := bounds.Dx(), bounds.Dy()
	targetRatio := float64(targetWidth) / float64(targetHeight)
	sourceRatio := float64(sourceWidth) / float64(sourceHeight)
	if sourceRatio > targetRatio {
		cropWidth := int(float64(sourceHeight) * targetRatio)
		left := bounds.Min.X + (sourceWidth-cropWidth)/2
		return image.Rect(left, bounds.Min.Y, left+cropWidth, bounds.Max.Y)
	}
	cropHeight := int(float64(sourceWidth) / targetRatio)
	top := bounds.Min.Y + (sourceHeight-cropHeight)/2
	return image.Rect(bounds.Min.X, top, bounds.Max.X, top+cropHeight)
}

func (s Service) normalizeShortDramaFirstFrameCandidate(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	candidate ShortDramaV2FirstFrameCandidate,
	model ShortDramaModelCanvas,
	output ShortDramaOutputCanvas,
) (ShortDramaV2FirstFrameCandidate, error) {
	if candidate.Asset == nil || s.ImageBaseAssets == nil || s.RenderedImages == nil {
		return candidate, fmt.Errorf("short drama first-frame normalization capability is unavailable")
	}
	readAndRender := func(width, height int) ([]byte, error) {
		reader, err := s.ImageBaseAssets.OpenImage(ctx, actor, projectID, candidate.Asset.AssetVersion)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return renderShortDramaCoverPNG(reader, width, height)
	}
	modelBytes, err := readAndRender(model.Width, model.Height)
	if err != nil {
		return candidate, err
	}
	requestContext := contract.RequestContext{
		RequestID: "short-drama-frame-" + candidate.ID,
		TraceID:   taskID,
		Actor:     actor,
	}
	modelAsset, err := s.RenderedImages.IngestRenderedImage(
		ctx, requestContext, projectID, candidate.ID+"-model-canvas", bytes.NewReader(modelBytes), int64(len(modelBytes)),
		model.Width, model.Height,
		[]contract.AssetVersionRef{candidate.Asset.AssetVersion}, nil,
	)
	if err != nil {
		return candidate, err
	}
	candidate.ModelCanvasAsset = &modelAsset
	if output.Width == model.Width && output.Height == model.Height {
		candidate.OutputCanvasAsset = &modelAsset
		return candidate, nil
	}
	outputBytes, err := renderShortDramaCoverPNG(bytes.NewReader(modelBytes), output.Width, output.Height)
	if err != nil {
		return candidate, err
	}
	outputAsset, err := s.RenderedImages.IngestRenderedImage(
		ctx, requestContext, projectID, candidate.ID+"-output-canvas", bytes.NewReader(outputBytes), int64(len(outputBytes)),
		output.Width, output.Height,
		[]contract.AssetVersionRef{modelAsset.AssetVersion}, nil,
	)
	if err != nil {
		return candidate, err
	}
	candidate.OutputCanvasAsset = &outputAsset
	return candidate, nil
}
