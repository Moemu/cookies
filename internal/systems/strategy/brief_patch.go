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
				if len(normalized) == 0 {
					normalized = append(normalized, "xiaohongshu")
				}
			default:
				return fmt.Errorf("%w: first release supports only xiaohongshu", ErrInvalidRequest)
			}
		}
		operation.Value = mustJSON(normalized)
	}
	return nil
}

func setBriefField(document *BriefDocument, path string, raw json.RawMessage) error {
	switch path {
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
			if value != "xiaohongshu" {
				return fmt.Errorf("%w: first release supports only xiaohongshu", ErrInvalidRequest)
			}
		}
		document.Channels = values
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
