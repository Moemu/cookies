package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const aiNativeVoiceoverFitPromptVersion = "ai-ad-voiceover-fit/douyin/v1"

type AINativeVoiceoverFitInput struct {
	ShotID      string `json:"shot_id"`
	DurationMS  int    `json:"duration_ms"`
	ProductName string `json:"product_name"`
	Voiceover   string `json:"voiceover"`
}

type AINativeVoiceoverFitSuggestion struct {
	ShotID             string `json:"shot_id"`
	OriginalVoiceover  string `json:"original_voiceover"`
	SuggestedVoiceover string `json:"suggested_voiceover"`
	DurationMS         int    `json:"duration_ms"`
	MaxCharacters      int    `json:"max_characters"`
	PromptVersion      string `json:"prompt_version"`
	ModelAlias         string `json:"model_alias"`
	ModelVersion       string `json:"model_version"`
	RouteRevisionID    string `json:"route_revision_id,omitempty"`
}

type SuggestAINativeVoiceoverFitRequest struct {
	ExpectedWorkspaceVersion int64  `json:"expected_workspace_version"`
	SpeechUnitID             string `json:"speech_unit_id"`
}

type AINativeVoiceoverFitter interface {
	Fit(context.Context, contract.ActorContext, contract.ProjectContext, AINativeVoiceoverFitInput) (AINativeVoiceoverFitSuggestion, error)
}

type ModelAINativeVoiceoverFitter struct {
	Text       AINativeScriptTextGenerator
	ModelAlias string
}

type modelAINativeVoiceoverFit struct {
	Voiceover string `json:"voiceover"`
}

func (s Service) SuggestAINativeVoiceoverFit(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request SuggestAINativeVoiceoverFitRequest) (AINativeVoiceoverFitSuggestion, error) {
	request.SpeechUnitID = strings.TrimSpace(request.SpeechUnitID)
	if s.Projects == nil || s.AINativeStoryboards == nil || s.AINativeVoiceoverFitter == nil || request.ExpectedWorkspaceVersion < 1 || request.SpeechUnitID == "" {
		return AINativeVoiceoverFitSuggestion{}, fmt.Errorf("AI native voiceover fitting dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeVoiceoverFitSuggestion{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return AINativeVoiceoverFitSuggestion{}, err
	}
	workspace, err := s.AINativeStoryboards.GetAINativeStoryboardWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeVoiceoverFitSuggestion{}, err
	}
	if workspace.WorkspaceVersion != request.ExpectedWorkspaceVersion || workspace.ProductionStatus != AINativeProductionFailedStatus || workspace.ProductionPlan == nil || workspace.Storyboard == nil {
		return AINativeVoiceoverFitSuggestion{}, ErrInvalidState
	}
	var failedSpeech *AINativeSpeechUnit
	for index := range workspace.ProductionPlan.SpeechUnits {
		unit := &workspace.ProductionPlan.SpeechUnits[index]
		if unit.ID != request.SpeechUnitID || len(unit.Attempts) == 0 {
			continue
		}
		attempt := unit.Attempts[len(unit.Attempts)-1]
		if attempt.Status == AINativeAttemptFailedStatus && attempt.ErrorCode == "SPEECH_DURATION_EXCEEDED" {
			failedSpeech = unit
		}
		break
	}
	if failedSpeech == nil {
		return AINativeVoiceoverFitSuggestion{}, ErrInvalidState
	}
	var shot *AINativeStoryboardShot
	for index := range workspace.Storyboard.Shots {
		if workspace.Storyboard.Shots[index].ID == failedSpeech.ShotID {
			shot = &workspace.Storyboard.Shots[index]
			break
		}
	}
	if shot == nil {
		return AINativeVoiceoverFitSuggestion{}, ErrInvalidState
	}
	return s.AINativeVoiceoverFitter.Fit(ctx, actor, project, AINativeVoiceoverFitInput{
		ShotID: shot.ID, DurationMS: shot.DurationMS, ProductName: workspace.Requirement.ProductName, Voiceover: shot.Voiceover,
	})
}

var aiNativeVoiceoverFitSchema = json.RawMessage(`{
  "type":"object","additionalProperties":false,"required":["voiceover"],
  "properties":{"voiceover":{"type":"string","minLength":1}}
}`)

func (f ModelAINativeVoiceoverFitter) Fit(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, input AINativeVoiceoverFitInput) (AINativeVoiceoverFitSuggestion, error) {
	input.ShotID = strings.TrimSpace(input.ShotID)
	input.ProductName = strings.TrimSpace(input.ProductName)
	input.Voiceover = strings.TrimSpace(input.Voiceover)
	if f.Text == nil || strings.TrimSpace(f.ModelAlias) == "" || input.ShotID == "" || input.DurationMS < 200 || input.Voiceover == "" {
		return AINativeVoiceoverFitSuggestion{}, fmt.Errorf("AI native voiceover fitter input is invalid")
	}
	maxCharacters := input.DurationMS * 5 / 1000
	if maxCharacters < 1 {
		maxCharacters = 1
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return AINativeVoiceoverFitSuggestion{}, err
	}
	system := fmt.Sprintf("你是抖音效果广告旁白精简编辑。保持商品事实、核心卖点和语气，不新增承诺；把旁白压缩到最多 %d 个字符，标点也计入。句子必须自然完整，只输出 JSON。", maxCharacters)
	response, err := f.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: actor, Project: project, ModelAlias: f.ModelAlias,
		Messages:         []provider.TextMessage{{Role: provider.TextRoleSystem, Content: system}, {Role: provider.TextRoleUser, Content: string(payload)}},
		OutputJSONSchema: aiNativeVoiceoverFitSchema,
	})
	if err != nil {
		return AINativeVoiceoverFitSuggestion{}, err
	}
	raw := response.StructuredOutput
	if len(raw) == 0 {
		raw = json.RawMessage(strings.TrimSpace(response.Text))
	}
	var output modelAINativeVoiceoverFit
	if err := json.Unmarshal(raw, &output); err != nil {
		return AINativeVoiceoverFitSuggestion{}, fmt.Errorf("decode AI native voiceover fit: %w", err)
	}
	suggested := strings.TrimSpace(output.Voiceover)
	if suggested == "" || utf8.RuneCountInString(suggested) > maxCharacters {
		return AINativeVoiceoverFitSuggestion{}, fmt.Errorf("AI native voiceover suggestion exceeds %d characters", maxCharacters)
	}
	return AINativeVoiceoverFitSuggestion{
		ShotID: input.ShotID, OriginalVoiceover: input.Voiceover, SuggestedVoiceover: suggested,
		DurationMS: input.DurationMS, MaxCharacters: maxCharacters, PromptVersion: aiNativeVoiceoverFitPromptVersion,
		ModelAlias: response.ModelAlias, ModelVersion: response.ModelVersion, RouteRevisionID: response.RouteRevisionID,
	}, nil
}
