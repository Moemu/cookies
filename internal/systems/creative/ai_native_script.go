package creative

import (
	"fmt"
	"strings"
	"time"
)

const (
	AINativeStageScript            = "script"
	AINativeScriptDraftStatus      = "draft"
	AINativeScriptConfirmedStatus  = "confirmed"
	AINativeScriptSupersededStatus = "superseded"
	AINativeScriptPurposeHook      = "hook"
	AINativeScriptPurposePain      = "pain"
	AINativeScriptPurposeProof     = "proof"
	AINativeScriptPurposeBenefit   = "benefit"
	AINativeScriptPurposeCTA       = "cta"
	aiNativeScriptContract         = "creative.ai-native.script/v1"
)

type AINativeScriptSegment struct {
	ID               string   `json:"id"`
	StartMS          int      `json:"start_ms"`
	EndMS            int      `json:"end_ms"`
	Purpose          string   `json:"purpose"`
	VisualIntent     string   `json:"visual_intent"`
	Voiceover        string   `json:"voiceover"`
	Subtitle         string   `json:"subtitle"`
	SellingPointIDs  []string `json:"selling_point_ids"`
	ConversionAction string   `json:"conversion_action,omitempty"`
}

type AINativeScriptGenerationMetadata struct {
	ModelAlias      string `json:"model_alias"`
	ModelVersion    string `json:"model_version"`
	RouteRevisionID string `json:"route_revision_id,omitempty"`
	PromptVersion   string `json:"prompt_version"`
	ProfileHash     string `json:"profile_hash"`
	InputTokens     int64  `json:"input_tokens,omitempty"`
	OutputTokens    int64  `json:"output_tokens,omitempty"`
	TotalTokens     int64  `json:"total_tokens,omitempty"`
	LatencyMS       int64  `json:"latency_ms,omitempty"`
}

type AINativeScriptRevision struct {
	ContractVersion            string                           `json:"contract_version"`
	Revision                   int64                            `json:"revision"`
	Status                     string                           `json:"status"`
	Title                      string                           `json:"title"`
	CreativeSummary            string                           `json:"creative_summary"`
	ChannelProfileID           string                           `json:"channel_profile_id"`
	ChannelProfileHash         string                           `json:"channel_profile_hash"`
	DurationSeconds            int                              `json:"duration_seconds"`
	Segments                   []AINativeScriptSegment          `json:"segments"`
	RegenerationNote           string                           `json:"regeneration_note,omitempty"`
	BasedOnRevision            *int64                           `json:"based_on_revision,omitempty"`
	BasedOnRequirementRevision int64                            `json:"based_on_requirement_revision"`
	BasedOnRequirementHash     string                           `json:"based_on_requirement_hash"`
	Generation                 AINativeScriptGenerationMetadata `json:"generation"`
	CreatedBy                  string                           `json:"created_by,omitempty"`
	ConfirmedBy                string                           `json:"confirmed_by,omitempty"`
	CreatedAt                  time.Time                        `json:"created_at,omitempty"`
	ConfirmedAt                *time.Time                       `json:"confirmed_at,omitempty"`
}

func (s AINativeScriptRevision) ValidateAgainst(requirement AINativeRequirementDraft) error {
	if s.ContractVersion != aiNativeScriptContract || s.Revision < 1 ||
		(s.Status != AINativeScriptDraftStatus && s.Status != AINativeScriptConfirmedStatus && s.Status != AINativeScriptSupersededStatus) ||
		strings.TrimSpace(s.Title) == "" || strings.TrimSpace(s.CreativeSummary) == "" ||
		strings.TrimSpace(s.ChannelProfileID) == "" || len(s.ChannelProfileHash) != 64 ||
		s.DurationSeconds != requirement.DurationSeconds || len(s.Segments) < 3 ||
		s.BasedOnRequirementRevision != requirement.Revision || len(s.BasedOnRequirementHash) != 64 ||
		strings.TrimSpace(s.Generation.ModelAlias) == "" || strings.TrimSpace(s.Generation.ModelVersion) == "" ||
		s.Generation.PromptVersion != aiNativeScriptPromptVersion || s.Generation.ProfileHash != s.ChannelProfileHash {
		return fmt.Errorf("AI native script is invalid")
	}
	knownPoints := make(map[string]struct{}, len(requirement.CoreSellingPoints))
	for _, point := range requirement.CoreSellingPoints {
		knownPoints[point.ID] = struct{}{}
	}
	lastEnd := 0
	hasHook, hasProof, hasCTA := false, false, false
	for index, segment := range s.Segments {
		if strings.TrimSpace(segment.ID) == "" || segment.StartMS != lastEnd || segment.EndMS <= segment.StartMS ||
			strings.TrimSpace(segment.VisualIntent) == "" || strings.TrimSpace(segment.Voiceover) == "" || strings.TrimSpace(segment.Subtitle) == "" {
			return fmt.Errorf("AI native script segment %d is invalid or timeline is not contiguous", index)
		}
		switch segment.Purpose {
		case AINativeScriptPurposeHook:
			hasHook = true
		case AINativeScriptPurposePain, AINativeScriptPurposeBenefit:
		case AINativeScriptPurposeProof:
			hasProof = true
		case AINativeScriptPurposeCTA:
			hasCTA = strings.TrimSpace(segment.ConversionAction) != ""
		default:
			return fmt.Errorf("AI native script segment %d purpose is invalid", index)
		}
		for _, pointID := range segment.SellingPointIDs {
			if _, ok := knownPoints[pointID]; !ok {
				return fmt.Errorf("AI native script segment %d references unknown selling point %q", index, pointID)
			}
		}
		lastEnd = segment.EndMS
	}
	if lastEnd != requirement.DurationSeconds*1000 || !hasHook || !hasProof || !hasCTA {
		return fmt.Errorf("AI native script must close at target duration and include hook, proof and CTA")
	}
	return nil
}
