package media

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestFFmpegFrameExtractorUsesRequestedTimestampAndCleansWorkFiles(t *testing.T) {
	workRoot := t.TempDir()
	runner := &frameRunner{png: framePNG(t)}
	extractor := FFmpegFrameExtractor{
		FFmpegPath: "ffmpeg", WorkRoot: workRoot, Runner: runner,
		Sources: frameVideoSource{version: assets.AssetVersion{MIMEType: "video/mp4", DurationMS: 35_740}, content: []byte("video")},
	}
	result, err := extractor.ExtractFrame(context.Background(), FrameExtractionRequest{
		OrganizationID: "org_1", ProjectID: "project_1",
		SourceVideo: contract.AssetVersionRef{AssetID: "video_1", Version: 1}, TimestampMS: 21_271,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsPair(runner.args, "-ss", "21.271") || result.MIMEType != "image/png" || result.Version != EvidenceFrameExtractorVersion {
		t.Fatalf("extraction result=%+v args=%v", result, runner.args)
	}
	if _, err := io.ReadAll(result.Content); err != nil {
		t.Fatal(err)
	}
	if err := result.Content.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(workRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("work files were not cleaned: entries=%v err=%v", entries, err)
	}
}

type frameVideoSource struct {
	version assets.AssetVersion
	content []byte
}

func (s frameVideoSource) OpenVideo(context.Context, contract.OrganizationID, contract.ProjectID, contract.AssetVersionRef) (assets.AssetVersion, io.ReadCloser, error) {
	return s.version, io.NopCloser(bytes.NewReader(s.content)), nil
}

type frameRunner struct {
	args []string
	png  []byte
}

func (r *frameRunner) Run(_ context.Context, _ string, args ...string) error {
	r.args = append([]string{}, args...)
	return os.WriteFile(filepath.Clean(args[len(args)-1]), r.png, 0o600)
}

func containsPair(values []string, first, second string) bool {
	for index := 0; index+1 < len(values); index++ {
		if values[index] == first && values[index+1] == second {
			return true
		}
	}
	return false
}

func framePNG(t *testing.T) []byte {
	t.Helper()
	var value bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&value, img); err != nil {
		t.Fatal(err)
	}
	return value.Bytes()
}
