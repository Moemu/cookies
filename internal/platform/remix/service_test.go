package remix

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestServiceCreatesAndReadsScopedPlan(t *testing.T) {
	t.Parallel()
	service := NewMemoryService(func() (string, error) { return "remixplan_1", nil })
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}

	created, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != "remixplan_1" || created.ProjectID != "project_1" || created.OrganizationID != "org_1" {
		t.Fatalf("created plan = %#v", created)
	}

	got, err := service.Get(context.Background(), actor, "project_1", created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ClientPlanID != "client_plan_1" || len(got.Segments) != 3 {
		t.Fatalf("got plan = %#v", got)
	}

	otherActor := actor
	otherActor.OrganizationID = "org_2"
	if _, err := service.Get(context.Background(), otherActor, "project_1", created.ID); err != ErrNotFound {
		t.Fatalf("cross-org Get() error = %v, want ErrNotFound", err)
	}
}

func TestServiceListsProjectPlansNewestFirst(t *testing.T) {
	t.Parallel()
	nextID := 0
	service := NewMemoryService(func() (string, error) {
		nextID++
		return "remixplan_" + string(rune('0'+nextID)), nil
	})
	now := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	service.nowUTC = func() time.Time {
		now = now.Add(time.Minute)
		return now
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}

	first, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	secondRequest := validCreatePlanRequest()
	secondRequest.ClientPlanID = "client_plan_2"
	second, err := service.Create(context.Background(), actor, "project_1", secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Create(context.Background(), actor, "project_2", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}

	plans, err := service.List(context.Background(), actor, "project_1", 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(plans) != 2 || plans[0].ID != second.ID || plans[1].ID != first.ID {
		t.Fatalf("plans = %#v", plans)
	}

	limited, err := service.List(context.Background(), actor, "project_1", 1)
	if err != nil {
		t.Fatalf("List(limit=1) error = %v", err)
	}
	if len(limited) != 1 || limited[0].ID != second.ID {
		t.Fatalf("limited = %#v", limited)
	}
}

func TestServiceCreatesRenderJobForExistingPlan(t *testing.T) {
	t.Parallel()
	nextID := 0
	service := NewMemoryService(func() (string, error) {
		nextID++
		if nextID == 1 {
			return "remixplan_1", nil
		}
		return "remixrender_1", nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	plan, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}

	job, err := service.CreateRenderJob(context.Background(), actor, "project_1", CreateRenderJobRequest{PlanID: plan.ID, TargetQuality: "draft"})
	if err != nil {
		t.Fatalf("CreateRenderJob() error = %v", err)
	}
	if job.ID != "remixrender_1" || job.Status != RenderQueued || job.TargetFormat != "mp4" || job.TargetQuality != "draft" {
		t.Fatalf("job = %#v", job)
	}

	got, err := service.GetRenderJob(context.Background(), actor, "project_1", job.ID)
	if err != nil {
		t.Fatalf("GetRenderJob() error = %v", err)
	}
	if got.PlanID != plan.ID {
		t.Fatalf("got job = %#v", got)
	}

	if _, err := service.CreateRenderJob(context.Background(), actor, "project_1", CreateRenderJobRequest{PlanID: "missing"}); err != ErrNotFound {
		t.Fatalf("missing plan error = %v, want ErrNotFound", err)
	}
}

func TestCreatePlanRequestRejectsNonVideoClips(t *testing.T) {
	t.Parallel()
	request := validCreatePlanRequest()
	request.Segments[0].Clips[0].MimeType = "image/png"

	if err := request.Validate(); err == nil {
		t.Fatal("Validate() succeeded for image clip")
	}
}

func validCreatePlanRequest() CreatePlanRequest {
	return CreatePlanRequest{
		ClientPlanID:  "client_plan_1",
		TargetSeconds: 30,
		ActualSeconds: 14.4,
		Pace:          PaceBalanced,
		Segments: []SegmentPlan{
			validSegment(SegmentOpening, "前段", "asset_opening"),
			validSegment(SegmentMiddle, "中段", "asset_middle"),
			validSegment(SegmentEnding, "后段", "asset_ending"),
		},
		Warnings: []string{"duration is estimated"},
		Summary:  PlanSummary{SelectedAssets: 3, UsedAssets: 3, CoveragePercent: 48, Strategy: "balanced"},
	}
}

func validSegment(segment Segment, label string, assetID contract.AssetID) SegmentPlan {
	return SegmentPlan{
		Segment:       segment,
		Label:         label,
		TargetSeconds: 10,
		ActualSeconds: 4.8,
		Clips: []Clip{{
			ID:              string(segment) + "_clip_1",
			Segment:         segment,
			AssetVersion:    contract.AssetVersionRef{AssetID: assetID, Version: 1},
			Label:           string(assetID) + " · v1",
			SourceType:      "upload",
			MimeType:        "video/mp4",
			Aspect:          "vertical",
			StartSeconds:    0,
			DurationSeconds: 4.8,
			InPointSeconds:  0,
			OutPointSeconds: 4.8,
			Score:           0.8,
			Reason:          "test",
		}},
	}
}
