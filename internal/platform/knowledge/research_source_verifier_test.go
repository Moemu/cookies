package knowledge

import (
	"net"
	"testing"
)

func TestPublicResearchIPRejectsSSRFAddressClasses(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254",
		"0.0.0.0", "::1", "fc00::1", "fe80::1", "224.0.0.1",
	} {
		if publicResearchIP(net.ParseIP(raw)) {
			t.Fatalf("%s must not be reachable by the research verifier", raw)
		}
	}
	if !publicResearchIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public global-unicast address should be accepted")
	}
}

func TestNormalizedVerificationTextMatchesAcrossMarkupWhitespace(t *testing.T) {
	t.Parallel()
	page := "增长率 <b>达到 18%</b>，且样本覆盖 2026 年上半年。"
	excerpt := "增长率达到18%，且样本覆盖2026年上半年"
	if !stringsContainsNormalized(page, excerpt) {
		t.Fatal("expected normalized excerpt match")
	}
}

func stringsContainsNormalized(page, excerpt string) bool {
	page = researchHTMLTag.ReplaceAllString(page, " ")
	return len(normalizedVerificationText(excerpt)) > 0 &&
		containsString(normalizedVerificationText(page), normalizedVerificationText(excerpt))
}

func containsString(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return true
		}
	}
	return false
}
