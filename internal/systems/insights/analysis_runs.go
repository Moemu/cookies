package insights

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// 分析任务留痕。表见 migrations/insights/20260730103000_insight_analysis_runs.up.sql。
//
// 03 §344 验收 12：任一分析结论能回溯到素材、数据截止时间、指标和 Skill 版本。
// 特征表每行只带 skill_id / skill_version，回答不了「哪个模型、哪一版提示词、
// 输入是什么、有没有出错」。这里补上那一段。
//
// 也是 §310（分析任务成功率、P95 时长）唯一的数据来源——所以失败也要留一行。

// AnalysisRunKind 目前只有一种。加新的分析类型时在这里加值，不要另建表：
// 运营面要的是「所有分析任务」的成功率。
type AnalysisRunKind string

const (
	AnalysisRunFeatureExtraction AnalysisRunKind = "feature_extraction"
)

// AnalysisRunStatus 三态。running 是有意存在的：任务开始时就落一行，
// 中途进程挂掉会留下一条永远 running 的记录——**那是特性不是缺陷**，
// 它让「悄无声息地失败」变得看得见，而不是查无此事。
type AnalysisRunStatus string

const (
	AnalysisRunRunning   AnalysisRunStatus = "running"
	AnalysisRunSucceeded AnalysisRunStatus = "succeeded"
	AnalysisRunFailed    AnalysisRunStatus = "failed"
)

// GenerationMode 区分真调了模型和兜底模板。
//
// **这两者绝不能混着统计成功率**：兜底路径的成功率恒定 100%，掺进去之后
// 供应商全挂的那天，运营面上看到的仍然是一片绿。
type GenerationMode string

const (
	GenerationModeModel    GenerationMode = "model"
	GenerationModeTemplate GenerationMode = "template"
)

// AnalysisRun 是一次分析任务的完整留痕。
type AnalysisRun struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`

	Kind    AnalysisRunKind   `json:"kind"`
	AssetID string            `json:"asset_id"`
	Status  AnalysisRunStatus `json:"status"`

	// 运行当时的素材类型。素材类型事后可被人改（AM-002），改了以后就查不出
	// 这次运行用的是哪套特征体系——而输出格式正是由它生成的。
	AssetType AssetType `json:"asset_type"`

	// 方法。ContentHash 让「版本号没动但 Skill 内容改了」也能被发现。
	SkillID          string `json:"skill_id"`
	SkillVersion     string `json:"skill_version"`
	SkillContentHash string `json:"skill_content_hash,omitempty"`
	PromptVersion    string `json:"prompt_version,omitempty"`

	// 模型与路由。ModelVersion 是供应商实际返回的那一版，不是我们请求的别名——
	// 别名背后换了模型时，只有它能把两批结果区分开。
	ProviderCode    string         `json:"provider_code,omitempty"`
	ModelAlias      string         `json:"model_alias,omitempty"`
	ModelVersion    string         `json:"model_version,omitempty"`
	RouteRevisionID string         `json:"route_revision_id,omitempty"`
	ResponseMode    string         `json:"response_mode,omitempty"`
	GenerationMode  GenerationMode `json:"generation_mode,omitempty"`

	// 输入指纹与规模。全文不入表（09 §7 日志默认不记录内容全文）。
	InputHash    string          `json:"input_hash,omitempty"`
	InputSummary json.RawMessage `json:"input_summary,omitempty"`

	// 结果指纹：同样输入 + 同样方法 → 同样 ResultHash，可重放（§304）。
	ResultHash    string   `json:"result_hash,omitempty"`
	FeatureCount  int      `json:"feature_count"`
	DroppedFields []string `json:"dropped_fields,omitempty"`

	// 指标类分析的数据截止时间（§304）。内容特征提取不读投放数据，因此为 nil；
	// 这个 nil 有含义：它说明这次运行的结论与投放数据无关。
	DataThrough *time.Time `json:"data_through,omitempty"`

	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	LatencyMS        int `json:"latency_ms"`

	// 失败原因。Message 是给人看的，不放模型返回的原文（09 §7）。
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// validate 是写库前的最后一道，和迁移里的 CHECK 一一对应。
//
// 两处都写是有意的：数据库那道保证任何写入路径都逃不掉，这一道保证错误在
// 服务层就被说清楚——MySQL 的 CHECK 报错只会给一个约束名，排查不下去。
func (r AnalysisRun) validate() error {
	if r.Kind != AnalysisRunFeatureExtraction {
		return fmt.Errorf("%w: 未知的分析任务类型 %q", ErrInvalidRequest, string(r.Kind))
	}
	if strings.TrimSpace(r.AssetID) == "" {
		return fmt.Errorf("%w: 分析任务必须指向一条素材", ErrInvalidRequest)
	}
	if _, ok := FeatureSchemaFor(r.AssetType); !ok {
		return fmt.Errorf("%w: 素材类型 %q 没有特征体系，无法记录这次分析用了哪套", ErrInvalidRequest, string(r.AssetType))
	}
	switch r.Status {
	case AnalysisRunRunning:
		if r.FinishedAt != nil {
			return fmt.Errorf("%w: 还在跑的任务不该有结束时间", ErrInvalidRequest)
		}
	case AnalysisRunSucceeded:
		if r.ErrorCode != "" || r.ErrorMessage != "" {
			return fmt.Errorf("%w: 成功的任务不该留错误信息，否则会被读成本次出了错", ErrInvalidRequest)
		}
		if r.FinishedAt == nil {
			return fmt.Errorf("%w: 结束的任务必须有结束时间，否则耗时统计会算进没跑完的", ErrInvalidRequest)
		}
	case AnalysisRunFailed:
		if strings.TrimSpace(r.ErrorMessage) == "" {
			return fmt.Errorf("%w: 失败必须写清原因，否则运营面只能看到一个数字，查不下去", ErrInvalidRequest)
		}
		if r.FinishedAt == nil {
			return fmt.Errorf("%w: 结束的任务必须有结束时间，否则耗时统计会算进没跑完的", ErrInvalidRequest)
		}
	default:
		return fmt.Errorf("%w: 未知的分析任务状态 %q", ErrInvalidRequest, string(r.Status))
	}
	if r.GenerationMode != "" && r.GenerationMode != GenerationModeModel && r.GenerationMode != GenerationModeTemplate {
		return fmt.Errorf("%w: 未知的产出方式 %q", ErrInvalidRequest, string(r.GenerationMode))
	}
	return nil
}

// Duration 返回这次运行耗时；还没结束返回 0。
// 运营面的 P95 只统计已结束的运行，未结束的耗时是未知，不是零。
func (r AnalysisRun) Duration() time.Duration {
	if r.FinishedAt == nil {
		return 0
	}
	return r.FinishedAt.Sub(r.StartedAt)
}

// AnalysisRunFilter 查询条件。
type AnalysisRunFilter struct {
	AssetID  string
	Kind     AnalysisRunKind
	Statuses []AnalysisRunStatus
	Limit    int
}

// hashPayload 算 sha256 并转十六进制。输入和结果指纹都用它。
func hashPayload(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

// hashFeatureInputs 把翻译后的特征算成一个稳定指纹。
//
// **先排序再算**：map 遍历顺序在 Go 里是随机的，不排序的话同样的结果
// 每次算出的哈希都不一样，「可重放」就成了一句空话（§304）。
func hashFeatureInputs(inputs []FeatureInput) string {
	sorted := make([]FeatureInput, len(inputs))
	copy(sorted, inputs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })
	encoded, err := json.Marshal(sorted)
	if err != nil {
		// FeatureInput 里全是可序列化的基本类型，走不到这里；
		// 真走到了也不能让整次提取失败——指纹算不出来比结论丢了轻。
		return ""
	}
	return hashPayload(encoded)
}

// AnalysisRunRepository 与 AssetRepository 分开：分析任务是只增不改的流水，
// 素材是有状态机的实体，两者生命周期不同。
type AnalysisRunRepository interface {
	CreateAnalysisRun(context.Context, AnalysisRun) (AnalysisRun, error)
	FinishAnalysisRun(context.Context, AnalysisRun) (AnalysisRun, error)
	ListAnalysisRuns(context.Context, contract.OrganizationID, contract.ProjectID, AnalysisRunFilter) ([]AnalysisRun, error)
}
