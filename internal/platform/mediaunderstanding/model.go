package mediaunderstanding

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const ContractVersion = "platform-media-understanding-artifact/v1"
const DefaultProfile = "strategy.multimodal.p0"
const DefaultProfileVersion = "v2"
const PromptVersion = "media.understand.v2"
const SchemaVersion = "media-understanding-output/v2"

type Status string

const (
	StatusRunning Status = "running"
	StatusReady   Status = "ready"
	StatusPartial Status = "partial"
	StatusFailed  Status = "failed"
)

type Locator struct {
	Kind        string                    `json:"kind"`
	AssetRef    contract.ProjectAssetRef  `json:"asset_ref"`
	TimestampMS *int64                    `json:"timestamp_ms,omitempty"`
	FrameRef    *contract.ProjectAssetRef `json:"frame_ref,omitempty"`
}

type Evidence struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Locator    Locator `json:"locator"`
}

type Keyframe struct {
	TimestampMS int64                    `json:"timestamp_ms"`
	FrameRef    contract.ProjectAssetRef `json:"frame_ref"`
}

type ModelLineage struct {
	ProviderCode    string `json:"provider_code,omitempty"`
	ModelAlias      string `json:"model_alias,omitempty"`
	ModelVersion    string `json:"model_version,omitempty"`
	RouteRevisionID string `json:"route_revision_id,omitempty"`
	PromptVersion   string `json:"prompt_version"`
	SchemaVersion   string `json:"schema_version"`
}

type TranscriptionLineage struct {
	ProviderCode string `json:"provider_code"`
	ModelVersion string `json:"model_version"`
}

type Classification struct {
	Code         string   `json:"code"`
	Confidence   float64  `json:"confidence"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type Classifications struct {
	MediaFormat  Classification `json:"media_format"`
	ContentStyle Classification `json:"content_style"`
}

type Artifact struct {
	ContractVersion      string                   `json:"contract_version"`
	ID                   string                   `json:"id"`
	OrganizationID       contract.OrganizationID  `json:"organization_id"`
	ProjectID            contract.ProjectID       `json:"project_id"`
	AssetRef             contract.ProjectAssetRef `json:"asset_ref"`
	AssetKind            contract.AssetKind       `json:"asset_kind"`
	AssetSHA256          string                   `json:"asset_sha256"`
	Profile              string                   `json:"profile"`
	ProfileVersion       string                   `json:"profile_version"`
	InputIdentityHash    string                   `json:"input_identity_hash"`
	Status               Status                   `json:"status"`
	JobID                string                   `json:"job_id,omitempty"`
	Summary              string                   `json:"summary,omitempty"`
	VisibleText          []Evidence               `json:"visible_text"`
	Observations         []Evidence               `json:"observations"`
	Inferences           []Evidence               `json:"inferences"`
	Risks                []Evidence               `json:"risks"`
	Unknowns             []Evidence               `json:"unknowns"`
	Keyframes            []Keyframe               `json:"keyframes"`
	Transcript           []Evidence               `json:"transcript"`
	TranscriptionLineage *TranscriptionLineage    `json:"transcription_lineage,omitempty"`
	Classifications      Classifications          `json:"classifications"`
	Warnings             []string                 `json:"warnings"`
	Lineage              ModelLineage             `json:"model_lineage"`
	ContentHash          string                   `json:"content_hash"`
	ErrorCode            string                   `json:"error_code,omitempty"`
	ErrorMessage         string                   `json:"error_message,omitempty"`
	CreatedBy            contract.Principal       `json:"created_by"`
	CreatedAt            time.Time                `json:"created_at"`
	UpdatedAt            time.Time                `json:"updated_at"`
}

func (a Artifact) Validate() error {
	if a.ContractVersion != ContractVersion || strings.TrimSpace(a.ID) == "" || a.OrganizationID == "" || a.ProjectID == "" {
		return fmt.Errorf("media understanding identity is invalid")
	}
	if err := a.AssetRef.Validate(); err != nil || a.AssetRef.ProjectID != a.ProjectID {
		return fmt.Errorf("media understanding asset reference is invalid")
	}
	if a.AssetKind != contract.AssetImage && a.AssetKind != contract.AssetVideo {
		return fmt.Errorf("media understanding asset kind is invalid")
	}
	if len(a.AssetSHA256) != 64 || len(a.InputIdentityHash) != 64 || strings.TrimSpace(a.Profile) == "" || strings.TrimSpace(a.ProfileVersion) == "" {
		return fmt.Errorf("media understanding identity hash is invalid")
	}
	switch a.Status {
	case StatusRunning, StatusReady, StatusPartial, StatusFailed:
	default:
		return fmt.Errorf("media understanding status is invalid")
	}
	for _, list := range [][]Evidence{a.VisibleText, a.Observations, a.Inferences, a.Risks, a.Unknowns, a.Transcript} {
		for _, evidence := range list {
			if strings.TrimSpace(evidence.ID) == "" || strings.TrimSpace(evidence.Text) == "" || evidence.Confidence < 0 || evidence.Confidence > 1 {
				return fmt.Errorf("media understanding evidence is invalid")
			}
			if evidence.Locator.AssetRef != a.AssetRef {
				return fmt.Errorf("media understanding evidence points to another asset")
			}
		}
	}
	if len(a.Transcript) > 0 {
		if a.TranscriptionLineage == nil || strings.TrimSpace(a.TranscriptionLineage.ProviderCode) == "" || strings.TrimSpace(a.TranscriptionLineage.ModelVersion) == "" {
			return fmt.Errorf("media understanding transcription lineage is required")
		}
	}
	if !validMediaFormatCode(a.Classifications.MediaFormat.Code) || !validContentStyleCode(a.Classifications.ContentStyle.Code) {
		return fmt.Errorf("media understanding classification code is invalid")
	}
	for _, classification := range []Classification{a.Classifications.MediaFormat, a.Classifications.ContentStyle} {
		if classification.Confidence < 0 || classification.Confidence > 1 {
			return fmt.Errorf("media understanding classification confidence is invalid")
		}
	}
	return nil
}

func validMediaFormatCode(value string) bool {
	switch value {
	case "", "unknown", "single_image", "gif", "image_group", "video", "vertical_video":
		return true
	default:
		return false
	}
}

func validContentStyleCode(value string) bool {
	switch value {
	case "", "unknown", "single_speaker", "multiple_speakers", "human_product_demo_no_voice", "product_voiceover", "product_demo_no_voice", "story_drama":
		return true
	default:
		return false
	}
}

func emptyEvidence() []Evidence  { return []Evidence{} }
func emptyKeyframes() []Keyframe { return []Keyframe{} }
func emptyWarnings() []string    { return []string{} }
