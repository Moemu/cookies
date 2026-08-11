package insights

import (
	"context"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// 「查」模式：下一轮要做素材，先看以前什么有效。
//
// 这一段替代了原来的「投前洞察」页。那一页的五个视图——策略证据、创意建议、
// 历史模式、风险与反例、引用记录——前四个只是同一批经验按不同条件筛，
// 第五个是每条经验自己的引用历史。它们不是五个功能，是一个功能的五种切法，
// 所以做成筛选器和展开层，不做成五个入口。

// ExperienceLookup 是「查」的条件。每一格空着表示不限。
type ExperienceLookup struct {
	Brand     string `json:"brand,omitempty"`
	Product   string `json:"product,omitempty"`
	Channel   string `json:"channel,omitempty"`
	AdType    string `json:"ad_type,omitempty"`
	Objective string `json:"objective,omitempty"`
	Audience  string `json:"audience,omitempty"`
	// Feature 按内容特征找：「有没有关于开场的经验」。
	Feature string `json:"feature,omitempty"`

	// IncludeObserved 打开之后连「👁 只是观察」的也给。默认关着——
	// 查的人多数时候要的是能照着做的东西，混进观察会让他分不清哪条能信。
	IncludeObserved bool `json:"include_observed,omitempty"`
	Limit           int  `json:"limit,omitempty"`
}

// ExperienceMatch 是一条命中。Matched 说清「凭什么推给你」，
// Default 说清「能不能直接照着做」。
type ExperienceMatch struct {
	Experience Experience `json:"experience"`
	Matched    []string   `json:"matched"`
	Default    bool       `json:"default"`
}

// conditionHit 判断一格。经验没写这一格 = 不限，能匹配任何取值；写了就必须对上。
//
// 反过来做（没写就不匹配）会把绝大多数经验筛没：大部分经验只写了两三格适用条件，
// 而查的人会把当前项目的几格全填上。
//
// 适用条件每一格是一组值而不是一个值——一条经验可以同时适用于抖音和小红书。
// 命中的判断是「查的那个值在这组里」，不是相等。
func conditionHit(experienceValues []string, lookupValue string) (hit bool, restricted bool) {
	if len(experienceValues) == 0 {
		return true, false
	}
	lookupValue = strings.TrimSpace(lookupValue)
	if lookupValue == "" {
		return true, false // 查的人没限定这一格，就不用它来卡
	}
	for _, value := range experienceValues {
		if strings.EqualFold(strings.TrimSpace(value), lookupValue) {
			return true, true
		}
	}
	return false, true
}

func matchApplicability(value Experience, lookup ExperienceLookup) (ExperienceMatch, bool) {
	// 「查」只给在用的。待定的还没人背书，停用的已经被人撤下——
	// 混进去的话，看的人分不出哪条是能照着做的。
	if value.Status != ExperienceConfirmed {
		return ExperienceMatch{}, false
	}
	reusable := value.Reusable()
	if !reusable && !lookup.IncludeObserved {
		return ExperienceMatch{}, false
	}

	conditions := []struct {
		label      string
		experience []string
		lookup     string
	}{
		{"品牌", value.Applicability.Brands, lookup.Brand},
		{"产品", value.Applicability.Products, lookup.Product},
		{"渠道", value.Applicability.Channels, lookup.Channel},
		{"广告类型", value.Applicability.CreativeTypes, lookup.AdType},
		{"目标", value.Applicability.Objectives, lookup.Objective},
		{"受众", value.Applicability.Audiences, lookup.Audience},
	}
	matched := make([]string, 0, len(conditions)+1)
	for _, condition := range conditions {
		hit, restricted := conditionHit(condition.experience, condition.lookup)
		if !hit {
			return ExperienceMatch{}, false
		}
		if restricted {
			matched = append(matched, condition.label)
		}
	}
	// 按内容特征找的时候，结论和建议动作都算。人问「有没有关于开场的经验」，
	// 想不起来那条经验当初把「开场」写在了哪一栏。
	if feature := strings.TrimSpace(lookup.Feature); feature != "" {
		if !mentions(value.Conclusion, feature) &&
			!mentions(value.RecommendedAction, feature) &&
			!mentionedInAny(value.ContentBasis.Features, feature) {
			return ExperienceMatch{}, false
		}
		matched = append(matched, "内容特征")
	}
	return ExperienceMatch{Experience: value, Matched: matched, Default: reusable}, true
}

// mentions 是子串匹配，不是相等。按特征找的人输的是「开场」，
// 经验里写的是「开场三秒露脸」——要相等就一条也找不到。
// （prelaunch.go 里的 containsFold 是整值相等，那是给筛选用的，两回事。）
func mentions(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func mentionedInAny(haystack []string, needle string) bool {
	for _, value := range haystack {
		if mentions(value, needle) {
			return true
		}
	}
	return false
}

// LookupExperiences 回答「这一轮的条件下，以前什么有效」。
func (s Service) LookupExperiences(ctx context.Context, actor contract.ActorContext,
	projectID contract.ProjectID, lookup ExperienceLookup) ([]ExperienceMatch, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	// 先按状态取在用的，再在内存里过适用条件。适用条件有六格、每格是一组值、
	// 还有「空着等于不限」这条规则，写成 SQL 是六段 JSON_CONTAINS 加 OR，
	// 改一格条件就要改一次 SQL；在用的经验一个项目也就几十条，内存里过一遍
	// 更清楚也更好改。
	values, err := s.Repository.ListExperiences(ctx, actor.OrganizationID, projectID,
		ExperienceConfirmed, normalizeLimit(100))
	if err != nil {
		return nil, err
	}
	matches := make([]ExperienceMatch, 0, len(values))
	for _, value := range values {
		match, ok := matchApplicability(value, lookup)
		if !ok {
			continue
		}
		matches = append(matches, match)
		if lookup.Limit > 0 && len(matches) >= lookup.Limit {
			break
		}
	}
	return matches, nil
}
