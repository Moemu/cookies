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

// 分组要和导航上的二级视图一一对应，顺序也一致（03 §78、19 §289）。
// 少一个就会有一个标签点开是空的，多一个就会有一组设置没有入口。
//
// 名词表是重构时加的第六个：设计把「这个模块每件事只有一个叫法」当成一条生效中的
// 规则，规则就该和阈值摆在同一页，理由也一样——你改不了，但你有权知道。
func TestGroupsMatchTheNavigationViews(t *testing.T) {
	settings := mustSettings(t)
	want := []string{"样本门槛", "观察窗口", "通知", "确认权限", "报告模板", "名词表"}
	if len(settings.Groups) != len(want) {
		t.Fatalf("分组数 %d，导航上是 %d 个二级视图", len(settings.Groups), len(want))
	}
	for index, label := range want {
		if settings.Groups[index].Label != label {
			t.Errorf("第 %d 组是 %q，导航上是 %q", index+1, settings.Groups[index].Label, label)
		}
	}
}

// 整页只读是有意的（03 §17.3 未定「最低样本由谁配置」）。Editable 一旦为 true，
// 前端会渲染出改不动的输入框。同时空切片不能序列化成 null——前端取 .length 会白屏，
// 这个坑在能力运营那批已经踩过一次。
func TestPageIsReadOnlyAndNeverSerializesNullLists(t *testing.T) {
	settings := mustSettings(t)
	if settings.Editable {
		t.Error("整页必须只读")
	}
	if strings.TrimSpace(settings.EditableNote) == "" {
		t.Error("只读就要说明为什么只读、想改走什么路径")
	}
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
