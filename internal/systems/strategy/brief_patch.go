package strategy

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
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
		if operation.FieldPath != "channels" {
			continue
		}
		var values []string
		if err := json.Unmarshal(operation.Value, &values); err != nil || len(values) == 0 {
			return fmt.Errorf("%w: channels must be a non-empty string array", ErrInvalidRequest)
		}
		normalized := make([]string, 0, len(values))
		for _, value := range values {
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "xiaohongshu", "小红书", "小红书图文", "rednote", "red note", "redbook", "red book":
				normalized = appendUnique(normalized, "xiaohongshu")
			case "douyin", "抖音", "抖音短视频":
				if patch.ContractVersion != "strategy-brief-patch/v2" {
					return fmt.Errorf("%w: unsupported strategy platform %q", ErrInvalidRequest, value)
				}
				normalized = appendUnique(normalized, "douyin")
			case "taobao_tmall", "taobao", "tmall", "淘宝", "天猫", "淘宝天猫", "淘宝/天猫":
				if patch.ContractVersion != "strategy-brief-patch/v2" {
					return fmt.Errorf("%w: unsupported strategy platform %q", ErrInvalidRequest, value)
				}
				normalized = appendUnique(normalized, "taobao_tmall")
			case "wechat_ecosystem", "wechat", "微信", "公众号", "视频号", "微信生态", "私域":
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
	default:
		return fmt.Errorf("%w: field_path %q is not writable", ErrInvalidRequest, path)
	}
}

func decodeString(raw json.RawMessage, target *string, path string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" || len(value) > 4096 {
		return fmt.Errorf("%w: %s must be a non-empty string", ErrInvalidRequest, path)
	}
	*target = strings.TrimSpace(value)
	return nil
}

func ComputeCompleteness(document BriefDocument, states map[string]FieldState) Completeness {
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
		required = append([]struct {
			path  string
			value bool
		}{
			{"brand.name", strings.TrimSpace(document.Brand.Name) != ""},
			{"product.name", strings.TrimSpace(document.Product.Name) != ""},
			{"industry", strings.TrimSpace(document.Industry) != ""},
			{"region", strings.TrimSpace(document.Region) != ""},
			{"language", strings.TrimSpace(document.Language) != ""},
		}, required...)
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
