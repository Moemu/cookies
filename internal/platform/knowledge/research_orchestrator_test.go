package knowledge

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNormalizeResearchFindingRequiresTwoIndependentlyVerifiedSupportingDomains(t *testing.T) {
	sources := map[string]ResearchSource{
		"https://one.example/report": {ID: "source_1"},
		"https://two.example/study":  {ID: "source_2"},
	}
	verified := map[verifiedEvidenceKey]VerifiedResearchSource{
		{CanonicalURL: "https://one.example/report", Excerpt: "evidence one"}: {ContentHash: "hash_1", ExcerptFound: true},
		{CanonicalURL: "https://two.example/study", Excerpt: "evidence two"}:  {ContentHash: "hash_2", ExcerptFound: true},
	}
	finding := normalizeResearchFinding(ExternalResearchFinding{
		Claim: "目标受众更常通过短视频发现新品", TimeScope: "2026-H1", Confidence: "high",
		TargetArtifact: "strategy", TargetFieldPath: "channel_strategy",
		Implication: "把短视频设为首触达测试渠道", ProposedValue: json.RawMessage(`{"primary":"short_video"}`),
		SupportingEvidence: []ExternalResearchEvidence{
			{URL: "https://one.example/report", Excerpt: "evidence one"},
			{URL: "https://two.example/study", Excerpt: "evidence two"},
		},
	}, 2, sources, verified)
	if finding.Status != "verified" || len(finding.SupportingSourceIDs) != 2 {
		t.Fatalf("finding = %#v", finding)
	}

	sameDomainSources := map[string]ResearchSource{
		"https://one.example/report": {ID: "source_1"},
		"https://one.example/copy":   {ID: "source_copy"},
	}
	sameDomainVerified := map[verifiedEvidenceKey]VerifiedResearchSource{
		{CanonicalURL: "https://one.example/report", Excerpt: "evidence one"}: {ContentHash: "hash_1", ExcerptFound: true},
		{CanonicalURL: "https://one.example/copy", Excerpt: "evidence copy"}:  {ContentHash: "hash_2", ExcerptFound: true},
	}
	finding = normalizeResearchFinding(ExternalResearchFinding{
		Claim: "转载形成了两个 URL", TargetArtifact: "brief", TargetFieldPath: "proposition",
		Implication: "不能把同域转载当成交叉验证",
		SupportingEvidence: []ExternalResearchEvidence{
			{URL: "https://one.example/report", Excerpt: "evidence one"},
			{URL: "https://one.example/copy", Excerpt: "evidence copy"},
		},
	}, 1, sameDomainSources, sameDomainVerified)
	if finding.Status != "tentative" {
		t.Fatalf("same-domain evidence was promoted: %#v", finding)
	}
}

func TestNormalizeResearchFindingSurfacesVerifiedCounterEvidenceAsConflict(t *testing.T) {
	sources := map[string]ResearchSource{
		"https://support.example/report": {ID: "source_support"},
		"https://counter.example/study":  {ID: "source_counter"},
	}
	verified := map[verifiedEvidenceKey]VerifiedResearchSource{
		{CanonicalURL: "https://support.example/report", Excerpt: "support"}: {ContentHash: "hash_1", ExcerptFound: true},
		{CanonicalURL: "https://counter.example/study", Excerpt: "counter"}:  {ContentHash: "hash_2", ExcerptFound: true},
	}
	finding := normalizeResearchFinding(ExternalResearchFinding{
		Claim: "短视频是首触达渠道", TargetArtifact: "strategy", TargetFieldPath: "channel_strategy",
		Implication:         "保留多渠道验证",
		SupportingEvidence:  []ExternalResearchEvidence{{URL: "https://support.example/report", Excerpt: "support"}},
		ConflictingEvidence: []ExternalResearchEvidence{{URL: "https://counter.example/study", Excerpt: "counter"}},
	}, 1, sources, verified)
	if finding.Status != "conflicting" || len(finding.ConflictingSourceIDs) != 1 {
		t.Fatalf("counter-evidence was hidden: %#v", finding)
	}
}

func TestResearchSourceVerificationConflictWinsAndUnverifiedCitationsStayModelCited(t *testing.T) {
	results := []ExternalResearchResult{{Findings: []ExternalResearchFinding{{
		SupportingEvidence: []ExternalResearchEvidence{
			{URL: "https://mixed.example/report", Excerpt: "support"},
			{URL: "https://unverified.example/report", Excerpt: "model only"},
		},
		ConflictingEvidence: []ExternalResearchEvidence{{URL: "https://mixed.example/report", Excerpt: "counter"}},
	}}}}
	verified := map[verifiedEvidenceKey]VerifiedResearchSource{
		{CanonicalURL: "https://mixed.example/report", Excerpt: "support"}: {ContentHash: "body_hash", ExcerptFound: true},
		{CanonicalURL: "https://mixed.example/report", Excerpt: "counter"}: {ContentHash: "body_hash", ExcerptFound: true},
	}
	values := researchSourceVerificationByURL(results, verified)
	if len(values) != 1 || values["https://mixed.example/report"].Status != "conflicted" {
		t.Fatalf("verification transitions = %#v", values)
	}
	if _, promoted := values["https://unverified.example/report"]; promoted {
		t.Fatal("model-only citation was promoted without excerpt verification")
	}
}

func TestResearchBudgetStopReasonUsesServerOwnedLimits(t *testing.T) {
	started := time.Date(2026, time.August, 10, 10, 0, 0, 0, time.UTC)
	run := ResearchRun{CurrentRound: 2, MaxRounds: 6, TimeBudgetSeconds: 900, TokenBudget: 100, StartedAt: &started,
		Usage: &ResearchUsage{TotalTokens: 100}}
	if reason := researchBudgetStopReason(run, started.Add(time.Minute)); reason != "token_budget" {
		t.Fatalf("reason = %q", reason)
	}
	run.Usage.TotalTokens = 99
	if reason := researchBudgetStopReason(run, started.Add(15*time.Minute)); reason != "time_budget" {
		t.Fatalf("reason = %q", reason)
	}
	run.CurrentRound = run.MaxRounds
	if reason := researchBudgetStopReason(run, started.Add(time.Second)); reason != "max_rounds" {
		t.Fatalf("reason = %q", reason)
	}
}

func TestResearchReportCitationAuditRequiresVerifiedMappedEvidence(t *testing.T) {
	findings := []ResearchFinding{{
		Status: "conflicting", Target: ResearchFindingTarget{Artifact: "strategy", FieldPath: "channel_strategy"},
		ProposedValue:       json.RawMessage(`{"primary":"short_video"}`),
		SupportingSourceIDs: []string{"source_support"}, ConflictingSourceIDs: []string{"source_counter"},
	}}
	sources := []ResearchSource{
		{ID: "source_support", VerificationStatus: "content_verified"},
		{ID: "source_counter", VerificationStatus: "conflicted"},
	}
	if !researchReportCitationAudit(findings, sources) {
		t.Fatal("verified support and counter-evidence failed citation audit")
	}
	sources[1].VerificationStatus = "model_cited"
	if researchReportCitationAudit(findings, sources) {
		t.Fatal("model-only counter-evidence passed citation audit")
	}
}
