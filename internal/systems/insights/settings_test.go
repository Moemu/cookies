package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/shikanon/cookies/internal/platform/contract"
	"strings"
	"testing"
)

// 20 §121 要求「重要阈值显示影响说明和默认推荐」。22 §239 记的问题就是
// 「缺少实际阈值影响说明」——所以少写一句 Effect 不是排版瑕疵，是这一页没做到它
// 唯一要做的事。以后往 sample / window 组里加阈值时，这条会拦下来。
func TestEveryEffectiveSettingExplainsItsImpactAndRecommendation(t *testing.T) {
	settings := mustSettings(t)
	for _, group := range settings.Groups {
		if group.State != SettingInEffect {
			continue
		}
		if len(group.Items) == 0 {
			t.Fatalf("分组 %s 标成生效中却一条设置都没有", group.Key)
		}
		for _, item := range group.Items {
			if strings.TrimSpace(item.Value) == "" {
				t.Errorf("%s.%s 没有当前生效值", group.Key, item.Key)
			}
			if strings.TrimSpace(item.Effect) == "" {
				t.Errorf("%s.%s 没有影响说明（20 §121）", group.Key, item.Key)
			}
			if strings.TrimSpace(item.Recommended) == "" {
				t.Errorf("%s.%s 没有默认推荐（20 §121）", group.Key, item.Key)
			}
			if strings.TrimSpace(item.Source) == "" {
				t.Errorf("%s.%s 没有指出值在代码里的位置", group.Key, item.Key)
			}
			if strings.TrimSpace(item.Basis) == "" {
				// 允许写「无文档依据」，但不允许留空——留空看不出是没依据还是忘了写。
				t.Errorf("%s.%s 没有写文档依据（没有依据也要显式写出来）", group.Key, item.Key)
			}
		}
	}
}

// 这一页的全部价值在于「页面上的数字就是代码里正在用的那个」。抄一份常量过来，
// 改了代码忘了改这里，这一页就从说明变成误导——比不做更糟。
func TestDisplayedValuesComeFromTheConstantsActuallyInUse(t *testing.T) {
	settings := mustSettings(t)
	items := map[string]SettingItem{}
	for _, group := range settings.Groups {
		for _, item := range group.Items {
			items[item.Key] = item
		}
	}
	for key, want := range numericSettingConstants() {
		item, ok := items[key]
		if !ok {
			t.Errorf("设置页少了 %s", key)
			continue
		}
		if !strings.Contains(item.Value, want) {
			t.Errorf("%s 显示 %q，代码里的常量是 %s——两者必须一致", key, item.Value, want)
		}
	}
}

// 每个数值阈值在代码里的当前取值。两个测试共用：一个盯着页面别和代码漂开，
// 一个盯着「默认推荐」别和当前值漂开。
func numericSettingConstants() map[string]string {
	return map[string]string{
		"sufficient_sample_impressions":  fmt.Sprint(sufficientSampleImpressions),
		"directional_sample_impressions": fmt.Sprint(directionalSampleImpressions),
		"min_driver_assets":              fmt.Sprint(minDriverAssets),
		"min_trend_days":                 fmt.Sprint(minTrendDays),
		"min_anomaly_days":               fmt.Sprint(minAnomalyDays),
		"anomaly_mad_multiple":           fmt.Sprintf("%.1f", anomalyMADMultiple),
		"min_evaluation_samples":         fmt.Sprint(minEvaluationSamples),
		"max_window_days":                fmt.Sprint(maxWindowDays),
		"prelaunch_quality_window_days":  fmt.Sprint(preLaunchQualityWindowDays),
		"freshness_delayed_after_days":   fmt.Sprint(freshnessDelayedAfterDays),
		"max_comparison_assets":          fmt.Sprint(maxComparisonAssets),
		"max_import_rows":                fmt.Sprint(maxImportRows),
		"max_feature_values_per_field":   fmt.Sprint(maxFeatureValuesPerField),
	}
}

// 前端在当前值和默认推荐不一致时会打一行「（当前值与推荐不一致）」，意思是有人调过它。
// 数值阈值眼下一个都没调过，所以两边必须逐字相同——包括单位后缀。写成「10 条已复核特征」
// 对「10 条」，页面会凭空报出一个不存在的改动，看的人无从分辨那是真调过还是文案不齐。
//
// 只管数值项。确认权限那组的「当前值」是这个权限管到哪些接口、「推荐」是该发给谁，
// 本来就是两件事，拿来比对没有意义。
func TestRecommendedIsWordForWordTheCurrentValueWhileNothingHasBeenTuned(t *testing.T) {
	numeric := numericSettingConstants()
	for _, group := range mustSettings(t).Groups {
		for _, item := range group.Items {
			if _, ok := numeric[item.Key]; !ok {
				continue
			}
			if item.Deviates {
				t.Errorf("%s.%s 当前值 %q 与推荐 %q 不一致：真调过就在这里说明原因，"+
					"只是单位没写齐就补齐", group.Key, item.Key, item.Value, item.Recommended)
			}
		}
	}
}

// 「还没做」和「做了但当前为空」在设置页上必须是两件事。前者要说清楚缺什么，
// 否则读的人会默认它在后台默默生效——通知尤其容易被这样误读。
func TestUnbuiltGroupsSayWhatIsMissingInsteadOfShowingAnEmptyForm(t *testing.T) {
	settings := mustSettings(t)
	unbuilt := 0
	for _, group := range settings.Groups {
		if group.State != SettingNotBuilt {
			continue
		}
		unbuilt++
		if len(group.Items) != 0 {
			t.Errorf("分组 %s 标成未建设却列出了设置项", group.Key)
		}
		if len(group.Missing) == 0 {
			t.Errorf("分组 %s 标成未建设却没说缺什么", group.Key)
		}
	}
	if unbuilt != 2 {
		t.Fatalf("当前应有 2 个未建设分组（通知、报告模板），实际 %d 个", unbuilt)
	}
}

// 每一组都要有去处：落在设置页四组之一，或者明写「不展示」。
//
// 收敛成一个「设置」入口之后，组和二级视图不再一一对应——判定阈值那一屏由
// 样本门槛 + 观察窗口拼出来（窗口天数本来就是判定标准的一部分），名词表并进
// 变量字典。所以这条测试盯的不再是顺序，而是**没有一组是孤儿**：漏标 View
// 的组会从页面上悄悄消失，而它的值还在生效。
func TestEveryGroupLandsInOneOfTheFourViews(t *testing.T) {
	settings := mustSettings(t)
	allowed := map[SettingsView]bool{
		ViewThresholds: true, ViewHealth: true, ViewDictionary: true, ViewPermission: true,
	}
	seen := map[SettingsView]bool{}
	for _, group := range settings.Groups {
		if group.View == ViewNone {
			// 不展示只有一个正当理由：这一组根本还没做。做了却不展示，
			// 就是一组在生效、但没人看得见的设置。
			if group.State != SettingNotBuilt {
				t.Errorf("分组 %s 在生效却不在设置页上任何一组里", group.Key)
			}
			continue
		}
		if !allowed[group.View] {
			t.Errorf("分组 %s 落在 %q，不是四组之一", group.Key, group.View)
		}
		seen[group.View] = true
	}
	// 数据体检那一屏的内容来自质量报告接口，不出现在这里，所以不查它。
	for _, view := range []SettingsView{ViewThresholds, ViewDictionary, ViewPermission} {
		if !seen[view] {
			t.Errorf("%q 这一组在后端一条设置都没有，页面上会是空的", view)
		}
	}
}

// 可写与否要标在**条**上：整组可写会把「异常判定倍数」这种统计口径、
// 「导入行数上限」这种防呆开关一起放出去。同时空切片不能序列化成 null——
// 前端取 .length 会白屏，这个坑在能力运营那批已经踩过一次。
func TestOnlyTheJudgementThresholdsAreEditable(t *testing.T) {
	settings := mustSettings(t)
	if !settings.Editable {
		t.Error("判定阈值已经可以改了，整页不该再标成只读")
	}
	if strings.TrimSpace(settings.EditableNote) == "" {
		t.Error("要说清哪些能改、改了会怎样、哪些不能改以及为什么")
	}
	locked := []string{"anomaly_mad_multiple", "max_import_rows", "max_window_days", "min_evaluation_samples"}
	editable := map[string]bool{}
	for _, group := range settings.Groups {
		for _, item := range group.Items {
			if item.EditableKey != "" {
				editable[item.Key] = true
			}
		}
	}
	for _, key := range locked {
		if editable[key] {
			t.Errorf("%s 不该可写：它要么是统计口径，要么是防呆上限，做成可配置等于做成可关闭", key)
		}
	}
	if len(editable) != 7 {
		t.Errorf("可写的格子有 %d 个，判定阈值一共 7 个", len(editable))
	}
}

// EditableKey 是前端保存时用的键名。写错一个字母，页面上照样渲染出输入框、
// 照样能点保存，后端收到一个不认识的键，静默丢掉——人以为改了，实际没改。
// 所以这些键必须逐个对得上 Thresholds 的 JSON 字段。
func TestEditableKeysAreRealThresholdFields(t *testing.T) {
	sample := 1
	raw, err := json.Marshal(Thresholds{
		SufficientImpressions: &sample, DirectionalImpressions: &sample,
		MinTrendDays: &sample, MinAnomalyDays: &sample, MinDriverAssets: &sample,
		MaxComparisonAssets: &sample, QualityWindowDays: &sample,
	})
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("反序列化失败：%v", err)
	}
	for _, group := range mustSettings(t).Groups {
		for _, item := range group.Items {
			if item.EditableKey == "" {
				continue
			}
			if _, ok := fields[item.EditableKey]; !ok {
				t.Errorf("%s 的 editable_key=%q 在 Thresholds 里没有这个字段——存下去会被静默丢掉",
					item.Key, item.EditableKey)
			}
		}
	}
}

// 有人调过之后，页面上要显示调过的值，并标出它偏离了出厂推荐。
// 这一页写的要是判定真正在用的那份，否则人按着页面上的数字去理解结论，会理解错。
func TestSettingsShowTheThresholdsInEffectNotTheConstants(t *testing.T) {
	tuned := 2000
	service := testService()
	service.Thresholds = stubThresholds{set: ThresholdSet{
		Version: 5, Values: Thresholds{SufficientImpressions: &tuned},
	}}
	settings, err := service.GetInsightSettings(context.Background(), testActor(), "k_project_1")
	if err != nil {
		t.Fatalf("读取设置失败：%v", err)
	}
	for _, group := range settings.Groups {
		for _, item := range group.Items {
			if item.Key != "sufficient_sample_impressions" {
				continue
			}
			if !strings.Contains(item.Value, "2000") {
				t.Errorf("充分门槛显示 %q，实际生效的是 2000", item.Value)
			}
			if !item.Deviates {
				t.Error("调过的格子要标出来，否则没人知道它和出厂推荐不一样了")
			}
			return
		}
	}
	t.Fatal("设置页上找不到充分门槛")
}

type stubThresholds struct{ set ThresholdSet }

func (s stubThresholds) LatestThresholdSet(context.Context, contract.OrganizationID) (ThresholdSet, error) {
	return s.set, nil
}

func (s stubThresholds) AppendThresholdSet(_ context.Context, value ThresholdSet) (ThresholdSet, error) {
	return value, nil
}

func (s stubThresholds) ListThresholdSets(context.Context, contract.OrganizationID, int) ([]ThresholdSet, error) {
	return []ThresholdSet{s.set}, nil
}

func TestSettingsNeverSerializeNullLists(t *testing.T) {
	settings := mustSettings(t)
	if settings.ProjectScoped {
		t.Error("这些值对整个部署生效，不分 Project")
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	if strings.Contains(string(encoded), ":null") {
		t.Errorf("响应里出现 null：%s", encoded)
	}
}

func TestSettingsRequireReadScope(t *testing.T) {
	if _, err := testService().GetInsightSettings(context.Background(), contract.ActorContext{OrganizationID: "k_org_1"}, "k_project_1"); err == nil {
		t.Fatal("没有 insights.read 也能读设置")
	}
}

func mustSettings(t *testing.T) InsightSettings {
	t.Helper()
	settings, err := testService().GetInsightSettings(context.Background(), testActor(), "k_project_1")
	if err != nil {
		t.Fatalf("读取设置失败：%v", err)
	}
	return settings
}
