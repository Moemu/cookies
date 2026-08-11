package creative

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/provider"
)

func TestAINativeStoryboardRejectsTimelineGapsAndSyntheticProductIdentity(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := validAINativeStoryboard()
	if err := storyboard.ValidatePlanAgainst(requirement, script); err != nil {
		t.Fatalf("valid storyboard rejected: %v", err)
	}

	storyboard.Shots[1].StartMS++
	if err := storyboard.ValidatePlanAgainst(requirement, script); err == nil {
		t.Fatal("timeline gap must be rejected")
	}

	storyboard = validAINativeStoryboard()
	storyboard.Assets[0].Source = AINativeStoryboardAssetSourceAIGenerated
	if err := storyboard.ValidatePlanAgainst(requirement, script); err == nil {
		t.Fatal("AI-generated media must not replace product identity")
	}

	storyboard = validAINativeStoryboard()
	storyboard.Assets[1].Role = AINativeStoryboardAssetRoleSceneReference
	if err := storyboard.ValidatePlanAgainst(requirement, script); err == nil {
		t.Fatal("storyboard must include a person identity reference")
	}
}

func TestAINativeStoryboardPersistsSuccessfulAssetsBeforeAnotherAssetFails(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := validAINativeStoryboard()
	operationVersion := int64(1)
	workspace := AINativeRequirementWorkspace{
		WorkspaceID: "workspace_1", CreativeIntakeID: "intake_1", CreativeTaskID: "task_1", OrganizationID: "org_1", ProjectID: "project_1",
		Status: AINativeRequirementConfirmedStatus, CurrentStage: AINativeStageStoryboard, WorkspaceVersion: 3,
		ActiveOperationID: "storyboard_operation_1", ActiveOperationVersion: &operationVersion,
		CurrentRevision: requirement.Revision, Requirement: requirement, ScriptStatus: AINativeScriptConfirmedStatus,
		CurrentScriptRevision: &script.Revision, ConfirmedScriptRevision: &script.Revision, Script: &script,
		StoryboardStatus: AINativeStoryboardGeneratingStatus, StoryboardPlan: &storyboard,
		CreatedBy: "user_1", ConfirmedBy: "user_1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	repository := &storyboardProgressRepository{workspace: workspace}
	preparer := &storyboardProgressAssetPreparer{
		refs: map[string]*contract.AssetVersionRef{
			"person_1": {AssetID: "asset_person", Version: 1},
		},
		errs: map[string]error{"scene_1": errors.New("image gateway unavailable")},
	}
	service := Service{Projects: testProjects{}, AINativeStoryboards: repository, AINativeStoryboardPlanner: &storyboardPlannerStub{}, AINativeStoryboardAssetPreparer: preparer}
	operation := AINativeStoryboardOperation{ID: "storyboard_operation_1", Version: 1, WorkspaceID: "workspace_1", ProjectID: "project_1", RequirementRevision: 1, ScriptRevision: 1, ActorID: "user_1"}
	payload, _ := json.Marshal(AINativeStoryboardJobPayload{Operation: operation})

	_, err := service.HandleAINativeStoryboardJob(context.Background(), jobruntime.Claim{Job: contract.Job{Kind: AINativeStoryboardJobKind, OrganizationID: "org_1", ProjectID: "project_1"}, Payload: payload})
	if err == nil {
		t.Fatal("asset failure must keep the storyboard operation unsuccessful")
	}
	assets := repository.workspace.StoryboardPlan.Assets
	if assets[1].Status != AINativeStoryboardAssetReady || assets[1].AssetRef == nil || assets[1].AssetRef.AssetID != "asset_person" {
		t.Fatalf("successful asset was not durably retained: %#v", assets[1])
	}
	if assets[2].Status != AINativeStoryboardAssetFailed {
		t.Fatalf("failed asset status was not retained: %#v", assets[2])
	}
}

func TestRegenerateAINativeStoryboardAssetReplacesOnlySelectedAIAsset(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := validAINativeStoryboard()
	for index := range storyboard.Assets {
		if storyboard.Assets[index].AssetRef == nil {
			storyboard.Assets[index].AssetRef = &contract.AssetVersionRef{AssetID: contract.AssetID("asset_" + storyboard.Assets[index].ID), Version: 1}
		}
		storyboard.Assets[index].Status = AINativeStoryboardAssetReady
		storyboard.Assets[index].GenerationAttempt = 1
	}
	workspace := AINativeRequirementWorkspace{
		WorkspaceID: "workspace_1", CreativeIntakeID: "intake_1", CreativeTaskID: "task_1", OrganizationID: "org_1", ProjectID: "project_1",
		Status: AINativeRequirementConfirmedStatus, CurrentStage: AINativeStageStoryboard, WorkspaceVersion: 7,
		CurrentRevision: requirement.Revision, Requirement: requirement, ScriptStatus: AINativeScriptConfirmedStatus,
		CurrentScriptRevision: &script.Revision, ConfirmedScriptRevision: &script.Revision, Script: &script,
		StoryboardStatus: AINativeStoryboardDraftStatus, CurrentStoryboardRevision: &storyboard.Revision, Storyboard: &storyboard,
		CreatedBy: "user_1", ConfirmedBy: "user_1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	repository := &storyboardProgressRepository{workspace: workspace}
	scheduler := &storyboardRecordingScheduler{}
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	service := Service{Projects: testProjects{}, AINativeStoryboards: repository, AINativeStoryboardAssetPreparer: &storyboardProgressAssetPreparer{}, AINativeStoryboardScheduler: scheduler,
		NewID: func(prefix string) (string, error) { return prefix + "_regenerate", nil }, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}

	updated, err := service.RegenerateAINativeStoryboardAsset(context.Background(), actor, "project_1", "workspace_1", "person_1", RegenerateAINativeStoryboardAssetRequest{
		ExpectedWorkspaceVersion: 7,
		Feedback:                 "  保留通勤场景，人物改为成年女性背影  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.StoryboardStatus != AINativeStoryboardGeneratingStatus || len(scheduler.operations) != 1 {
		t.Fatalf("asset regeneration was not scheduled: workspace=%#v operations=%#v", updated, scheduler.operations)
	}
	assets := updated.StoryboardPlan.Assets
	if assets[1].AssetRef != nil || assets[1].Status != AINativeStoryboardAssetPlanned || assets[1].GenerationAttempt != 2 || assets[1].RegenerationFeedback != "保留通勤场景，人物改为成年女性背影" {
		t.Fatalf("selected asset was not reset for regeneration: %#v", assets[1])
	}
	if assets[0].AssetRef == nil || assets[2].AssetRef == nil || assets[0].GenerationAttempt != 1 || assets[2].GenerationAttempt != 1 {
		t.Fatalf("unselected assets were changed: %#v", assets)
	}
}

func TestRegenerateAINativeStoryboardAssetRejectsOverlongFeedback(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := readyConfirmedAINativeStoryboard()
	storyboard.Status = AINativeStoryboardDraftStatus
	workspace := AINativeRequirementWorkspace{
		WorkspaceID: "workspace_1", OrganizationID: "org_1", ProjectID: "project_1", Status: AINativeRequirementConfirmedStatus,
		CurrentStage: AINativeStageStoryboard, WorkspaceVersion: 7, CurrentRevision: requirement.Revision, Requirement: requirement,
		ScriptStatus: AINativeScriptConfirmedStatus, CurrentScriptRevision: &script.Revision, ConfirmedScriptRevision: &script.Revision, Script: &script,
		StoryboardStatus: AINativeStoryboardDraftStatus, CurrentStoryboardRevision: &storyboard.Revision, Storyboard: &storyboard,
		CreatedBy: "user_1", ConfirmedBy: "user_1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	service := Service{Projects: testProjects{}, AINativeStoryboards: &storyboardProgressRepository{workspace: workspace}, AINativeStoryboardAssetPreparer: &storyboardProgressAssetPreparer{}, AINativeStoryboardScheduler: &storyboardRecordingScheduler{}}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}

	_, err := service.RegenerateAINativeStoryboardAsset(context.Background(), actor, "project_1", "workspace_1", "person_1", RegenerateAINativeStoryboardAssetRequest{
		ExpectedWorkspaceVersion: 7,
		Feedback:                 strings.Repeat("改", 501),
	})
	if !errors.Is(err, ErrInvalidAINativeRequirement) {
		t.Fatalf("overlong feedback error = %v, want ErrInvalidAINativeRequirement", err)
	}
}

type storyboardProgressRepository struct{ workspace AINativeRequirementWorkspace }

func (r *storyboardProgressRepository) GetAINativeStoryboardWorkspace(context.Context, contract.OrganizationID, contract.ProjectID, string) (AINativeRequirementWorkspace, error) {
	return r.workspace, nil
}
func (r *storyboardProgressRepository) BeginAINativeStoryboardGeneration(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, operation AINativeStoryboardOperation, _ time.Time) (AINativeRequirementWorkspace, error) {
	r.workspace.StoryboardStatus = AINativeStoryboardGeneratingStatus
	r.workspace.ActiveOperationID = operation.ID
	r.workspace.ActiveOperationVersion = &operation.Version
	r.workspace.WorkspaceVersion++
	return r.workspace, nil
}
func (r *storyboardProgressRepository) SaveAINativeStoryboardPlan(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, _ AINativeStoryboardOperation, plan AINativeStoryboardRevision, _ time.Time) (AINativeRequirementWorkspace, error) {
	r.workspace.StoryboardPlan = &plan
	return r.workspace, nil
}
func (r *storyboardProgressRepository) CompleteAINativeStoryboardGeneration(context.Context, contract.OrganizationID, contract.ProjectID, string, AINativeStoryboardOperation, AINativeStoryboardRevision, string, time.Time) (AINativeRequirementWorkspace, error) {
	return AINativeRequirementWorkspace{}, errors.New("unexpected completion")
}
func (r *storyboardProgressRepository) FailAINativeStoryboardGeneration(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, _ string, _ int64, code, message string, _ time.Time) error {
	r.workspace.StoryboardStatus = AINativeStoryboardFailedStatus
	r.workspace.StoryboardErrorCode = code
	r.workspace.StoryboardErrorMessage = message
	return nil
}
func (r *storyboardProgressRepository) AppendAINativeStoryboardRevision(context.Context, AINativeRequirementWorkspace, int64, string, time.Time) (AINativeRequirementWorkspace, error) {
	return AINativeRequirementWorkspace{}, errors.New("unused")
}
func (r *storyboardProgressRepository) ConfirmAINativeStoryboard(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, int64, string, time.Time) (AINativeRequirementWorkspace, error) {
	return AINativeRequirementWorkspace{}, errors.New("unused")
}
func (r *storyboardProgressRepository) GetAINativeStoryboardReopenImpact(context.Context, contract.OrganizationID, contract.ProjectID, string) (AINativeReopenImpact, error) {
	return AINativeReopenImpact{}, errors.New("unused")
}
func (r *storyboardProgressRepository) ReopenAINativeStoryboard(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (AINativeRequirementWorkspace, error) {
	return AINativeRequirementWorkspace{}, errors.New("unused")
}

type storyboardProgressAssetPreparer struct {
	refs map[string]*contract.AssetVersionRef
	errs map[string]error
}

type storyboardRecordingScheduler struct{ operations []AINativeStoryboardOperation }

func (s *storyboardRecordingScheduler) ScheduleAINativeStoryboard(_ context.Context, operation AINativeStoryboardOperation) error {
	s.operations = append(s.operations, operation)
	return nil
}

func (p *storyboardProgressAssetPreparer) PrepareAINativeStoryboardAsset(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, _ AINativeStoryboardOperation, asset AINativeStoryboardAsset) (*contract.AssetVersionRef, *time.Time, error) {
	return p.refs[asset.ID], nil, p.errs[asset.ID]
}

type storyboardPlannerStub struct{}

func (*storyboardPlannerStub) Plan(context.Context, contract.ActorContext, contract.ProjectContext, AINativeRequirementDraft, AINativeScriptRevision, ChannelCreativeProfile) (AINativeStoryboardRevision, error) {
	return AINativeStoryboardRevision{}, errors.New("unused")
}

func TestModelAINativeStoryboardPlannerPreservesProductAssetsAndReturnsCompleteShots(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	output, err := json.Marshal(modelAINativeStoryboard{
		Assets: []modelAINativeStoryboardAsset{
			{ID: "person_1", Role: AINativeStoryboardAssetRolePersonIdentity, Name: "通勤女性", GenerationBrief: "自然通勤状态，人物一致"},
			{ID: "scene_1", Role: AINativeStoryboardAssetRoleSceneReference, Name: "地铁入口", GenerationBrief: "清晨城市地铁入口"},
		},
		Shots: []modelAINativeStoryboardShot{
			{ID: "shot_1", StartMS: 0, EndMS: 4000, VisualContent: "通勤包内液体泼洒的痛点", SubjectsProductsActions: "人物查看包内，商品杯保持清晰可辨", ShotSize: "近景", CameraMovement: "快速推近", ReferenceAssetIDs: []string{"product_1", "person_1", "scene_1"}, Voiceover: "通勤包又湿了？", Subtitle: "通勤漏液太麻烦", SoundEffect: "泼水声", BGMDirection: "快节奏起势", Transition: "硬切", ProductIdentityRequired: true},
			{ID: "shot_2", StartMS: 4000, EndMS: 15000, VisualContent: "展示纯钛杯身与随行动作", SubjectsProductsActions: "人物拿起真实商品并放入通勤包", ShotSize: "中近景", CameraMovement: "平稳环绕", ReferenceAssetIDs: []string{"product_1", "person_1"}, Voiceover: "纯钛杯身，轻巧随行。", Subtitle: "纯钛材质", SoundEffect: "轻触声", BGMDirection: "节奏延续", Transition: "动作匹配切", ProductIdentityRequired: true},
			{ID: "shot_3", StartMS: 15000, EndMS: 20000, VisualContent: "商品定格与行动引导", SubjectsProductsActions: "真实商品正面定格", ShotSize: "特写", CameraMovement: "轻微推进", ReferenceAssetIDs: []string{"product_1"}, Voiceover: "点击了解更多。", Subtitle: "点击了解更多", SoundEffect: "提示音", BGMDirection: "收束落点", Transition: "淡出", ProductIdentityRequired: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := &aiNativeTextGeneratorStub{response: provider.SynchronousResponse{ProviderCode: "ark", ModelAlias: "cookies.text.standard", ModelVersion: "seed", StructuredOutput: output}}
	profile, err := NewChannelCreativeProfileRegistry().Resolve("douyin", "performance", "v1")
	if err != nil {
		t.Fatal(err)
	}
	planner := ModelAINativeStoryboardPlanner{Text: text, ModelAlias: "cookies.text.standard"}
	storyboard, err := planner.Plan(context.Background(), contract.ActorContext{}, contract.ProjectContext{}, requirement, script, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(storyboard.Assets) != 3 || storyboard.Assets[0].AssetRef == nil || storyboard.Assets[0].Source != AINativeStoryboardAssetSourceProductImport {
		t.Fatalf("real product asset was not preserved: %#v", storyboard.Assets)
	}
	if storyboard.Assets[1].Status != AINativeStoryboardAssetPlanned || storyboard.Assets[1].GenerationBrief == "" {
		t.Fatalf("missing person asset was not planned: %#v", storyboard.Assets[1])
	}
	if len(storyboard.Shots) != 3 || storyboard.Shots[2].EndMS != 20000 || text.request.OutputJSONSchema == nil {
		t.Fatalf("unexpected storyboard: %#v", storyboard)
	}
	if !strings.Contains(text.request.Messages[0].Content, "每秒不超过 5 个中文字符") {
		t.Fatalf("storyboard prompt does not constrain narration to the shot capacity: %s", text.request.Messages[0].Content)
	}
}

func TestModelAINativeVoiceoverFitterReturnsSuggestionWithinShotCapacity(t *testing.T) {
	output := json.RawMessage(`{"voiceover":"出门拉杆箱总晃还容易卡"}`)
	text := &aiNativeTextGeneratorStub{response: provider.SynchronousResponse{
		ProviderCode: "ark", ModelAlias: "cookies.text.standard", ModelVersion: "seed", StructuredOutput: output,
	}}
	fitter := ModelAINativeVoiceoverFitter{Text: text, ModelAlias: "cookies.text.standard"}

	suggestion, err := fitter.Fit(context.Background(), contract.ActorContext{}, contract.ProjectContext{}, AINativeVoiceoverFitInput{
		ShotID: "shot_001", DurationMS: 3000, ProductName: "途加拉杆箱", Voiceover: "夏天出门拉杆箱总晃不稳还容易卡？",
	})
	if err != nil {
		t.Fatal(err)
	}
	if suggestion.ShotID != "shot_001" || suggestion.OriginalVoiceover != "夏天出门拉杆箱总晃不稳还容易卡？" || suggestion.SuggestedVoiceover != "出门拉杆箱总晃还容易卡" || suggestion.MaxCharacters != 15 {
		t.Fatalf("unexpected voiceover suggestion: %#v", suggestion)
	}
	if !strings.Contains(text.request.Messages[0].Content, "最多 15 个字符") {
		t.Fatalf("voiceover fitter prompt does not contain the shot capacity: %s", text.request.Messages[0].Content)
	}
}

func TestSuggestAINativeVoiceoverFitOnlyTargetsFailedDurationSpeechUnit(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := validAINativeStoryboard()
	storyboard.Status = AINativeStoryboardConfirmedStatus
	failedAt := time.Now()
	workspace := AINativeRequirementWorkspace{
		WorkspaceID: "workspace_1", OrganizationID: "org_1", ProjectID: "project_1", WorkspaceVersion: 9,
		Requirement: requirement, Script: &script, Storyboard: &storyboard,
		ProductionStatus: AINativeProductionFailedStatus,
		ProductionPlan: &AINativeProductionPlan{SpeechUnits: []AINativeSpeechUnit{{
			ID: "speech-unit-01", ShotID: "shot_1", Attempts: []AINativeGenerationAttempt{{
				ID: "attempt_1", Status: AINativeAttemptFailedStatus, ErrorCode: "SPEECH_DURATION_EXCEEDED", ErrorMessage: "voiceover too long", CreatedAt: failedAt, UpdatedAt: failedAt,
			}},
		}}},
	}
	repository := &storyboardProgressRepository{workspace: workspace}
	fitter := &recordingAINativeVoiceoverFitter{suggestion: AINativeVoiceoverFitSuggestion{
		ShotID: "shot_1", OriginalVoiceover: storyboard.Shots[0].Voiceover, SuggestedVoiceover: "通勤箱又湿了", DurationMS: 4000, MaxCharacters: 20,
	}}
	service := Service{Projects: testProjects{}, AINativeStoryboards: repository, AINativeVoiceoverFitter: fitter}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}

	suggestion, err := service.SuggestAINativeVoiceoverFit(context.Background(), actor, "project_1", "workspace_1", SuggestAINativeVoiceoverFitRequest{
		ExpectedWorkspaceVersion: 9, SpeechUnitID: "speech-unit-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if suggestion.SuggestedVoiceover != "通勤箱又湿了" || fitter.input.ShotID != "shot_1" || fitter.input.DurationMS != 4000 {
		t.Fatalf("unexpected fit orchestration: suggestion=%#v input=%#v", suggestion, fitter.input)
	}
}

type recordingAINativeVoiceoverFitter struct {
	input      AINativeVoiceoverFitInput
	suggestion AINativeVoiceoverFitSuggestion
}

func (f *recordingAINativeVoiceoverFitter) Fit(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, input AINativeVoiceoverFitInput) (AINativeVoiceoverFitSuggestion, error) {
	f.input = input
	return f.suggestion, nil
}

func TestModelAINativeStoryboardPlannerKeepsFixedProductWhenGeneratedAssetReusesItsID(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	output, err := json.Marshal(modelAINativeStoryboard{
		Assets: []modelAINativeStoryboardAsset{
			{ID: "product_1", Role: AINativeStoryboardAssetRolePersonIdentity, Name: "通勤女性", GenerationBrief: "自然通勤状态，人物一致"},
			{ID: "scene_1", Role: AINativeStoryboardAssetRoleSceneReference, Name: "地铁入口", GenerationBrief: "清晨城市地铁入口"},
		},
		Shots: []modelAINativeStoryboardShot{
			{ID: "shot_1", StartMS: 0, EndMS: 4000, VisualContent: "通勤痛点", SubjectsProductsActions: "人物背着真实商品快步行走", ShotSize: "中景", CameraMovement: "跟拍", ReferenceAssetIDs: []string{"product_1", "scene_1"}, Voiceover: "通勤装得多。", Subtitle: "轻松通勤", SoundEffect: "脚步声", BGMDirection: "轻快", Transition: "硬切", ProductIdentityRequired: true},
			{ID: "shot_2", StartMS: 4000, EndMS: 15000, VisualContent: "展示商品结构", SubjectsProductsActions: "人物取下真实商品并展示", ShotSize: "中近景", CameraMovement: "环绕", ReferenceAssetIDs: []string{"product_1"}, Voiceover: "容量与舒适兼顾。", Subtitle: "舒适大容量", SoundEffect: "拉链声", BGMDirection: "节奏延续", Transition: "动作切", ProductIdentityRequired: true},
			{ID: "shot_3", StartMS: 15000, EndMS: 20000, VisualContent: "商品定格", SubjectsProductsActions: "真实商品正面定格", ShotSize: "特写", CameraMovement: "轻推", ReferenceAssetIDs: []string{"product_1"}, Voiceover: "点击了解更多。", Subtitle: "点击了解更多", SoundEffect: "提示音", BGMDirection: "收束", Transition: "淡出", ProductIdentityRequired: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := &aiNativeTextGeneratorStub{response: provider.SynchronousResponse{ProviderCode: "ark", ModelAlias: "cookies.text.standard", ModelVersion: "seed", StructuredOutput: output}}
	profile, err := NewChannelCreativeProfileRegistry().Resolve("douyin", "performance", "v1")
	if err != nil {
		t.Fatal(err)
	}

	storyboard, err := (ModelAINativeStoryboardPlanner{Text: text, ModelAlias: "cookies.text.standard"}).Plan(context.Background(), contract.ActorContext{}, contract.ProjectContext{}, requirement, script, profile)
	if err != nil {
		t.Fatal(err)
	}
	assetIDs := make(map[string]bool, len(storyboard.Assets))
	personID := ""
	for _, asset := range storyboard.Assets {
		if assetIDs[asset.ID] {
			t.Fatalf("storyboard contains duplicate asset ID %q", asset.ID)
		}
		assetIDs[asset.ID] = true
		if asset.Role == AINativeStoryboardAssetRolePersonIdentity {
			personID = asset.ID
		}
	}
	if !assetIDs["product_1"] || personID == "" || personID == "product_1" {
		t.Fatalf("fixed product and generated person IDs were not preserved independently: %#v", storyboard.Assets)
	}
	if !containsString(storyboard.Shots[0].ReferenceAssetIDs, "product_1") || !containsString(storyboard.Shots[0].ReferenceAssetIDs, personID) {
		t.Fatalf("colliding reference was not expanded to the fixed product and renamed person: %#v", storyboard.Shots[0].ReferenceAssetIDs)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func validAINativeStoryboardInputs() (AINativeRequirementDraft, AINativeScriptRevision) {
	requirement := validRequirementForScript()
	requirement.Media = []AINativeRequirementMedia{{ID: "media_1", Role: "product", Source: "product_link", AssetRef: &contract.AssetVersionRef{AssetID: "asset_product", Version: 1}}}
	script := validAINativeScript()
	script.Status = AINativeScriptConfirmedStatus
	return requirement, script
}

func validAINativeStoryboard() AINativeStoryboardRevision {
	ref := contract.AssetVersionRef{AssetID: "asset_product", Version: 1}
	return AINativeStoryboardRevision{
		ContractVersion: aiNativeStoryboardContract, Revision: 1, Status: AINativeStoryboardDraftStatus,
		DurationSeconds: 20, BasedOnRequirementRevision: 1, BasedOnRequirementHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		BasedOnScriptRevision: 1, BasedOnScriptHash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		ChannelProfileID: "douyin.performance.v1", ChannelProfileHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Generation: AINativeStoryboardGenerationMetadata{ModelAlias: "cookies.text.standard", ModelVersion: "test", PromptVersion: aiNativeStoryboardPromptVersion, ProfileHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Assets: []AINativeStoryboardAsset{
			{ID: "product_1", Role: AINativeStoryboardAssetRoleProductIdentity, Name: "商品主图", Source: AINativeStoryboardAssetSourceProductImport, AssetRef: &ref, Status: AINativeStoryboardAssetReady},
			{ID: "person_1", Role: AINativeStoryboardAssetRolePersonIdentity, Name: "通勤女性", Source: AINativeStoryboardAssetSourceAIGenerated, GenerationBrief: "自然通勤人物", Status: AINativeStoryboardAssetPlanned},
			{ID: "scene_1", Role: AINativeStoryboardAssetRoleSceneReference, Name: "通勤场景", Source: AINativeStoryboardAssetSourceAIGenerated, GenerationBrief: "真实城市通勤场景", Status: AINativeStoryboardAssetPlanned},
		},
		Shots: []AINativeStoryboardShot{
			{ID: "shot_1", StartMS: 0, EndMS: 4000, DurationMS: 4000, VisualContent: "痛点开场", SubjectsProductsActions: "人物查看包内并展示商品", ShotSize: "近景", CameraMovement: "推近", ReferenceAssetIDs: []string{"product_1", "person_1"}, Voiceover: "通勤包又湿了？", Subtitle: "漏液太麻烦", SoundEffect: "泼水声", BGMDirection: "快节奏", Transition: "硬切", ProductIdentityRequired: true},
			{ID: "shot_2", StartMS: 4000, EndMS: 15000, DurationMS: 11000, VisualContent: "卖点证明", SubjectsProductsActions: "人物展示真实商品", ShotSize: "中近景", CameraMovement: "环绕", ReferenceAssetIDs: []string{"product_1"}, Voiceover: "纯钛杯身，轻巧随行。", Subtitle: "纯钛材质", SoundEffect: "轻触声", BGMDirection: "节奏延续", Transition: "动作切", ProductIdentityRequired: true},
			{ID: "shot_3", StartMS: 15000, EndMS: 20000, DurationMS: 5000, VisualContent: "行动引导", SubjectsProductsActions: "真实商品正面定格", ShotSize: "特写", CameraMovement: "轻推", ReferenceAssetIDs: []string{"product_1"}, Voiceover: "点击了解更多。", Subtitle: "点击了解更多", SoundEffect: "提示音", BGMDirection: "收束", Transition: "淡出", ProductIdentityRequired: true},
		},
	}
}
