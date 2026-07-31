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

func TestRegistrySelectsEverySupportedPlatformSkill(t *testing.T) {
	t.Parallel()
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	values := registry.Select(
		[]string{"xiaohongshu", "douyin", "taobao_tmall", "wechat_ecosystem"},
		"新品认知",
	)
	names := map[string]bool{}
	for _, value := range values {
		names[value.Name] = true
	}
	for _, expected := range []string{
		"channel.xiaohongshu",
		"channel.douyin",
		"channel.taobao_tmall",
		"channel.wechat_ecosystem",
		"objective.awareness",
	} {
		if !names[expected] {
			t.Fatalf("missing %s in selected skills: %#v", expected, names)
		}
	}
}

func TestRegistryListsStableDescriptors(t *testing.T) {
	t.Parallel()
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	descriptors := registry.List(false)
	if len(descriptors) != 14 {
		t.Fatalf("descriptor count = %d, want 14", len(descriptors))
	}
	for _, descriptor := range descriptors {
		if descriptor.Name == "" || descriptor.Version == "" || descriptor.ContentHash == "" {
			t.Fatalf("incomplete descriptor: %#v", descriptor)
		}
		if len(descriptor.Instructions) != 0 {
			t.Fatalf("instructions must be hidden by default: %#v", descriptor)
		}
		if len(descriptor.Match) == 0 || len(descriptor.QualityChecks) == 0 {
			t.Fatalf("descriptor lacks matching or quality checks: %#v", descriptor)
		}
	}
	withInstructions := registry.List(true)
	if len(withInstructions) != len(descriptors) || len(withInstructions[0].Instructions) == 0 {
		t.Fatal("authorized descriptor listing must include instructions")
	}
}

func TestCreativeTaskSkillsAreExplicitAndDoNotChangeDefaultSelection(t *testing.T) {
	t.Parallel()
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	values := registry.Select([]string{"xiaohongshu"}, "新品认知")
	if len(values) != 2 || values[0].Name != "channel.xiaohongshu" ||
		values[1].Name != "objective.awareness" {
		t.Fatalf("default selection changed after creative skills: %#v", values)
	}
	creative, err := registry.SelectCreativeTask("xiaohongshu_image_text")
	if err != nil {
		t.Fatal(err)
	}
	if creative.Name != "creative_task.xiaohongshu_image_text" ||
		creative.Version != "v1.0.0" || creative.ContentHash == "" {
		t.Fatalf("unexpected creative skill: %#v", creative)
	}
}

func TestCreativeTaskSkillRequiresExactBusinessCode(t *testing.T) {
	t.Parallel()
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.SelectCreativeTask("not-a-business"); err == nil {
		t.Fatal("unknown business code must not select a creative skill")
	}
}
