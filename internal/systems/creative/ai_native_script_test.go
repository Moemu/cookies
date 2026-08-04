package creative

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type memoryAINativeScriptRepository struct {
	workspace AINativeRequirementWorkspace
	scripts   map[int64]AINativeScriptRevision
}

func (r *memoryAINativeScriptRepository) GetAINativeScriptWorkspace(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string) (AINativeRequirementWorkspace, error) {
	if r.workspace.OrganizationID != organizationID || r.workspace.ProjectID != projectID || r.workspace.WorkspaceID != workspaceID {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	return r.workspace, nil
}

func (r *memoryAINativeScriptRepository) BeginAINativeScriptGeneration(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, operation AINativeScriptOperation, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.workspace.OrganizationID != organizationID || r.workspace.ProjectID != projectID || r.workspace.WorkspaceID != workspaceID {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	if r.workspace.WorkspaceVersion != operation.ExpectedWorkspaceVersion || r.workspace.Status != AINativeRequirementConfirmedStatus {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	r.workspace.CurrentStage = AINativeStageScript
	r.workspace.ScriptStatus = AINativeScriptGeneratingStatus
	r.workspace.ActiveOperationID = operation.ID
	r.workspace.ActiveOperationVersion = &operation.Version
	r.workspace.WorkspaceVersion++
	r.workspace.UpdatedAt = now
	return r.workspace, nil
}

func (r *memoryAINativeScriptRepository) CompleteAINativeScriptGeneration(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, operation AINativeScriptOperation, script AINativeScriptRevision, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.workspace.ActiveOperationID != operation.ID || r.workspace.ActiveOperationVersion == nil || *r.workspace.ActiveOperationVersion != operation.Version {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if r.scripts == nil {
		r.scripts = map[int64]AINativeScriptRevision{}
	}
	nextRevision := int64(1)
	if r.workspace.CurrentScriptRevision != nil {
		nextRevision = *r.workspace.CurrentScriptRevision + 1
		previous := r.scripts[*r.workspace.CurrentScriptRevision]
		previous.Status = AINativeScriptSupersededStatus
		r.scripts[previous.Revision] = previous
	}
	script.Revision, script.Status, script.CreatedBy, script.CreatedAt = nextRevision, AINativeScriptDraftStatus, actorID, now
	r.scripts[nextRevision] = script
	r.workspace.CurrentScriptRevision = &nextRevision
	r.workspace.ScriptStatus = AINativeScriptDraftStatus
	r.workspace.Script = &script
	r.workspace.ActiveOperationID, r.workspace.ActiveOperationVersion = "", nil
	r.workspace.WorkspaceVersion++
	r.workspace.UpdatedAt = now
	return r.workspace, nil
}

func (r *memoryAINativeScriptRepository) FailAINativeScriptGeneration(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, operationID string, operationVersion int64, _ string, _ string, now time.Time) error {
	if r.workspace.ActiveOperationID != operationID || r.workspace.ActiveOperationVersion == nil || *r.workspace.ActiveOperationVersion != operationVersion {
		return ErrVersionConflict
	}
	r.workspace.ScriptStatus = AINativeScriptFailedStatus
	r.workspace.ActiveOperationID, r.workspace.ActiveOperationVersion = "", nil
	r.workspace.UpdatedAt = now
	return nil
}

func (r *memoryAINativeScriptRepository) AppendAINativeScriptRevision(_ context.Context, next AINativeRequirementWorkspace, expectedRevision int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.workspace.CurrentScriptRevision == nil || *r.workspace.CurrentScriptRevision != expectedRevision || r.workspace.ScriptStatus != AINativeScriptDraftStatus {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	previous := r.scripts[expectedRevision]
	previous.Status = AINativeScriptSupersededStatus
	r.scripts[expectedRevision] = previous
	next.Script.Revision, next.Script.Status, next.Script.CreatedBy, next.Script.CreatedAt = expectedRevision+1, AINativeScriptDraftStatus, actorID, now
	nextRevision := expectedRevision + 1
	next.CurrentScriptRevision = &nextRevision
	next.WorkspaceVersion = r.workspace.WorkspaceVersion + 1
	next.UpdatedAt = now
	r.scripts[nextRevision] = *next.Script
	r.workspace = next
	return next, nil
}

func (r *memoryAINativeScriptRepository) ConfirmAINativeScript(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, expectedRevision int64, expectedWorkspaceVersion int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.workspace.CurrentScriptRevision == nil || *r.workspace.CurrentScriptRevision != expectedRevision || r.workspace.WorkspaceVersion != expectedWorkspaceVersion || r.workspace.ScriptStatus != AINativeScriptDraftStatus {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	script := r.scripts[expectedRevision]
	script.Status, script.ConfirmedBy = AINativeScriptConfirmedStatus, actorID
	script.ConfirmedAt = &now
	r.scripts[expectedRevision] = script
	r.workspace.Script, r.workspace.ScriptStatus = &script, AINativeScriptConfirmedStatus
	r.workspace.ConfirmedScriptRevision = &expectedRevision
	r.workspace.WorkspaceVersion++
	r.workspace.UpdatedAt = now
	return r.workspace, nil
}

func (r *memoryAINativeScriptRepository) GetAINativeScriptReopenImpact(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, workspaceID string) (AINativeReopenImpact, error) {
	if r.workspace.WorkspaceID != workspaceID || r.workspace.ScriptStatus != AINativeScriptConfirmedStatus || r.workspace.ConfirmedScriptRevision == nil {
		return AINativeReopenImpact{}, ErrInvalidState
	}
	return AINativeReopenImpact{WorkspaceID: workspaceID, Stage: AINativeStageScript, ExpectedWorkspaceVersion: r.workspace.WorkspaceVersion, SupersededScriptRevisions: []int64{*r.workspace.ConfirmedScriptRevision}}, nil
}

func (r *memoryAINativeScriptRepository) ReopenAINativeScript(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, expectedWorkspaceVersion int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.workspace.WorkspaceVersion != expectedWorkspaceVersion || r.workspace.ScriptStatus != AINativeScriptConfirmedStatus || r.workspace.CurrentScriptRevision == nil {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	previousRevision := *r.workspace.CurrentScriptRevision
	previous := r.scripts[previousRevision]
	previous.Status = AINativeScriptSupersededStatus
	r.scripts[previousRevision] = previous
	nextRevision := previousRevision + 1
	next := previous
	next.Revision, next.Status, next.CreatedBy, next.CreatedAt = nextRevision, AINativeScriptDraftStatus, actorID, now
	next.ConfirmedBy, next.ConfirmedAt = "", nil
	next.BasedOnRevision = &previousRevision
	r.scripts[nextRevision] = next
	r.workspace.Script, r.workspace.ScriptStatus = &next, AINativeScriptDraftStatus
	r.workspace.CurrentScriptRevision, r.workspace.ConfirmedScriptRevision = &nextRevision, nil
	r.workspace.WorkspaceVersion++
	r.workspace.UpdatedAt = now
	return r.workspace, nil
}

type capturingAINativeScriptScheduler struct{ operation AINativeScriptOperation }

func (s *capturingAINativeScriptScheduler) ScheduleAINativeScript(_ context.Context, operation AINativeScriptOperation) error {
	s.operation = operation
	return nil
}

func TestAINativeScriptValidateRequiresClosedTimelineAndKnownSellingPoints(t *testing.T) {
	requirement := AINativeRequirementDraft{
		Revision:          1,
		DurationSeconds:   20,
		CoreSellingPoints: []AINativeEditableText{{ID: "point_1", Text: "纯钛材质"}},
	}
	script := validAINativeScript()
	if err := script.ValidateAgainst(requirement); err != nil {
		t.Fatalf("valid script rejected: %v", err)
	}

	script.Segments[1].StartMS = 4500
	if err := script.ValidateAgainst(requirement); err == nil {
		t.Fatal("timeline gap must be rejected")
	}

	script = validAINativeScript()
	script.Segments[1].SellingPointIDs = []string{"invented_point"}
	if err := script.ValidateAgainst(requirement); err == nil {
		t.Fatal("unknown selling point reference must be rejected")
	}
}

func TestAINativeScriptTimelineClosesAtEverySupportedDuration(t *testing.T) {
	for _, duration := range []int{15, 20, 30} {
		t.Run(fmt.Sprintf("%d_seconds", duration), func(t *testing.T) {
			requirement := AINativeRequirementDraft{Revision: 1, DurationSeconds: duration, CoreSellingPoints: []AINativeEditableText{{ID: "point_1", Text: "纯钛材质"}}}
			script := validAINativeScript()
			script.DurationSeconds = duration
			script.Segments[0].EndMS = duration * 200
			script.Segments[1].StartMS = script.Segments[0].EndMS
			script.Segments[1].EndMS = duration * 750
			script.Segments[2].StartMS = script.Segments[1].EndMS
			script.Segments[2].EndMS = duration * 1000
			if err := script.ValidateAgainst(requirement); err != nil {
				t.Fatalf("supported duration rejected: %v", err)
			}
		})
	}
}

func TestModelAINativeScriptPlannerReturnsOneCompleteStructuredScript(t *testing.T) {
	output, err := json.Marshal(modelAINativeScript{
		Title:           "通勤杯也能轻装上阵",
		CreativeSummary: "从通勤漏液痛点切入，用真实动作证明纯钛杯的随行价值。",
		Segments: []modelAINativeScriptSegment{
			{ID: "segment_1", StartMS: 0, EndMS: 4000, Purpose: AINativeScriptPurposeHook, VisualIntent: "包内水渍与干净杯身快速对比", Voiceover: "通勤包又被杯子弄湿了？", Subtitle: "通勤漏液真的很麻烦"},
			{ID: "segment_2", StartMS: 4000, EndMS: 15000, Purpose: AINativeScriptPurposeProof, VisualIntent: "近景展示杯体并放入通勤包", Voiceover: "纯钛杯身，轻巧随行。", Subtitle: "纯钛材质，轻巧随行", SellingPointIDs: []string{"point_1"}},
			{ID: "segment_3", StartMS: 15000, EndMS: 20000, Purpose: AINativeScriptPurposeCTA, VisualIntent: "产品与通勤场景同框收束", Voiceover: "点击了解这只随行钛杯。", Subtitle: "点击了解更多", ConversionAction: "点击了解商品"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := &aiNativeTextGeneratorStub{response: provider.SynchronousResponse{
		ProviderCode: "ark", ModelAlias: "cookies.text.standard", ModelVersion: "doubao-seed-2.0-pro",
		StructuredOutput: output, RouteRevisionID: "route_1", Usage: &provider.TokenUsage{InputTokens: 100, OutputTokens: 200, TotalTokens: 300},
	}}
	profile, err := NewChannelCreativeProfileRegistry().Resolve("douyin", "performance", "v1")
	if err != nil {
		t.Fatal(err)
	}
	requirement := AINativeRequirementDraft{
		ContractVersion: "creative.ai-native.requirement/v1", Revision: 2, Status: AINativeRequirementConfirmedStatus,
		ProductName: "纯钛随行杯", ProductDescription: "适合通勤的纯钛杯",
		TargetAudiences:   []AINativeEditableText{{ID: "audience_1", Text: "通勤人群"}},
		CoreSellingPoints: []AINativeEditableText{{ID: "point_1", Text: "纯钛材质"}},
		Channel:           "douyin", AspectRatio: "9:16", DurationSeconds: 20, Language: "zh-CN",
	}
	planner := ModelAINativeScriptPlanner{Text: text, ModelAlias: "cookies.text.standard"}
	script, err := planner.Plan(context.Background(), contract.ActorContext{}, contract.ProjectContext{}, requirement, profile, "")
	if err != nil {
		t.Fatal(err)
	}
	if script.ContractVersion != "creative.ai-native.script/v1" || len(script.Segments) != 3 || script.Segments[2].EndMS != 20000 {
		t.Fatalf("unexpected script: %#v", script)
	}
	if script.Generation.ModelVersion != "doubao-seed-2.0-pro" || script.Generation.RouteRevisionID != "route_1" || script.Generation.ProfileHash != profile.ContentHash {
		t.Fatalf("generation lineage missing: %#v", script.Generation)
	}
	if text.request.ModelAlias != "cookies.text.standard" || len(text.request.OutputJSONSchema) == 0 || len(text.request.Messages) != 2 {
		t.Fatalf("structured provider contract was not used: %#v", text.request)
	}
}

func TestAINativeScriptGenerationRunsAsRecoverableWorkspaceOperation(t *testing.T) {
	requirements := &memoryAINativeRequirementRepository{}
	service := Service{
		Projects: testProjects{}, AINativeProducts: &aiNativeProductResolverStub{product: testAINativeProduct()},
		AINativeRequirementPlanner: DeterministicAINativeRequirementPlanner{}, AINativeRequirements: requirements,
		AINativeProductMediaImporter: &aiNativeProductMediaImporterStub{},
		NewID:                        func(prefix string) (string, error) { return prefix + "_1", nil },
		Now:                          func() time.Time { return time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC) },
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}
	created, err := service.AnalyzeAINativeRequirement(context.Background(), actor, "project_1", AnalyzeAINativeRequirementRequest{ProductLink: "https://v.douyin.com/example/"})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := service.ConfirmAINativeRequirement(context.Background(), actor, "project_1", created.WorkspaceID, ConfirmAINativeRequirementRequest{ExpectedRevision: created.CurrentRevision})
	if err != nil {
		t.Fatal(err)
	}
	scripts := &memoryAINativeScriptRepository{workspace: confirmed}
	scheduler := &capturingAINativeScriptScheduler{}
	service.AINativeScripts = scripts
	service.AINativeScriptScheduler = scheduler
	service.AINativeScriptProfiles = NewChannelCreativeProfileRegistry()
	service.AINativeScriptPlanner = staticAINativeScriptPlanner{script: validAINativeScript()}

	generating, err := service.GenerateAINativeScript(context.Background(), actor, "project_1", created.WorkspaceID, GenerateAINativeScriptRequest{ExpectedWorkspaceVersion: confirmed.WorkspaceVersion})
	if err != nil {
		t.Fatal(err)
	}
	if generating.ScriptStatus != AINativeScriptGeneratingStatus || generating.ActiveOperationID == "" || scheduler.operation.ID != generating.ActiveOperationID {
		t.Fatalf("script operation was not persisted and scheduled: %#v operation=%#v", generating, scheduler.operation)
	}
	claimPayload, _ := json.Marshal(AINativeScriptJobPayload{Operation: scheduler.operation})
	_, err = service.HandleAINativeScriptJob(context.Background(), jobruntime.Claim{Job: contract.Job{Kind: AINativeScriptJobKind, OrganizationID: actor.OrganizationID, ProjectID: "project_1"}, Payload: claimPayload})
	if err != nil {
		t.Fatal(err)
	}
	completed := scripts.workspace
	if completed.Script == nil || completed.ScriptStatus != AINativeScriptDraftStatus || completed.CurrentScriptRevision == nil || completed.ActiveOperationID != "" {
		t.Fatalf("script operation did not complete into a recoverable draft: %#v", completed)
	}
	if completed.Script.BasedOnRequirementRevision != confirmed.CurrentRevision || len(completed.Script.BasedOnRequirementHash) != 64 {
		t.Fatalf("requirement lineage missing: %#v", completed.Script)
	}
}

func TestAINativeScriptJobPersistsTerminalPlannerFailure(t *testing.T) {
	revision := int64(1)
	operationVersion := int64(1)
	requirement := validRequirementForScript()
	workspace := AINativeRequirementWorkspace{
		WorkspaceID: "workspace_1", CreativeIntakeID: "intake_1", CreativeTaskID: "task_1", OrganizationID: "org_1", ProjectID: "project_1",
		Status: AINativeRequirementConfirmedStatus, CurrentStage: AINativeStageScript, WorkspaceVersion: 2,
		CurrentRevision: 1, ConfirmedRevision: &revision, Requirement: requirement, ScriptStatus: AINativeScriptGeneratingStatus,
		ActiveOperationID: "operation_1", ActiveOperationVersion: &operationVersion,
		CreatedBy: "user_1", ConfirmedBy: "user_1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	repository := &memoryAINativeScriptRepository{workspace: workspace}
	service := Service{Projects: testProjects{}, AINativeScripts: repository, AINativeScriptPlanner: staticAINativeScriptPlanner{err: errors.New("provider unavailable")}, AINativeScriptProfiles: NewChannelCreativeProfileRegistry()}
	operation := AINativeScriptOperation{ID: "operation_1", Version: 1, WorkspaceID: "workspace_1", ActorID: "user_1"}
	payload, _ := json.Marshal(AINativeScriptJobPayload{Operation: operation})
	_, err := service.HandleAINativeScriptJob(context.Background(), jobruntime.Claim{Job: contract.Job{Kind: AINativeScriptJobKind, OrganizationID: "org_1", ProjectID: "project_1"}, Payload: payload})
	if err == nil || repository.workspace.ScriptStatus != AINativeScriptFailedStatus || repository.workspace.ActiveOperationID != "" {
		t.Fatalf("planner failure was not persisted: err=%v workspace=%#v", err, repository.workspace)
	}
}

func TestAINativeScriptEditConfirmAndReopenPreserveRevisionLineage(t *testing.T) {
	requirement := validAINativeWorkspaceRequirement()
	script := validAINativeScript()
	script.BasedOnRequirementRevision = 1
	script.BasedOnRequirementHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	now := time.Date(2026, 8, 4, 13, 0, 0, 0, time.UTC)
	revision := int64(1)
	workspace := AINativeRequirementWorkspace{WorkspaceID: "workspace_1", CreativeIntakeID: "intake_1", CreativeTaskID: "task_1",
		OrganizationID: "org_1", ProjectID: "project_1", Status: AINativeRequirementConfirmedStatus, CurrentStage: AINativeStageScript,
		WorkspaceVersion: 3, CurrentRevision: 1, ConfirmedRevision: &revision, Requirement: requirement,
		ScriptStatus: AINativeScriptDraftStatus, CurrentScriptRevision: &revision, Script: &script,
		CreatedBy: "user_1", ConfirmedBy: "user_1", CreatedAt: now, UpdatedAt: now}
	repository := &memoryAINativeScriptRepository{workspace: workspace, scripts: map[int64]AINativeScriptRevision{1: script}}
	service := Service{Projects: testProjects{}, AINativeScripts: repository, NewID: func(prefix string) (string, error) { return prefix + "_1", nil }, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}

	edited := script
	edited.Title = "更直接的通勤钛杯脚本"
	updated, err := service.UpdateAINativeScript(context.Background(), actor, "project_1", "workspace_1", UpdateAINativeScriptRequest{ExpectedRevision: 1, Script: edited})
	if err != nil {
		t.Fatal(err)
	}
	if updated.CurrentScriptRevision == nil || *updated.CurrentScriptRevision != 2 || repository.scripts[1].Status != AINativeScriptSupersededStatus {
		t.Fatalf("script edit did not append a revision: %#v", updated)
	}
	confirmed, err := service.ConfirmAINativeScript(context.Background(), actor, "project_1", "workspace_1", ConfirmAINativeScriptRequest{ExpectedRevision: 2, ExpectedWorkspaceVersion: updated.WorkspaceVersion})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.ScriptStatus != AINativeScriptConfirmedStatus || confirmed.ConfirmedScriptRevision == nil {
		t.Fatalf("script was not frozen: %#v", confirmed)
	}
	impact, err := service.GetAINativeReopenImpact(context.Background(), actor, "project_1", "workspace_1", AINativeStageScript)
	if err != nil || len(impact.SupersededScriptRevisions) != 1 {
		t.Fatalf("unexpected script reopen impact: %#v err=%v", impact, err)
	}
	reopened, err := service.ReopenAINativeScript(context.Background(), actor, "project_1", "workspace_1", ReopenAINativeRequirementRequest{ExpectedWorkspaceVersion: confirmed.WorkspaceVersion, InvalidateDownstream: true})
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ScriptStatus != AINativeScriptDraftStatus || reopened.CurrentScriptRevision == nil || *reopened.CurrentScriptRevision != 3 || repository.scripts[2].Status != AINativeScriptSupersededStatus {
		t.Fatalf("script reopen did not create a new draft revision: %#v", reopened)
	}
}

type staticAINativeScriptPlanner struct {
	script AINativeScriptRevision
	err    error
}

func (p staticAINativeScriptPlanner) Plan(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, requirement AINativeRequirementDraft, profile ChannelCreativeProfile, _ string) (AINativeScriptRevision, error) {
	p.script.ChannelProfileID = profile.ID
	p.script.ChannelProfileHash = profile.ContentHash
	p.script.Generation.ProfileHash = profile.ContentHash
	p.script.BasedOnRequirementRevision = requirement.Revision
	if len(requirement.CoreSellingPoints) > 0 && len(p.script.Segments) > 1 {
		p.script.Segments[1].SellingPointIDs = []string{requirement.CoreSellingPoints[0].ID}
	}
	return p.script, p.err
}

func validRequirementForScript() AINativeRequirementDraft {
	return AINativeRequirementDraft{ContractVersion: "creative.ai-native.requirement/v1", Revision: 1, Status: AINativeRequirementConfirmedStatus,
		ProductName: "纯钛随行杯", ProductDescription: "适合通勤的纯钛杯", TargetAudiences: []AINativeEditableText{{ID: "audience_1", Text: "通勤人群"}},
		CoreSellingPoints: []AINativeEditableText{{ID: "point_1", Text: "纯钛材质"}}, Channel: "douyin", AspectRatio: "9:16", DurationSeconds: 20, Language: "zh-CN"}
}

func validAINativeWorkspaceRequirement() AINativeRequirementDraft {
	requirement := testAINativeProduct()
	return AINativeRequirementDraft{ContractVersion: "creative.ai-native.requirement/v1", Revision: 1, Status: AINativeRequirementDraftStatus,
		Product: requirement, ProductName: "纯钛随行杯", ProductDescription: "适合通勤的纯钛杯",
		TargetAudiences:   []AINativeEditableText{{ID: "audience_1", Text: "通勤人群"}},
		CoreSellingPoints: []AINativeEditableText{{ID: "point_1", Text: "纯钛材质"}}, Channel: "douyin", AspectRatio: "9:16", DurationSeconds: 20, Language: "zh-CN",
		Generation: AINativeGenerationMetadata{Mode: "model", ModelAlias: "cookies.text.standard", ModelVersion: "seed", PromptVersion: aiNativeRequirementPromptVersion}}
}

func validAINativeScript() AINativeScriptRevision {
	return AINativeScriptRevision{
		ContractVersion: "creative.ai-native.script/v1", Revision: 1, Status: AINativeScriptDraftStatus,
		Title: "通勤杯也能轻装上阵", CreativeSummary: "痛点、证明与行动引导组成的一份完整脚本。",
		ChannelProfileID: "douyin.performance.v1", ChannelProfileHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		DurationSeconds:            20,
		BasedOnRequirementRevision: 1,
		BasedOnRequirementHash:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Generation: AINativeScriptGenerationMetadata{ModelAlias: "cookies.text.standard", ModelVersion: "test", PromptVersion: aiNativeScriptPromptVersion,
			ProfileHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Segments: []AINativeScriptSegment{
			{ID: "segment_1", StartMS: 0, EndMS: 4000, Purpose: AINativeScriptPurposeHook, VisualIntent: "展示通勤包内水渍", Voiceover: "通勤包又湿了？", Subtitle: "通勤漏液太麻烦"},
			{ID: "segment_2", StartMS: 4000, EndMS: 15000, Purpose: AINativeScriptPurposeProof, VisualIntent: "展示纯钛杯体和随行动作", Voiceover: "纯钛杯身，轻巧随行。", Subtitle: "纯钛材质", SellingPointIDs: []string{"point_1"}},
			{ID: "segment_3", StartMS: 15000, EndMS: 20000, Purpose: AINativeScriptPurposeCTA, VisualIntent: "产品定格与行动引导", Voiceover: "点击了解更多。", Subtitle: "点击了解更多", ConversionAction: "点击了解商品"},
		},
	}
}
