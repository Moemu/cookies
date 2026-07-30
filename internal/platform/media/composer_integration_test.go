package media

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

// This test is intentionally opt-in because CI does not install FFmpeg. Local
// phase acceptance supplies all three paths explicitly.
func TestFFmpegComposerProducesPlayablePreRollOutput(t *testing.T) {
	ffmpegPath := os.Getenv("COOKIES_TEST_FFMPEG_PATH")
	ffprobePath := os.Getenv("COOKIES_TEST_FFPROBE_PATH")
	videoPath := os.Getenv("COOKIES_TEST_MAIN_VIDEO")
	if ffmpegPath == "" || ffprobePath == "" || videoPath == "" {
		t.Skip("set COOKIES_TEST_FFMPEG_PATH, COOKIES_TEST_FFPROBE_PATH, and COOKIES_TEST_MAIN_VIDEO")
	}
	contents, err := os.ReadFile(videoPath)
	if err != nil {
		t.Fatal(err)
	}
	probe := assets.FFprobeVideoProbe{Path: ffprobePath, WorkRoot: t.TempDir()}
	metadata, err := probe.Probe(context.Background(), contents)
	if err != nil {
		t.Fatalf("probe fixture: %v", err)
	}
	source := integrationVideoSource{contents: contents, metadata: metadata}
	composer := FFmpegComposer{FFmpegPath: ffmpegPath, WorkRoot: t.TempDir(), Sources: source, Probe: probe}
	output, err := composer.ComposePreRoll(context.Background(), PreRollCompositionRequest{
		OrganizationID: "org_test",
		ProjectID:      "project_test",
		PreRollVideo:   contract.AssetVersionRef{AssetID: "asset_preroll", Version: 1},
		MainVideo:      contract.AssetVersionRef{AssetID: "asset_main", Version: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer output.Content.Close()
	if output.SizeBytes < 1 || output.Metadata.VideoCodec != "h264" || output.Metadata.WidthPixels != 720 || output.Metadata.HeightPixels != 1280 {
		t.Fatalf("unexpected composition output: size=%d metadata=%+v", output.SizeBytes, output.Metadata)
	}
	if output.Metadata.DurationMS < metadata.DurationMS*2-250 || output.Metadata.DurationMS > metadata.DurationMS*2+250 {
		t.Fatalf("composition duration = %dms, input = %dms", output.Metadata.DurationMS, metadata.DurationMS)
	}
}

type integrationVideoSource struct {
	contents []byte
	metadata assets.VideoMetadata
}

func (s integrationVideoSource) OpenVideo(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, ref contract.AssetVersionRef) (assets.AssetVersion, io.ReadCloser, error) {
	return assets.AssetVersion{
		AssetID:      ref.AssetID,
		Version:      ref.Version,
		MIMEType:     "video/mp4",
		SizeBytes:    int64(len(s.contents)),
		DurationMS:   s.metadata.DurationMS,
		WidthPixels:  s.metadata.WidthPixels,
		HeightPixels: s.metadata.HeightPixels,
		FrameRate:    s.metadata.FrameRate,
		VideoCodec:   s.metadata.VideoCodec,
		AudioCodec:   s.metadata.AudioCodec,
	}, io.NopCloser(bytes.NewReader(s.contents)), nil
}
