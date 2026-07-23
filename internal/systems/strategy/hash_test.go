package strategy

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestPackageFixtureUsesCrossModuleHashRule(t *testing.T) {
	t.Parallel()
	contents, err := os.ReadFile("../../../api/fixtures/strategy-package-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(contents, &fixture); err != nil {
		t.Fatal(err)
	}
	approval, ok := fixture["approval"].(map[string]any)
	if !ok {
		t.Fatal("fixture approval is missing")
	}
	stored, _ := approval["content_hash"].(string)
	delete(approval, "content_hash")
	calculated, err := contract.NewContentHash(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if stored != string(calculated) {
		t.Fatalf("fixture content_hash = %q, want %q", stored, calculated)
	}
}

func TestPackageHashExcludesApprovalHashAndDetectsMutation(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	hash, err := PackageContentHash(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Approval.ContentHash = hash
	if err := VerifyPackageContentHash(snapshot); err != nil {
		t.Fatalf("verify original: %v", err)
	}
	snapshot.Strategy.Objective = "mutated"
	if err := VerifyPackageContentHash(snapshot); err == nil {
		t.Fatal("mutated package unexpectedly verified")
	}
}

func TestPackageHashGoldenVector(t *testing.T) {
	t.Parallel()
	hash, err := PackageContentHash(packageHashFixture())
	if err != nil {
		t.Fatal(err)
	}
	const want contract.ContentHash = "sha256:be0ecb262e8ae5773b66533471aedef71b2d44920c32bd788bd3a4b41d4f667d"
	if hash != want {
		t.Fatalf("hash = %q, want %q", hash, want)
	}
}

func packageHashFixture() PackageSnapshot {
	confirmedAt := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	return PackageSnapshot{
		ContractVersion: "strategy-package/v1", PackageID: "package_1", PackageVersion: 1,
		OrganizationID: "org_1", ProjectID: "project_1", StrategyID: "strategy_1", StrategyRevision: 1,
		Brief: BriefVersion{
			BriefID: "brief_1", Version: 1, OrganizationID: "org_1", ProjectID: "project_1",
			Snapshot: BriefDocument{
				ContractVersion: "strategy-brief-version/v1", Campaign: BriefCampaign{Objective: "认知"},
				Audience: BriefAudience{Primary: "研发负责人"}, Proposition: "提效",
				Channels: []string{"xiaohongshu"}, Constraints: []string{},
			},
			FieldStates: map[string]FieldState{}, ContentHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			SourceDraftID: "draft_1", SourceDraftVersion: 2, ConfirmedBy: "user_1", ConfirmedAt: confirmedAt,
		},
		Strategy: StrategyDocument{
			ContractVersion: "strategy-draft/v1", Objective: "认知",
			Audience:                StrategyAudience{Primary: "研发负责人", Insights: []string{}},
			Proposition:             "提效",
			ChannelStrategy:         []ChannelStrategy{{Platform: "xiaohongshu", Role: "种草", Formats: []string{"图文"}}},
			CreativeRecommendations: []string{"案例"}, Constraints: []string{},
			BudgetAndCadence: BudgetAndCadence{}, ExperimentMatrix: []Experiment{},
			Measurement: []string{}, AssumptionsAndGaps: []string{},
			Lineage: StrategyLineage{BriefID: "brief_1", BriefVersion: 1, ProjectContextVersion: 1, SkillVersions: map[string]string{"strategy.strategy.generate": "v1.0.0"}},
		},
		Readiness: Readiness{PublishBlockers: []ValidationError{}, CreativeReady: true},
		Approval:  PackageApproval{ReviewID: "review_1", ApprovedBy: "user_1", ApprovedAt: confirmedAt},
	}
}
