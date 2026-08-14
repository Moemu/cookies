package computeruse

import (
	"net/url"
	"strings"
	"unicode"
)

const redactedValue = "[REDACTED]"

var sensitiveEvidenceKeys = []string{
	"password", "passcode", "secret", "token", "cookie", "authorization",
	"session", "storage", "phone", "mobile", "email", "customer",
	"advertiser", "accountname", "accountbalance", "balance",
}

func RedactEvidence(value Evidence) Evidence {
	value.BeforePageFacts = redactFacts(value.BeforePageFacts)
	value.AfterPageFacts = redactFacts(value.AfterPageFacts)
	value.FieldReadback = redactFacts(value.FieldReadback)
	value.DiffKeys = redactDiffKeys(value.DiffKeys)
	value.PageReference = redactPageReference(value.PageReference)
	value.RedactionVersion = "computer-use-redaction/v1"
	return value
}

func redactFacts(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	redacted := make(map[string]string, len(values))
	for key, value := range values {
		if sensitiveEvidenceKey(key) {
			redacted[key] = redactedValue
			continue
		}
		redacted[key] = value
	}
	return redacted
}

func redactDiffKeys(values []string) []string {
	redacted := make([]string, len(values))
	for i, value := range values {
		if sensitiveEvidenceKey(value) {
			redacted[i] = "redacted_field"
		} else {
			redacted[i] = value
		}
	}
	return redacted
}

func sensitiveEvidenceKey(value string) bool {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
	for _, candidate := range sensitiveEvidenceKeys {
		if strings.Contains(normalized, candidate) {
			return true
		}
	}
	return false
}

func redactPageReference(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return value
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
