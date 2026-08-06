package creative

import (
	"context"
	"encoding/json"
	"errors"
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

type storyboardProgressRepository struct{ workspace AINativeRequirementWorkspace }

func (r *storyboardProgressRepository) GetAINativeStoryboardWorkspace(context.Context, contract.OrganizationID, contract.ProjectID, string) (AINativeRequirementWorkspace, error) {
	return r.workspace, nil
}
func (r *storyboardProgressRepository) BeginAINativeStoryboardGeneration(context.Context, contract.OrganizationID, contract.ProjectID, string, AINativeStoryboardOperation, time.Time) (AINativeRequirementWorkspace, error) {
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
