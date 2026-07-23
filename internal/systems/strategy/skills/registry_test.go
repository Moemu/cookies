package skills

import "testing"

func TestDefaultRegistrySelectsPlatformAndObjectiveSkills(t *testing.T) {
	t.Parallel()
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	values := registry.Select([]string{"xiaohongshu"}, "获取 B2B 销售线索")
	if len(values) != 2 {
		t.Fatalf("selected %d skills, want 2: %#v", len(values), values)
	}
	if values[0].ContentHash == "" || values[1].ContentHash == "" {
		t.Fatal("selected skills must carry immutable content hashes")
	}
	if values[0].Name != "channel.xiaohongshu" {
		t.Fatalf("platform skill must be applied first: %#v", values)
	}
	names := map[string]bool{values[0].Name: true, values[1].Name: true}
	if !names["channel.xiaohongshu"] || !names["objective.lead_generation"] {
		t.Fatalf("unexpected selected skills: %#v", names)
	}
}

func TestRegistryDefaultsUnknownObjectiveToAwareness(t *testing.T) {
	t.Parallel()
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	values := registry.Select(nil, "新品发布")
	if len(values) != 1 || values[0].Name != "objective.awareness" {
		t.Fatalf("selected skills = %#v", values)
	}
}
