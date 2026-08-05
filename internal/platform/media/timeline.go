package media

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const TimelineRendererVersion = "ffmpeg-ai-ad-timeline/v1"

type TimelineAudioRole string

const (
	TimelineAudioVoiceover TimelineAudioRole = "voiceover"
	TimelineAudioMusic     TimelineAudioRole = "music"
	TimelineAudioSFX       TimelineAudioRole = "sfx"
)

type TimelineVideoClip struct {
	ID       string
	Asset    contract.AssetVersionRef
	StartMS  int
	EndMS    int
	SourceIn int
}

type TimelineAudioClip struct {
	ID       string
	Role     TimelineAudioRole
	Asset    contract.AssetVersionRef
	StartMS  int
	EndMS    int
	SourceIn int
	GainDB   float64
	Loop     bool
}

type TimelineCaption struct {
	StartMS int
	EndMS   int
	Text    string
}

type TimelineRenderRequest struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	DurationMS     int
	Width          int
	Height         int
	FrameRate      int
	SampleRate     int
	Video          []TimelineVideoClip
	Audio          []TimelineAudioClip
	Captions       []TimelineCaption
}

func (r TimelineRenderRequest) Validate() error {
	if r.OrganizationID == "" || r.ProjectID == "" || r.DurationMS < 1000 || r.Width != 720 || r.Height != 1280 || r.FrameRate != 30 || r.SampleRate != 48000 || len(r.Video) == 0 {
		return fmt.Errorf("timeline scope and Douyin output specification are required")
	}
	video := append([]TimelineVideoClip(nil), r.Video...)
	sort.Slice(video, func(i, j int) bool { return video[i].StartMS < video[j].StartMS })
	cursor := 0
	for index, clip := range video {
		if strings.TrimSpace(clip.ID) == "" || clip.Asset.Validate() != nil || clip.StartMS != cursor || clip.EndMS <= clip.StartMS || clip.EndMS > r.DurationMS || clip.SourceIn < 0 {
			return fmt.Errorf("video timeline must be closed at clip %d", index+1)
		}
		cursor = clip.EndMS
	}
	if cursor != r.DurationMS {
		return fmt.Errorf("video timeline must be closed through the master duration")
	}
	for index, clip := range r.Audio {
		if strings.TrimSpace(clip.ID) == "" || clip.Asset.Validate() != nil || clip.StartMS < 0 || clip.EndMS <= clip.StartMS || clip.EndMS > r.DurationMS || clip.SourceIn < 0 || (clip.Role != TimelineAudioVoiceover && clip.Role != TimelineAudioMusic && clip.Role != TimelineAudioSFX) {
			return fmt.Errorf("audio clip %d is invalid", index+1)
		}
	}
	for index, caption := range r.Captions {
		if caption.StartMS < 0 || caption.EndMS <= caption.StartMS || caption.EndMS > r.DurationMS || strings.TrimSpace(caption.Text) == "" || len([]rune(caption.Text)) > 80 {
			return fmt.Errorf("caption %d is invalid", index+1)
		}
	}
	return nil
}

type TimelineProgress struct {
	Percent   int
	OutTimeMS int
}

type TimelineProgressFunc func(TimelineProgress) error

type TimelineRenderer interface {
	Render(context.Context, TimelineRenderRequest, TimelineProgressFunc) (CompositionOutput, error)
}
