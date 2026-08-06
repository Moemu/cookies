package media

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

type goldenTimelineSource struct{ files map[contract.AssetID]string }

func (s goldenTimelineSource) open(ref contract.AssetVersionRef) (assets.AssetVersion, io.ReadCloser, error) {
	contents, err := os.ReadFile(s.files[ref.AssetID])
	if err != nil {
		return assets.AssetVersion{}, nil, err
	}
	return assets.AssetVersion{AssetID: ref.AssetID, Version: ref.Version, SizeBytes: int64(len(contents))}, io.NopCloser(bytes.NewReader(contents)), nil
}
func (s goldenTimelineSource) OpenVideo(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, ref contract.AssetVersionRef) (assets.AssetVersion, io.ReadCloser, error) {
	return s.open(ref)
}
func (s goldenTimelineSource) OpenAudio(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, ref contract.AssetVersionRef) (assets.AssetVersion, io.ReadCloser, error) {
	return s.open(ref)
}

func TestFFmpegTimelinePreviewAndExportMatchGoldenMedia(t *testing.T) {
	ffmpegPath, ffprobePath := os.Getenv("COOKIES_TEST_FFMPEG_PATH"), os.Getenv("COOKIES_TEST_FFPROBE_PATH")
	videoA, videoB, audio := os.Getenv("COOKIES_TEST_TIMELINE_VIDEO_A"), os.Getenv("COOKIES_TEST_TIMELINE_VIDEO_B"), os.Getenv("COOKIES_TEST_TIMELINE_AUDIO")
	if ffmpegPath == "" || ffprobePath == "" || videoA == "" || videoB == "" || audio == "" {
		t.Skip("set timeline FFmpeg golden fixture environment")
	}
	refs := map[contract.AssetID]string{"video-a": videoA, "video-b": videoB, "music": audio}
	source := goldenTimelineSource{files: refs}
	probe := assets.FFprobeVideoProbe{Path: ffprobePath, WorkRoot: t.TempDir()}
	renderer := FFmpegTimelineRenderer{FFmpegPath: ffmpegPath, WorkRoot: t.TempDir(), Videos: source, Audio: source, Probe: probe}
	request := TimelineRenderRequest{OrganizationID: "org_golden", ProjectID: "project_golden", DurationMS: 12000, Width: 720, Height: 1280, FrameRate: 30, SampleRate: 48000, Video: []TimelineVideoClip{{ID: "a", Asset: contract.AssetVersionRef{AssetID: "video-a", Version: 1}, EndMS: 6000}, {ID: "b", Asset: contract.AssetVersionRef{AssetID: "video-b", Version: 1}, StartMS: 6000, EndMS: 12000}}, Audio: []TimelineAudioClip{{ID: "music", Role: TimelineAudioMusic, Asset: contract.AssetVersionRef{AssetID: "music", Version: 1}, EndMS: 12000, GainDB: -12, Loop: true}}, Captions: []TimelineCaption{{StartMS: 1000, EndMS: 5000, Text: "固定黄金字幕"}}}
	render := func(name string) string {
		output, err := renderer.Render(context.Background(), request, nil)
		if err != nil {
			t.Fatal(err)
		}
		contents, err := io.ReadAll(output.Content)
		if err != nil {
			t.Fatal(err)
		}
		if err = output.Content.Close(); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), name+".mp4")
		if err = os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	preview, exported := render("preview"), render("export")
	previewMeta, err := probeFile(context.Background(), probe, preview)
	if err != nil {
		t.Fatal(err)
	}
	exportMeta, err := probeFile(context.Background(), probe, exported)
	if err != nil {
		t.Fatal(err)
	}
	if previewMeta.DurationMS != exportMeta.DurationMS || previewMeta.WidthPixels != 720 || previewMeta.HeightPixels != 1280 || previewMeta.VideoCodec != "h264" || previewMeta.AudioCodec == "" {
		t.Fatalf("metadata mismatch preview=%+v export=%+v", previewMeta, exportMeta)
	}
	for _, selector := range []string{"0:v:0", "0:a:0"} {
		if goldenDigest(t, ffmpegPath, preview, selector) != goldenDigest(t, ffmpegPath, exported, selector) {
			t.Fatalf("preview/export %s frame digest mismatch", selector)
		}
	}
}

func probeFile(ctx context.Context, probe assets.FFprobeVideoProbe, path string) (assets.VideoMetadata, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return assets.VideoMetadata{}, err
	}
	return probe.Probe(ctx, contents)
}
func goldenDigest(t *testing.T, ffmpegPath, path, selector string) string {
	t.Helper()
	command := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-i", path, "-map", selector, "-f", "framemd5", "-")
	contents, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
