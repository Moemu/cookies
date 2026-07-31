package creative

import (
	"context"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestCommerceFixtureWorkspacePersistsDraftApprovalAndAttempt(t *testing.T) {
	t.Parallel()
	service := testService()
	core := service.Repository.(*memoryRepository)
	workspaces := &memoryCommerceWorkspaceRepository{
		core:     core,
		attempts: map[string][]CommerceGenerationAttempt{},
	}
	service.CommerceWorkspaces = workspaces
	service.ViralRemakes = core

	product := contract.AssetVersionRef{AssetID: "asset_product", Version: 1}
	first := contract.AssetVersionRef{AssetID: "asset_first", Version: 1}
	last := contract.AssetVersionRef{AssetID: "asset_last", Version: 1}
	service.Assets = testAssetReader{snapshots: map[contract.AssetID]CreativeAssetSnapshot{
		product.AssetID: {Ref: product, Kind: contract.AssetImage, MIMEType: "image/jpeg", Ready: true},
		first.AssetID:   {Ref: first, Kind: contract.AssetImage, MIMEType: "image/jpeg", Ready: true},
		last.AssetID:    {Ref: last, Kind: contract.AssetImage, MIMEType: "image/jpeg", Ready: true},
	}}
	rc := testRequestContext()
	request := EnsureCommerceFixtureWorkspaceRequest{
		Template:     TemplateReference{ID: CommerceWindowRevealTemplateID, Version: 1},
		ProductAsset: product,
		FirstFrame:   first,
		LastFrame:    last,
	}
	workspace, err := service.EnsureCommerceFixtureWorkspace(
		context.Background(), rc, "project_1", "commerce-fixture-project-1", request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Task.PerformanceMode != PerformanceModeCommercePreroll ||
		workspace.VideoDraft == nil || workspace.VideoDraft.CommercePreroll == nil ||
		workspace.VideoDraft.Revision != 1 {
		t.Fatalf("workspace = %#v", workspace)
	}
	replayed, err := service.EnsureCommerceFixtureWorkspace(
		context.Background(), rc, "project_1", "commerce-fixture-project-1", request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Task.ID != workspace.Task.ID || len(core.tasks) != 1 {
		t.Fatalf("fixture replay created another task: first=%s replay=%s tasks=%d", workspace.Task.ID, replayed.Task.ID, len(core.tasks))
	}

	revised, err := service.UpdateCommercePrerollDraft(
		context.Background(),
		rc.Actor,
		"project_1",
		workspace.Task.ID,
		UpdateCommercePrerollDraftRequest{
			ExpectedRevision: 1,
			Template:         TemplateReference{ID: CommerceWindowRevealTemplateID, Version: 1},
			Motion:           "一只戴浅色手套的手只完成一次横向擦拭。",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if revised.VideoDraft.Revision != 2 ||
		revised.VideoDraft.CommercePreroll.Approval != nil {
		t.Fatalf("revised workspace = %#v", revised.VideoDraft)
	}
	confirmed, err := service.ConfirmCommerceGeneration(
		context.Background(),
		rc.Actor,
		"project_1",
		workspace.Task.ID,
		ConfirmCommerceGenerationRequest{ExpectedRevision: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.VideoDraft.Revision != 3 ||
		confirmed.VideoDraft.CommercePreroll.Approval == nil {
		t.Fatalf("confirmed workspace = %#v", confirmed.VideoDraft)
	}
	input, promptHash, err := service.CommerceProviderInput(
		context.Background(), rc.Actor, "project_1", workspace.Task.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if input.InputMode != "first_last_frame" || len(input.ConditioningAssets) != 2 ||
		promptHash != confirmed.VideoDraft.CommercePreroll.Plan.Prompt.Hash {
		t.Fatalf("provider input = %#v, prompt hash = %q", input, promptHash)
	}
	attempt, err := service.RegisterCommerceGenerationAttempt(
		context.Background(), rc.Actor, "project_1", workspace.Task.ID, "provider_job_1",
	)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := service.GetLatestCommerceWorkspace(
		context.Background(), rc.Actor, "project_1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.CommerceGenerationAttempts) != 1 ||
		restored.CommerceGenerationAttempts[0].ID != attempt.ID ||
		restored.CommerceGenerationAttempts[0].ProviderJobID != "provider_job_1" {
		t.Fatalf("restored attempts = %#v", restored.CommerceGenerationAttempts)
	}
}

type memoryCommerceWorkspaceRepository struct {
	core      *memoryRepository
	taskID    string
	fixtureID string
	attempts  map[string][]CommerceGenerationAttempt
}

func (r *memoryCommerceWorkspaceRepository) EnsureCommerceFixtureWorkspace(
	_ context.Context,
	intake CreativeIntake,
	task CreativeTask,
	draft VideoDraft,
	fixtureID string,
	_ int64,
	_ string,
) (TaskDetail, bool, error) {
	if r.taskID != "" {
		value, err := r.GetCommerceWorkspace(context.Background(), intake.OrganizationID, intake.ProjectID, r.taskID)
		return value, false, err
	}
	r.fixtureID = fixtureID
	r.taskID = task.ID
	r.core.intakes[intake.ID] = intake
	copy := draft
	r.core.tasks[task.ID] = TaskDetail{
		Task: task, Intake: intake, VideoDraft: &copy,
		ProductionJobs: []ProductionJob{},
	}
	value, err := r.GetCommerceWorkspace(context.Background(), intake.OrganizationID, intake.ProjectID, task.ID)
	return value, true, err
}

func (r *memoryCommerceWorkspaceRepository) GetLatestCommerceWorkspace(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
) (TaskDetail, error) {
	if r.taskID == "" {
		return TaskDetail{}, ErrNotFound
	}
	return r.GetCommerceWorkspace(ctx, organizationID, projectID, r.taskID)
}

func (r *memoryCommerceWorkspaceRepository) GetCommerceWorkspace(
	_ context.Context,
	_ contract.OrganizationID,
	_ contract.ProjectID,
	taskID string,
) (TaskDetail, error) {
	value, ok := r.core.tasks[taskID]
	if !ok {
		return TaskDetail{}, ErrNotFound
	}
	value.CommerceGenerationAttempts = append([]CommerceGenerationAttempt{}, r.attempts[taskID]...)
	return value, nil
}

func (r *memoryCommerceWorkspaceRepository) CreateCommerceGenerationAttempt(
	_ context.Context,
	_ contract.OrganizationID,
	_ contract.ProjectID,
	attempt CommerceGenerationAttempt,
) (CommerceGenerationAttempt, error) {
	for _, existing := range r.attempts[attempt.TaskID] {
		if existing.ProviderJobID == attempt.ProviderJobID {
			return existing, nil
		}
	}
	r.attempts[attempt.TaskID] = append(r.attempts[attempt.TaskID], attempt)
	return attempt, nil
}
