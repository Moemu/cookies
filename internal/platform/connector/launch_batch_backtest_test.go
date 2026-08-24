package connector

import (
	"fmt"
	"testing"
	"time"
)

func TestBuildLaunchBatchBacktestUsesTrueCreateTimeAndProjectBatches(t *testing.T) {
	externalAccount := "123456789"
	coreHeader := []string{"时间-天", "项目ID", "单元ID", "转化目标", "计费类型", "深度转化目标", "投放模式", "单元出价", "深度转化出价", "ROI系数", "消耗", "展示数", "点击数", "转化数"}
	supplementHeader := []string{"时间-天", "项目ID", "单元ID", "营销目的", "项目类型", "是否搜索蓝海流量项目", "营销场景", "关键行为名称", "线索多载体优选", "是否原生营销", "消耗", "展示数", "点击数", "转化数"}
	coreRows, supplementRows := [][]string{coreHeader}, [][]string{supplementHeader}
	objects := []ObjectSnapshot{}
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, location)
	for batch := 0; batch < 30; batch++ {
		launch := start.AddDate(0, 0, batch)
		project := fmt.Sprintf("20%03d", batch)
		for unit := 0; unit < 2; unit++ {
			promotion := fmt.Sprintf("30%03d%d", batch, unit)
			objects = append(objects, ObjectSnapshot{FactHeader: FactHeader{AvailableAt: launch.UTC(), ValidFrom: launch.UTC(), QualityStatus: QualityAccept}, ID: "object-" + promotion, ObjectKind: "promotion", ObjectRef: opaqueRef(promotion), ParentRef: opaqueRef(project), State: map[string]any{"promotion_create_time": launch.Format("2006-01-02 15:04:05")}})
			for day := 0; day < 7; day++ {
				date := launch.AddDate(0, 0, day).Format("2006-01-02")
				coreRows = append(coreRows, []string{date, project, promotion, "APP下单", "OCPM", "无", "自动", "100", "0", "1", "1.00", "10", "2", "1"})
				supplementRows = append(supplementRows, []string{date, project, promotion, "应用推广", "标准", "否", "电商", "", "否", "是", "1.00", "10", "2", "1"})
			}
		}
	}
	// Platform report splits can duplicate one grain. Aggregation must occur before the join.
	supplementRows = append(supplementRows, []string{start.Format("2006-01-02"), "20000", "300000", "应用推广", "标准", "否", "电商", "", "否", "是", "0.00", "0", "0", "0"})
	result, err := BuildLaunchBatchBacktest(LaunchBatchBacktestRequest{
		ExternalAccount: externalAccount,
		Core:            OfflineXLSXSource{Name: "基础数据_" + externalAccount + "_core.xlsx", Content: buildOfflineTestXLSX(t, coreRows)},
		Supplement:      OfflineXLSXSource{Name: "基础数据_" + externalAccount + "_supplement.xlsx", Content: buildOfflineTestXLSX(t, supplementRows)},
		Inventory:       CanonicalSnapshot{Objects: objects},
		TimeZone:        "Asia/Shanghai",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready_for_probabilistic_shadow" || result.Audit.PromotionCount != 60 || result.Audit.CreateTimeMatchedPromotionCount != 60 || result.Audit.EligibleBatchCount != 30 {
		t.Fatalf("status=%s audit=%#v", result.Status, result.Audit)
	}
	if result.Audit.SupplementDuplicateKeyCount != 1 || result.Audit.ConfigurationDistinctCounts["计费类型"] != 1 {
		t.Fatalf("audit=%#v", result.Audit)
	}
	if result.Split.TrainingBatches != 24 || result.Split.HoldoutBatches != 6 || result.Split.HoldoutDates != 6 {
		t.Fatalf("split=%#v", result.Split)
	}
	for _, evaluation := range result.Evaluation {
		if evaluation.WAPE != 0 || evaluation.Central80Coverage != 1 {
			t.Fatalf("evaluation=%#v", evaluation)
		}
	}
}

func TestBuildLaunchBatchBacktestRejectsUnreconciledReports(t *testing.T) {
	externalAccount := "123456789"
	core := [][]string{{"时间-天", "项目ID", "单元ID", "转化目标", "计费类型", "深度转化目标", "投放模式", "单元出价", "深度转化出价", "ROI系数", "消耗", "展示数", "点击数", "转化数"}, {"2026-07-01", "2001", "3001", "APP下单", "OCPM", "无", "自动", "100", "0", "1", "1.00", "10", "2", "1"}}
	supplement := [][]string{{"时间-天", "项目ID", "单元ID", "营销目的", "项目类型", "是否搜索蓝海流量项目", "营销场景", "关键行为名称", "线索多载体优选", "是否原生营销", "消耗", "展示数", "点击数", "转化数"}, {"2026-07-01", "2001", "3001", "应用推广", "标准", "否", "电商", "", "否", "是", "2.00", "10", "2", "1"}}
	_, err := BuildLaunchBatchBacktest(LaunchBatchBacktestRequest{ExternalAccount: externalAccount, Core: OfflineXLSXSource{Name: "基础数据_" + externalAccount + "_core.xlsx", Content: buildOfflineTestXLSX(t, core)}, Supplement: OfflineXLSXSource{Name: "基础数据_" + externalAccount + "_supplement.xlsx", Content: buildOfflineTestXLSX(t, supplement)}, TimeZone: "Asia/Shanghai"})
	if err == nil {
		t.Fatal("unreconciled reports were accepted")
	}
}

func TestCalibrateLaunchBatchScenarioRetainsHeavyTail(t *testing.T) {
	values := make([]launchBatchCase, 0, 25)
	for index := 0; index < 25; index++ {
		spend := float64(100 + index)
		if index >= 20 {
			spend = float64(10_000 + index*100)
		}
		values = append(values, launchBatchCase{Spend: spend, Impressions: spend * 2, Clicks: spend / 10, ActiveUnits: 2, TopSpendShare: 0.5})
	}
	result := calibrateLaunchBatchScenario(values)
	if result.TypicalBatchCount != 20 || result.BreakoutBatchCount != 5 || result.BreakoutProbability <= 0.2 || result.Breakout[0].P50 <= result.Typical[0].P90 {
		t.Fatalf("scenario=%#v", result)
	}
}
