package creative

import (
	"context"
	"errors"
	"testing"
)

func TestFreezeVersionCreatesImmutableSnapshotFromCurrentDraft(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-lifecycle-intake", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest())
	if err != nil {
		t.Fatal(err)
	}

	version, duplicate, err := service.FreezeVersion(context.Background(), rc, "project_1", task.ID, FreezeVersionRequest{
		DraftVersion: 1,
	}, "creative-freeze-1")
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		t.Fatal("first freeze must not be marked as an idempotent replay")
	}
	if version.Status != CreativeVersionCreated || version.Version != 1 || version.DraftVersion != 1 {
		t.Fatalf("version = %#v", version)
	}
	if version.ContentHash == "" || version.Snapshot.Body == "" || len(version.Snapshot.ImagePlan) != 4 {
		t.Fatalf("immutable snapshot was not persisted: %#v", version)
	}

	replayed, duplicate, err := service.FreezeVersion(context.Background(), rc, "project_1", task.ID, FreezeVersionRequest{
		DraftVersion: 1,
	}, "creative-freeze-1")
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate || replayed.ID != version.ID || replayed.ContentHash != version.ContentHash {
		t.Fatalf("idempotent replay = %#v, duplicate=%t", replayed, duplicate)
	}
}

func TestReviseDraftCreatesNextRevisionAndRejectsStaleWrite(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-revise-intake", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest())
	if err != nil {
		t.Fatal(err)
	}

	updated, err := service.ReviseDraft(context.Background(), rc.Actor, "project_1", task.ID, ReviseDraftRequest{
		ExpectedVersion: 1,
		TitleCandidates: []string{"夏日轻饮新提案", "午后清爽感", "一杯打开好心情"},
		Body:            "将产品放进真实的午后休息场景。",
		Topics:          []string{"#夏日饮品", "#午后时光"},
		CoverCopy:       "午后清爽感",
		ImagePlan:       []ImagePlanItem{{Order: 1, Purpose: "封面", VisualBrief: "窗边的清爽饮品", Caption: "午后清爽感"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.Body != "将产品放进真实的午后休息场景。" {
		t.Fatalf("updated draft=%+v", updated)
	}
	if _, err := service.ReviseDraft(context.Background(), rc.Actor, "project_1", task.ID, ReviseDraftRequest{ExpectedVersion: 1, TitleCandidates: updated.TitleCandidates, Body: updated.Body, Topics: updated.Topics, CoverCopy: updated.CoverCopy, ImagePlan: updated.ImagePlan}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale revision error=%v, want %v", err, ErrVersionConflict)
	}
}
