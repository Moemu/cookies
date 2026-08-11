package insights

import (
	"context"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// 判定阈值。
//
// 这些数字决定三档结论怎么判：多少曝光算「充分」、几天才给趋势、几条素材才谈
// 驱动因素。它们原来是散在三个文件里的代码常量——看不见，也调不了。
// 一个看不见的阈值和一个错的阈值，在使用者那里是同一种东西。
//
// 改成可写之后有两条硬约束：
//   - **只增版本，从不原地改**；
//   - **每条结论盖上它用的那一版号**。
//
// 少了任何一条，改完阈值之后历史结论就说不清是按什么标准算的。

// Thresholds 是「有人调过的那些」。
//
// 每格都是指针：**nil 表示没人调过这一格，用代码默认值**，不是 0。
// 存成 0 的话，将来改了代码默认值，那些从没被调过的部署不会跟着走，
// 而它们本来应该跟着走。
type Thresholds struct {
	SufficientImpressions  *int `json:"sufficient_impressions,omitempty"`
	DirectionalImpressions *int `json:"directional_impressions,omitempty"`
	MinTrendDays           *int `json:"min_trend_days,omitempty"`
	MinAnomalyDays         *int `json:"min_anomaly_days,omitempty"`
	MinDriverAssets        *int `json:"min_driver_assets,omitempty"`
	MaxComparisonAssets    *int `json:"max_comparison_assets,omitempty"`
	QualityWindowDays      *int `json:"quality_window_days,omitempty"`
}

// 天数的下限。允许设成 1，等于允许人把「拒绝下结论」这条规则关掉，
// 而那是这个模块和一张 BI 报表的全部区别。
const (
	floorTrendDays   = 3
	floorAnomalyDays = 4
)

func (t Thresholds) Validate() error {
	positives := []*int{
		t.SufficientImpressions, t.DirectionalImpressions, t.MinTrendDays,
		t.MinAnomalyDays, t.MinDriverAssets, t.MaxComparisonAssets, t.QualityWindowDays,
	}
	for _, value := range positives {
		if value != nil && *value <= 0 {
			return ErrInvalidRequest
		}
	}
	if t.MinTrendDays != nil && *t.MinTrendDays < floorTrendDays {
		return ErrInvalidRequest
	}
	if t.MinAnomalyDays != nil && *t.MinAnomalyDays < floorAnomalyDays {
		return ErrInvalidRequest
	}
	// 「充分」不能低于「有方向」。反过来设的话，一个样本量会同时满足两档，
	// 谁生效取决于代码里 if 的先后——阈值页上两个看起来独立的数字，
	// 实际效果却由实现顺序决定，这种配置必须在保存时就拦下来。
	//
	// 两边都先落到解析后的值再比：只调其中一格的人，改的是这一格和另一格
	// **默认值**的关系，只看填了的那两格会把这种改法放过去。
	sufficient := pickInt(t.SufficientImpressions, sufficientSampleImpressions)
	directional := pickInt(t.DirectionalImpressions, directionalSampleImpressions)
	if sufficient < directional {
		return ErrInvalidRequest
	}
	return nil
}

// ThresholdSet 是落库的一版。
type ThresholdSet struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	Version        int64                   `json:"version"`
	Values         Thresholds              `json:"values"`
	Reason         string                  `json:"reason"`
	ChangedBy      string                  `json:"changed_by"`
	ChangedAt      time.Time               `json:"changed_at"`
}

// ResolvedThresholds 是判定真正拿去用的那份：每格都有值，且带着版本号。
//
// 判定链路上一律传这个，不传 ThresholdSet——传后者的话，每个用到阈值的地方
// 都要自己做一次「nil 就取默认」，迟早有一处漏了，那一处就会拿 0 去比。
type ResolvedThresholds struct {
	Version int64 `json:"version"`

	SufficientImpressions  int `json:"sufficient_impressions"`
	DirectionalImpressions int `json:"directional_impressions"`
	MinTrendDays           int `json:"min_trend_days"`
	MinAnomalyDays         int `json:"min_anomaly_days"`
	MinDriverAssets        int `json:"min_driver_assets"`
	MaxComparisonAssets    int `json:"max_comparison_assets"`
	QualityWindowDays      int `json:"quality_window_days"`
}

// orDefaults 把没填的格子补成出厂设定。
//
// 判定链路上的函数很多，漏传一处就会拿 0 去比——那样任何样本量都算充分、
// 任何天数都够画趋势，是最坏的一种错：错得悄无声息，而且错向「更敢下结论」。
//
// 逐格兜底而不是整份替换：一份只填了两格的阈值，剩下几格也该是默认值，
// 不该因为其中一格没填就把填了的那几格一起丢掉。
func (t ResolvedThresholds) orDefaults() ResolvedThresholds {
	value := defaultThresholds()
	value.Version = t.Version
	value.SufficientImpressions = positiveOr(t.SufficientImpressions, value.SufficientImpressions)
	value.DirectionalImpressions = positiveOr(t.DirectionalImpressions, value.DirectionalImpressions)
	value.MinTrendDays = positiveOr(t.MinTrendDays, value.MinTrendDays)
	value.MinAnomalyDays = positiveOr(t.MinAnomalyDays, value.MinAnomalyDays)
	value.MinDriverAssets = positiveOr(t.MinDriverAssets, value.MinDriverAssets)
	value.MaxComparisonAssets = positiveOr(t.MaxComparisonAssets, value.MaxComparisonAssets)
	value.QualityWindowDays = positiveOr(t.QualityWindowDays, value.QualityWindowDays)
	return value
}

func positiveOr(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func pickInt(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

// defaultThresholds 是出厂设定，全部来自代码常量。
//
// **代码常量仍然是默认值的唯一来源**：落库的只有「有人调过的那些」。
// 这样常量改了，没调过的部署跟着走；调过的部署保持人当初的决定。
func defaultThresholds() ResolvedThresholds {
	return ResolvedThresholds{
		Version:                0,
		SufficientImpressions:  sufficientSampleImpressions,
		DirectionalImpressions: directionalSampleImpressions,
		MinTrendDays:           minTrendDays,
		MinAnomalyDays:         minAnomalyDays,
		MinDriverAssets:        minDriverAssets,
		MaxComparisonAssets:    maxComparisonAssets,
		QualityWindowDays:      preLaunchQualityWindowDays,
	}
}

func resolve(set ThresholdSet) ResolvedThresholds {
	value := defaultThresholds()
	value.Version = set.Version
	value.SufficientImpressions = pickInt(set.Values.SufficientImpressions, value.SufficientImpressions)
	value.DirectionalImpressions = pickInt(set.Values.DirectionalImpressions, value.DirectionalImpressions)
	value.MinTrendDays = pickInt(set.Values.MinTrendDays, value.MinTrendDays)
	value.MinAnomalyDays = pickInt(set.Values.MinAnomalyDays, value.MinAnomalyDays)
	value.MinDriverAssets = pickInt(set.Values.MinDriverAssets, value.MinDriverAssets)
	value.MaxComparisonAssets = pickInt(set.Values.MaxComparisonAssets, value.MaxComparisonAssets)
	value.QualityWindowDays = pickInt(set.Values.QualityWindowDays, value.QualityWindowDays)
	return value
}

// ThresholdRepository 只有读最新一版和追加一版两件事。**没有更新方法**——
// 接口上就不给原地改的口子，比在注释里写「请勿修改」可靠。
type ThresholdRepository interface {
	// LatestThresholdSet 返回版本号最大的那一版；一版都没有时返回零值和 ErrNotFound。
	LatestThresholdSet(context.Context, contract.OrganizationID) (ThresholdSet, error)
	AppendThresholdSet(context.Context, ThresholdSet) (ThresholdSet, error)
	ListThresholdSets(context.Context, contract.OrganizationID, int) ([]ThresholdSet, error)
}

// SaveThresholdsRequest 的理由是必填的。改判定标准是一件要负责的事；
// 写不出理由的改动多半是试出来的，而试出来的阈值三个月后没人说得清为什么是这个数。
type SaveThresholdsRequest struct {
	Values Thresholds `json:"values"`
	Reason string     `json:"reason"`
}

func (r SaveThresholdsRequest) Validate() error {
	if strings.TrimSpace(r.Reason) == "" {
		return ErrInvalidRequest
	}
	if len(r.Reason) > 1000 {
		return ErrInvalidRequest
	}
	return r.Values.Validate()
}

// currentThresholds 是判定链路的唯一入口。读不到就用出厂设定——
// 阈值读失败不该让整个分析页打不开，跑默认值至少还能出结论，且版本号 0
// 会在页面上如实显示成「出厂设定」。
func (s Service) currentThresholds(ctx context.Context, org contract.OrganizationID) ResolvedThresholds {
	if s.Thresholds == nil {
		return defaultThresholds()
	}
	set, err := s.Thresholds.LatestThresholdSet(ctx, org)
	if err != nil {
		return defaultThresholds()
	}
	return resolve(set)
}

func (s Service) GetThresholds(ctx context.Context, actor contract.ActorContext,
	projectID contract.ProjectID) (ResolvedThresholds, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return ResolvedThresholds{}, err
	}
	return s.currentThresholds(ctx, actor.OrganizationID), nil
}

// ListThresholdHistory 是阈值的改动史。它和「只增版本」是一件事的两面：
// 只增版本让历史存得下来，这个方法让历史看得见——看不见的话，
// 一条盖着「第 3 版」的结论，人查不到第 3 版当时是什么数、为什么改。
func (s Service) ListThresholdHistory(ctx context.Context, actor contract.ActorContext,
	projectID contract.ProjectID, limit int) ([]ThresholdSet, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if s.Thresholds == nil {
		return []ThresholdSet{}, nil
	}
	return s.Thresholds.ListThresholdSets(ctx, actor.OrganizationID, normalizeLimit(limit))
}

// SaveThresholds 追加一版，不改任何已有的行。
func (s Service) SaveThresholds(ctx context.Context, actor contract.ActorContext,
	projectID contract.ProjectID, request SaveThresholdsRequest) (ResolvedThresholds, error) {
	// 用 ScopeConfirm 而不是 ScopeWrite：改的是全组织的判定标准，
	// 影响面比写一条数据大得多，应该和「确认经验」同一个门槛。
	if err := s.ready(actor, projectID, ScopeConfirm); err != nil {
		return ResolvedThresholds{}, err
	}
	if s.Thresholds == nil {
		return ResolvedThresholds{}, ErrNotFound
	}
	if err := request.Validate(); err != nil {
		return ResolvedThresholds{}, err
	}
	id, err := s.idGenerator()("thresholdset")
	if err != nil {
		return ResolvedThresholds{}, err
	}
	next := int64(1)
	if previous, findErr := s.Thresholds.LatestThresholdSet(ctx, actor.OrganizationID); findErr == nil {
		next = previous.Version + 1
	}
	saved, err := s.Thresholds.AppendThresholdSet(ctx, ThresholdSet{
		ID: id, OrganizationID: actor.OrganizationID, Version: next,
		Values: request.Values, Reason: strings.TrimSpace(request.Reason),
		ChangedBy: actor.Principal.ID, ChangedAt: s.now(),
	})
	if err != nil {
		return ResolvedThresholds{}, err
	}
	return resolve(saved), nil
}
