package creative

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/media"
)

func TestCompileEditingTimelineV1BuildsFrozenVerticalRenderRequest(t *testing.T) {
	timeline := EditingTimelineV1{
		SchemaVersion: EditingTimelineSchemaV1,
		OutputProfile: EditingMVPVerticalOutputProfile,
		DurationMS:    20000,
		Tracks: []EditingTimelineTrack{
			{
				ID: "primary-video", Role: EditingTrackPrimaryVideo,
				Clips: []EditingTimelineClip{
					{ID: "preroll", AssetRef: &contract.AssetVersionRef{AssetID: "asset_preroll", Version: 1}, TimelineStartMS: 0, TimelineEndMS: 6000, SourceInMS: 0, SourceOutMS: 6000},
					{ID: "source-video", AssetRef: &contract.AssetVersionRef{AssetID: "asset_source", Version: 3}, TimelineStartMS: 6000, TimelineEndMS: 20000, SourceInMS: 2000, SourceOutMS: 16000},
				},
			},
			{
				ID: "captions", Role: EditingTrackCaption,
				Clips: []EditingTimelineClip{{ID: "caption-1", TimelineStartMS: 0, TimelineEndMS: 2600, Text: "先看 6 秒前贴"}},
			},
			{
				ID: "music", Role: EditingTrackMusic,
				Clips: []EditingTimelineClip{{ID: "music-1", AssetRef: &contract.AssetVersionRef{AssetID: "asset_music", Version: 1}, TimelineStartMS: 0, TimelineEndMS: 20000, SourceInMS: 1000, SourceOutMS: 21000, GainDB: -12, Loop: true}},
			},
		},
	}

	request, err := CompileEditingTimelineV1(timeline, "org_1", "project_1")
	if err != nil {
		t.Fatal(err)
	}
	if request.OrganizationID != "org_1" || request.ProjectID != "project_1" || request.DurationMS != 20000 || request.Width != 720 || request.Height != 1280 || request.FrameRate != 30 || request.SampleRate != 48000 {
		t.Fatalf("unexpected render profile: %#v", request)
	}
	wantVideo := []media.TimelineVideoClip{
		{ID: "preroll", Asset: contract.AssetVersionRef{AssetID: "asset_preroll", Version: 1}, StartMS: 0, EndMS: 6000, SourceIn: 0},
		{ID: "source-video", Asset: contract.AssetVersionRef{AssetID: "asset_source", Version: 3}, StartMS: 6000, EndMS: 20000, SourceIn: 2000},
	}
	if len(request.Video) != len(wantVideo) {
		t.Fatalf("video clips = %#v, want %#v", request.Video, wantVideo)
	}
	for index, want := range wantVideo {
		if request.Video[index] != want {
			t.Fatalf("video clip %d = %#v, want %#v", index, request.Video[index], want)
		}
	}
	if len(request.Audio) != 1 || request.Audio[0].Role != media.TimelineAudioMusic || request.Audio[0].SourceIn != 1000 || !request.Audio[0].Loop || request.Audio[0].GainDB != -12 {
		t.Fatalf("music clip = %#v", request.Audio)
	}
	if len(request.Captions) != 1 || request.Captions[0].Text != "先看 6 秒前贴" {
		t.Fatalf("captions = %#v", request.Captions)
	}
}

func TestCompileEditingTimelineV1GoldenShortDramaFixture(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("testdata", "editing-timeline-v1", "short-drama-preroll-with-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var timeline EditingTimelineV1
	if err := json.Unmarshal(contents, &timeline); err != nil {
		t.Fatal(err)
	}
	request, err := CompileEditingTimelineV1(timeline, "org_fixture", "project_fixture")
	if err != nil {
		t.Fatal(err)
	}
	if request.DurationMS != 20000 || len(request.Video) != 2 || len(request.Captions) != 1 || len(request.Audio) != 1 {
		t.Fatalf("golden fixture did not compile to the expected render request: %#v", request)
	}
	if request.Video[0].Asset.AssetID != "asset_short_drama_preroll" || request.Video[1].Asset.Version != 3 || request.Audio[0].Role != media.TimelineAudioMusic {
		t.Fatalf("golden fixture provenance changed: %#v", request)
	}
}

func TestCompileEditingTimelineV1M0GoldenFixtureSet(t *testing.T) {
	fixtures := []struct {
		name               string
		video, audio, text int
	}{
		{name: "short-drama-preroll-with-source.json", video: 2, audio: 1, text: 1},
		{name: "single-source-trim.json", video: 1},
		{name: "two-video-caption.json", video: 2, text: 2},
		{name: "voiceover-and-music.json", video: 1, audio: 2, text: 1},
		{name: "music-source-offset-loop.json", video: 2, audio: 1},
		{name: "sfx-over-video.json", video: 1, audio: 1},
		{name: "three-primary-clips.json", video: 3},
		{name: "caption-only-multiple.json", video: 1, text: 3},
		{name: "short-drama-ten-second.json", video: 2, text: 1},
		{name: "long-source-trim.json", video: 1, audio: 1},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join("testdata", "editing-timeline-v1", fixture.name))
			if err != nil {
				t.Fatal(err)
			}
			var timeline EditingTimelineV1
			if err := json.Unmarshal(contents, &timeline); err != nil {
				t.Fatal(err)
			}
			request, err := CompileEditingTimelineV1(timeline, "org_fixture", "project_fixture")
			if err != nil {
				t.Fatal(err)
			}
			if len(request.Video) != fixture.video || len(request.Audio) != fixture.audio || len(request.Captions) != fixture.text {
				t.Fatalf("render request counts = video:%d audio:%d caption:%d, want video:%d audio:%d caption:%d", len(request.Video), len(request.Audio), len(request.Captions), fixture.video, fixture.audio, fixture.text)
			}
		})
	}
}

func BenchmarkCompileEditingTimelineV1ThirtySecondsTwentyClips(b *testing.B) {
	clips := make([]EditingTimelineClip, 20)
	for index := range clips {
		start := index * 1500
		ref := contract.AssetVersionRef{AssetID: contract.AssetID("asset-benchmark"), Version: 1}
		clips[index] = EditingTimelineClip{ID: fmt.Sprintf("clip-%02d", index+1), AssetRef: &ref, TimelineStartMS: start, TimelineEndMS: start + 1500, SourceOutMS: 1500}
	}
	timeline := EditingTimelineV1{SchemaVersion: EditingTimelineSchemaV1, OutputProfile: EditingMVPVerticalOutputProfile, DurationMS: 30000, Tracks: []EditingTimelineTrack{{ID: "video-primary", Role: EditingTrackPrimaryVideo, Clips: clips}}}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := CompileEditingTimelineV1(timeline, "org_benchmark", "project_benchmark"); err != nil {
			b.Fatal(err)
		}
	}
}
