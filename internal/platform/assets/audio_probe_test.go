package assets

import (
	"context"
	"os"
	"testing"
)

func TestFFprobeAudioProbeAcceptsFixtureWAV(t *testing.T) {
	path := os.Getenv("COOKIES_TEST_FFPROBE_PATH")
	if path == "" {
		t.Skip("COOKIES_TEST_FFPROBE_PATH is not configured")
	}
	metadata, err := (FFprobeAudioProbe{Path: path, WorkRoot: t.TempDir()}).Probe(context.Background(), testWAV(48000, 600), "audio/wav")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.DurationMS < 590 || metadata.DurationMS > 610 || metadata.Codec != "pcm_s16le" || metadata.Channels != 1 || metadata.SampleRate != 48000 {
		t.Fatalf("audio metadata = %#v", metadata)
	}
}
