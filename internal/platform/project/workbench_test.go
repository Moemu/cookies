package project

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMergeWorkbenchAssetPointersKeepsReviewStateAndAddsAssetVersions(t *testing.T) {
	older := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	confirmed := 1
	persisted := []WorkbenchAssetVersionPointer{{
		ID: "pointer_1", AssetID: "asset_1", WorkingVersion: 1,
		QualityCheckedVersion: &confirmed, HumanConfirmedVersion: &confirmed,
		Versions:  []WorkbenchAssetVersion{{Version: 1, SourceLabel: "first"}},
		UpdatedAt: older,
	}}
	discovered := []WorkbenchAssetVersionPointer{{
		ID: "asset_1", AssetID: "asset_1", WorkingVersion: 2, Owner: "creative",
		Versions: []WorkbenchAssetVersion{
			{Version: 2, SourceLabel: "second"},
			{Version: 1, SourceLabel: "duplicate"},
		},
		UpdatedAt: newer,
	}}

	got := mergeWorkbenchAssetPointers(persisted, discovered)

	if len(got) != 1 {
		t.Fatalf("len=%d, want 1", len(got))
	}
	if got[0].WorkingVersion != 2 || got[0].QualityCheckedVersion == nil || got[0].HumanConfirmedVersion == nil {
		t.Fatalf("review state was not preserved while advancing working version: %#v", got[0])
	}
	if len(got[0].Versions) != 2 {
		t.Fatalf("versions=%#v, want two unique versions", got[0].Versions)
	}
	if got[0].UpdatedAt != newer || got[0].Owner != "creative" {
		t.Fatalf("discovered metadata was not merged: %#v", got[0])
	}
}

func TestMergeWorkbenchAssetPointersIncludesAssetsWithoutReviewRows(t *testing.T) {
	discovered := []WorkbenchAssetVersionPointer{{
		ID: "asset_2", AssetID: "asset_2", WorkingVersion: 1,
		Versions: []WorkbenchAssetVersion{{Version: 1}},
	}}

	got := mergeWorkbenchAssetPointers(nil, discovered)

	if len(got) != 1 || got[0].AssetID != "asset_2" {
		t.Fatalf("got=%#v, want discovered asset pointer", got)
	}
}

func TestWorkbenchReviewFlowPersistsQualityAndConfirmation(t *testing.T) {
	store := &stubProjectStore{workbench: Workbench{
		Project: WorkbenchProject{ProjectID: "project_1", OrganizationID: "org_1"},
		AssetVersionPointers: []WorkbenchAssetVersionPointer{{
			ID: "asset_1", AssetID: "asset_1", WorkingVersion: 1,
			Versions: []WorkbenchAssetVersion{{Version: 1}},
		}},
		QualityCheckRuns: []WorkbenchQualityCheckRun{}, MaterialConfirmations: []WorkbenchMaterialConfirmation{},
	}}
	service := Service{
		Store:      store,
		Authorizer: allowingAuthorizer{},
		NewID: func(prefix string) (string, error) {
			return prefix + "_1", nil
		},
	}
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
	}

	run, err := service.RunWorkbenchQualityCheck(context.Background(), actor, "project_1", RunWorkbenchQualityCheckRequest{AssetID: "asset_1", AssetVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "passed" || store.saved.AssetVersionPointers[0].QualityCheckedVersion == nil {
		t.Fatalf("quality check was not persisted: %#v", store.saved)
	}
	duplicateRun, err := service.RunWorkbenchQualityCheck(context.Background(), actor, "project_1", RunWorkbenchQualityCheckRequest{AssetID: "asset_1", AssetVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if duplicateRun.ID != run.ID || len(store.saved.QualityCheckRuns) != 1 {
		t.Fatalf("quality check retry created a duplicate: %#v", store.saved.QualityCheckRuns)
	}

	confirmation, err := service.RecordWorkbenchMaterialConfirmation(context.Background(), actor, "project_1", RecordWorkbenchMaterialConfirmationRequest{
		AssetID: "asset_1", AssetVersion: 1, Status: "confirmed", Scope: "project_1", Note: "ready",
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmation.ConfirmedBy != "user_1" || store.saved.AssetVersionPointers[0].HumanConfirmedVersion == nil {
		t.Fatalf("confirmation was not persisted: %#v", store.saved)
	}
	duplicateConfirmation, err := service.RecordWorkbenchMaterialConfirmation(context.Background(), actor, "project_1", RecordWorkbenchMaterialConfirmationRequest{
		AssetID: "asset_1", AssetVersion: 1, Status: "confirmed", Scope: "project_1", Note: "ready",
	})
	if err != nil {
		t.Fatal(err)
	}
	if duplicateConfirmation.ID != confirmation.ID || len(store.saved.MaterialConfirmations) != 1 {
		t.Fatalf("confirmation retry created a duplicate: %#v", store.saved.MaterialConfirmations)
	}
}

type allowingAuthorizer struct{}

func (allowingAuthorizer) AuthorizeProject(context.Context, contract.ActorContext, contract.ProjectID) error {
	return nil
}
