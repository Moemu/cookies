package insights

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func runningRun() AnalysisRun {
	started := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	return AnalysisRun{
		ID: "run-1", OrganizationID: "org-1", ProjectID: "proj-1",
		Kind: AnalysisRunFeatureExtraction, AssetID: "asset-1",
		AssetType: AssetTypeXiaohongshuNote, Status: AnalysisRunRunning,
		SkillID: "insight.feature.xiaohongshu_note", SkillVersion: "1.0.0",
		StartedAt: started, CreatedBy: "user-1", CreatedAt: started, UpdatedAt: started,
	}
}

func finishedAt(run AnalysisRun, minutes int) *time.Time {
	value := run.StartedAt.Add(time.Duration(minutes) * time.Minute)
	return &value
}

func TestRunningRunIsValid(t *testing.T) {
	if err := runningRun().validate(); err != nil {
		t.Fatalf("刚开始跑的任务应该是合法的：%v", err)
	}
}

// 失败必须写清原因。只留一个 failed 状态的话，运营面上只能看到成功率掉了，
// 点进去什么也查不到——而这正是最需要查下去的时刻。
func TestFailedRunMustCarryAReason(t *testing.T) {
	run := runningRun()
	run.Status = AnalysisRunFailed
	run.FinishedAt = finishedAt(run, 1)
	err := run.validate()
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("没写原因的失败记录应该被拒，实际 %v", err)
	}

	run.ErrorCode = "provider_unavailable"
	run.ErrorMessage = "供应商在 30 秒内没有返回"
	if err := run.validate(); err != nil {
		t.Fatalf("写了原因就应该通过：%v", err)
	}
}

// 成功不留错误信息。留着的话，重试成功之后那条记录既是成功又带着上一次的错误，
// 页面上没法说清这次到底怎么了。
func TestSucceededRunMustNotCarryAnError(t *testing.T) {
	run := runningRun()
	run.Status = AnalysisRunSucceeded
	run.FinishedAt = finishedAt(run, 1)
	run.ErrorMessage = "上一次重试时的错误"
	if err := run.validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("成功却带着错误信息应该被拒，实际 %v", err)
	}
}

// 结束了必须有结束时间，还在跑就不能有。
// 没有这条，P95 时长会把没跑完的任务按 0 算进去，越是卡住的任务越显得快。
func TestFinishTimeMustMatchStatus(t *testing.T) {
	running := runningRun()
	running.FinishedAt = finishedAt(running, 1)
	if err := running.validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("还在跑却有结束时间应该被拒，实际 %v", err)
	}

	done := runningRun()
	done.Status = AnalysisRunSucceeded
	if err := done.validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("结束了却没有结束时间应该被拒，实际 %v", err)
	}
}

// 素材类型必须有特征体系。没有的话，这次运行用的是哪套输出格式就无从谈起——
// 而输出格式正是由它生成的（03 §15②）。
func TestRunRequiresAKnownAssetType(t *testing.T) {
	run := runningRun()
	run.AssetType = AssetTypeUnknown
	if err := run.validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("未识别类型的素材不该产生分析任务，实际 %v", err)
	}
}

// 没跑完的耗时是「未知」，不是零。把它当零会拉低 P95，让卡住的任务反而看起来最快。
func TestUnfinishedRunHasNoDuration(t *testing.T) {
	if got := runningRun().Duration(); got != 0 {
		t.Fatalf("还在跑的任务不该有耗时，实际 %v", got)
	}
	done := runningRun()
	done.Status = AnalysisRunSucceeded
	done.FinishedAt = finishedAt(done, 3)
	if got := done.Duration(); got != 3*time.Minute {
		t.Fatalf("耗时算错了：%v", got)
	}
}

// 结果指纹必须与特征的先后顺序无关。
//
// Go 的 map 遍历顺序是随机的，模型返回的字段顺序也不保证——不排序的话，
// 同样的结果每次算出的哈希都不一样，「可重放」（03 §304）就成了一句空话：
// 你永远比不出「这次和上次是不是同一个结论」。
func TestResultHashIgnoresFeatureOrder(t *testing.T) {
	first := []FeatureInput{
		{Key: "cta", Value: FeatureValue{Kind: FeatureKindEnum, Terms: []string{"引导评论"}}, Confidence: ConfidenceHigh},
		{Key: "image_count", Value: FeatureValue{Kind: FeatureKindNumber, Number: 4}, Confidence: ConfidenceMedium},
	}
	second := []FeatureInput{first[1], first[0]}
	if hashFeatureInputs(first) != hashFeatureInputs(second) {
		t.Fatal("同样的特征换个顺序就算出了不同的指纹，重放判定会永远说「不一样」")
	}
	if hashFeatureInputs(first) == "" {
		t.Fatal("指纹算不出来")
	}
}

// 内容不同必须指纹不同，否则指纹什么也证明不了。
func TestResultHashChangesWithContent(t *testing.T) {
	base := []FeatureInput{{Key: "cta", Value: FeatureValue{Kind: FeatureKindEnum, Terms: []string{"引导评论"}}, Confidence: ConfidenceHigh}}
	changedValue := []FeatureInput{{Key: "cta", Value: FeatureValue{Kind: FeatureKindEnum, Terms: []string{"引导收藏"}}, Confidence: ConfidenceHigh}}
	// 置信度也要进指纹：同样的取值、模型这次很确定上次在猜，是两个不同的结论。
	changedConfidence := []FeatureInput{{Key: "cta", Value: FeatureValue{Kind: FeatureKindEnum, Terms: []string{"引导评论"}}, Confidence: ConfidenceLow}}
	if hashFeatureInputs(base) == hashFeatureInputs(changedValue) {
		t.Fatal("取值变了指纹却没变")
	}
	if hashFeatureInputs(base) == hashFeatureInputs(changedConfidence) {
		t.Fatal("置信度变了指纹却没变")
	}
}

// data_through 为 nil 是有含义的，不是漏填：内容特征提取不读投放数据，
// 这次结论与数据截止时间无关。序列化时它必须整个不出现，
// 而不是变成一个零值时间——那会被读成「数据截止到公元 1 年」。
func TestContentRunOmitsDataThrough(t *testing.T) {
	encoded, err := json.Marshal(runningRun())
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if _, present := decoded["data_through"]; present {
		t.Fatal("内容提取不读投放数据，data_through 不该出现")
	}
	if _, present := decoded["finished_at"]; present {
		t.Fatal("还没结束的任务不该有 finished_at")
	}
}
