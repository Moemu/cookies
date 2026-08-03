package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

const EvidenceFrameExtractorVersion = "ffmpeg-png-v1"

type FrameExtractionRequest struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	SourceVideo    contract.AssetVersionRef
	TimestampMS    int64
}

func (r FrameExtractionRequest) Validate() error {
	if r.OrganizationID == "" || r.ProjectID == "" {
		return fmt.Errorf("organization_id and project_id are required")
	}
	if err := r.SourceVideo.Validate(); err != nil {
		return fmt.Errorf("source video: %w", err)
	}
	if r.TimestampMS < 0 {
		return fmt.Errorf("timestamp_ms must not be negative")
	}
	return nil
}

type ExtractedFrame struct {
	Content   io.ReadCloser
	SizeBytes int64
	MIMEType  string
	Version   string
}

type FrameExtractor interface {
	ExtractFrame(context.Context, FrameExtractionRequest) (ExtractedFrame, error)
}

type FFmpegFrameExtractor struct {
	FFmpegPath string
	WorkRoot   string
	Sources    VideoSource
	Runner     CommandRunner
}

func (e FFmpegFrameExtractor) ExtractFrame(ctx context.Context, request FrameExtractionRequest) (ExtractedFrame, error) {
	if err := request.Validate(); err != nil {
		return ExtractedFrame{}, err
	}
	if strings.TrimSpace(e.FFmpegPath) == "" || e.Sources == nil {
		return ExtractedFrame{}, fmt.Errorf("frame extraction capability is unavailable")
	}
	runner := e.Runner
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	workRoot := strings.TrimSpace(e.WorkRoot)
	if workRoot == "" {
		workRoot = filepath.Join(".data", "video-work")
	}
	if err := os.MkdirAll(workRoot, 0o700); err != nil {
		return ExtractedFrame{}, fmt.Errorf("create media work root: %w", err)
	}
	workDir, err := os.MkdirTemp(workRoot, "frame-*")
	if err != nil {
		return ExtractedFrame{}, fmt.Errorf("create frame extraction directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(workDir) }

	version, source, err := e.Sources.OpenVideo(ctx, request.OrganizationID, request.ProjectID, request.SourceVideo)
	if err != nil {
		cleanup()
		return ExtractedFrame{}, err
	}
	if version.MIMEType != "video/mp4" {
		_ = source.Close()
		cleanup()
		return ExtractedFrame{}, fmt.Errorf("source asset must be video/mp4")
	}
	durationMS := version.DurationMS
	if durationMS <= 0 && version.Media.DurationSeconds > 0 {
		durationMS = int64(version.Media.DurationSeconds * 1000)
	}
	if durationMS > 0 && request.TimestampMS >= durationMS {
		_ = source.Close()
		cleanup()
		return ExtractedFrame{}, fmt.Errorf("timestamp_ms is outside the source video duration")
	}
	inputPath := filepath.Join(workDir, "source.mp4")
	input, err := os.OpenFile(inputPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err == nil {
		_, err = io.Copy(input, source)
	}
	closeInputErr := error(nil)
	if input != nil {
		closeInputErr = input.Close()
	}
	closeSourceErr := source.Close()
	if err != nil || closeInputErr != nil || closeSourceErr != nil {
		cleanup()
		return ExtractedFrame{}, fmt.Errorf("copy source video: %v", firstError(err, closeInputErr, closeSourceErr))
	}
	outputPath := filepath.Join(workDir, "frame.png")
	seconds := strconv.FormatFloat(float64(request.TimestampMS)/1000, 'f', 3, 64)
	if err := runner.Run(ctx, e.FFmpegPath, "-hide_banner", "-loglevel", "error", "-ss", seconds, "-i", inputPath, "-frames:v", "1", "-f", "image2", outputPath); err != nil {
		cleanup()
		return ExtractedFrame{}, err
	}
	info, err := os.Stat(outputPath)
	if err != nil || info.Size() < 1 || info.Size() > assets.MaxImageBytes {
		cleanup()
		return ExtractedFrame{}, fmt.Errorf("extracted frame is missing or outside the supported size")
	}
	file, err := os.Open(outputPath)
	if err != nil {
		cleanup()
		return ExtractedFrame{}, err
	}
	return ExtractedFrame{
		Content:   &cleanupReadCloser{ReadCloser: file, cleanup: cleanup},
		SizeBytes: info.Size(), MIMEType: "image/png", Version: EvidenceFrameExtractorVersion,
	}, nil
}

func firstError(values ...error) error {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
