package skills

import (
	"strings"
	"testing"
)

// 六类素材各要有一份 Skill，一份不多一份不少。
//
// 类型串在这里是硬编码的，因为 insights 包 import 了 skills，反过来 import 会成环。
// 权威定义在 internal/systems/insights/features.go:19-24——那边加一类素材，
// 这条测试会失败，提醒的正是「新素材类型没有提取指令」这件事：不加的话，
// 那类素材要等到有人点了提取按钮才会发现没 Skill，而那时他已经在等结果了。
func TestEveryAssetTypeHasExactlyOneSkill(t *testing.T) {
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("加载内嵌 Skill 失败：%v", err)
	}
	want := []string{
		"brand_ad", "digital_human_ad", "hit_replica_ad",
		"preroll_ad", "wechat_article", "xiaohongshu_note",
	}
	all := registry.All()
	if len(all) != len(want) {
		t.Fatalf("有 %d 份 Skill，素材类型有 %d 种", len(all), len(want))
	}
	for _, assetType := range want {
		if _, ok := registry.For(assetType); !ok {
			t.Errorf("素材类型 %s 没有提取 Skill", assetType)
		}
	}
}

// All() 按素材类型排序返回。顺序固定不是为了好看：能力运营的 Skills 视图直接照这个
// 顺序渲染，顺序随文件遍历变的话，同一份数据每次刷新排法都不一样。
func TestAllReturnsAStableOrder(t *testing.T) {
	registry := mustRegistry(t)
	// 排序键是 assetType，Snapshot 上没有这个字段，所以直接看注册表内部那一份。
	previous := ""
	for _, skill := range registry.ordered {
		if skill.AssetType <= previous {
			t.Fatalf("%s 排在 %s 后面，没有按素材类型排序", skill.AssetType, previous)
		}
		previous = skill.AssetType
	}
	first, second := registry.All(), registry.All()
	for index := range first {
		if first[index].Name != second[index].Name {
			t.Fatalf("两次 All() 顺序不同：第 %d 个是 %s / %s", index, first[index].Name, second[index].Name)
		}
	}
}

// ContentHash 存在的全部理由：**版本号是人手写的，会忘记改**。改了指令却没动版本号，
// 库里两批结果会顶着同一个版本号，而它们其实不可比。所以指令、persona、复核重点里
// 任何一个字变了，哈希都必须跟着变。
func TestContentHashCoversEveryFieldThatShapesOrExplainsTheOutput(t *testing.T) {
	base := Skill{
		Name: "insight.extract.demo", Version: "v1.0.0", AssetType: "demo",
		Persona:      "你是分析员。",
		Instructions: []string{"只填看得出来的字段。"},
		ReviewFocus:  []string{"类型最容易判错。"},
	}
	original := base.snapshot().ContentHash

	for name, mutate := range map[string]func(Skill) Skill{
		"改 persona": func(s Skill) Skill { s.Persona = "你是审核员。"; return s },
		"改一句指令":     func(s Skill) Skill { s.Instructions = []string{"看不出来就猜一个。"}; return s },
		"加一句指令":     func(s Skill) Skill { s.Instructions = append(s.Instructions, "补一条。"); return s },
		"改复核重点":     func(s Skill) Skill { s.ReviewFocus = []string{"别的地方容易错。"}; return s },
		"只改版本号":     func(s Skill) Skill { s.Version = "v1.0.1"; return s },
	} {
		if mutate(base).snapshot().ContentHash == original {
			t.Errorf("%s 之后内容哈希没变——那两批结果会被当成同一份指令产出的", name)
		}
	}

	// 反过来：什么都不改，两次算出来必须一样，否则哈希没法用来比对。
	if base.snapshot().ContentHash != original {
		t.Error("同一份 Skill 两次算出的哈希不同")
	}
}

// Snapshot 会被写进 AnalysisRun 留痕。它必须是一份拷贝：留痕被改动过，
// 「这批结果是按哪份指令产出的」这个问题就再也答不准了。
func TestSnapshotDoesNotShareSlicesWithTheRegistry(t *testing.T) {
	registry := mustRegistry(t)
	snapshot, ok := registry.For("wechat_article")
	if !ok {
		t.Fatal("公众号图文没有 Skill")
	}
	if len(snapshot.Instructions) == 0 || len(snapshot.ReviewFocus) == 0 {
		t.Fatal("快照里的指令或复核重点是空的")
	}
	snapshot.Instructions[0] = "被改掉了"
	snapshot.ReviewFocus[0] = "被改掉了"

	again, _ := registry.For("wechat_article")
	if again.Instructions[0] == "被改掉了" || again.ReviewFocus[0] == "被改掉了" {
		t.Error("快照和注册表共享了底层数组，改一份会连带改掉另一份")
	}
}

// 缺字段的 Skill 必须整个拒绝，不能带着残缺进注册表。review_focus 尤其：
// 它空着等于对复核的人说「随便看看」，而人工复核是这套系统唯一的质量闸门。
func TestValidateRejectsSkillsThatWouldSilentlyDegradeExtraction(t *testing.T) {
	complete := Skill{
		Name: "n", Version: "v1", AssetType: "demo",
		Persona: "p", Instructions: []string{"i"}, ReviewFocus: []string{"r"},
	}
	if err := complete.Validate(); err != nil {
		t.Fatalf("完整的 Skill 被拒了：%v", err)
	}
	for name, broken := range map[string]Skill{
		"缺 name":          func() Skill { s := complete; s.Name = ""; return s }(),
		"缺 version":       func() Skill { s := complete; s.Version = " "; return s }(),
		"缺 asset_type":    func() Skill { s := complete; s.AssetType = ""; return s }(),
		"缺 persona":       func() Skill { s := complete; s.Persona = ""; return s }(),
		"指令为空":            func() Skill { s := complete; s.Instructions = nil; return s }(),
		"review_focus 为空": func() Skill { s := complete; s.ReviewFocus = nil; return s }(),
	} {
		if err := broken.Validate(); err == nil {
			t.Errorf("%s 的 Skill 通过了校验", name)
		}
	}
}

// 每一份内嵌 Skill 都要过同一道校验。这条不是重复 DefaultRegistry 的错误处理——
// 它保证的是「仓库里现有的六份此刻确实合法」，而不是「不合法时会报错」。
func TestShippedSkillsAreAllValid(t *testing.T) {
	for _, snapshot := range mustRegistry(t).All() {
		if strings.TrimSpace(snapshot.Persona) == "" {
			t.Errorf("%s 没有 persona", snapshot.Name)
		}
		if len(snapshot.Instructions) == 0 {
			t.Errorf("%s 没有提取指令", snapshot.Name)
		}
		if len(snapshot.ReviewFocus) == 0 {
			t.Errorf("%s 没有复核重点", snapshot.Name)
		}
		if strings.TrimSpace(snapshot.ContentHash) == "" {
			t.Errorf("%s 没有内容哈希，留痕就回溯不到具体指令", snapshot.Name)
		}
	}
}

// Skill 里不写输出格式——输出格式由 features.go 的特征体系生成（extraction_schema.go）。
// 两处各写一份的话，加一个特征字段就要改两个地方，而漏改的那次不会报错，
// 只会安静地少提一个字段。这条盯着指令文本里别混进 JSON 结构。
func TestSkillsDoNotRestateTheOutputSchema(t *testing.T) {
	for _, snapshot := range mustRegistry(t).All() {
		for _, line := range append(append([]string{snapshot.Persona}, snapshot.Instructions...), snapshot.ReviewFocus...) {
			if strings.Contains(line, "{") || strings.Contains(line, "\": ") {
				t.Errorf("%s 的指令里出现了 JSON 结构片段：%q——输出格式只能有一处来源", snapshot.Name, line)
			}
		}
	}
}

// 没有 Skill 的素材类型要明确返回「没有」，而不是给一份空的。拿着空 persona 和
// 空指令去调模型，会得到一份看上去正常、实际没有任何约束的结果。
func TestUnknownAssetTypeIsReportedAsMissing(t *testing.T) {
	registry := mustRegistry(t)
	for _, assetType := range []string{"", "unknown", "douyin_video"} {
		if _, ok := registry.For(assetType); ok {
			t.Errorf("素材类型 %q 不该有 Skill", assetType)
		}
	}
}

func mustRegistry(t *testing.T) Registry {
	t.Helper()
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("加载内嵌 Skill 失败：%v", err)
	}
	return registry
}
