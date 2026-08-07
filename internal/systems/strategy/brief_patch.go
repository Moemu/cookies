package strategy

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type PatchOrigin string

const (
	PatchFromUser  PatchOrigin = "user"
	PatchFromModel PatchOrigin = "model"
)

func ApplyBriefPatch(draft BriefDraft, patch BriefPatch, origin PatchOrigin, actorID string, now time.Time) (BriefDraft, error) {
	expected := patch.ExpectedVersion
	if expected == 0 {
		expected = patch.BaseVersion
	}
	if expected != draft.Version {
		return BriefDraft{}, ErrVersionConflict
	}
	if draft.Status != "open" || len(patch.Operations) == 0 {
		return BriefDraft{}, ErrInvalidState
	}
	if len(patch.Operations) > 32 {
		return BriefDraft{}, fmt.Errorf("%w: too many patch operations", ErrInvalidRequest)
	}
	if draft.FieldStates == nil {
		draft.FieldStates = map[string]FieldState{}
	}
	for _, operation := range patch.Operations {
		if operation.Op != "set" || !json.Valid(operation.Value) {
			return BriefDraft{}, fmt.Errorf("%w: only valid set operations are supported", ErrInvalidRequest)
		}
		currentState, exists := draft.FieldStates[operation.FieldPath]
		if origin == PatchFromModel && exists && currentState.Confirmation == "confirmed" {
			currentState.Conflicts = append(currentState.Conflicts, operation.Source)
			draft.FieldStates[operation.FieldPath] = currentState
			continue
		}
		if err := setBriefField(&draft.Document, operation.FieldPath, operation.Value); err != nil {
			return BriefDraft{}, err
		}
		state := FieldState{
			FieldPath: operation.FieldPath, Source: operation.Source, Confidence: operation.Confidence,
			Confirmation: operation.Confirmation, UpdatedBy: actorID, UpdatedAt: now.UTC(), Conflicts: []FieldSource{},
		}
		if origin == PatchFromUser {
			state.Source = FieldSource{Type: "user_edit", ID: actorID}
			state.Confidence = "high"
			state.Confirmation = "confirmed"
		} else {
			if state.Source.Type == "" || state.Source.ID == "" {
				return BriefDraft{}, fmt.Errorf("%w: model patch source is required", ErrInvalidRequest)
			}
			if state.Confidence != "low" && state.Confidence != "medium" && state.Confidence != "high" {
				return BriefDraft{}, fmt.Errorf("%w: model confidence is invalid", ErrInvalidRequest)
			}
			state.Confirmation = "unconfirmed"
		}
		draft.FieldStates[operation.FieldPath] = state
	}
	draft.Version++
	draft.UpdatedBy = actorID
	draft.UpdatedAt = now.UTC()
	draft.Completeness = ComputeCompleteness(draft.Document, draft.FieldStates)
	return draft, nil
}

func normalizeModelBriefPatch(patch *BriefPatch) error {
	if patch == nil {
		return fmt.Errorf("%w: brief patch is required", ErrInvalidRequest)
	}
	for index := range patch.Operations {
		operation := &patch.Operations[index]
		switch operation.FieldPath {
		case "product.candidates":
			values, err := normalizeModelProductCandidates(operation.Value)
			if err != nil {
				return err
			}
			operation.Value = mustJSON(values)
		case "product.selling_points", "product.evidence", "channels", "constraints",
			"reference_ids", "creative.tone", "creative.mandatory_elements", "creative.prohibited_claims":
			values, err := normalizeModelStringArray(operation.Value, operation.FieldPath)
			if err != nil {
				return err
			}
			operation.Value = mustJSON(values)
		case "brand.name", "product.name", "product.category", "industry", "region", "language",
			"campaign.objective", "audience.primary", "proposition", "budget.total",
			"schedule.window", "measurement.primary_kpi":
			value, err := normalizeModelString(operation.Value, operation.FieldPath)
			if err != nil {
				return err
			}
			operation.Value = mustJSON(value)
		}
		if operation.FieldPath != "channels" {
			continue
		}
		var values []string
		_ = json.Unmarshal(operation.Value, &values)
		normalized := make([]string, 0, len(values))
		for _, value := range values {
			channel := strings.ToLower(strings.TrimSpace(value))
			switch {
			case channel == "xiaohongshu" || strings.Contains(channel, "小红书") ||
				strings.Contains(channel, "rednote") || strings.Contains(channel, "red note") ||
				strings.Contains(channel, "redbook") || strings.Contains(channel, "red book"):
				normalized = appendUnique(normalized, "xiaohongshu")
			case channel == "douyin" || strings.Contains(channel, "抖音"):
				if patch.ContractVersion != "strategy-brief-patch/v2" {
					return fmt.Errorf("%w: unsupported strategy platform %q", ErrInvalidRequest, value)
				}
				normalized = appendUnique(normalized, "douyin")
			case channel == "taobao_tmall" || strings.Contains(channel, "taobao") ||
				strings.Contains(channel, "tmall") || strings.Contains(channel, "淘宝") ||
				strings.Contains(channel, "天猫"):
				if patch.ContractVersion != "strategy-brief-patch/v2" {
					return fmt.Errorf("%w: unsupported strategy platform %q", ErrInvalidRequest, value)
				}
				normalized = appendUnique(normalized, "taobao_tmall")
			case channel == "wechat_ecosystem" || strings.Contains(channel, "wechat") ||
				strings.Contains(channel, "微信") || strings.Contains(channel, "公众号") ||
				strings.Contains(channel, "视频号") || strings.Contains(channel, "私域"):
				if patch.ContractVersion != "strategy-brief-patch/v2" {
					return fmt.Errorf("%w: unsupported strategy platform %q", ErrInvalidRequest, value)
				}
				normalized = appendUnique(normalized, "wechat_ecosystem")
			default:
				return fmt.Errorf("%w: unsupported strategy platform %q", ErrInvalidRequest, value)
			}
		}
		operation.Value = mustJSON(normalized)
	}
	return nil
}

func normalizeModelString(raw json.RawMessage, path string) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		value = strings.TrimSpace(value)
		if value != "" {
			return value, nil
		}
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || len(values) == 0 {
		return "", fmt.Errorf("%w: %s must be a non-empty string", ErrInvalidRequest, path)
	}
	normalized := make([]string, 0, len(values))
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			return "", fmt.Errorf("%w: %s entries must not be empty", ErrInvalidRequest, path)
		}
		normalized = append(normalized, item)
	}
	return strings.Join(normalized, "；"), nil
}

func normalizeModelStringArray(raw json.RawMessage, path string) ([]string, error) {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		var value string
		if stringErr := json.Unmarshal(raw, &value); stringErr != nil || strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%w: %s must be a non-empty string array", ErrInvalidRequest, path)
		}
		values = []string{strings.TrimSpace(value)}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%w: %s must be a non-empty string array", ErrInvalidRequest, path)
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("%w: %s entries must not be empty", ErrInvalidRequest, path)
		}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func setBriefField(document *BriefDocument, path string, raw json.RawMessage) error {
	switch path {
	case "brand.name":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: brand.name requires Brief v2", ErrInvalidRequest)
		}
		return decodeString(raw, &document.Brand.Name, path)
	case "product.name":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: product.name requires Brief v2", ErrInvalidRequest)
		}
		return decodeString(raw, &document.Product.Name, path)
	case "product.category":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: product.category requires Brief v2", ErrInvalidRequest)
		}
		return decodeOptionalString(raw, &document.Product.Category, path)
	case "product.selling_points":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: product.selling_points requires Brief v2", ErrInvalidRequest)
		}
		return decodeStringSlice(raw, &document.Product.SellingPoints, path)
	case "product.evidence":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: product.evidence requires Brief v2", ErrInvalidRequest)
		}
		return decodeStringSlice(raw, &document.Product.Evidence, path)
	case "product.candidates":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: product.candidates requires Brief v2", ErrInvalidRequest)
		}
		values, err := normalizeModelProductCandidates(raw)
		if err != nil {
			return err
		}
		document.Product.Candidates = values
		return nil
	case "product.asset_refs":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: product.asset_refs requires Brief v2", ErrInvalidRequest)
		}
		var values []contract.AssetVersionRef
		if err := json.Unmarshal(raw, &values); err != nil || len(values) == 0 || len(values) > 20 {
			return fmt.Errorf("%w: product.asset_refs must contain 1 to 20 asset references", ErrInvalidRequest)
		}
		seen := make(map[contract.AssetVersionRef]struct{}, len(values))
		for _, value := range values {
			if err := value.Validate(); err != nil {
				return fmt.Errorf("%w: product.asset_refs contains an invalid reference: %v", ErrInvalidRequest, err)
			}
			if _, exists := seen[value]; exists {
				return fmt.Errorf("%w: product.asset_refs contains a duplicate reference", ErrInvalidRequest)
			}
			seen[value] = struct{}{}
		}
		document.Product.AssetRefs = append([]contract.AssetVersionRef(nil), values...)
		return nil
	case "industry":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: industry requires Brief v2", ErrInvalidRequest)
		}
		return decodeString(raw, &document.Industry, path)
	case "region":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: region requires Brief v2", ErrInvalidRequest)
		}
		return decodeString(raw, &document.Region, path)
	case "language":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: language requires Brief v2", ErrInvalidRequest)
		}
		return decodeString(raw, &document.Language, path)
	case "campaign.objective":
		return decodeString(raw, &document.Campaign.Objective, path)
	case "audience.primary":
		return decodeString(raw, &document.Audience.Primary, path)
	case "proposition":
		return decodeString(raw, &document.Proposition, path)
	case "channels":
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil || len(values) == 0 {
			return fmt.Errorf("%w: channels must be a non-empty string array", ErrInvalidRequest)
		}
		for _, value := range values {
			if !supportedPlatform(value) {
				return fmt.Errorf("%w: unsupported strategy platform %q", ErrInvalidRequest, value)
			}
			if document.ContractVersion == "strategy-brief-version/v1" && value != "xiaohongshu" {
				return fmt.Errorf("%w: Brief v1 supports only xiaohongshu", ErrInvalidRequest)
			}
		}
		document.Channels = values
		if document.ContractVersion == "strategy-brief-version/v2" {
			document.PlatformBriefs = syncPlatformBriefs(document.PlatformBriefs, values)
		}
		return nil
	case "budget.total":
		return decodeString(raw, &document.Budget.Total, path)
	case "schedule.window":
		return decodeString(raw, &document.Schedule.Window, path)
	case "constraints":
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil {
			return fmt.Errorf("%w: constraints must be a string array", ErrInvalidRequest)
		}
		document.Constraints = values
		return nil
	case "measurement.primary_kpi":
		return decodeString(raw, &document.Measurement.PrimaryKPI, path)
	case "reference_ids":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: reference_ids requires Brief v2", ErrInvalidRequest)
		}
		return decodeStringSlice(raw, &document.ReferenceIDs, path)
	case "creative.tone":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: creative.tone requires Brief v2", ErrInvalidRequest)
		}
		return decodeStringSlice(raw, &document.Creative.Tone, path)
	case "creative.mandatory_elements":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: creative.mandatory_elements requires Brief v2", ErrInvalidRequest)
		}
		return decodeStringSlice(raw, &document.Creative.MandatoryElements, path)
	case "creative.prohibited_claims":
		if document.ContractVersion != "strategy-brief-version/v2" {
			return fmt.Errorf("%w: creative.prohibited_claims requires Brief v2", ErrInvalidRequest)
		}
		return decodeStringSlice(raw, &document.Creative.ProhibitedClaims, path)
	default:
		return fmt.Errorf("%w: field_path %q is not writable", ErrInvalidRequest, path)
	}
}

func normalizeModelProductCandidates(raw json.RawMessage) ([]BriefProductCandidate, error) {
	var values []BriefProductCandidate
	if err := json.Unmarshal(raw, &values); err != nil || len(values) == 0 || len(values) > 12 {
		return nil, fmt.Errorf("%w: product.candidates must contain 1 to 12 candidates", ErrInvalidRequest)
	}
	seen := make(map[string]struct{}, len(values))
	for index := range values {
		candidate := &values[index]
		candidate.Name = strings.TrimSpace(candidate.Name)
		candidate.Category = strings.TrimSpace(candidate.Category)
		if candidate.Name == "" || len([]rune(candidate.Name)) > 160 {
			return nil, fmt.Errorf("%w: product.candidates contains an invalid name", ErrInvalidRequest)
		}
		key := strings.ToLower(candidate.Name)
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("%w: product.candidates contains duplicate names", ErrInvalidRequest)
		}
		seen[key] = struct{}{}
		lists := []*[]string{
			&candidate.SellingPoints, &candidate.Evidence,
			&candidate.MandatoryElements, &candidate.ProhibitedClaims,
		}
		for _, list := range lists {
			if len(*list) > 20 {
				return nil, fmt.Errorf("%w: product.candidates contains too many facts", ErrInvalidRequest)
			}
			normalized := make([]string, 0, len(*list))
			for _, item := range *list {
				item = strings.TrimSpace(item)
				if item == "" || len([]rune(item)) > 500 {
					return nil, fmt.Errorf("%w: product.candidates contains an invalid fact", ErrInvalidRequest)
				}
				normalized = appendUnique(normalized, item)
			}
			*list = normalized
		}
		candidate.SourceRefs = uniqueFieldSources(candidate.SourceRefs)
	}
	return values, nil
}

func uniqueFieldSources(values []FieldSource) []FieldSource {
	result := make([]FieldSource, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value.Type = strings.TrimSpace(value.Type)
		value.ID = strings.TrimSpace(value.ID)
		value.Locator = strings.TrimSpace(value.Locator)
		if value.Type == "" || value.ID == "" {
			continue
		}
		key := value.Type + "\x00" + value.ID + "\x00" + value.Locator
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func decodeString(raw json.RawMessage, target *string, path string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" || len(value) > 4096 {
		return fmt.Errorf("%w: %s must be a non-empty string", ErrInvalidRequest, path)
	}
	*target = strings.TrimSpace(value)
	return nil
}

func decodeOptionalString(raw json.RawMessage, target *string, path string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || len(value) > 4096 {
		return fmt.Errorf("%w: %s must be a string", ErrInvalidRequest, path)
	}
	*target = strings.TrimSpace(value)
	return nil
}

func ComputeCompleteness(document BriefDocument, states map[string]FieldState) Completeness {
	if document.ContractVersion == BriefContractVersionV3 {
		return computeBriefV3Completeness(document, states)
	}
	required := []struct {
		path  string
		value bool
	}{
		{"campaign.objective", strings.TrimSpace(document.Campaign.Objective) != ""},
		{"audience.primary", strings.TrimSpace(document.Audience.Primary) != ""},
		{"proposition", strings.TrimSpace(document.Proposition) != ""},
		{"channels", len(document.Channels) > 0},
	}
	result := Completeness{Blockers: []ValidationError{}, Warnings: []ValidationError{}}
	if document.ContractVersion == "strategy-brief-version/v2" {
		// Brief v2 predates the Requirement aggregate, but its confirmation gate
		// now follows the same business rule: only facts required to start a
		// Creative intake are blockers. Brand taxonomy, region, language and
		// channels remain useful context instead of mandatory form fields.
		required = []struct {
			path  string
			value bool
		}{
			{"product.name", strings.TrimSpace(document.Product.Name) != ""},
			{"campaign.objective", strings.TrimSpace(document.Campaign.Objective) != ""},
			{"audience.primary", strings.TrimSpace(document.Audience.Primary) != ""},
		}
	}
	for _, field := range required {
		if !field.value {
			result.Blockers = append(result.Blockers, ValidationError{Field: field.path, Reason: "必填信息缺失"})
			continue
		}
		if states[field.path].Confirmation != "confirmed" {
			result.Blockers = append(result.Blockers, ValidationError{Field: field.path, Reason: "需要用户确认"})
		}
	}
	if strings.TrimSpace(document.Budget.Total) == "" {
		result.Warnings = append(result.Warnings, ValidationError{Field: "budget.total", Reason: "未提供预算，投放准备度将为未就绪"})
	}
	if strings.TrimSpace(document.Measurement.PrimaryKPI) == "" {
		result.Warnings = append(result.Warnings, ValidationError{Field: "measurement.primary_kpi", Reason: "未提供核心指标，洞察准备度将为未就绪"})
	}
	if document.ContractVersion == "strategy-brief-version/v2" && strings.TrimSpace(document.Proposition) == "" {
		result.Warnings = append(result.Warnings, ValidationError{Field: "proposition", Reason: "未提供核心主张，快捷创作将以产品主题兜底"})
	}
	if document.ContractVersion == "strategy-brief-version/v2" && len(document.Channels) == 0 {
		result.Warnings = append(result.Warnings, ValidationError{Field: "channels", Reason: "未指定渠道，快捷业务将使用其默认渠道"})
	}
	result.Ready = len(result.Blockers) == 0
	return result
}

func computeBriefV3Completeness(document BriefDocument, states map[string]FieldState) Completeness {
	required := []struct {
		path  string
		value bool
	}{
		{"core.objective", strings.TrimSpace(document.Core.Objective) != ""},
		{"core.deliverable_intent", strings.TrimSpace(document.Core.DeliverableIntent) != ""},
		{"core.product_or_subject", strings.TrimSpace(document.Core.ProductOrSubject) != ""},
		{"core.audience", strings.TrimSpace(document.Core.Audience) != ""},
	}
	result := Completeness{Blockers: []ValidationError{}, Warnings: []ValidationError{}}
	for _, field := range required {
		if !field.value {
			result.Blockers = append(result.Blockers, ValidationError{Field: field.path, Reason: "缺少进入创作所需的核心信息"})
			continue
		}
		if states[field.path].Confirmation != "confirmed" {
			result.Blockers = append(result.Blockers, ValidationError{Field: field.path, Reason: "需要用户确认"})
		}
	}
	for _, unknown := range document.Unknowns {
		if unknown.RequiredFor == "creative_intake" {
			result.Blockers = append(result.Blockers, ValidationError{Field: "unknowns." + unknown.ID, Reason: unknown.Question})
		}
	}
	for _, conflict := range document.Conflicts {
		if conflict.Status == "open" {
			result.Blockers = append(result.Blockers, ValidationError{Field: "conflicts." + conflict.ID, Reason: "存在尚未解决的关键冲突"})
		}
	}
	if len(document.Assumptions) > 0 {
		result.Warnings = append(result.Warnings, ValidationError{Field: "assumptions", Reason: "包含尚未证实的假设，生成结果会明确标注"})
	}
	result.Ready = len(result.Blockers) == 0
	return result
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func syncPlatformBriefs(existing []BriefPlatform, channels []string) []BriefPlatform {
	byPlatform := make(map[string]BriefPlatform, len(existing))
	for _, value := range existing {
		byPlatform[value.Platform] = value
	}
	result := make([]BriefPlatform, 0, len(channels))
	for _, channel := range channels {
		value := byPlatform[channel]
		value.Platform = channel
		result = append(result, value)
	}
	return result
}
