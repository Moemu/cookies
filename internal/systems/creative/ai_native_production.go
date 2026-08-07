package creative

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const (
	aiNativeProductionPlanContract = "creative.ai-native.production-plan/v1"

	AINativeStageProduction              = "production"
	AINativeProductionPreparedStatus     = "prepared"
	AINativeProductionRunningStatus      = "running"
	AINativeProductionReadyStatus        = "assets_ready"
	AINativeProductionRenderingStatus    = "rendering"
	AINativeProductionCompletedStatus    = "completed"
	AINativeProductionRenderFailedStatus = "render_failed"
	AINativeProductionFailedStatus       = "failed"
	AINativeProductionCancelledStatus    = "cancelled"

	AINativeAttemptPlannedStatus   = "planned"
	AINativeAttemptSubmittedStatus = "submitted"
	AINativeAttemptRunningStatus   = "running"
	AINativeAttemptIngestingStatus = "ingesting"
	AINativeAttemptSucceededStatus = "succeeded"
	AINativeAttemptFailedStatus    = "failed"
	AINativeAttemptCancelledStatus = "cancelled"
)

type AINativeGenerationAttempt struct {
	ID             string                    `json:"id"`
	Ordinal        int                       `json:"ordinal"`
	RetryOf        string                    `json:"retry_of,omitempty"`
	Status         string                    `json:"status"`
	ProviderJobID  string                    `json:"provider_job_id,omitempty"`
	OutputAssetRef *contract.AssetVersionRef `json:"output_asset_ref,omitempty"`
	ErrorCode      string                    `json:"error_code,omitempty"`
	ErrorMessage   string                    `json:"error_message,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

type AINativeGenerationUnit struct {
	ID                      string                      `json:"id"`
	Order                   int                         `json:"order"`
	ShotIDs                 []string                    `json:"shot_ids"`
	StartMS                 int                         `json:"start_ms"`
	EndMS                   int                         `json:"end_ms"`
	DurationSeconds         int                         `json:"duration_seconds"`
	Prompt                  string                      `json:"prompt"`
	PromptHash              string                      `json:"prompt_hash"`
	AspectRatio             string                      `json:"aspect_ratio"`
	Resolution              string                      `json:"resolution"`
	ReferenceAsset          *contract.AssetVersionRef   `json:"reference_asset,omitempty"`
	ReferenceRole           string                      `json:"reference_role,omitempty"`
	ProductIdentityRequired bool                        `json:"product_identity_required"`
	Attempts                []AINativeGenerationAttempt `json:"attempts"`
	SelectedAttemptID       string                      `json:"selected_attempt_id,omitempty"`
}

func (u AINativeGenerationUnit) ProviderInput(projectID contract.ProjectID) (provider.VideoGenerationInput, error) {
	input := provider.VideoGenerationInput{Prompt: u.Prompt, DurationSeconds: u.DurationSeconds, AspectRatio: u.AspectRatio, Resolution: u.Resolution, AudioPolicy: provider.VideoAudioSilent, InputMode: provider.VideoInputTextOnly}
	if u.ReferenceAsset != nil {
		input.InputMode = provider.VideoInputReferenceImage
		input.ConditioningAssets = []provider.VideoConditioningAsset{{Role: provider.VideoConditioningReferenceImage, Reference: contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: *u.ReferenceAsset}}}
	}
	return input, input.Validate()
}

type AINativeSpeechUnit struct {
	ID                string                      `json:"id"`
	Order             int                         `json:"order"`
	ShotID            string                      `json:"shot_id"`
	StartMS           int                         `json:"start_ms"`
	EndMS             int                         `json:"end_ms"`
	Text              string                      `json:"text"`
	Language          string                      `json:"language"`
	VoiceAlias        string                      `json:"voice_alias"`
	SpeakingRate      float64                     `json:"speaking_rate,omitempty"`
	NormalizedText    string                      `json:"normalized_text,omitempty"`
	AudioCodec        string                      `json:"audio_codec,omitempty"`
	SampleRate        int                         `json:"sample_rate,omitempty"`
	DurationMS        int                         `json:"duration_ms,omitempty"`
	WordTimings       []provider.SpeechWordTiming `json:"word_timings,omitempty"`
	ProviderSnapshot  string                      `json:"provider_snapshot,omitempty"`
	Attempts          []AINativeGenerationAttempt `json:"attempts"`
	SelectedAttemptID string                      `json:"selected_attempt_id,omitempty"`
}

type AINativeProductionPlan struct {
	ContractVersion           string                   `json:"contract_version"`
	Revision                  int64                    `json:"revision"`
	Status                    string                   `json:"status"`
	BasedOnStoryboardRevision int64                    `json:"based_on_storyboard_revision"`
	BasedOnStoryboardHash     string                   `json:"based_on_storyboard_hash"`
	ChannelProfileID          string                   `json:"channel_profile_id"`
	ChannelProfileHash        string                   `json:"channel_profile_hash"`
	TotalDurationMS           int                      `json:"total_duration_ms"`
	AspectRatio               string                   `json:"aspect_ratio"`
	VideoModelAlias           string                   `json:"video_model_alias"`
	SpeechModelAlias          string                   `json:"speech_model_alias"`
	Units                     []AINativeGenerationUnit `json:"units"`
	SpeechUnits               []AINativeSpeechUnit     `json:"speech_units"`
	Render                    *AINativeRenderState     `json:"render,omitempty"`
	CreatedAt                 time.Time                `json:"created_at"`
	UpdatedAt                 time.Time                `json:"updated_at"`
}

type AINativeRenderState struct {
	ID              string                    `json:"id"`
	Status          string                    `json:"status"`
	ProgressPercent int                       `json:"progress_percent"`
	ETASeconds      int                       `json:"eta_seconds"`
	RendererVersion string                    `json:"renderer_version"`
	OutputAssetRef  *contract.AssetVersionRef `json:"output_asset_ref,omitempty"`
	ErrorCode       string                    `json:"error_code,omitempty"`
	ErrorMessage    string                    `json:"error_message,omitempty"`
	StartedAt       time.Time                 `json:"started_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

type AINativeProductionProgress struct {
	Status                   string   `json:"status"`
	ProgressPercent          int      `json:"progress_percent"`
	CurrentStep              string   `json:"current_step"`
	CompletedVideoUnits      int      `json:"completed_video_units"`
	TotalVideoUnits          int      `json:"total_video_units"`
	CompletedVideoDurationMS int      `json:"completed_video_duration_ms"`
	CompletedSpeechUnits     int      `json:"completed_speech_units"`
	TotalSpeechUnits         int      `json:"total_speech_units"`
	ETASeconds               int      `json:"eta_seconds"`
	AvailableActions         []string `json:"available_actions"`
}

func CompileAINativeProductionPlan(requirement AINativeRequirementDraft, storyboard AINativeStoryboardRevision, projectID contract.ProjectID, now time.Time) (AINativeProductionPlan, error) {
	if storyboard.Status != AINativeStoryboardConfirmedStatus || projectID == "" || storyboard.DurationSeconds != requirement.DurationSeconds {
		return AINativeProductionPlan{}, ErrInvalidState
	}
	assetByID := make(map[string]AINativeStoryboardAsset, len(storyboard.Assets))
	for _, asset := range storyboard.Assets {
		if asset.Status != AINativeStoryboardAssetReady || asset.AssetRef == nil || asset.AssetRef.Validate() != nil {
			return AINativeProductionPlan{}, fmt.Errorf("storyboard asset %s is not ready", asset.ID)
		}
		assetByID[asset.ID] = asset
	}
	storyboardHash, err := contract.CanonicalJSONHash(storyboard)
	if err != nil {
		return AINativeProductionPlan{}, err
	}
	plan := AINativeProductionPlan{ContractVersion: aiNativeProductionPlanContract, Revision: 1, Status: AINativeProductionPreparedStatus,
		BasedOnStoryboardRevision: storyboard.Revision, BasedOnStoryboardHash: storyboardHash, ChannelProfileID: storyboard.ChannelProfileID,
		ChannelProfileHash: storyboard.ChannelProfileHash, TotalDurationMS: requirement.DurationSeconds * 1000, AspectRatio: requirement.AspectRatio,
		VideoModelAlias: "cookies.video.standard", SpeechModelAlias: "cookies.speech.standard", Units: []AINativeGenerationUnit{}, SpeechUnits: []AINativeSpeechUnit{}, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
	unitOrder := 0
	usedNonProductReferences := map[string]struct{}{}
	for shotIndex, shot := range storyboard.Shots {
		cursor := shot.StartMS
		for cursor < shot.EndMS {
			remaining := shot.EndMS - cursor
			durationMS := remaining
			if durationMS > 6000 {
				durationMS = 6000
				if remaining-durationMS > 0 && remaining-durationMS < 4000 {
					durationMS = remaining - 4000
				}
			}
			providerDuration := (durationMS + 999) / 1000
			if providerDuration < 4 {
				providerDuration = 4
			}
			unitOrder++
			unit := AINativeGenerationUnit{ID: fmt.Sprintf("video-unit-%02d", unitOrder), Order: unitOrder, ShotIDs: []string{shot.ID}, StartMS: cursor, EndMS: cursor + durationMS,
				DurationSeconds: providerDuration, Prompt: compileAINativeVideoUnitPrompt(requirement, shot, cursor-shot.StartMS, durationMS), AspectRatio: requirement.AspectRatio, Resolution: "720p",
				ProductIdentityRequired: shot.ProductIdentityRequired, Attempts: []AINativeGenerationAttempt{}}
			selectedReferenceID := ""
			selectedReferencePriority := 0
			for _, refID := range shot.ReferenceAssetIDs {
				asset, ok := assetByID[refID]
				if !ok {
					return AINativeProductionPlan{}, fmt.Errorf("shot %s references missing asset %s", shot.ID, refID)
				}
				if shot.ProductIdentityRequired && asset.Role == AINativeStoryboardAssetRoleProductIdentity {
					ref := *asset.AssetRef
					unit.ReferenceAsset, unit.ReferenceRole = &ref, asset.Role
					break
				}
				if shot.ProductIdentityRequired {
					continue
				}
				priority := 0
				switch asset.Role {
				case AINativeStoryboardAssetRoleCompositionReference:
					priority = 3
				case AINativeStoryboardAssetRoleSceneReference:
					priority = 2
				case AINativeStoryboardAssetRoleProductIdentity:
					priority = 1
				default:
					continue
				}
				if asset.Role != AINativeStoryboardAssetRoleProductIdentity {
					if _, reused := usedNonProductReferences[string(asset.AssetRef.AssetID)]; reused {
						continue
					}
				}
				if priority > selectedReferencePriority {
					ref := *asset.AssetRef
					unit.ReferenceAsset, unit.ReferenceRole = &ref, asset.Role
					selectedReferenceID, selectedReferencePriority = string(asset.AssetRef.AssetID), priority
				}
			}
			if selectedReferenceID != "" && unit.ReferenceRole != AINativeStoryboardAssetRoleProductIdentity {
				usedNonProductReferences[selectedReferenceID] = struct{}{}
			}
			if shot.ProductIdentityRequired && unit.ReferenceRole != AINativeStoryboardAssetRoleProductIdentity {
				return AINativeProductionPlan{}, fmt.Errorf("shot %s requires a real product identity asset", shot.ID)
			}
			unit.PromptHash, err = contract.CanonicalJSONHash(struct {
				ID, Prompt     string
				StartMS, EndMS int
			}{unit.ID, unit.Prompt, unit.StartMS, unit.EndMS})
			if err != nil {
				return AINativeProductionPlan{}, err
			}
			if _, err := unit.ProviderInput(projectID); err != nil {
				return AINativeProductionPlan{}, err
			}
			plan.Units = append(plan.Units, unit)
			cursor += durationMS
		}
		if strings.TrimSpace(shot.Voiceover) != "" {
			plan.SpeechUnits = append(plan.SpeechUnits, AINativeSpeechUnit{ID: fmt.Sprintf("speech-unit-%02d", len(plan.SpeechUnits)+1), Order: len(plan.SpeechUnits) + 1,
				ShotID: shot.ID, StartMS: shot.StartMS, EndMS: shot.EndMS, Text: strings.TrimSpace(shot.Voiceover), Language: requirement.Language, VoiceAlias: "cookies.voice.douyin.default", WordTimings: []provider.SpeechWordTiming{}, Attempts: []AINativeGenerationAttempt{}})
		}
		_ = shotIndex
	}
	return plan, nil
}

func compileAINativeVideoUnitPrompt(requirement AINativeRequirementDraft, shot AINativeStoryboardShot, offsetMS, durationMS int) string {
	return strings.Join([]string{
		"Create a vertical Douyin performance-ad video segment without audio or embedded text.",
		fmt.Sprintf("Product: %s. Preserve the exact product appearance from the reference image.", requirement.ProductName),
		fmt.Sprintf("Narrative shot %s, source interval offset %dms, target material %dms.", shot.ID, offsetMS, durationMS),
		"Visual: " + shot.VisualContent,
		"Subjects, product and actions: " + shot.SubjectsProductsActions,
		"Framing and camera: " + shot.ShotSize + "; " + shot.CameraMovement,
		"Transition intent: " + shot.Transition,
		"No captions, subtitles, BGM, voiceover, watermark, invented logo or packaging text.",
	}, "\n")
}

func (p AINativeProductionPlan) Progress(now time.Time) AINativeProductionProgress {
	result := AINativeProductionProgress{Status: p.Status, TotalVideoUnits: len(p.Units), TotalSpeechUnits: len(p.SpeechUnits), AvailableActions: []string{"cancel_production"}}
	for _, unit := range p.Units {
		if selectedAttemptSucceeded(unit.Attempts, unit.SelectedAttemptID) {
			result.CompletedVideoUnits++
			result.CompletedVideoDurationMS += unit.EndMS - unit.StartMS
		}
	}
	for _, unit := range p.SpeechUnits {
		if selectedAttemptSucceeded(unit.Attempts, unit.SelectedAttemptID) {
			result.CompletedSpeechUnits++
		}
	}
	progress := 5
	if p.TotalDurationMS > 0 {
		progress += 55 * result.CompletedVideoDurationMS / p.TotalDurationMS
	}
	if len(p.SpeechUnits) > 0 {
		progress += 10 * result.CompletedSpeechUnits / len(p.SpeechUnits)
	}
	result.ProgressPercent = progress
	result.CurrentStep = "正在生成视频片段"
	if result.CompletedVideoUnits == len(p.Units) {
		result.CurrentStep = "正在生成旁白"
	}
	if p.Status == AINativeProductionReadyStatus {
		result.ProgressPercent, result.CurrentStep, result.AvailableActions = 70, "视频片段与旁白已就绪", []string{}
	} else if p.Status == AINativeProductionRenderingStatus && p.Render != nil {
		result.ProgressPercent = 70 + p.Render.ProgressPercent*20/100
		result.CurrentStep, result.ETASeconds, result.AvailableActions = "正在合成字幕、旁白与最终视频", p.Render.ETASeconds, []string{"cancel_production"}
	} else if p.Status == AINativeProductionCompletedStatus && p.Render != nil {
		result.ProgressPercent, result.CurrentStep, result.ETASeconds, result.AvailableActions = 100, "最终广告视频已生成", 0, []string{"preview", "download"}
	} else if p.Status == AINativeProductionRenderFailedStatus {
		result.ProgressPercent, result.CurrentStep, result.ETASeconds, result.AvailableActions = 70, "最终视频渲染失败", 0, []string{"retry_render"}
	} else if p.Status == AINativeProductionFailedStatus {
		result.CurrentStep, result.AvailableActions = "部分视频片段或旁白生成失败", []string{"retry_failed_unit"}
	} else if p.Status == AINativeProductionCancelledStatus {
		result.CurrentStep, result.AvailableActions = "视频生产已取消", []string{}
	}
	if p.Status == AINativeProductionRunningStatus {
		remainingVideoMS := p.TotalDurationMS - result.CompletedVideoDurationMS
		remainingSpeech := len(p.SpeechUnits) - result.CompletedSpeechUnits
		result.ETASeconds = remainingVideoMS/1000*8 + remainingSpeech*20
	}
	_ = now
	return result
}

func selectedAttemptSucceeded(attempts []AINativeGenerationAttempt, selectedID string) bool {
	for _, attempt := range attempts {
		if attempt.ID == selectedID && attempt.Status == AINativeAttemptSucceededStatus && attempt.OutputAssetRef != nil {
			return true
		}
	}
	return false
}

func (p AINativeProductionPlan) RetryUnit(unitID, attemptID string, now time.Time) (AINativeProductionPlan, error) {
	for index := range p.Units {
		unit := &p.Units[index]
		if unit.ID != unitID {
			continue
		}
		if selectedAttemptSucceeded(unit.Attempts, unit.SelectedAttemptID) || len(unit.Attempts) == 0 || unit.Attempts[len(unit.Attempts)-1].Status != AINativeAttemptFailedStatus {
			return AINativeProductionPlan{}, ErrInvalidState
		}
		failedAttempt := unit.Attempts[len(unit.Attempts)-1]
		if strings.HasPrefix(failedAttempt.ErrorCode, "InputImageSensitiveContentDetected") && !unit.ProductIdentityRequired && unit.ReferenceRole == AINativeStoryboardAssetRolePersonIdentity {
			unit.ReferenceAsset, unit.ReferenceRole = nil, ""
		}
		retryOf := failedAttempt.ID
		unit.Attempts = append(unit.Attempts, AINativeGenerationAttempt{ID: attemptID, Ordinal: len(unit.Attempts) + 1, RetryOf: retryOf, Status: AINativeAttemptPlannedStatus, CreatedAt: now.UTC(), UpdatedAt: now.UTC()})
		p.Status, p.UpdatedAt = AINativeProductionRunningStatus, now.UTC()
		return p, nil
	}
	for index := range p.SpeechUnits {
		unit := &p.SpeechUnits[index]
		if unit.ID != unitID {
			continue
		}
		if selectedAttemptSucceeded(unit.Attempts, unit.SelectedAttemptID) || len(unit.Attempts) == 0 || unit.Attempts[len(unit.Attempts)-1].Status != AINativeAttemptFailedStatus {
			return AINativeProductionPlan{}, ErrInvalidState
		}
		retryOf := unit.Attempts[len(unit.Attempts)-1].ID
		unit.Attempts = append(unit.Attempts, AINativeGenerationAttempt{ID: attemptID, Ordinal: len(unit.Attempts) + 1, RetryOf: retryOf, Status: AINativeAttemptPlannedStatus, CreatedAt: now.UTC(), UpdatedAt: now.UTC()})
		p.Status, p.UpdatedAt = AINativeProductionRunningStatus, now.UTC()
		return p, nil
	}
	return AINativeProductionPlan{}, ErrNotFound
}
