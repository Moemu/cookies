package creative

import (
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/media"
)

const EditingTimelineSchemaV1 = "editing-timeline/v1"

type EditingOutputProfile struct {
	ID         string `json:"id"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	FrameRate  int    `json:"frame_rate"`
	SampleRate int    `json:"sample_rate"`
}

var EditingMVPVerticalOutputProfile = EditingOutputProfile{
	ID: "cookies-editing-vertical-v1", Width: 720, Height: 1280, FrameRate: 30, SampleRate: 48000,
}

type EditingTrackRole string

const (
	EditingTrackPrimaryVideo EditingTrackRole = "primary_video"
	EditingTrackCaption      EditingTrackRole = "caption"
	EditingTrackVoiceover    EditingTrackRole = "voiceover"
	EditingTrackMusic        EditingTrackRole = "music"
	EditingTrackSFX          EditingTrackRole = "sfx"
)

type EditingTimelineV1 struct {
	SchemaVersion string                 `json:"schema_version"`
	OutputProfile EditingOutputProfile   `json:"output_profile"`
	DurationMS    int                    `json:"duration_ms"`
	Tracks        []EditingTimelineTrack `json:"tracks"`
}

type EditingTimelineTrack struct {
	ID    string                `json:"id"`
	Role  EditingTrackRole      `json:"role"`
	Clips []EditingTimelineClip `json:"clips"`
}

type EditingTimelineClip struct {
	ID              string                    `json:"id"`
	AssetRef        *contract.AssetVersionRef `json:"asset_ref,omitempty"`
	TimelineStartMS int                       `json:"timeline_start_ms"`
	TimelineEndMS   int                       `json:"timeline_end_ms"`
	SourceInMS      int                       `json:"source_in_ms,omitempty"`
	SourceOutMS     int                       `json:"source_out_ms,omitempty"`
	Text            string                    `json:"text,omitempty"`
	GainDB          float64                   `json:"gain_db,omitempty"`
	Loop            bool                      `json:"loop,omitempty"`
}

// CompileEditingTimelineV1 translates a frozen editing timeline into the
// renderer-neutral request accepted by Media. It deliberately owns no storage,
// authorization, or render-job behavior; those belong to the future EditTask
// module that supplies a previously authorized immutable timeline version.
func CompileEditingTimelineV1(timeline EditingTimelineV1, organizationID contract.OrganizationID, projectID contract.ProjectID) (media.TimelineRenderRequest, error) {
	if organizationID == "" || projectID == "" {
		return media.TimelineRenderRequest{}, fmt.Errorf("editing timeline organization and project are required")
	}
	if err := timeline.Validate(); err != nil {
		return media.TimelineRenderRequest{}, err
	}
	request := media.TimelineRenderRequest{
		OrganizationID: organizationID, ProjectID: projectID, DurationMS: timeline.DurationMS,
		Width: timeline.OutputProfile.Width, Height: timeline.OutputProfile.Height,
		FrameRate: timeline.OutputProfile.FrameRate, SampleRate: timeline.OutputProfile.SampleRate,
	}
	for _, track := range timeline.Tracks {
		for _, clip := range track.Clips {
			switch track.Role {
			case EditingTrackPrimaryVideo:
				request.Video = append(request.Video, media.TimelineVideoClip{ID: clip.ID, Asset: *clip.AssetRef, StartMS: clip.TimelineStartMS, EndMS: clip.TimelineEndMS, SourceIn: clip.SourceInMS})
			case EditingTrackCaption:
				request.Captions = append(request.Captions, media.TimelineCaption{StartMS: clip.TimelineStartMS, EndMS: clip.TimelineEndMS, Text: clip.Text})
			case EditingTrackVoiceover, EditingTrackMusic, EditingTrackSFX:
				request.Audio = append(request.Audio, media.TimelineAudioClip{ID: clip.ID, Role: mediaRoleForEditingTrack(track.Role), Asset: *clip.AssetRef, StartMS: clip.TimelineStartMS, EndMS: clip.TimelineEndMS, SourceIn: clip.SourceInMS, GainDB: clip.GainDB, Loop: clip.Loop})
			}
		}
	}
	if err := request.Validate(); err != nil {
		return media.TimelineRenderRequest{}, err
	}
	return request, nil
}

func (t EditingTimelineV1) Validate() error {
	if t.SchemaVersion != EditingTimelineSchemaV1 {
		return fmt.Errorf("editing timeline schema_version must be %s", EditingTimelineSchemaV1)
	}
	if t.OutputProfile != EditingMVPVerticalOutputProfile {
		return fmt.Errorf("editing timeline output profile must be %s", EditingMVPVerticalOutputProfile.ID)
	}
	if t.DurationMS < 1000 || len(t.Tracks) == 0 {
		return fmt.Errorf("editing timeline duration and tracks are required")
	}
	seenRoles := map[EditingTrackRole]bool{}
	primaryClipCount := 0
	primaryCursor := 0
	for trackIndex, track := range t.Tracks {
		if strings.TrimSpace(track.ID) == "" || !validEditingTrackRole(track.Role) || seenRoles[track.Role] {
			return fmt.Errorf("editing timeline track %d is invalid", trackIndex+1)
		}
		seenRoles[track.Role] = true
		if len(track.Clips) == 0 {
			return fmt.Errorf("editing timeline track %s has no clips", track.ID)
		}
		for clipIndex, clip := range track.Clips {
			if err := validateEditingClip(track.Role, clip, t.DurationMS); err != nil {
				return fmt.Errorf("editing timeline track %s clip %d: %w", track.ID, clipIndex+1, err)
			}
			if track.Role == EditingTrackPrimaryVideo {
				if clip.TimelineStartMS != primaryCursor {
					return fmt.Errorf("primary video timeline must be closed at clip %d", primaryClipCount+1)
				}
				primaryCursor = clip.TimelineEndMS
				primaryClipCount++
			}
		}
	}
	if !seenRoles[EditingTrackPrimaryVideo] || primaryClipCount == 0 || primaryCursor != t.DurationMS {
		return fmt.Errorf("editing timeline primary video must be closed through the master duration")
	}
	return nil
}

func validEditingTrackRole(role EditingTrackRole) bool {
	return role == EditingTrackPrimaryVideo || role == EditingTrackCaption || role == EditingTrackVoiceover || role == EditingTrackMusic || role == EditingTrackSFX
}

func validateEditingClip(role EditingTrackRole, clip EditingTimelineClip, durationMS int) error {
	if strings.TrimSpace(clip.ID) == "" || clip.TimelineStartMS < 0 || clip.TimelineEndMS <= clip.TimelineStartMS || clip.TimelineEndMS > durationMS {
		return fmt.Errorf("timeline range is invalid")
	}
	if role == EditingTrackCaption {
		if clip.AssetRef != nil || strings.TrimSpace(clip.Text) == "" || len([]rune(clip.Text)) > 80 {
			return fmt.Errorf("caption clip is invalid")
		}
		return nil
	}
	if clip.AssetRef == nil || clip.AssetRef.Validate() != nil || clip.SourceInMS < 0 || clip.SourceOutMS <= clip.SourceInMS || clip.SourceOutMS-clip.SourceInMS != clip.TimelineEndMS-clip.TimelineStartMS {
		return fmt.Errorf("media clip asset or source range is invalid")
	}
	if strings.TrimSpace(clip.Text) != "" {
		return fmt.Errorf("media clip cannot contain caption text")
	}
	return nil
}

func mediaRoleForEditingTrack(role EditingTrackRole) media.TimelineAudioRole {
	switch role {
	case EditingTrackVoiceover:
		return media.TimelineAudioVoiceover
	case EditingTrackMusic:
		return media.TimelineAudioMusic
	default:
		return media.TimelineAudioSFX
	}
}
