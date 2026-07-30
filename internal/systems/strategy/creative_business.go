package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/strategy/creativecatalog"
)

const creativeRecommendationPolicyVersion = "strategy.creative.recommend.v2"

type CreativeBusinessCatalog struct {
	CatalogHash string                    `json:"catalog_hash"`
	Items       []creativecatalog.Profile `json:"items"`
}

type RecommendationSignals struct {
	ObjectiveType      string   `json:"objective_type"`
	Channels           []string `json:"channels"`
	DeliverableType    string   `json:"deliverable_type"`
	DeliverableTypes   []string `json:"deliverable_types"`
	Industry           string   `json:"industry"`
	AssetRoles         []string `json:"asset_roles"`
	ReferencePresent   bool     `json:"reference_present"`
	ContentContext     string   `json:"content_context"`
	BrandGoal          bool     `json:"brand_goal"`
	ProductImageCount  int      `json:"product_image_count"`
	ProductVideoCount  int      `json:"product_video_count"`
	AnalyzedAssetCount int      `json:"analyzed_asset_count"`
}

type CreativeBusinessRecommendation struct {
	BusinessCode     string              `json:"business_code"`
	DisplayName      string              `json:"display_name"`
	Rank             int                 `json:"rank,omitempty"`
	Score            int                 `json:"score"`
	Eligible         bool                `json:"eligible"`
	Confidence       string              `json:"confidence"`
	Reasons          []string            `json:"reasons"`
	MissingSignals   []string            `json:"missing_signals"`
	Warnings         []string            `json:"warnings"`
	ExclusionReasons []string            `json:"exclusion_reasons"`
	ProfileRef       creativecatalog.Ref `json:"profile_ref"`
}

type RecommendationSnapshot struct {
	PolicyVersion string                           `json:"policy_version"`
	CatalogHash   string                           `json:"catalog_hash"`
	BriefID       string                           `json:"brief_id"`
	BriefVersion  int64                            `json:"brief_version"`
	BriefHash     string                           `json:"brief_hash"`
	Signals       RecommendationSignals            `json:"signals"`
	Media         CreativeMediaAssessment          `json:"media"`
	Recommended   []CreativeBusinessRecommendation `json:"recommended"`
	Alternatives  []CreativeBusinessRecommendation `json:"alternatives"`
}

func (s Service) EnsureCreativeBusinessCatalog(ctx context.Context) error {
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		return err
	}
	for _, profile := range registry.All() {
		payload, err := json.Marshal(profile)
		if err != nil {
			return err
		}
		_, err = s.DB.ExecContext(ctx, `INSERT IGNORE INTO strategy_creative_business_profiles
			(business_code, generation, version, display_name, summary, lifecycle,
			 selectable, display_order, profile, content_hash, skill_name, skill_version,
			 skill_content_hash, owner, reviewed_by, reviewed_at, published_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)`,
			profile.BusinessCode, profile.Generation, profile.Version, profile.DisplayName,
			profile.Summary, profile.Lifecycle, profile.Selectable, profile.DisplayOrder,
			payload, profile.ContentHash, profile.SkillName, profile.SkillVersion,
			profile.SkillContentHash, profile.Owner, profile.ReviewedBy, profile.ReviewedAt,
			profile.PublishedAt)
		if err != nil {
			return err
		}
		var storedHash, storedSkillHash string
		if err := s.DB.QueryRowContext(ctx, `SELECT content_hash, skill_content_hash
			FROM strategy_creative_business_profiles
			WHERE business_code = ? AND generation = ?`,
			profile.BusinessCode, profile.Generation).Scan(&storedHash, &storedSkillHash); err != nil {
			return err
		}
		if storedHash != profile.ContentHash || storedSkillHash != profile.SkillContentHash {
			return fmt.Errorf("creative business profile %s generation %d is immutable and its hash changed",
				profile.BusinessCode, profile.Generation)
		}
	}
	return nil
}

func (s Service) ListCreativeBusinesses(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
) (CreativeBusinessCatalog, error) {
	if !s.CreativeTaskPlanningEnabled {
		return CreativeBusinessCatalog{}, ErrFeatureDisabled
	}
	if err := requireScope(actor, ScopeRead); err != nil {
		return CreativeBusinessCatalog{}, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return CreativeBusinessCatalog{}, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT p.profile, p.content_hash, p.skill_content_hash
		FROM strategy_creative_business_profiles p
		WHERE p.lifecycle <> 'draft'
		  AND NOT EXISTS (
		    SELECT 1 FROM strategy_creative_business_profiles newer
		    WHERE newer.business_code = p.business_code
		      AND newer.lifecycle <> 'draft'
		      AND newer.generation > p.generation
		  )
		ORDER BY p.display_order, p.business_code`)
	if err != nil {
		return CreativeBusinessCatalog{}, err
	}
	defer rows.Close()
	items := []creativecatalog.Profile{}
	for rows.Next() {
		var payload json.RawMessage
		var contentHash, skillHash string
		if err := rows.Scan(&payload, &contentHash, &skillHash); err != nil {
			return CreativeBusinessCatalog{}, err
		}
		var profile creativecatalog.Profile
		if err := json.Unmarshal(payload, &profile); err != nil {
			return CreativeBusinessCatalog{}, err
		}
		profile.ContentHash = contentHash
		profile.SkillContentHash = skillHash
		items = append(items, profile)
	}
	if err := rows.Err(); err != nil {
		return CreativeBusinessCatalog{}, err
	}
	hash, err := creativeCatalogHash(items)
	if err != nil {
		return CreativeBusinessCatalog{}, err
	}
	return CreativeBusinessCatalog{CatalogHash: hash, Items: items}, nil
}

func (s Service) GetCreativeBusiness(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	businessCode string,
) (creativecatalog.Profile, error) {
	catalog, err := s.ListCreativeBusinesses(ctx, actor, projectID)
	if err != nil {
		return creativecatalog.Profile{}, err
	}
	for _, profile := range catalog.Items {
		if profile.BusinessCode == strings.TrimSpace(businessCode) {
			return profile, nil
		}
	}
	return creativecatalog.Profile{}, ErrNotFound
}

func (s Service) RecommendCreativeBusinesses(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	briefID string,
	briefVersion int64,
	limit int,
) (RecommendationSnapshot, error) {
	if !s.CreativeTaskPlanningEnabled {
		return RecommendationSnapshot{}, ErrFeatureDisabled
	}
	if limit == 0 {
		limit = 3
	}
	if limit < 1 || limit > 3 || strings.TrimSpace(briefID) == "" || briefVersion < 1 {
		return RecommendationSnapshot{}, ErrInvalidRequest
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return RecommendationSnapshot{}, err
	}
	brief, err := s.GetBriefVersion(ctx, actor, briefID, briefVersion)
	if err != nil {
		return RecommendationSnapshot{}, err
	}
	if brief.ProjectID != projectID {
		return RecommendationSnapshot{}, ErrProjectAccessDenied
	}
	catalog, err := s.ListCreativeBusinesses(ctx, actor, projectID)
	if err != nil {
		return RecommendationSnapshot{}, err
	}
	media := s.assessCreativeMedia(
		ctx, actor, projectID, briefMediaCandidates(brief.Snapshot),
	)
	signals := recommendationSignals(brief.Snapshot, media)
	candidates := make([]CreativeBusinessRecommendation, 0, len(catalog.Items))
	for _, profile := range catalog.Items {
		if !profile.Selectable || profile.Lifecycle != "active" {
			continue
		}
		candidates = append(candidates, evaluateCreativeBusiness(signals, profile))
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].BusinessCode < candidates[j].BusinessCode
	})
	eligible := make([]CreativeBusinessRecommendation, 0, len(candidates))
	alternatives := make([]CreativeBusinessRecommendation, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Eligible {
			eligible = append(eligible, candidate)
		} else {
			alternatives = append(alternatives, candidate)
		}
	}
	if limit > len(eligible) {
		limit = len(eligible)
	}
	recommended := append([]CreativeBusinessRecommendation(nil), eligible[:limit]...)
	for index := range recommended {
		recommended[index].Rank = index + 1
	}
	alternatives = append(eligible[limit:], alternatives...)
	return RecommendationSnapshot{
		PolicyVersion: creativeRecommendationPolicyVersion,
		CatalogHash:   catalog.CatalogHash, BriefID: brief.BriefID, BriefVersion: brief.Version,
		BriefHash: string(brief.ContentHash), Signals: signals, Media: media,
		Recommended: recommended, Alternatives: alternatives,
	}, nil
}

func creativeCatalogHash(items []creativecatalog.Profile) (string, error) {
	refs := make([]creativecatalog.Ref, 0, len(items))
	for _, profile := range items {
		refs = append(refs, profile.Ref())
	}
	hash, err := contract.NewContentHash(refs)
	return string(hash), err
}

func recommendationSignals(
	brief BriefDocument,
	mediaValues ...CreativeMediaAssessment,
) RecommendationSignals {
	objective := strings.ToLower(strings.TrimSpace(brief.Campaign.Objective))
	signals := RecommendationSignals{
		ObjectiveType: "unknown", Channels: normalizedChannels(brief.Channels),
		DeliverableType: "unknown", Industry: strings.ToLower(strings.TrimSpace(brief.Industry)),
		DeliverableTypes: []string{}, AssetRoles: []string{}, ReferencePresent: false,
		ContentContext: "unknown",
	}
	switch {
	case containsAnyFold(objective, "安装", "install", "下载"):
		signals.ObjectiveType = "install"
	case containsAnyFold(objective, "召回", "reactivation"):
		signals.ObjectiveType = "reactivation"
	case containsAnyFold(objective, "销售", "下单", "成交", "sales"):
		signals.ObjectiveType = "sales"
	case containsAnyFold(objective, "转化", "购买", "conversion"):
		signals.ObjectiveType = "conversion"
	case containsAnyFold(objective, "线索", "留资", "获客", "lead"):
		signals.ObjectiveType = "lead"
	case containsAnyFold(objective, "种草", "考虑", "教育", "consideration"):
		signals.ObjectiveType = "consideration"
	case containsAnyFold(objective, "品牌", "认知", "心智", "brand", "awareness"):
		signals.ObjectiveType = "awareness"
	}
	signals.BrandGoal = containsAnyFold(objective, "品牌", "认知", "心智", "brand", "awareness")
	switch {
	case containsAnyFold(objective, "短剧") || containsAnyFold(signals.Industry, "short_drama", "短剧"):
		signals.ContentContext = "short_drama"
	case containsAnyFold(objective, "深度", "解释", "科普", "教育"):
		signals.ContentContext = "deep_explanation"
	}
	if len(mediaValues) > 0 {
		for _, item := range mediaValues[0].Items {
			if item.Usefulness == CreativeMediaUnavailable {
				continue
			}
			switch item.Kind {
			case string(contract.AssetImage):
				signals.ProductImageCount++
				signals.AssetRoles = append(signals.AssetRoles, "product_image")
			case string(contract.AssetVideo):
				signals.ProductVideoCount++
				signals.AssetRoles = append(signals.AssetRoles, "product_video")
			}
			if item.Usefulness == CreativeMediaSemantic {
				signals.AnalyzedAssetCount++
			}
		}
	}
	for _, platform := range brief.PlatformBriefs {
		for _, format := range platform.ContentFormats {
			value := strings.ToLower(strings.TrimSpace(format))
			switch {
			case containsAnyFold(value, "图文", "image_text", "article", "文章"):
				signals.DeliverableTypes = appendUniqueString(signals.DeliverableTypes, "image_text")
			case containsAnyFold(value, "视频", "video"):
				signals.DeliverableTypes = appendUniqueString(signals.DeliverableTypes, "video")
			}
		}
	}
	switch len(signals.DeliverableTypes) {
	case 0:
		signals.DeliverableType = "unknown"
	case 1:
		signals.DeliverableType = signals.DeliverableTypes[0]
	default:
		signals.DeliverableType = "mixed"
	}
	return signals
}

func evaluateCreativeBusiness(
	signals RecommendationSignals,
	profile creativecatalog.Profile,
) CreativeBusinessRecommendation {
	result := CreativeBusinessRecommendation{
		BusinessCode: profile.BusinessCode, DisplayName: profile.DisplayName,
		Eligible: true, Confidence: "low",
		Reasons: []string{}, MissingSignals: []string{}, Warnings: []string{},
		ExclusionReasons: []string{},
		ProfileRef:       profile.Ref(),
	}
	positiveMatches := 0
	for _, rule := range profile.MatchRules {
		matched, missing := recommendationRuleMatches(signals, rule)
		if missing {
			result.MissingSignals = appendUniqueString(result.MissingSignals, rule.Field)
		}
		if creativeRecommendationRuleRequired(profile.BusinessCode, rule) && !matched {
			result.Eligible = false
			result.ExclusionReasons = appendUniqueString(
				result.ExclusionReasons, "缺少必要条件："+rule.Reason,
			)
		}
		if !matched {
			continue
		}
		result.Score += rule.Weight
		if rule.Weight > 0 {
			positiveMatches++
			result.Reasons = appendUniqueString(result.Reasons, rule.Reason)
		} else if rule.Weight < 0 {
			result.Warnings = appendUniqueString(result.Warnings, rule.Reason)
		}
	}
	if len(result.Reasons) == 0 {
		result.Reasons = append(result.Reasons, "当前 Brief 信息不足，建议补充任务目标和交付形式后再比较")
	}
	switch {
	case result.Eligible && result.Score >= 70 && positiveMatches >= 2:
		result.Confidence = "high"
	case result.Eligible && result.Score >= 40:
		result.Confidence = "medium"
	default:
		result.Confidence = "low"
	}
	return result
}

func creativeRecommendationRuleRequired(
	businessCode string,
	rule creativecatalog.RecommendationRule,
) bool {
	if rule.Required {
		return true
	}
	requiredRuleByBusiness := map[string]string{
		"xiaohongshu_image_text":  "channel_xhs",
		"wechat_official_article": "channel_wechat",
		"short_drama_preroll":     "context_short_drama",
		"game_preroll":            "industry_game",
		"commerce_preroll":        "objective_conversion",
		"viral_remake":            "has_reference",
		"brand_video":             "brand_goal",
	}
	return requiredRuleByBusiness[businessCode] == rule.ID
}

func recommendationRuleMatches(
	signals RecommendationSignals,
	rule creativecatalog.RecommendationRule,
) (matched bool, missing bool) {
	var values []string
	switch rule.Field {
	case "objective_type":
		values = []string{signals.ObjectiveType}
	case "channels":
		values = signals.Channels
	case "deliverable_type":
		values = signals.DeliverableTypes
		if len(values) == 0 {
			values = []string{signals.DeliverableType}
		}
	case "industry":
		values = []string{signals.Industry}
	case "asset_roles":
		values = signals.AssetRoles
	case "reference_present":
		values = []string{strconv.FormatBool(signals.ReferencePresent)}
	case "content_context":
		values = []string{signals.ContentContext}
	case "brand_goal":
		values = []string{strconv.FormatBool(signals.BrandGoal)}
	default:
		return false, true
	}
	normalized := normalizedUnique(values)
	present := len(normalized) > 0 && !(len(normalized) == 1 &&
		(normalized[0] == "" || normalized[0] == "unknown"))
	switch rule.Operator {
	case "present":
		return present, false
	case "count_gte":
		if !present {
			return false, true
		}
		return len(normalized) >= rule.Number, false
	case "equals", "in", "contains":
		if !present && !containsString(rule.Values, "unknown") {
			return false, true
		}
		for _, value := range normalized {
			if containsString(rule.Values, value) {
				return true, false
			}
		}
		return false, false
	default:
		return false, true
	}
}

func normalizedChannels(values []string) []string {
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case containsAnyFold(value, "小红书", "xiaohongshu", "xhs"):
			value = "xiaohongshu"
		case containsAnyFold(value, "微信公众号", "公众号", "微信生态", "wechat"):
			value = "wechat_ecosystem"
		case containsAnyFold(value, "抖音", "douyin", "tiktok"):
			value = "douyin"
		case containsAnyFold(value, "快手", "kuaishou"):
			value = "kuaishou"
		case containsAnyFold(value, "淘宝", "天猫", "taobao", "tmall"):
			value = "taobao_tmall"
		}
		result = appendUniqueString(result, value)
	}
	return result
}

func normalizedUnique(values []string) []string {
	result := []string{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !containsString(result, value) {
			result = append(result, value)
		}
	}
	return result
}

func containsString(values []string, candidate string) bool {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == candidate {
			return true
		}
	}
	return false
}

func appendUniqueString(values []string, value string) []string {
	if !containsString(values, value) {
		return append(values, value)
	}
	return values
}

func containsAnyFold(value string, candidates ...string) bool {
	value = strings.ToLower(value)
	for _, candidate := range candidates {
		if strings.Contains(value, strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}

func scanCreativeBusinessProfile(row interface{ Scan(...any) error }) (creativecatalog.Profile, error) {
	var payload json.RawMessage
	var contentHash, skillHash string
	if err := row.Scan(&payload, &contentHash, &skillHash); err != nil {
		if err == sql.ErrNoRows {
			return creativecatalog.Profile{}, ErrNotFound
		}
		return creativecatalog.Profile{}, err
	}
	var profile creativecatalog.Profile
	if err := json.Unmarshal(payload, &profile); err != nil {
		return creativecatalog.Profile{}, err
	}
	profile.ContentHash = contentHash
	profile.SkillContentHash = skillHash
	return profile, nil
}

func (s Service) getCreativeBusinessVersion(
	ctx context.Context,
	businessCode string,
	generation int64,
) (creativecatalog.Profile, error) {
	return scanCreativeBusinessProfile(s.DB.QueryRowContext(ctx, `SELECT profile, content_hash,
		skill_content_hash FROM strategy_creative_business_profiles
		WHERE business_code = ? AND generation = ?`, businessCode, generation))
}
