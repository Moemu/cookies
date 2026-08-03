package insights

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// invalidf 包一层 ErrInvalidRequest 并带上人话。
// 前端只会把 message 原样贴给用户，所以每一句都要能直接读懂。
func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidRequest}, args...)...)
}

// 实验中心（03 §7.3 / AM-009，导航见 19 §284）。
//
// 投后分析里的「驱动因素」和「素材对比」是**事后**从已投数据里认出来的：一组素材
// 恰好在某个特征上一致，表现也恰好更好。它排除不掉选择效应——这批素材本来就是被
// 人挑出来投的，挑的动作本身可能就和表现相关。所以那边的结论最多说到「相关」。
//
// 实验补的是另一半：**变量、分组、样本门槛在看到结果之前定死**。定死这件事由
// status 保证——draft 时随便改，一旦开跑就冻住，谁也不能再往表现好的那组里补素材。
// 少了这个冻结，「事先登记」就只是个说法。
//
// 统计口径不在这个文件里，在 group_compare.go，和素材对比共用同一套。这里只负责
// 「谁跟谁比、够不够格比、能不能下结论」。

// ExperimentStatus 三态。
type ExperimentStatus string

const (
	// ExperimentDraft 设计中：变量、分组、门槛都还能改。
	ExperimentDraft ExperimentStatus = "draft"
	// ExperimentRunning 已开跑：分组冻结，只能看读数。
	ExperimentRunning ExperimentStatus = "running"
	// ExperimentConcluded 已下结论：连人写的解读一起冻住。
	ExperimentConcluded ExperimentStatus = "concluded"
)

func (s ExperimentStatus) valid() bool {
	switch s {
	case ExperimentDraft, ExperimentRunning, ExperimentConcluded:
		return true
	}
	return false
}

// ExperimentVerdict 是系统给的判定，不是人写的解读。两者分开存：
// 判定由数据和门槛决定，解读由人负责，混在一起就分不清哪句话是谁说的。
type ExperimentVerdict string

const (
	// VerdictSupported 假设成立：区间不重叠，且变体优于基线。
	VerdictSupported ExperimentVerdict = "supported"
	// VerdictRefuted 假设被推翻：区间不重叠，但变体反而更差。
	VerdictRefuted ExperimentVerdict = "refuted"
	// VerdictInconclusive 分不出来：区间重叠，样本再多也只能说没看出差别。
	VerdictInconclusive ExperimentVerdict = "inconclusive"
)

// Experiment 是一次事先登记的对照实验。
type Experiment struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`

	Title      string `json:"title"`
	Hypothesis string `json:"hypothesis"`
	// SourceExperienceID 是这个假设的出处（投前洞察的假设卡「拿去验证」）。
	// 为空表示是空白新建的。留着它，实验有结论之后才知道该回头更新哪张卡。
	SourceExperienceID string `json:"source_experience_id,omitempty"`

	// AssetType 决定用哪套特征体系，VariableKey 必须是那套里的字段。
	AssetType     AssetType `json:"asset_type"`
	VariableKey   string    `json:"variable_key"`
	VariableLabel string    `json:"variable_label"`

	// ControlledKeys 是要求各组之间保持一致的其他特征。不一致不拦，只给黄牌——
	// 真实素材很难只差一个变量，硬拦会让实验一个也建不起来；但不说出来，
	// 结论就会被当成干净的对照。
	ControlledKeys []string `json:"controlled_keys"`

	// MinImpressions 是**事先**定的每组最低展示量。事后再定门槛等于允许
	// 「凑够了就说显著」，所以它一旦开跑就不能改。
	MinImpressions int64 `json:"min_impressions"`

	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`

	Status ExperimentStatus `json:"status"`

	// Verdict 是系统判的，Interpretation 是人写的。下结论时两者都要有。
	Verdict        ExperimentVerdict `json:"verdict,omitempty"`
	Interpretation string            `json:"interpretation,omitempty"`
	ConcludedBy    string            `json:"concluded_by,omitempty"`
	ConcludedAt    *time.Time        `json:"concluded_at,omitempty"`

	StartedAt *time.Time `json:"started_at,omitempty"`
	Version   int64      `json:"version"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Variants 由仓储一并读出，不单独出接口：一个实验离开它的分组没有意义。
	Variants []ExperimentVariant `json:"variants"`
}

// ExperimentVariant 是实验的一组。基线组有且只有一个。
type ExperimentVariant struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	ExperimentID   string                  `json:"experiment_id"`

	Name string `json:"name"`
	// VariableValue 是这一组在被测变量上的取值。入组的素材必须和它对得上。
	VariableValue string `json:"variable_value"`
	IsBaseline    bool   `json:"is_baseline"`

	AssetIDs  []string  `json:"asset_ids"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (e Experiment) validate() error {
	if strings.TrimSpace(e.Title) == "" || len(e.Title) > 200 {
		return invalidf("实验必须有名字，且不超过 200 字")
	}
	if len(e.Hypothesis) > 2000 {
		return invalidf("假设最长 2000 字")
	}
	schema, ok := FeatureSchemaFor(e.AssetType)
	if !ok {
		return invalidf("素材类型 %q 没有特征体系，定不出被测变量", string(e.AssetType))
	}
	field, found := schema.Field(e.VariableKey)
	if !found {
		return invalidf("「%s」不是这个素材类型的特征字段，不能当被测变量", e.VariableKey)
	}
	if !comparableKind(field.Kind) {
		return invalidf("「%s」是自由文本，两条素材的取值几乎不可能一样，做不了对照变量", field.Label)
	}
	for _, key := range e.ControlledKeys {
		if key == e.VariableKey {
			return invalidf("「%s」既是被测变量又要求控住，这两件事互相矛盾", field.Label)
		}
		if _, found := schema.Field(key); !found {
			return invalidf("要控住的「%s」不是这个素材类型的特征字段", key)
		}
	}
	if e.MinImpressions <= 0 {
		return invalidf("每组最低展示量必须大于 0——门槛为 0 等于没有门槛")
	}
	if e.WindowEnd.Before(e.WindowStart) {
		return invalidf("观察窗口的结束日期早于开始日期")
	}
	if !e.Status.valid() {
		return invalidf("未知的实验状态 %q", string(e.Status))
	}
	if e.Status == ExperimentConcluded && strings.TrimSpace(e.Interpretation) == "" {
		return invalidf("下结论必须写解读：只留一个判定，读的人无从知道这条结论该怎么用")
	}
	return nil
}

func (v ExperimentVariant) validate() error {
	if strings.TrimSpace(v.Name) == "" || len(v.Name) > 64 {
		return invalidf("分组必须有名字，且不超过 64 字")
	}
	if strings.TrimSpace(v.VariableValue) == "" || len(v.VariableValue) > 200 {
		return invalidf("分组必须写明它在被测变量上的取值")
	}
	if len(v.AssetIDs) > 200 {
		return invalidf("一组最多 200 条素材")
	}
	return nil
}

// window 把实验自己的观察窗口折成指标查询用的窗口。
func (e Experiment) window() MetricWindow {
	return MetricWindow{Start: e.WindowStart, End: e.WindowEnd}
}

// baseline 返回基线组。没有基线组时第二个返回值为 false。
func (e Experiment) baseline() (ExperimentVariant, bool) {
	for _, variant := range e.Variants {
		if variant.IsBaseline {
			return variant, true
		}
	}
	return ExperimentVariant{}, false
}

// --- 读数 ---

// VariantSample 是一组的样本量，以及它够不够事先定的门槛。
//
// **这一层永远返回**，包括样本不够的时候：知道「差多少」才知道还要投多久。
// 不够的时候被藏起来的是对比数字，不是样本数字。
type VariantSample struct {
	VariantID     string `json:"variant_id"`
	Name          string `json:"name"`
	VariableValue string `json:"variable_value"`
	IsBaseline    bool   `json:"is_baseline"`

	Assets      int   `json:"assets"`
	AssetsWith  int   `json:"assets_with_data"`
	Impressions int64 `json:"impressions"`
	Clicks      int64 `json:"clicks"`
	// Meets 为 false 时，这一组参与的所有对比都不出数字。
	Meets bool  `json:"meets_threshold"`
	Short int64 `json:"short_by"`
}

// ExperimentComparison 是「某个变体组 vs 基线组」。
type ExperimentComparison struct {
	VariantID     string `json:"variant_id"`
	VariantName   string `json:"variant_name"`
	VariantValue  string `json:"variant_value"`
	BaselineID    string `json:"baseline_id"`
	BaselineName  string `json:"baseline_name"`
	BaselineValue string `json:"baseline_value"`

	// Blocked 为 true 时 Result 一定是 nil。
	//
	// **这是有意的**：样本不达标却把 CTR 差异显示出来，人会先看见数字再看见提示，
	// 然后记住数字。不给数字是唯一可靠的拦法。
	Blocked bool   `json:"blocked"`
	Blocker string `json:"blocker,omitempty"`

	Result  *GroupComparison  `json:"result,omitempty"`
	Verdict ExperimentVerdict `json:"verdict,omitempty"`
}

// ExperimentReadout 是实验详情页看到的全部读数。
type ExperimentReadout struct {
	Window  MetricWindow  `json:"window"`
	Caliber MetricCaliber `json:"caliber"`
	// Comparable 沿用投后分析的口径判定：窗口内混了币种/归因窗口/指标版本时为 false。
	Comparable       bool   `json:"comparable"`
	ComparableReason string `json:"comparable_reason,omitempty"`

	Samples     []VariantSample        `json:"samples"`
	Comparisons []ExperimentComparison `json:"comparisons"`

	// Ready 为 true 表示每一组都过了门槛，可以下结论。
	Ready bool `json:"ready"`
	// Verdict 是主结论：只有一个变体组时就是它的判定；多个时取最强的那条。
	Verdict ExperimentVerdict `json:"verdict,omitempty"`
	Notes   []string          `json:"notes"`
}

// ExperimentDetail 是实验加它的读数。
type ExperimentDetail struct {
	Experiment Experiment        `json:"experiment"`
	Readout    ExperimentReadout `json:"readout"`
}

// --- 请求 ---

type CreateExperimentRequest struct {
	Title              string    `json:"title"`
	Hypothesis         string    `json:"hypothesis"`
	SourceExperienceID string    `json:"source_experience_id"`
	AssetType          AssetType `json:"asset_type"`
	VariableKey        string    `json:"variable_key"`
	ControlledKeys     []string  `json:"controlled_keys"`
	MinImpressions     int64     `json:"min_impressions"`
	WindowStart        string    `json:"window_start"`
	WindowEnd          string    `json:"window_end"`
	// Variants 至少两组，其中恰好一组是基线。一次建完：分两步建的话，
	// 中间那个「只有一组」的状态没有任何意义，却要多一套校验去容忍它。
	Variants []CreateVariantRequest `json:"variants"`
}

type CreateVariantRequest struct {
	Name          string `json:"name"`
	VariableValue string `json:"variable_value"`
	IsBaseline    bool   `json:"is_baseline"`
}

type AttachExperimentAssetRequest struct {
	AssetID string `json:"asset_id"`
}

// AttachExperimentAssetResult 带回黄牌。
//
// 变量取值对不上是硬拦（返回 error）；控住的变量不一致只给黄牌，因为真实素材
// 很难只差一个变量——硬拦会让实验一个都建不起来，不说又会让结论被当成干净对照。
type AttachExperimentAssetResult struct {
	Variant  ExperimentVariant `json:"variant"`
	Warnings []string          `json:"warnings"`
}

type ConcludeExperimentRequest struct {
	ExpectedVersion int64  `json:"expected_version"`
	Interpretation  string `json:"interpretation"`
}

type ExperimentFilter struct {
	Status ExperimentStatus
	Limit  int
}

// ExperimentRepository 与 Repository 分开：实验有自己的生命周期，
// 和复盘报告、经验条目除了同属这个模块以外没有共同点。
type ExperimentRepository interface {
	CreateExperiment(context.Context, Experiment) (Experiment, error)
	ListExperiments(context.Context, contract.OrganizationID, contract.ProjectID, ExperimentFilter) ([]Experiment, error)
	GetExperiment(context.Context, contract.OrganizationID, contract.ProjectID, string) (Experiment, error)
	// UpdateVariantAssets 只改一组的素材清单，不动实验本身。
	UpdateVariantAssets(context.Context, contract.OrganizationID, contract.ProjectID, string, string, []string, time.Time) (ExperimentVariant, error)
	// UpdateExperimentStatus 承担开跑与下结论两次状态迁移，带乐观锁。
	UpdateExperimentStatus(context.Context, UpdateExperimentStatusInput) (Experiment, error)
}

// UpdateExperimentStatusInput 把两次迁移合成一个入口：两者都是「校验当前状态 +
// 写状态 + 写伴随字段 + 版本加一」，分两个方法只会让乐观锁写两遍。
type UpdateExperimentStatusInput struct {
	OrganizationID  contract.OrganizationID
	ProjectID       contract.ProjectID
	ID              string
	ExpectedVersion int64
	From            ExperimentStatus
	To              ExperimentStatus
	Verdict         ExperimentVerdict
	Interpretation  string
	ActorID         string
	Now             time.Time
}
