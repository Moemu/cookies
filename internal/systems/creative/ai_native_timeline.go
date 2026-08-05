package creative

import (
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/media"
)

func CompileAINativeTimeline(plan AINativeProductionPlan, storyboard AINativeStoryboardRevision, organizationID contract.OrganizationID, projectID contract.ProjectID) (media.TimelineRenderRequest, error) {
	if plan.Status != AINativeProductionReadyStatus && plan.Status != AINativeProductionRenderingStatus || storyboard.Status != AINativeStoryboardConfirmedStatus || plan.BasedOnStoryboardRevision != storyboard.Revision {
		return media.TimelineRenderRequest{}, ErrInvalidState
	}
	request := media.TimelineRenderRequest{OrganizationID: organizationID, ProjectID: projectID, DurationMS: plan.TotalDurationMS, Width: 720, Height: 1280, FrameRate: 30, SampleRate: 48000,
		Video: []media.TimelineVideoClip{}, Audio: []media.TimelineAudioClip{}, Captions: []media.TimelineCaption{}}
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
	for _, shot := range storyboard.Shots {
		text := strings.TrimSpace(shot.Subtitle)
		if text == "" {
			text = strings.TrimSpace(shot.Voiceover)
		}
		if text != "" {
			request.Captions = append(request.Captions, media.TimelineCaption{StartMS: shot.StartMS, EndMS: shot.EndMS, Text: text})
		}
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
