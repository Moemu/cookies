package insights

import (
	"errors"
	"testing"
)

// 系统发现在**提交那一刻**才定格进去，不是草稿一建就补。
//
// 草稿是活的：人一边看分析一边往里记，这中间窗口没变但数据每天在变。
// 一建就补的话，人记第一笔时补进来的那批，到提交的时候数字早就不是那个数了，
// 而报告上不会写它是哪天算的。
func TestSubmitReviewFreezesSystemFindingsAtSubmitTime(t *testing.T) {
	t.Parallel()

	pinned := []ReportFinding{{
		Text: "15 秒版本点击率更高。", Origin: OriginPinned,
		Dimension: "comparisons", Variable: "duration",
		Judgement: judge(ConfidenceSufficient, "样本充分、区间不重叠。"),
	}}
	system := []ReportFinding{
		{Text: "时长 15s 组更高。", Origin: OriginSystem, Dimension: "comparisons", Variable: "duration"},
		{Text: "开场有人脸的一组转化更好。", Origin: OriginSystem, Dimension: "drivers", Variable: "opening_face"},
	}

	frozen := mergeFindings(pinned, system)
	if len(frozen) != 2 {
		t.Fatalf("定格结果应该是 2 条，得到 %d 条", len(frozen))
	}
	if frozen[0].Origin != OriginPinned {
		t.Error("人记的排在前面")
	}
}

func TestSubmitReviewRequestValidation(t *testing.T) {
	t.Parallel()

	valid := SubmitReviewRequest{ExecutionID: "exec_1", ExpectedVersion: 3}
	if err := valid.Validate(); err != nil {
		t.Fatalf("合法请求被拒了：%v", err)
	}

	noExecution := SubmitReviewRequest{ExpectedVersion: 3}
	if err := noExecution.Validate(); err == nil {
		// 提交是全流程唯一必须回答「这份复盘算哪次投放」的地方。不回答的话，
		// 这份复盘沉淀出的经验就没有来源执行，下一轮引用它的人无从追溯。
		t.Error("没有投放执行的提交应该被拒")
	}

	noVersion := SubmitReviewRequest{ExecutionID: "exec_1"}
	if err := noVersion.Validate(); err == nil {
		t.Error("没有版本号的提交应该被拒——并发编辑会静默覆盖")
	}
}

// 已提交的复盘不能再提交一次。第二次提交会用今天的数据重新定格一遍系统发现，
// 而引用了第一版的人手上那份就变成假的了。
func TestSubmitReviewRejectsAlreadyConfirmedReports(t *testing.T) {
	t.Parallel()

	report := InsightReport{Status: ReportConfirmed, Version: 1}
	if err := checkSubmittable(report, 1); !errors.Is(err, ErrInvalidState) {
		t.Errorf("已确认的复盘应该拒绝提交，得到 %v", err)
	}
}

func TestSubmitReviewChecksVersion(t *testing.T) {
	t.Parallel()

	report := InsightReport{Status: ReportDraft, Version: 5}
	if err := checkSubmittable(report, 3); !errors.Is(err, ErrVersionConflict) {
		t.Errorf("版本对不上应该报冲突，得到 %v", err)
	}
	if err := checkSubmittable(report, 5); err != nil {
		t.Errorf("版本对得上应该放行，得到 %v", err)
	}
}
