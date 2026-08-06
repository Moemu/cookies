package media

import (
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestTimelineRenderRequestAcceptsClosedVerticalTimeline(t *testing.T) {
	request := TimelineRenderRequest{
		OrganizationID: "org_1", ProjectID: "project_1", DurationMS: 20000,
		Width: 720, Height: 1280, FrameRate: 30, SampleRate: 48000,
		Video: []TimelineVideoClip{
			{ID: "v1", Asset: contract.AssetVersionRef{AssetID: "video_1", Version: 1}, StartMS: 0, EndMS: 4000},
			{ID: "v2", Asset: contract.AssetVersionRef{AssetID: "video_2", Version: 1}, StartMS: 4000, EndMS: 20000},
		},
		Audio:    []TimelineAudioClip{{ID: "voice_1", Role: TimelineAudioVoiceover, Asset: contract.AssetVersionRef{AssetID: "audio_1", Version: 1}, StartMS: 0, EndMS: 3500}},
		Captions: []TimelineCaption{{StartMS: 0, EndMS: 3500, Text: "通勤背包，容量真的够吗？"}},
	}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	request.Video[1].StartMS = 4500
	if err := request.Validate(); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("timeline gap should be rejected: %v", err)
	}
}

func TestASSSubtitlesEscapeContentAndUseVerticalSafeArea(t *testing.T) {
	contents, err := BuildASSSubtitles([]TimelineCaption{{StartMS: 1200, EndMS: 3600, Text: "轻便 {耐磨}\\防泼水\n适合通勤"}}, 720, 1280)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, expected := range []string{"PlayResX: 720", "PlayResY: 1280", ",70,70,80,1", "0:00:01.20,0:00:03.60", `\{耐磨\}`, `\\防泼水`, `\N适合通勤`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("ASS output missing %q:\n%s", expected, text)
		}
	}
}

func TestBuildTimelineFilterIncludesDuckingMixAndSubtitleBurnIn(t *testing.T) {
	request := TimelineRenderRequest{DurationMS: 15000, Width: 720, Height: 1280, FrameRate: 30, SampleRate: 48000,
		Video: []TimelineVideoClip{{ID: "v1", StartMS: 0, EndMS: 15000}},
		Audio: []TimelineAudioClip{
			{ID: "voice", Role: TimelineAudioVoiceover, StartMS: 0, EndMS: 5000, GainDB: 0},
			{ID: "music", Role: TimelineAudioMusic, StartMS: 0, EndMS: 15000, GainDB: -12, Loop: true},
			{ID: "sfx", Role: TimelineAudioSFX, StartMS: 4500, EndMS: 5200, GainDB: -3},
		}}
	graph, videoLabel, audioLabel := BuildTimelineFilter(request, "subtitles.ass")
	for _, expected := range []string{"concat=n=1:v=1:a=0", "subtitles=", "sidechaincompress", "amix=inputs=4", "loudnorm", "alimiter"} {
		if !strings.Contains(graph, expected) {
			t.Fatalf("filter graph missing %q: %s", expected, graph)
		}
	}
	if videoLabel != "[videoout]" || audioLabel != "[audioout]" {
		t.Fatalf("unexpected output labels: %s %s", videoLabel, audioLabel)
	}
}

func TestBuildTimelineFilterUsesAudioSourceInBeforeLooping(t *testing.T) {
	request := TimelineRenderRequest{DurationMS: 15000, Width: 720, Height: 1280, FrameRate: 30, SampleRate: 48000,
		Video: []TimelineVideoClip{{ID: "v1", Asset: contract.AssetVersionRef{AssetID: "video", Version: 1}, StartMS: 0, EndMS: 15000}},
		Audio: []TimelineAudioClip{{ID: "music", Role: TimelineAudioMusic, Asset: contract.AssetVersionRef{AssetID: "music", Version: 1}, StartMS: 0, EndMS: 15000, SourceIn: 1200, Loop: true}},
	}
	graph, _, _ := BuildTimelineFilter(request, "subtitles.ass")
	if !strings.Contains(graph, "atrim=start=1.2") {
		t.Fatalf("looping music must honor source_in before loop: %s", graph)
	}
}

func TestTimelineRenderProfileAcceptsFrozen15_20And30SecondDurations(t *testing.T) {
	for _, duration := range []int{15000, 20000, 30000} {
		request := TimelineRenderRequest{OrganizationID: "org_1", ProjectID: "project_1", DurationMS: duration, Width: 720, Height: 1280, FrameRate: 30, SampleRate: 48000,
			Video: []TimelineVideoClip{{ID: "video", Asset: contract.AssetVersionRef{AssetID: "video", Version: 1}, StartMS: 0, EndMS: duration}}}
		if err := request.Validate(); err != nil {
			t.Fatalf("%dms fixture rejected: %v", duration, err)
		}
	}
}
