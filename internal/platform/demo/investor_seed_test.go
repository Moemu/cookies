package demo

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/project"
)

func TestEnsureCanonicalInvestorDemoSeedsFreshStore(t *testing.T) {
	ctx := context.Background()
	actor := demoActor()
	projects := newFakeInvestorProjectStore()
	assetStore := &fakeInvestorAssetStore{}

	result, err := EnsureCanonicalInvestorDemo(ctx, actor, projects, assetStore)
	if err != nil {
		t.Fatalf("seed fresh demo: %v", err)
	}
	if result.ProjectID != InvestorDemoProjectID {
		t.Fatalf("project id=%q, want %q", result.ProjectID, InvestorDemoProjectID)
	}
	if projects.ensureProjectCalls != 1 {
		t.Fatalf("ensure project calls=%d, want 1", projects.ensureProjectCalls)
	}
	if len(assetStore.assets) != len(investorDemoAssets) {
		t.Fatalf("seeded assets=%d, want %d", len(assetStore.assets), len(investorDemoAssets))
	}
	if len(projects.tasks[InvestorDemoProjectID]) != len(investorDemoTasks) {
		t.Fatalf("seeded tasks=%d, want %d", len(projects.tasks[InvestorDemoProjectID]), len(investorDemoTasks))
	}
	if len(projects.operations[InvestorDemoProjectID]) != len(investorDemoOperations(InvestorDemoProjectID)) {
		t.Fatalf("seeded operations=%d, want %d", len(projects.operations[InvestorDemoProjectID]), len(investorDemoOperations(InvestorDemoProjectID)))
	}
	changeSet := projects.changeSets[InvestorDemoProjectID]["changeset_demo_precision_evidence_budget"]
	if changeSet.Status != project.ChangeSetPreflightPassed || changeSet.Preflight == nil || !changeSet.Preflight.Passed {
		t.Fatalf("change set preflight not seeded: %#v", changeSet)
	}
	if len(changeSet.ArtifactRefs) != 4 {
		t.Fatalf("change set artifact refs=%d, want 4", len(changeSet.ArtifactRefs))
	}
	if len(projects.auditEvents[InvestorDemoProjectID]) != 2 {
		t.Fatalf("audit events=%d, want 2", len(projects.auditEvents[InvestorDemoProjectID]))
	}
	for _, task := range projects.tasks[InvestorDemoProjectID] {
		for _, id := range append(task.SourceArtifactIDs, task.OutputArtifactIDs...) {
			if id == "" || id[:6] != "asset_" {
				t.Fatalf("task %s has non-stable asset ref id %q", task.ID, id)
			}
		}
	}
}

func TestImportLocalDemoDataStoresEveryFileAndSeedsWalkthrough(t *testing.T) {
	ctx := context.Background()
	actor := demoActor()
	projects := newFakeInvestorProjectStore()
	assetStore := &fakeInvestorAssetStore{}
	if _, err := EnsureCanonicalInvestorDemo(ctx, actor, projects, assetStore); err != nil {
		t.Fatalf("seed base demo: %v", err)
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "brief.pdf"), []byte("demo brief"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "video.mp4"), []byte("demo video"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := ImportLocalDemoData(ctx, actor, projects, assetStore, assets.NewMemoryBlobStore(), "assets", directory, 1)
	if err != nil {
		t.Fatalf("import demo data: %v", err)
	}
	if result.DocumentCount != 1 || result.VideoCount != 1 || len(result.AssetRefs) != 2 {
		t.Fatalf("unexpected import result: %#v", result)
	}
	if len(assetStore.assets) != len(investorDemoAssets)+2 {
		t.Fatalf("imported assets were not recorded: %d", len(assetStore.assets))
	}
	if _, ok := projects.tasks[InvestorDemoProjectID]["task_demo_imported_brief_to_video"]; !ok {
		t.Fatalf("brief-to-video walkthrough task was not seeded: %#v", projects.tasks[InvestorDemoProjectID])
	}
	if _, ok := projects.operations[InvestorDemoProjectID]["INSIGHT-DEMO-DATA-01"]; !ok {
		t.Fatalf("demo-data insight was not seeded: %#v", projects.operations[InvestorDemoProjectID])
	}
	firstCounts := projects.counts()
	if _, err := ImportLocalDemoData(ctx, actor, projects, assetStore, assets.NewMemoryBlobStore(), "assets", directory, 1); err != nil {
		t.Fatalf("rerun import: %v", err)
	}
	if !reflect.DeepEqual(firstCounts, projects.counts()) {
		t.Fatalf("import is not idempotent: first=%#v second=%#v", firstCounts, projects.counts())
	}
}

func TestEnsureCanonicalInvestorDemoIsIdempotentForExistingDemo(t *testing.T) {
	ctx := context.Background()
	actor := demoActor()
	projects := newFakeInvestorProjectStore()
	assetStore := &fakeInvestorAssetStore{}
	otherProjectID := contract.ProjectID("project_user_created")
	projects.tasks[otherProjectID] = map[string]project.BusinessTask{
		"user_task": {ID: "user_task", ProjectID: otherProjectID, Name: "用户项目任务", Status: project.BusinessTaskDraft},
	}
	projects.tasks[InvestorDemoProjectID] = map[string]project.BusinessTask{
		"task_demo_precision_strategy": {
			ID: "task_demo_precision_strategy", OrganizationID: actor.OrganizationID, ProjectID: InvestorDemoProjectID,
			Type: project.BusinessTaskStrategy, Name: "旧策略任务", Objective: "旧目标", Status: project.BusinessTaskDraft,
			SourceTaskIDs: []string{}, SourceArtifactIDs: []string{}, OutputArtifactIDs: []string{}, Version: 3,
		},
	}

	if _, err := EnsureCanonicalInvestorDemo(ctx, actor, projects, assetStore); err != nil {
		t.Fatalf("seed existing demo: %v", err)
	}
	firstCounts := projects.counts()
	strategy := projects.tasks[InvestorDemoProjectID]["task_demo_precision_strategy"]
	if strategy.Name != "精度证据增长策略评审" || strategy.Status != project.BusinessTaskReady || strategy.Version != 4 {
		t.Fatalf("strategy task was not updated in place: %#v", strategy)
	}
	if _, ok := projects.tasks[otherProjectID]["user_task"]; !ok || len(projects.tasks[otherProjectID]) != 1 {
		t.Fatalf("seed polluted unrelated user project: %#v", projects.tasks[otherProjectID])
	}

	if _, err := EnsureCanonicalInvestorDemo(ctx, actor, projects, assetStore); err != nil {
		t.Fatalf("rerun seed existing demo: %v", err)
	}
	secondCounts := projects.counts()
	if !reflect.DeepEqual(firstCounts, secondCounts) {
		t.Fatalf("seed is not idempotent: first=%#v second=%#v", firstCounts, secondCounts)
	}
	if len(projects.auditEvents[InvestorDemoProjectID]) != 2 {
		t.Fatalf("rerun duplicated audit events: %#v", projects.auditEvents[InvestorDemoProjectID])
	}
}

func demoActor() contract.ActorContext {
	return contract.ActorContext{
		OrganizationID: "org_demo",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_demo"},
		Scopes:         []contract.Scope{"project.read", "project.write"},
	}
}

type fakeInvestorAssetStore struct {
	assets map[contract.AssetID]assets.SeedAsset
}

func (s *fakeInvestorAssetStore) EnsureSeedAsset(_ context.Context, seed assets.SeedAsset, _ time.Time) (contract.ProjectAssetRef, error) {
	if s.assets == nil {
		s.assets = map[contract.AssetID]assets.SeedAsset{}
	}
	if err := seed.Validate(); err != nil {
		return contract.ProjectAssetRef{}, err
	}
	s.assets[seed.AssetID] = seed
	return contract.ProjectAssetRef{ProjectID: seed.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: seed.AssetID, Version: 1}}, nil
}

type fakeInvestorProjectStore struct {
	ensureProjectCalls int
	project            project.Project
	runtime            project.ProjectRuntime
	workbench          project.Workbench
	tasks              map[contract.ProjectID]map[string]project.BusinessTask
	operations         map[contract.ProjectID]map[string]project.OperationalRecord
	changeSets         map[contract.ProjectID]map[string]project.ChangeSet
	auditEvents        map[contract.ProjectID]map[string]project.AuditEvent
}

func newFakeInvestorProjectStore() *fakeInvestorProjectStore {
	return &fakeInvestorProjectStore{
		tasks:       map[contract.ProjectID]map[string]project.BusinessTask{},
		operations:  map[contract.ProjectID]map[string]project.OperationalRecord{},
		changeSets:  map[contract.ProjectID]map[string]project.ChangeSet{},
		auditEvents: map[contract.ProjectID]map[string]project.AuditEvent{},
	}
}

func (s *fakeInvestorProjectStore) EnsureCanonicalDemoProject(_ context.Context, actor contract.ActorContext, seed project.CanonicalDemoProject) (project.Project, error) {
	s.ensureProjectCalls++
	s.project = project.Project{
		ID: seed.ProjectID, OrganizationID: actor.OrganizationID, Name: seed.Name, Status: project.StatusActive,
		PrimaryBrandID: &seed.BrandID, PrimaryBrandStatus: "active", ProjectContextVersion: 1,
	}
	return s.project, nil
}

func (s *fakeInvestorProjectStore) UpsertProjectRuntime(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, runtime project.ProjectRuntime) error {
	s.runtime = runtime
	return nil
}
func (s *fakeInvestorProjectStore) UpsertWorkbench(_ context.Context, workbench project.Workbench) error {
	s.workbench = workbench
	return nil
}

func (s *fakeInvestorProjectStore) CreateBusinessTask(_ context.Context, task project.BusinessTask) error {
	s.ensureTaskMap(task.ProjectID)
	s.tasks[task.ProjectID][task.ID] = task
	return nil
}

func (s *fakeInvestorProjectStore) ListBusinessTasks(_ context.Context, _ contract.OrganizationID, projectID contract.ProjectID) ([]project.BusinessTask, error) {
	result := make([]project.BusinessTask, 0, len(s.tasks[projectID]))
	for _, task := range s.tasks[projectID] {
		result = append(result, task)
	}
	return result, nil
}

func (s *fakeInvestorProjectStore) UpdateBusinessTask(_ context.Context, task project.BusinessTask) error {
	s.ensureTaskMap(task.ProjectID)
	task.Version++
	s.tasks[task.ProjectID][task.ID] = task
	return nil
}

func (s *fakeInvestorProjectStore) CreateOperationalRecord(_ context.Context, record project.OperationalRecord) error {
	s.ensureOperationMap(record.ProjectID)
	s.operations[record.ProjectID][record.ID] = record
	return nil
}

func (s *fakeInvestorProjectStore) ListOperationalRecords(_ context.Context, _ contract.OrganizationID, projectID contract.ProjectID) ([]project.OperationalRecord, error) {
	result := make([]project.OperationalRecord, 0, len(s.operations[projectID]))
	for _, record := range s.operations[projectID] {
		result = append(result, record)
	}
	return result, nil
}

func (s *fakeInvestorProjectStore) UpdateOperationalRecord(_ context.Context, record project.OperationalRecord) error {
	s.ensureOperationMap(record.ProjectID)
	s.operations[record.ProjectID][record.ID] = record
	return nil
}

func (s *fakeInvestorProjectStore) DeleteOperationalRecord(_ context.Context, _ contract.OrganizationID, projectID contract.ProjectID, recordID string) error {
	delete(s.operations[projectID], recordID)
	return nil
}

func (s *fakeInvestorProjectStore) CreateChangeSet(_ context.Context, changeSet project.ChangeSet) error {
	s.ensureChangeSetMap(changeSet.ProjectID)
	s.changeSets[changeSet.ProjectID][changeSet.ID] = changeSet
	return nil
}

func (s *fakeInvestorProjectStore) ListChangeSets(_ context.Context, _ contract.OrganizationID, projectID contract.ProjectID) ([]project.ChangeSet, error) {
	result := make([]project.ChangeSet, 0, len(s.changeSets[projectID]))
	for _, changeSet := range s.changeSets[projectID] {
		result = append(result, changeSet)
	}
	return result, nil
}

func (s *fakeInvestorProjectStore) UpdateChangeSet(_ context.Context, changeSet project.ChangeSet) error {
	s.ensureChangeSetMap(changeSet.ProjectID)
	changeSet.Version++
	s.changeSets[changeSet.ProjectID][changeSet.ID] = changeSet
	return nil
}

func (s *fakeInvestorProjectStore) AppendAuditEvent(_ context.Context, event project.AuditEvent) error {
	if s.auditEvents[event.ProjectID] == nil {
		s.auditEvents[event.ProjectID] = map[string]project.AuditEvent{}
	}
	s.auditEvents[event.ProjectID][event.ID] = event
	return nil
}

func (s *fakeInvestorProjectStore) ListAuditEvents(_ context.Context, _ contract.OrganizationID, projectID contract.ProjectID) ([]project.AuditEvent, error) {
	result := make([]project.AuditEvent, 0, len(s.auditEvents[projectID]))
	for _, event := range s.auditEvents[projectID] {
		result = append(result, event)
	}
	return result, nil
}

func (s *fakeInvestorProjectStore) ensureTaskMap(projectID contract.ProjectID) {
	if s.tasks[projectID] == nil {
		s.tasks[projectID] = map[string]project.BusinessTask{}
	}
}

func (s *fakeInvestorProjectStore) ensureOperationMap(projectID contract.ProjectID) {
	if s.operations[projectID] == nil {
		s.operations[projectID] = map[string]project.OperationalRecord{}
	}
}

func (s *fakeInvestorProjectStore) ensureChangeSetMap(projectID contract.ProjectID) {
	if s.changeSets[projectID] == nil {
		s.changeSets[projectID] = map[string]project.ChangeSet{}
	}
}

func (s *fakeInvestorProjectStore) counts() map[string]int {
	totalTasks := 0
	for _, tasks := range s.tasks {
		totalTasks += len(tasks)
	}
	totalOperations := 0
	for _, operations := range s.operations {
		totalOperations += len(operations)
	}
	totalChangeSets := 0
	for _, changeSets := range s.changeSets {
		totalChangeSets += len(changeSets)
	}
	totalAudit := 0
	for _, events := range s.auditEvents {
		totalAudit += len(events)
	}
	return map[string]int{
		"tasks":       totalTasks,
		"operations":  totalOperations,
		"change_sets": totalChangeSets,
		"audit":       totalAudit,
	}
}
