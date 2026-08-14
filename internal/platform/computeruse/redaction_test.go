package computeruse

import "testing"

func TestRedactEvidenceRemovesSensitivePageData(t *testing.T) {
	value := RedactEvidence(Evidence{
		BeforePageFacts: map[string]string{"advertiser_name": "Acme", "daily_budget": "100"},
		AfterPageFacts:  map[string]string{"customerEmail": "ops@example.com", "phone": "13800138000"},
		FieldReadback:   map[string]string{"authorization_token": "Bearer secret", "project_id": "project_1"},
		DiffKeys:        []string{"phone", "daily_budget"},
		PageReference:   "https://user:password@example.com/form?access_token=secret#customer",
	})
	for _, facts := range []map[string]string{value.BeforePageFacts, value.AfterPageFacts, value.FieldReadback} {
		for key, got := range facts {
			if sensitiveEvidenceKey(key) && got != redactedValue {
				t.Fatalf("sensitive %s=%q", key, got)
			}
		}
	}
	if value.BeforePageFacts["daily_budget"] != "100" || value.FieldReadback["project_id"] != "project_1" {
		t.Fatalf("non-sensitive facts changed: %#v %#v", value.BeforePageFacts, value.FieldReadback)
	}
	if value.DiffKeys[0] != "redacted_field" || value.PageReference != "https://example.com/form" || value.RedactionVersion != "computer-use-redaction/v1" {
		t.Fatalf("redaction metadata=%#v", value)
	}
}
