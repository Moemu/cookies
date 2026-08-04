package delivery

import (
	"testing"
	"time"
)

func TestThreeTierFixturesAreDeterministicAndLayered(t *testing.T) {
	v := canonicalTestVersion(t)
	first, err := compileThreeTierFixture(v, ThreeTierFixtureGoldenPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := compileThreeTierFixture(v, ThreeTierFixtureGoldenPath)
	if err != nil {
		t.Fatal(err)
	}
	firstHash, _ := snapshotHash(first)
	secondHash, _ := snapshotHash(second)
	if firstHash != secondHash || len(first.Groups) != 1 || len(first.Groups[0].Plans) != 2 || len(first.Groups[0].Plans[0].Creatives) != 2 || len(first.Groups[0].Plans[1].Creatives) != 1 {
		t.Fatalf("not the frozen 1/2/3 fixture: %#v", first)
	}
	if len(first.Groups[0].Fields) == 0 || len(first.Groups[0].Plans[0].Fields) == 0 || len(first.Groups[0].Plans[0].Creatives[0].Fields) == 0 {
		t.Fatal("fields must exist at all three layers")
	}
	if first.Scenario != ThreeTierFixtureGoldenPath || first.FixtureScenario != ThreeTierFixtureGoldenPath {
		t.Fatalf("fixture scenario missing: %#v", first)
	}
	first.GeneratedAt = time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	checks := RunPreflight(DeliveryPlanVersion{ThreeTierConfiguration: first})
	if !preflightCheckPassed(checks, "three_tier_structure") {
		t.Fatal("golden fixture must pass explicit structure preflight")
	}
}

func preflightCheckPassed(checks []PreflightCheck, code string) bool {
	for _, check := range checks {
		if check.Code == code {
			return check.Passed
		}
	}
	return false
}

func TestThreeTierNegativeFixturesFailOnlyConfigurationPreflight(t *testing.T) {
	v := canonicalTestVersion(t)
	for fixture, code := range map[ThreeTierFixture]string{ThreeTierFixtureMissingRequiredField: "three_tier_required_fields", ThreeTierFixtureOrphanDependency: "three_tier_dependencies", ThreeTierFixtureMissingConfirmation: "three_tier_confirmation"} {
		c, err := compileThreeTierFixture(v, fixture)
		if err != nil {
			t.Fatal(err)
		}
		v.ThreeTierConfiguration = c
		found := false
		for _, check := range RunPreflight(v) {
			if check.Code == code && !check.Passed {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s did not fail %s", fixture, code)
		}
	}
}

func TestThreeTierConfigurationChangesButOmissionKeepsLegacyHash(t *testing.T) {
	v := canonicalTestVersion(t)
	legacy, err := PlanCanonicalHash(v)
	if err != nil {
		t.Fatal(err)
	}
	c, err := compileThreeTierFixture(v, ThreeTierFixtureGoldenPath)
	if err != nil {
		t.Fatal(err)
	}
	v.ThreeTierConfiguration = c
	withConfig, err := PlanCanonicalHash(v)
	if err != nil {
		t.Fatal(err)
	}
	if legacy == withConfig {
		t.Fatal("configuration must bind canonical hash")
	}
	v.ThreeTierConfiguration = nil
	restored, err := PlanCanonicalHash(v)
	if err != nil {
		t.Fatal(err)
	}
	if restored != legacy {
		t.Fatal("omitted configuration changed legacy hash")
	}
}
