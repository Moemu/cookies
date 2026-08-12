package insights

import "strings"

// 提交复盘。
//
// 这一步做三件事，缺一件这份复盘都不完整：
//  1. 补上「这份复盘算哪次投放」——全流程唯一必须回答它的地方；
//  2. 把系统发现定格进去，和人一路记的那几笔合并去重；
//  3. 状态改成已确认，从此不可改。
//
// 三件事必须在一条 UPDATE 里。分开做的话，中间断电会留下一份「已确认但没有系统发现」
// 的报告，而它看起来和正常的一模一样——没人会怀疑那份复盘漏了东西。

// SubmitReviewRequest 是提交复盘的入参。
type SubmitReviewRequest struct {
	// ExecutionID 是这份复盘算哪次投放。草稿是人在分析页记第一笔时自动建的，
	// 那时候还没到这个问题；提交的时候必须回答。
	ExecutionID string `json:"execution_id"`
	// ExpectedVersion 防并发覆盖：两个人同时开着这份草稿，后提交的那个
	// 会把先提交的删改抹掉，而两边都不会看到任何提示。
	ExpectedVersion int64 `json:"expected_version"`
}

func (r SubmitReviewRequest) Validate() error {
	if strings.TrimSpace(r.ExecutionID) == "" {
		return ErrInvalidRequest
	}
	if r.ExpectedVersion <= 0 {
		return ErrInvalidRequest
	}
	return nil
}

// hasContent 判断这份复盘要不要出现在列表里。
//
// 空草稿是「记一笔」自动建了但人什么都没记的残留。它和「人记了又全删了」不一样：
// 后者是一个明确的决定——这一轮我看过，什么都不值得留——清掉它等于抹掉那个决定。
// 所以判据是 digest 长度，不是「有几条没被删」。
func hasContent(report InsightReport) bool {
	if report.Status != ReportDraft {
		return true
	}
	return len(report.Digest) > 0
}

// checkSubmittable 单独拆出来，是为了让「什么样的复盘能提交」这条规则
// 能被直接测到，不用先造一个仓储。
func checkSubmittable(report InsightReport, expectedVersion int64) error {
	if report.Status != ReportDraft {
		return ErrInvalidState
	}
	if report.Version != expectedVersion {
		return ErrVersionConflict
	}
	return nil
}
