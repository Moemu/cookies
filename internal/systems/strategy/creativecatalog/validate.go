package creativecatalog

import (
	"fmt"
	"strings"
)

var allowedRuleFields = map[string]bool{
	"objective_type": true, "channels": true, "deliverable_type": true,
	"industry": true, "asset_roles": true, "reference_present": true,
	"content_context": true, "brand_goal": true,
}

var allowedRuleOperators = map[string]bool{
	"equals": true, "in": true, "contains": true, "present": true, "count_gte": true,
}

var allowedQuestionTypes = map[string]bool{
	"text": true, "textarea": true, "single_select": true, "multi_select": true,
	"boolean": true, "asset_ref": true, "reference_locator": true,
}

var allowedRequiredStages = map[string]bool{
	"recommendation": true, "strategy": true, "production": true,
}

var allowedOutputTypes = map[string]bool{
	"string": true, "string_array": true, "boolean": true,
}

func (p Profile) Validate() error {
	if strings.TrimSpace(p.BusinessCode) == "" || p.Generation < 1 ||
		strings.TrimSpace(p.Version) == "" || strings.TrimSpace(p.DisplayName) == "" ||
		strings.TrimSpace(p.Summary) == "" || p.DisplayOrder < 1 ||
		strings.TrimSpace(p.SkillName) == "" || strings.TrimSpace(p.SkillVersion) == "" ||
		strings.TrimSpace(p.SkillContentHash) == "" || strings.TrimSpace(p.Owner) == "" ||
		p.PublishedAt.IsZero() {
		return fmt.Errorf("profile identity and publication fields are required")
	}
	switch p.Lifecycle {
	case "draft", "active", "deprecated", "retired":
	default:
		return fmt.Errorf("unsupported lifecycle %q", p.Lifecycle)
	}
	if p.Lifecycle == "retired" && p.Selectable {
		return fmt.Errorf("retired profile cannot be selectable")
	}
	if len(p.MatchRules) == 0 || len(p.MatchRules) > 32 {
		return fmt.Errorf("profile must contain 1..32 match rules")
	}
	ruleIDs := map[string]bool{}
	for _, rule := range p.MatchRules {
		if strings.TrimSpace(rule.ID) == "" || ruleIDs[rule.ID] ||
			!allowedRuleFields[rule.Field] || !allowedRuleOperators[rule.Operator] ||
			rule.Weight < -100 || rule.Weight > 100 || strings.TrimSpace(rule.Reason) == "" {
			return fmt.Errorf("invalid recommendation rule %q", rule.ID)
		}
		ruleIDs[rule.ID] = true
		if rule.Operator != "present" && rule.Operator != "count_gte" && len(rule.Values) == 0 {
			return fmt.Errorf("recommendation rule %q requires values", rule.ID)
		}
	}
	questionIDs := map[string]int{}
	for index, question := range p.Questions {
		if strings.TrimSpace(question.ID) == "" || questionIDs[question.ID] > 0 ||
			strings.TrimSpace(question.Label) == "" || !allowedQuestionTypes[question.Type] ||
			!allowedRequiredStages[question.RequiredFor] {
			return fmt.Errorf("invalid question %q", question.ID)
		}
		questionIDs[question.ID] = index + 1
		if (question.Type == "single_select" || question.Type == "multi_select") && len(question.Options) == 0 {
			return fmt.Errorf("question %q requires options", question.ID)
		}
		if question.DependsOn != nil {
			position, found := questionIDs[question.DependsOn.QuestionID]
			if !found || position > index {
				return fmt.Errorf("question %q dependency must reference an earlier question", question.ID)
			}
		}
	}
	outputKeys := map[string]bool{}
	for _, field := range p.OutputFields {
		if strings.TrimSpace(field.Key) == "" || outputKeys[field.Key] ||
			strings.TrimSpace(field.Label) == "" || !allowedOutputTypes[field.Type] ||
			strings.TrimSpace(field.Description) == "" {
			return fmt.Errorf("invalid output field %q", field.Key)
		}
		outputKeys[field.Key] = true
	}
	return nil
}
