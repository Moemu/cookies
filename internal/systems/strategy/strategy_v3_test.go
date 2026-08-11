package strategy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestUpgradeStrategyDocumentToV3CreatesSuccessorWithoutMutatingLegacy(t *testing.T) {
	t.Parallel()
	brief := strategyV3TestBrief()
	legacy := deterministicStrategy(brief, Draft{ProjectContextVersion: 3})
	legacyDirections := legacy.CreativeDirections()
	legacy.ContractVersion = "strategy-draft/v2"
	legacy.CreativeRecommendations = legacyDirections
	legacy.CreativeStrategy = nil
	legacy.Compliance = &ComplianceReport{
		ContractVersion: "strategy-compliance-report/v1", Passed: true,
		Issues: []ComplianceIssue{}, CheckedAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}
	before, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	upgraded, err := UpgradeStrategyDocumentToV3(legacy, brief, time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	afterLegacy, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(afterLegacy) {
		t.Fatal("successor conversion mutated the legacy snapshot")
	}
	if upgraded.ContractVersion != StrategyDraftContractV3 || upgraded.CreativeStrategy == nil ||
		len(upgraded.CreativeRecommendations) != 0 || len(upgraded.CreativeStrategy.Territories) != 3 {
		t.Fatalf("upgraded strategy = %#v", upgraded)
	}
	if err := upgraded.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, territory := range upgraded.CreativeStrategy.Territories {
		if len(territory.ChannelAdaptations) != len(brief.Snapshot.Channels) {
			t.Fatalf("territory does not cover every channel: %#v", territory)
		}
	}
}

func TestReadOnlyStrategyDecoderAcceptsFrozenContractsAndRejectsUnknownOnes(t *testing.T) {
	t.Parallel()
	brief := strategyV3TestBrief()
	v3 := deterministicStrategy(brief, Draft{ProjectContextVersion: 3})
	v3.Compliance = &ComplianceReport{ContractVersion: "strategy-compliance-report/v1", Passed: true, Issues: []ComplianceIssue{}, CheckedAt: time.Now().UTC()}
	v2 := v3
	legacyDirections := v3.CreativeDirections()
	v2.ContractVersion = "strategy-draft/v2"
	v2.CreativeRecommendations = legacyDirections
	v2.CreativeStrategy = nil
	for _, document := range []StrategyDocument{v2, v3} {
		raw, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeStrategyDocumentReadOnly(raw)
		if err != nil || decoded.ContractVersion != document.ContractVersion {
			t.Fatalf("decoded=%#v err=%v", decoded, err)
		}
	}
	unknown := json.RawMessage(strings.Replace(string(mustJSON(v3)), StrategyDraftContractV3, "strategy-draft/v99", 1))
	if _, err := DecodeStrategyDocumentReadOnly(unknown); err == nil {
		t.Fatal("unknown Strategy contract was accepted")
	}
}

func TestReadOnlyPackageDecoderRequiresMatchingStrategyContract(t *testing.T) {
	t.Parallel()
	brief := strategyV3TestBrief()
	document := deterministicStrategy(brief, Draft{ProjectContextVersion: 3})
	document.Compliance = &ComplianceReport{ContractVersion: "strategy-compliance-report/v1", Passed: true, Issues: []ComplianceIssue{}, CheckedAt: time.Now().UTC()}
	snapshot := PackageSnapshot{ContractVersion: "strategy-package/v3", Strategy: document}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodePackageSnapshotReadOnly(raw); err != nil {
		t.Fatal(err)
	}
	snapshot.ContractVersion = "strategy-package/v2"
	raw, _ = json.Marshal(snapshot)
	if _, err := decodePackageSnapshotReadOnly(raw); err == nil {
		t.Fatal("mismatched package and strategy contracts were accepted")
	}
}

func strategyV3TestBrief() BriefVersion {
	return BriefVersion{
		BriefID: "brief_v3", Version: 1,
		Snapshot: BriefDocument{
			ContractVersion: "strategy-brief-version/v2",
			Campaign:        BriefCampaign{Objective: "获取有效线索"},
			Audience:        BriefAudience{Primary: "制造企业研发负责人"},
			Proposition:     "用可验证的协作证据降低决策风险",
			Channels:        []string{"xiaohongshu", "douyin"},
			Measurement:     BriefMeasurement{PrimaryKPI: "有效线索数"},
			Creative:        BriefCreative{MandatoryElements: []string{"说明证据口径"}, ProhibitedClaims: []string{"行业第一"}},
			ReferenceIDs:    []string{"finding_01"},
		},
	}
}
