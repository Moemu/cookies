package insights

import (
	"encoding/json"
	"testing"
	"time"
)

// insights-v1.yaml 把 MetricOverview 的 series/assets/sources/platforms 声明成
// required 的数组。Go 的 nil slice 序列化出来是 null，不是 []——前端按契约把这些
// 字段当数组用（overview.sources.length），拿到 null 就直接崩在渲染里。
//
// 空窗口是最常见的情况：新部署的库里一条投放数据都没有，这四个字段全是 nil。
// 契约说是数组，就必须永远是数组。
func TestMetricOverviewRequiredArraysAreNeverNullOnEmptyWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	window := MetricWindow{Start: now.AddDate(0, 0, -30), End: now}

	// 新部署的服务器就是这个状态：接了数据源，但窗口里没有任何指标事实。
	overview := buildMetricOverview(window, nil, []DataSource{{ID: "ds-1"}}, now)

	raw, err := json.Marshal(overview)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("反序列化失败：%v", err)
	}

	// insights-v1.yaml MetricOverview.required 里的数组字段。
	for _, field := range []string{"series", "assets", "sources", "platforms"} {
		value, ok := decoded[field]
		if !ok {
			t.Errorf("%s 是 required 字段，不能不出现", field)
			continue
		}
		if string(value) == "null" {
			t.Errorf("%s 是 required 的数组，空窗口下必须是 []，不能是 null", field)
		}
	}
}
