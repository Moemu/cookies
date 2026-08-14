package creative

import (
	"fmt"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/media"
)

func CompileAINativeTimeline(plan AINativeProductionPlan, storyboard AINativeStoryboardRevision, organizationID contract.OrganizationID, projectID contract.ProjectID) (media.TimelineRenderRequest, error) {
	if plan.Status != AINativeProductionReadyStatus && plan.Status != AINativeProductionRenderingStatus || storyboard.Status != AINativeStoryboardConfirmedStatus || plan.BasedOnStoryboardRevision != storyboard.Revision {
		return media.TimelineRenderRequest{}, ErrInvalidState
	}
	preset := plan.OutputPreset
	if err := preset.Validate(); err != nil {
		// Production plans created before the v2 preset snapshot are restored as
		// the only legacy specification that existed at that time.
		if plan.AspectRatio != "9:16" {
			return media.TimelineRenderRequest{}, fmt.Errorf("production plan has no valid output preset")
		}
		preset = DefaultAINativeOutputPreset()
		preset.ProfileID, preset.ProfileHash = plan.ChannelProfileID, plan.ChannelProfileHash
	}
	if plan.AspectRatio != preset.AspectRatio || plan.ChannelProfileID != preset.ProfileID || plan.ChannelProfileHash != preset.ProfileHash {
		return media.TimelineRenderRequest{}, fmt.Errorf("production plan conflicts with its frozen output preset")
	}
	request := media.TimelineRenderRequest{OrganizationID: organizationID, ProjectID: projectID, DurationMS: plan.TotalDurationMS, Width: preset.Width, Height: preset.Height, FrameRate: 30, SampleRate: 48000,
		Video: []media.TimelineVideoClip{}, Audio: []media.TimelineAudioClip{}, Captions: []media.TimelineCaption{}, Overlays: []media.TimelineOverlay{}, OmitAudio: plan.DeliveryTreatment.Preset == AINativeDeliveryPresetCleanMaterial}
	for _, unit := range plan.Units {
		ref, ok := selectedAttemptAsset(unit.Attempts, unit.SelectedAttemptID)
		if !ok {
			return media.TimelineRenderRequest{}, fmt.Errorf("video unit %s has no selected Asset", unit.ID)
		}
		request.Video = append(request.Video, media.TimelineVideoClip{ID: unit.ID, Asset: ref, StartMS: unit.StartMS, EndMS: unit.EndMS})
	}
	for _, unit := range plan.SpeechUnits {
		ref, ok := selectedAttemptAsset(unit.Attempts, unit.SelectedAttemptID)
		if !ok {
			return media.TimelineRenderRequest{}, fmt.Errorf("speech unit %s has no selected Asset", unit.ID)
		}
		endMS := unit.StartMS + unit.DurationMS
		if unit.DurationMS <= 0 || endMS > unit.EndMS {
			endMS = unit.EndMS
		}
		request.Audio = append(request.Audio, media.TimelineAudioClip{ID: unit.ID, Role: media.TimelineAudioVoiceover, Asset: ref, StartMS: unit.StartMS, EndMS: endMS, GainDB: 0})
	}
	for _, cue := range plan.CaptionCues {
		request.Captions = append(request.Captions, media.TimelineCaption{StartMS: cue.StartMS, EndMS: cue.EndMS, Text: cue.Text})
	}
	for _, cue := range plan.SalesOverlayCues {
		request.Overlays = append(request.Overlays, media.TimelineOverlay{StartMS: cue.StartMS, EndMS: cue.EndMS, Text: cue.Text, Kind: cue.Kind})
	}
	for _, cue := range plan.AudioCues {
		if cue.AssetRef == nil {
			continue
		}
		role := media.TimelineAudioSFX
		if cue.Role == "music" {
			role = media.TimelineAudioMusic
		}
		request.Audio = append(request.Audio, media.TimelineAudioClip{ID: "audio-" + cue.Role + "-" + cue.ShotID, Role: role, Asset: *cue.AssetRef, StartMS: cue.StartMS, EndMS: cue.EndMS, GainDB: -12, FadeInMS: 200, FadeOutMS: 300, Loop: cue.Role == "music"})
	}
	if err := request.Validate(); err != nil {
		return media.TimelineRenderRequest{}, err
	}
	return request, nil
}

func selectedAttemptAsset(attempts []AINativeGenerationAttempt, selectedID string) (contract.AssetVersionRef, bool) {
	for _, attempt := range attempts {
		if attempt.ID == selectedID && attempt.Status == AINativeAttemptSucceededStatus && attempt.OutputAssetRef != nil && attempt.OutputAssetRef.Validate() == nil {
			return *attempt.OutputAssetRef, true
		}
	}
	return contract.AssetVersionRef{}, false
}
