package creative

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestManualIntakeNeedsClarificationBeforeCreatingTask(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-1", CreateIntakeRequest{
		Source: IntakeSourceManual, Channel: ChannelXiaohongshu, Tone: []string{}, VisualKeywords: []string{}, Mandatory: []string{}, Prohibited: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if intake.Status != IntakeNeedsClarification || len(intake.MissingFields) != 3 {
		t.Fatalf("intake = %#v", intake)
	}
	if _, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest()); !errors.Is(err, ErrIntakeNotReady) {
		t.Fatalf("error = %v, want %v", err, ErrIntakeNotReady)
	}
}

func TestManualReadyIntakeCreatesImageTextTaskAndDraft(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-2", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	if intake.Status != IntakeReady || intake.ConfirmedBy != "usr_1" {
		t.Fatalf("intake = %#v", intake)
	}
	task, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest())
	if err != nil {
		t.Fatal(err)
	}
	if task.Format != FormatImageText || task.Channel != ChannelXiaohongshu {
		t.Fatalf("task = %#v", task)
	}
	detail, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Draft.TitleCandidates) != 3 || len(detail.Draft.ImagePlan) != 4 || detail.Draft.CoverCopy == "" {
		t.Fatalf("draft = %#v", detail.Draft)
	}
}

func TestApprovedStrategyPackageCreatesReadyCreativeIntake(t *testing.T) {
	t.Parallel()
	service := testService()
	service.StrategyPackages = strategyPackageReader{snapshot: StrategyPackageSnapshot{
		PackageID: "package_1", PackageVersion: 2, ContentHash: "sha256:package", CreativeReady: true,
		Objective: "建立新品认知", Audience: "关注生活方式的上班族", CoreMessage: "一杯咖啡也可以成为从容开始的仪式",
		Concept: "温暖晨光中的咖啡桌", Tone: []string{"自然"}, VisualKeywords: []string{"晨光"}, Mandatory: []string{}, Prohibited: []string{},
	}}
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-strategy", CreateIntakeRequest{
		Source: IntakeSourceStrategyPackage, StrategyPackage: &StrategyPackageReference{PackageID: "package_1", PackageVersion: 2, ExpectedContentHash: "sha256:package"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if intake.Source != IntakeSourceStrategyPackage || intake.Status != IntakeReady || intake.Request.Objective == "" {
		t.Fatalf("intake = %#v", intake)
	}
	if _, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest()); err != nil {
		t.Fatalf("strategy intake did not create Creative task: %v", err)
	}
}

func TestCreateStrategyIntakeDeduplicatesTheSamePackageVersion(t *testing.T) {
	t.Parallel()
	service := testService()
	service.StrategyPackages = strategyPackageReader{snapshot: StrategyPackageSnapshot{PackageID: "package_1", PackageVersion: 1, ContentHash: "hash", CreativeReady: true, Objective: "目标", Audience: "受众", CoreMessage: "主张", Concept: "概念", Tone: []string{}, VisualKeywords: []string{}, Mandatory: []string{}, Prohibited: []string{}}}
	rc := testRequestContext()
	request := CreateIntakeRequest{Source: IntakeSourceStrategyPackage, StrategyPackage: &StrategyPackageReference{PackageID: "package_1", PackageVersion: 1, ExpectedContentHash: "hash"}}
	first, err := service.CreateIntake(context.Background(), rc, "project_1", "strategy-intake-first", request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateIntake(context.Background(), rc, "project_1", "strategy-intake-second", request)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("same strategy package should return its existing Intake: first=%q second=%q", first.ID, second.ID)
	}
}

func TestStrategyPackageWithoutCreativeReadinessNeedsClarification(t *testing.T) {
	t.Parallel()
	service := testService()
	service.StrategyPackages = strategyPackageReader{snapshot: StrategyPackageSnapshot{CreativeReady: false, Objective: "目标", Audience: "受众", CoreMessage: "主张", Concept: "概念", Tone: []string{}, VisualKeywords: []string{}, Mandatory: []string{}, Prohibited: []string{}}}
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-not-ready", CreateIntakeRequest{Source: IntakeSourceStrategyPackage, StrategyPackage: &StrategyPackageReference{PackageID: "package_2", PackageVersion: 1, ExpectedContentHash: "hash"}})
	if err != nil {
		t.Fatal(err)
	}
	if intake.Status != IntakeNeedsClarification || len(intake.MissingFields) != 1 || intake.MissingFields[0] != "strategy_package.creative_ready" {
		t.Fatalf("intake = %#v", intake)
	}
}

func TestIntakeIdempotencyDoesNotCreateAnotherIntake(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	first, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-3", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-3", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotency IDs = %q and %q", first.ID, second.ID)
	}
}

func TestTaskCreationAllowsSeveralDistinctDirectionsForTheSameIntake(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-4", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, CreateTaskRequest{ContentType: ContentTypeIngredientExplanation, Focus: "成分解释"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.Direction.ContentType == second.Direction.ContentType {
		t.Fatalf("task directions were not created separately: first=%#v second=%#v", first, second)
	}
}

func TestArchiveTaskHidesItFromActiveQueueButRetainsItsLineage(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-archive", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterCoverImageJob(context.Background(), rc.Actor, "project_1", task.ID, "provider_job_1"); err != nil {
		t.Fatal(err)
	}
	if err := service.ArchiveTask(context.Background(), rc.Actor, "project_1", task.ID); err != nil {
		t.Fatal(err)
	}
	active, err := service.ListTasks(context.Background(), rc.Actor, "project_1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("active tasks = %#v, want archived task omitted", active)
	}
	detail, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Task.Status != TaskArchived || len(detail.ProductionJobs) != 1 || detail.Draft.TaskID != task.ID {
		t.Fatalf("archived detail should retain lineage: %#v", detail)
	}
}

func validManualRequest() CreateIntakeRequest {
	return CreateIntakeRequest{
		Source: IntakeSourceManual, Channel: ChannelXiaohongshu, Objective: "建立新品认知", Audience: "关注生活方式的年轻上班族", CoreMessage: "一杯咖啡，也可以成为从容开始的仪式", CallToAction: "收藏这份晨间灵感",
		Concept: "柔和自然光下的蓝白咖啡桌", Tone: []string{"自然", "克制"}, VisualKeywords: []string{"蓝白", "晨光"}, Mandatory: []string{"产品主体"}, Prohibited: []string{},
	}
}

func defaultTaskRequest() CreateTaskRequest {
	return CreateTaskRequest{ContentType: ContentTypeLifestyle, Focus: "生活方式种草"}
}

func testRequestContext() contract.RequestContext {
	return contract.RequestContext{RequestID: "req_1", TraceID: "trace_1", Actor: contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}}
}

func testService() Service {
	sequence := 0
	return Service{Repository: &memoryRepository{intakes: map[string]CreativeIntake{}, tasks: map[string]TaskDetail{}, versions: map[string]CreativeVersion{}}, Projects: testProjects{}, Now: func() time.Time { return time.Date(2026, time.July, 23, 1, 0, 0, 0, time.UTC) }, NewID: func(prefix string) (string, error) { sequence++; return fmt.Sprintf("%s_%d", prefix, sequence), nil }}
}

type testProjects struct{}

type strategyPackageReader struct {
	snapshot StrategyPackageSnapshot
}

func (r strategyPackageReader) ReadForCreative(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, reference StrategyPackageReference) (StrategyPackageSnapshot, error) {
	value := r.snapshot
	if value.PackageID == "" {
		value.PackageID, value.PackageVersion, value.ContentHash = reference.PackageID, reference.PackageVersion, reference.ExpectedContentHash
	}
	if value.ContentHash != reference.ExpectedContentHash {
		return StrategyPackageSnapshot{}, fmt.Errorf("hash mismatch")
	}
	return value, nil
}

func (testProjects) RequireActiveContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	brand := contract.BrandID("brand_1")
	return contract.ProjectContext{OrganizationID: actor.OrganizationID, ProjectID: projectID, BrandID: &brand, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1}, nil
}

type memoryRepository struct {
	intakes  map[string]CreativeIntake
	tasks    map[string]TaskDetail
	versions map[string]CreativeVersion
}

func (r *memoryRepository) CreateIntake(_ context.Context, intake CreativeIntake) (CreativeIntake, bool, error) {
	for _, existing := range r.intakes {
		if existing.IdempotencyKey == intake.IdempotencyKey && existing.Principal == intake.Principal && existing.ProjectID == intake.ProjectID {
			if existing.RequestHash != intake.RequestHash {
				return CreativeIntake{}, false, ErrIdempotencyConflict
			}
			return existing, true, nil
		}
		if intake.Source == IntakeSourceStrategyPackage && existing.Source == IntakeSourceStrategyPackage && sameStrategyPackage(existing.Request.StrategyPackage, intake.Request.StrategyPackage) {
			return existing, true, nil
		}
	}
	r.intakes[intake.ID] = intake
	return intake, false, nil
}

func sameStrategyPackage(left, right *StrategyPackageReference) bool {
	return left != nil && right != nil && left.PackageID == right.PackageID && left.PackageVersion == right.PackageVersion && left.ExpectedContentHash == right.ExpectedContentHash
}
func (r *memoryRepository) ListIntakes(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ int) ([]CreativeIntake, error) {
	values := make([]CreativeIntake, 0, len(r.intakes))
	for _, value := range r.intakes {
		values = append(values, value)
	}
	return values, nil
}
func (r *memoryRepository) GetIntake(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string) (CreativeIntake, error) {
	value, ok := r.intakes[id]
	if !ok {
		return CreativeIntake{}, ErrNotFound
	}
	return value, nil
}

func (r *memoryRepository) CreateTask(_ context.Context, task CreativeTask, draft ImageTextDraft) (CreativeTask, error) {
	r.tasks[task.ID] = TaskDetail{Task: task, Intake: r.intakes[task.IntakeID], Draft: draft, ProductionJobs: []ProductionJob{}}
	return task, nil
}
func (r *memoryRepository) ListTasks(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ int) ([]CreativeTask, error) {
	values := make([]CreativeTask, 0, len(r.tasks))
	for _, value := range r.tasks {
		if value.Task.Status == TaskArchived {
			continue
		}
		values = append(values, value.Task)
	}
	return values, nil
}
func (r *memoryRepository) GetTaskDetail(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string) (TaskDetail, error) {
	value, ok := r.tasks[id]
	if !ok {
		return TaskDetail{}, ErrNotFound
	}
	return value, nil
}
func (r *memoryRepository) ArchiveTask(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, taskID string, now time.Time) error {
	value, ok := r.tasks[taskID]
	if !ok {
		return ErrNotFound
	}
	if value.Task.Status == TaskArchived {
		return ErrInvalidState
	}
	value.Task.Status = TaskArchived
	value.Task.UpdatedAt = now
	r.tasks[taskID] = value
	return nil
}
func (r *memoryRepository) ReviseDraft(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, taskID string, expectedVersion int64, draft ImageTextDraft) (ImageTextDraft, error) {
	value, ok := r.tasks[taskID]
	if !ok {
		return ImageTextDraft{}, ErrNotFound
	}
	if value.Draft.Version != expectedVersion || value.Task.Version != expectedVersion {
		return ImageTextDraft{}, ErrVersionConflict
	}
	value.Draft = draft
	value.Task.Version = draft.Version
	value.Task.UpdatedAt = draft.CreatedAt
	r.tasks[taskID] = value
	return draft, nil
}
func (r *memoryRepository) RegisterProductionJob(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, taskID string, job ProductionJob) error {
	value, ok := r.tasks[taskID]
	if !ok {
		return ErrNotFound
	}
	for _, existing := range value.ProductionJobs {
		if existing.Kind == job.Kind {
			if existing.ProviderJobID == job.ProviderJobID {
				return nil
			}
			return ErrProviderJobConflict
		}
	}
	value.ProductionJobs = append(value.ProductionJobs, job)
	r.tasks[taskID] = value
	return nil
}

func (r *memoryRepository) CreateVersion(_ context.Context, value CreativeVersion) (CreativeVersion, bool, error) {
	for _, existing := range r.versions {
		if existing.ProjectID == value.ProjectID && existing.CreatedBy == value.CreatedBy && existing.IdempotencyKey == value.IdempotencyKey {
			if existing.RequestHash != value.RequestHash {
				return CreativeVersion{}, false, ErrIdempotencyConflict
			}
			return existing, true, nil
		}
		if existing.TaskID == value.TaskID && existing.Version == value.Version {
			if !existing.ContentHash.Equal(value.ContentHash) {
				return CreativeVersion{}, false, ErrVersionConflict
			}
			return existing, false, nil
		}
	}
	r.versions[value.ID] = value
	return value, false, nil
}
