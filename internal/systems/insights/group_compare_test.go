package insights

import "testing"

// 造一组只有总量的素材切片。判定只看总量，byDate / features 在这几条测试里
// 都用不上——填上反而会让读的人以为它们参与了判定。
func slicesWith(impressions, clicks int64) []*assetSlice {
	return []*assetSlice{{
		assetID: "asset_x",
		total:   MetricCounts{Impressions: impressions, Clicks: clicks},
	}}
}

// 判定必须跟着配置走。同一批数据，把充分门槛调低，结论就该从「只有方向」
// 变成「充分」——这正是让阈值可写的全部意义。
//
// 这条测试也是「判定只有一处实现」的守卫：如果哪天有人在别处又读了一次常量，
// 那一处不会跟着配置变，而这里会先发现。
func TestVerdictFollowsTheConfiguredThreshold(t *testing.T) {
	t.Parallel()

	input := groupCompareInput{
		InGroup:      slicesWith(3000, 300),
		Rest:         slicesWith(3000, 150),
		SubjectLabel: "开场类型",
		Comparable:   true,
	}

	strict := input
	strict.Thresholds = defaultThresholds()
	strict.Thresholds.SufficientImpressions = 10000
	if got := compareGroups(strict).Confidence; got == ConfidenceSufficient {
		t.Error("3000 次曝光在 10000 的门槛下不该判成充分")
	}

	loose := input
	loose.Thresholds = defaultThresholds()
	loose.Thresholds.SufficientImpressions = 2000
	if got := compareGroups(loose).Confidence; got != ConfidenceSufficient {
		t.Errorf("门槛降到 2000 之后应该判成充分，得到 %q", got)
	}
}

// 没传阈值时走出厂设定。判定是全模块的公共函数，调用点很多，
// 漏传一处就拿 0 去比的话，任何样本量都会被判成充分——那是最坏的一种错。
func TestZeroThresholdsFallBackToDefaults(t *testing.T) {
	t.Parallel()

	input := groupCompareInput{
		InGroup:      slicesWith(10, 5),
		Rest:         slicesWith(10, 1),
		SubjectLabel: "开场类型",
		Comparable:   true,
	}
	if got := compareGroups(input).Confidence; got != ConfidenceLowSample {
		t.Errorf("没传阈值时应该退回默认值，10 次曝光该判成样本不足，得到 %q", got)
	}
}

// 样本下限也跟着配置走。把方向门槛压到 5，原本「比不出东西」的一组
// 就该开始出结论——它和充分门槛是同一套配置里的两格，不能只有一格生效。
func TestLowSampleFloorFollowsTheConfiguredThreshold(t *testing.T) {
	t.Parallel()

	input := groupCompareInput{
		InGroup:      slicesWith(100, 40),
		Rest:         slicesWith(100, 5),
		SubjectLabel: "开场类型",
		Comparable:   true,
		Thresholds:   defaultThresholds(),
	}
	if got := compareGroups(input).Confidence; got != ConfidenceLowSample {
		t.Fatalf("默认门槛下 100 次曝光应该判成样本不足，得到 %q", got)
	}

	input.Thresholds.DirectionalImpressions = 50
	input.Thresholds.SufficientImpressions = 60
	if got := compareGroups(input).Confidence; got == ConfidenceLowSample {
		t.Error("方向门槛降到 50 之后，100 次曝光不该再判成样本不足")
	}
}
