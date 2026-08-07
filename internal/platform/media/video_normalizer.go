package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

const ShortDramaVideoNormalizerVersion = "ffmpeg-short-drama-cover-crop/v1"

type VideoNormalizationRequest struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	SourceVideo    contract.AssetVersionRef
	Width          int
	Height         int
	FrameRate      int
}

func (r VideoNormalizationRequest) Validate() error {
	if r.OrganizationID == "" || r.ProjectID == "" || r.Width < 2 || r.Height < 2 || r.Width%2 != 0 || r.Height%2 != 0 || r.FrameRate < 1 || r.FrameRate > 60 {
		return fmt.Errorf("video normalization request is invalid")
	}
	if err := r.SourceVideo.Validate(); err != nil {
		return fmt.Errorf("source video: %w", err)
	}
	return nil
}

type VideoNormalizer interface {
	NormalizeVideo(context.Context, VideoNormalizationRequest) (CompositionOutput, error)
}

func (c FFmpegComposer) NormalizeVideo(ctx context.Context, request VideoNormalizationRequest) (CompositionOutput, error) {
	if err := request.Validate(); err != nil {
		return CompositionOutput{}, err
	}
	if strings.TrimSpace(c.FFmpegPath) == "" || c.Sources == nil || c.Probe == nil {
		return CompositionOutput{}, fmt.Errorf("video normalization capability is unavailable")
	}
	runner := c.Runner
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	workRoot := strings.TrimSpace(c.WorkRoot)
	if workRoot == "" {
		workRoot = filepath.Join(".data", "video-work")
	}
	if err := os.MkdirAll(workRoot, 0o700); err != nil {
		return CompositionOutput{}, fmt.Errorf("create media work root: %w", err)
	}
	workDir, err := os.MkdirTemp(workRoot, "short-drama-normalize-*")
	if err != nil {
		return CompositionOutput{}, err
	}
	cleanup := func() { _ = os.RemoveAll(workDir) }
	inputPath, outputPath := filepath.Join(workDir, "input.mp4"), filepath.Join(workDir, "output.mp4")
	if _, err := c.copySource(ctx, request.OrganizationID, request.ProjectID, request.SourceVideo, inputPath); err != nil {
		cleanup()
		return CompositionOutput{}, err
	}
	filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,fps=%d,setsar=1,format=yuv420p", request.Width, request.Height, request.Width, request.Height, request.FrameRate)
	if err := runner.Run(ctx, c.FFmpegPath,
		"-hide_banner", "-loglevel", "error", "-y", "-i", inputPath,
		"-map", "0:v:0", "-map", "0:a:0?", "-vf", filter,
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-ar", "48000", "-ac", "2", "-movflags", "+faststart", outputPath,
	); err != nil {
		cleanup()
		return CompositionOutput{}, fmt.Errorf("normalize short drama video: %w", err)
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		cleanup()
		return CompositionOutput{}, err
	}
	metadata, err := c.Probe.Probe(ctx, contents)
	if err != nil {
		cleanup()
		return CompositionOutput{}, fmt.Errorf("probe normalized short drama video: %w", err)
	}
	if metadata.WidthPixels != request.Width || metadata.HeightPixels != request.Height {
		cleanup()
		return CompositionOutput{}, fmt.Errorf("normalized short drama video dimensions are %dx%d, want %dx%d", metadata.WidthPixels, metadata.HeightPixels, request.Width, request.Height)
	}
	file, err := os.Open(outputPath)
	if err != nil {
		cleanup()
		return CompositionOutput{}, err
	}
	info, err := file.Stat()
	if err != nil || info.Size() < 1 || info.Size() > assets.MaxVideoBytes {
		file.Close()
		cleanup()
		return CompositionOutput{}, fmt.Errorf("normalized short drama video is invalid")
	}
	return CompositionOutput{Content: &cleanupReadCloser{ReadCloser: file, cleanup: cleanup}, SizeBytes: info.Size(), Metadata: metadata}, nil
}
