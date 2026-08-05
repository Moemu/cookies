package creativecatalog

import "testing"

func TestDefaultRegistryLoadsSevenCurrentProfiles(t *testing.T) {
	t.Parallel()
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	current := registry.Current()
	if len(current) != 7 {
		t.Fatalf("current profiles=%d, want 7", len(current))
	}
	if hash, err := registry.CatalogHash(); err != nil || len(hash) != 71 {
		t.Fatalf("catalog hash=%q err=%v", hash, err)
	}
	for _, profile := range current {
		if profile.ContentHash == "" || profile.SkillContentHash == "" || !profile.Selectable {
			t.Fatalf("incomplete current profile: %#v", profile)
		}
	}
}

func TestRegistryFindsBusinessByExactCode(t *testing.T) {
	t.Parallel()
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, found := registry.FindCurrent("commerce_preroll")
	if !found || profile.DisplayName != "电商前贴" {
		t.Fatalf("profile=%#v found=%v", profile, found)
	}
	if _, found := registry.FindCurrent("commerce"); found {
		t.Fatal("partial business code must not match")
	}
}
