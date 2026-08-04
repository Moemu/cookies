package creative

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type GenerateBrandVoiceClipRequest struct {
	ExpectedRevision int64  `json:"expected_revision"`
	ClipID           string `json:"clip_id"`
	VoiceAlias       string `json:"voice_alias"`
}

func (s Service) GenerateBrandFilmVoiceClip(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, taskID string, request GenerateBrandVoiceClipRequest) (TaskDetail, error) {
	detail, err := s.requireBrandFilmWorkspace(ctx, requestContext.Actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	if s.BrandFilmSpeech == nil || s.AudioAssets == nil || detail.VideoDraft.BrandFilm.Audio == nil {
		return TaskDetail{}, ErrInvalidState
	}
	audio := *detail.VideoDraft.BrandFilm.Audio
	mix := audio.CurrentMix()
	if mix == nil {
		return TaskDetail{}, ErrInvalidState
	}
	clip, err := findBrandVoiceClip(*mix, request.ClipID)
	if err != nil {
		return TaskDetail{}, err
	}
	voiceAlias := strings.TrimSpace(request.VoiceAlias)
	if voiceAlias == "" && len(audio.BlueprintVersions) > 0 {
		voiceAlias = audio.BlueprintVersions[len(audio.BlueprintVersions)-1].VoiceProfile.VoiceAlias
	}
	result, synthErr := s.BrandFilmSpeech.Synthesize(ctx, provider.SpeechSynthesisInput{OrganizationID: requestContext.Actor.OrganizationID, ModelAlias: provider.DefaultMiniMaxSpeechModelAlias, RequestID: taskID + ":" + request.ClipID, Text: clip.Label, VoiceAlias: voiceAlias, Language: "zh-CN", Format: "wav", SampleRate: 32000, SpeakingRate: 1, NeedTimestamps: true})
	now := s.now()
	if synthErr != nil {
		failed := recordBrandVoiceFailure(audio, request.ClipID, synthErr, now)
		return s.persistBrandAudioUpdate(ctx, requestContext.Actor, projectID, taskID, detail, failed, now)
	}
	if len(result.Audio) == 0 || result.DurationMS < 1 {
		return TaskDetail{}, fmt.Errorf("speech provider returned invalid audio")
	}
	identity, err := contract.CanonicalJSONHash(struct {
		TaskID, ClipID, VoiceAlias, ProviderSnapshot, Text string
		MixRevision                                        int64
	}{taskID, request.ClipID, voiceAlias, result.ModelAndVoiceSnapshot, clip.Label, mix.Revision})
	if err != nil {
		return TaskDetail{}, err
	}
	mime := "audio/wav"
	if result.Codec == "mp3" {
		mime = "audio/mpeg"
	}
	mixVersion := mix.Revision
	ref, err := s.AudioAssets.IngestDerivedAudio(ctx, requestContext, projectID, "brand-voice-"+identity[:32], bytes.NewReader(result.Audio), int64(len(result.Audio)), mime, []contract.ResourceRef{{Type: "creative_brand_audio_mix", ID: taskID, Version: &mixVersion}})
	if err != nil {
		return TaskDetail{}, err
	}
	updated, err := applyGeneratedBrandVoice(audio, request.ClipID, voiceAlias, ref.AssetVersion, result, requestContext.Actor.Principal.ID, now)
	if err != nil {
		return TaskDetail{}, err
	}
	return s.persistBrandAudioUpdate(ctx, requestContext.Actor, projectID, taskID, detail, updated, now)
}

func (s Service) ProbeBrandFilmSpeech(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (provider.SpeechCapability, error) {
	if !actor.HasScope(ScopeRead) {
		return provider.SpeechCapability{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return provider.SpeechCapability{}, err
	}
	prober, ok := s.BrandFilmSpeech.(provider.SpeechCapabilityProber)
	if !ok {
		return provider.SpeechCapability{Provider: "fixture", Available: false, ErrorCode: "capability_unavailable", ErrorMessage: "MiniMax speech is not configured; Fixture narration remains active"}, nil
	}
	return prober.ProbeSpeechCapability(ctx, actor.OrganizationID), nil
}

func findBrandVoiceClip(mix AudioMixVersion, clipID string) (AudioClip, error) {
	track := mix.Track(BrandAudioTrackVoiceover)
	if track == nil {
		return AudioClip{}, fmt.Errorf("voiceover track is missing")
	}
	for _, clip := range track.Clips {
		if clip.ID == clipID {
			return clip, nil
		}
	}
	return AudioClip{}, fmt.Errorf("voice clip %s not found", clipID)
}

func applyGeneratedBrandVoice(workspace BrandAudioWorkspace, clipID, voiceAlias string, ref contract.AssetVersionRef, result provider.SpeechSynthesisResult, createdBy string, now time.Time) (BrandAudioWorkspace, error) {
	raw, _ := json.Marshal(workspace)
	var next BrandAudioWorkspace
	if err := json.Unmarshal(raw, &next); err != nil {
		return BrandAudioWorkspace{}, err
	}
	variantIndex := -1
	for index := range next.Variants {
		if next.Variants[index].ID == next.ActiveVariantID {
			variantIndex = index
			break
		}
	}
	current := next.CurrentMix()
	if variantIndex < 0 || current == nil {
		return BrandAudioWorkspace{}, ErrInvalidState
	}
	mixRaw, _ := json.Marshal(current)
	var revised AudioMixVersion
	if err := json.Unmarshal(mixRaw, &revised); err != nil {
		return BrandAudioWorkspace{}, err
	}
	attemptID := fmt.Sprintf("audioattempt_tts_%s_%d", strings.TrimPrefix(clipID, "voice_clip_"), nextAttemptOrdinal(next.Attempts, clipID))
	found := false
	for trackIndex := range revised.Tracks {
		if revised.Tracks[trackIndex].Type != BrandAudioTrackVoiceover {
			continue
		}
		revised.Tracks[trackIndex].RightsStatus = "provider_generated"
		for clipIndex := range revised.Tracks[trackIndex].Clips {
			clip := &revised.Tracks[trackIndex].Clips[clipIndex]
			if clip.ID != clipID {
				continue
			}
			asset := ref
			clip.AssetRef, clip.FixtureURI, clip.GenerationAttemptID, clip.SourceInMS, clip.SourceOutMS = &asset, "", attemptID, 0, result.DurationMS
			clip.WordTimings = make([]AudioWordTiming, 0, len(result.WordTimings))
			for _, timing := range result.WordTimings {
				clip.WordTimings = append(clip.WordTimings, AudioWordTiming{Text: timing.Text, BeginMS: timing.BeginMS, EndMS: timing.EndMS})
			}
			found = true
		}
	}
	if !found {
		return BrandAudioWorkspace{}, fmt.Errorf("voice clip %s not found", clipID)
	}
	revised.ParentRevision, revised.Revision, revised.ID = current.Revision, current.Revision+1, fmt.Sprintf("audio_mix_%02d", current.Revision+1)
	revised.CreatedBy, revised.CreatedAt, revised.Status, revised.ChangeSummary, revised.ContentHash = createdBy, now, "assets_ready", "MiniMax 旁白已生成并替换："+voiceAlias, ""
	hash, err := contract.CanonicalJSONHash(revised)
	if err != nil {
		return BrandAudioWorkspace{}, err
	}
	revised.ContentHash = "sha256:" + hash
	retryOf := lastAudioAttemptID(next.Attempts, clipID)
	next.Attempts = append(next.Attempts, AudioGenerationAttempt{ID: attemptID, ClipID: clipID, Ordinal: nextAttemptOrdinal(next.Attempts, clipID), RetryOf: retryOf, Status: "succeeded", ProviderJobID: result.ProviderRequestID, Provider: "minimax", ProviderSnapshot: result.ModelAndVoiceSnapshot, OutputAssetRef: &ref, FixtureMode: false, CreatedAt: now, UpdatedAt: now})
	next.Variants[variantIndex].MixVersions = append(next.Variants[variantIndex].MixVersions, revised)
	next.Variants[variantIndex].ActiveMixRevision = revised.Revision
	next.ActiveRevision, next.Status, next.UpdatedAt, next.MixedPreview, next.FinalMixedAsset = revised.Revision, "assets_ready", now, nil, nil
	return next, next.Validate()
}

func recordBrandVoiceFailure(workspace BrandAudioWorkspace, clipID string, cause error, now time.Time) BrandAudioWorkspace {
	next := workspace
	ordinal := nextAttemptOrdinal(next.Attempts, clipID)
	code := "tts_failed"
	message := boundedError(cause)
	if value, ok := cause.(provider.SpeechProviderError); ok {
		code, message = value.Code, value.Message
	}
	next.Attempts = append(next.Attempts, AudioGenerationAttempt{ID: fmt.Sprintf("audioattempt_tts_%s_%d", strings.TrimPrefix(clipID, "voice_clip_"), ordinal), ClipID: clipID, Ordinal: ordinal, RetryOf: lastAudioAttemptID(next.Attempts, clipID), Status: "failed", Provider: "minimax", FixtureMode: false, ErrorCode: code, ErrorMessage: message, CreatedAt: now, UpdatedAt: now})
	next.Status, next.UpdatedAt = "tts_fallback", now
	return next
}

func nextAttemptOrdinal(attempts []AudioGenerationAttempt, clipID string) int {
	count := 0
	for _, item := range attempts {
		if item.ClipID == clipID && item.Ordinal > count {
			count = item.Ordinal
		}
	}
	return count + 1
}
func lastAudioAttemptID(attempts []AudioGenerationAttempt, clipID string) string {
	result := ""
	ordinal := 0
	for _, item := range attempts {
		if item.ClipID == clipID && item.Ordinal >= ordinal {
			result, ordinal = item.ID, item.Ordinal
		}
	}
	return result
}

func (s Service) persistBrandAudioUpdate(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, detail TaskDetail, audio BrandAudioWorkspace, now time.Time) (TaskDetail, error) {
	next := cloneBrandVideoDraft(*detail.VideoDraft)
	next.Revision++
	next.BrandFilm.Revision, next.BrandFilm.Audio, next.BrandFilm.UpdatedAt = next.Revision, &audio, now
	return s.persistBrandFilmDraft(ctx, actor, projectID, taskID, *detail.VideoDraft, next)
}
