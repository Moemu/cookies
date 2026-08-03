package creative

import (
	"context"
	"testing"
)

func TestBrandFilmFixtureCompletesPersistentPhaseZeroToTwo(t *testing.T) {
	t.Parallel()
	service := testService()
	repository := service.Repository.(*memoryRepository)
	service.ViralRemakes = repository
	service.BrandFilmPlanner = DeterministicBrandFilmPlanner{}
	ctx := context.Background()
	rc := testRequestContext()

	workspace, err := service.EnsureBrandFilmFixtureWorkspace(ctx, rc, "project_1", "brand-film-fixture-1")
	if err != nil {
		t.Fatal(err)
	}
	if workspace.VideoDraft == nil || workspace.VideoDraft.BrandFilm == nil {
		t.Fatal("brand film draft was not created")
	}
	taskID := workspace.Task.ID
	if workspace.VideoDraft.BrandFilm.PromptSeam.ContractVersion != "creative-brand-generation-seam/v1" {
		t.Fatalf("generation seam = %#v", workspace.VideoDraft.BrandFilm.PromptSeam)
	}

	workspace, err = service.AnalyzeBrandFilmBrief(ctx, rc.Actor, "project_1", taskID, BrandFilmRevisionRequest{ExpectedRevision: workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	analysis := *workspace.VideoDraft.BrandFilm.CurrentAnalysis()
	if analysis.ModelAlias != "fixture.deterministic" || len(analysis.AssetCandidates) == 0 {
		t.Fatalf("analysis = %#v", analysis)
	}
	for index := range analysis.AssetCandidates {
		if analysis.AssetCandidates[index].Role == "product_front" {
			analysis.AssetCandidates[index].UserConfirmed = true
			analysis.AssetCandidates[index].RightsStatus = "user_confirmed"
		}
	}
	workspace, err = service.UpdateBrandFilmBrief(ctx, rc.Actor, "project_1", taskID, UpdateBrandBriefAnalysisRequest{
		ExpectedRevision: workspace.VideoDraft.Revision,
		Analysis:         analysis,
	})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err = service.ConfirmBrandFilmBrief(ctx, rc.Actor, "project_1", taskID, BrandFilmRevisionRequest{ExpectedRevision: workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err = service.GenerateBrandFilmConcepts(ctx, rc.Actor, "project_1", taskID, BrandFilmRevisionRequest{ExpectedRevision: workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	concepts := workspace.VideoDraft.BrandFilm.CurrentConceptSet()
	if concepts == nil || len(concepts.Candidates) != 3 || workspace.VideoDraft.BrandFilm.SelectedConceptID != "" {
		t.Fatalf("concepts = %#v", concepts)
	}
	workspace, err = service.SelectBrandFilmConcept(ctx, rc.Actor, "project_1", taskID, SelectBrandConceptRequest{
		ExpectedRevision: workspace.VideoDraft.Revision,
		ConceptID:        concepts.Candidates[0].ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err = service.GenerateBrandFilmPlan(ctx, rc.Actor, "project_1", taskID, BrandFilmRevisionRequest{ExpectedRevision: workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	plan := workspace.VideoDraft.BrandFilm.CurrentPlan()
	if plan == nil || len(plan.Shots) < 3 || plan.Shots[len(plan.Shots)-1].EndSecond != 15 {
		t.Fatalf("plan = %#v", plan)
	}
	workspace, err = service.ConfirmBrandFilmPlan(ctx, rc.Actor, "project_1", taskID, BrandFilmRevisionRequest{ExpectedRevision: workspace.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	brand := workspace.VideoDraft.BrandFilm
	if brand.Stage != BrandFilmPlanConfirmed || !brand.CurrentPlan().Confirmed || brand.Readiness.GenerationReady {
		t.Fatalf("completed workspace = %#v", brand)
	}
	if len(workspace.ProductionJobs) != 0 {
		t.Fatalf("Phase 0-2 must not create provider jobs: %#v", workspace.ProductionJobs)
	}

	restored, err := service.GetLatestBrandFilmWorkspace(ctx, rc.Actor, "project_1")
	if err != nil {
		t.Fatal(err)
	}
	if restored.VideoDraft.Revision != workspace.VideoDraft.Revision || restored.VideoDraft.BrandFilm.Stage != BrandFilmPlanConfirmed {
		t.Fatalf("restored workspace = %#v", restored.VideoDraft.BrandFilm)
	}
}
