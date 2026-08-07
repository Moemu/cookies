package strategy

import "testing"

func TestComputeFullStrategyReadinessReportsMissingProposition(t *testing.T) {
	document := EmptyBriefDocumentV2()
	document.Campaign.Objective = "建立新品认知"
	document.Audience.Primary = "高端护肤用户"
	document.Channels = []string{"xiaohongshu", "douyin"}
	states := map[string]FieldState{
		"campaign.objective": {Confirmation: "confirmed"},
		"audience.primary":   {Confirmation: "confirmed"},
		"channels":           {Confirmation: "confirmed"},
	}

	readiness := computeFullStrategyReadiness(document, states)
	if readiness.Ready {
		t.Fatal("expected incomplete full strategy readiness")
	}
	if len(readiness.Blockers) != 1 || readiness.Blockers[0].Field != "proposition" {
		t.Fatalf("blockers = %#v, want proposition only", readiness.Blockers)
	}
}

func TestBriefReadinessMetadataDoesNotChangePackageHash(t *testing.T) {
	snapshot := packageHashFixture()
	before, err := PackageContentHash(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	readiness := Completeness{Ready: false, Blockers: []ValidationError{{Field: "proposition", Reason: "完整策略需要该信息"}}}
	snapshot.Brief.FullStrategyReadiness = &readiness
	after, err := PackageContentHash(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !before.Equal(after) {
		t.Fatalf("runtime readiness metadata changed immutable package hash: before=%s after=%s", before, after)
	}
}

func TestSupplementRevisionCannotFreezeWithMissingFullStrategyInput(t *testing.T) {
	base := int64(1)
	document := EmptyBriefDocumentV2()
	document.Product.Name = "第三代黄金复原蜜"
	document.Campaign.Objective = "新品认知"
	document.Audience.Primary = "高端护肤用户"
	document.Channels = []string{"xiaohongshu", "douyin"}
	states := map[string]FieldState{
		"product.name":       {Confirmation: "confirmed"},
		"campaign.objective": {Confirmation: "confirmed"},
		"audience.primary":   {Confirmation: "confirmed"},
		"channels":           {Confirmation: "confirmed"},
	}
	draft := BriefDraft{BaseBriefVersion: &base, Document: document, FieldStates: states}

	problems := briefConfirmationProblems(draft)
	if len(problems) != 1 || problems[0].Field != "proposition" {
		t.Fatalf("confirmation problems = %#v, want proposition only", problems)
	}
}
