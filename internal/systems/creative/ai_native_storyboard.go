package creative

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	AINativeStageStoryboard = "storyboard"

	AINativeStoryboardDraftStatus      = "draft"
	AINativeStoryboardConfirmedStatus  = "confirmed"
	AINativeStoryboardSupersededStatus = "superseded"
	AINativeStoryboardGeneratingStatus = "generating"
	AINativeStoryboardFailedStatus     = "failed"

	AINativeStoryboardAssetRoleProductIdentity      = "product_identity"
	AINativeStoryboardAssetRolePersonIdentity       = "person_identity"
	AINativeStoryboardAssetRoleSceneReference       = "scene_reference"
	AINativeStoryboardAssetRoleCompositionReference = "composition_reference"
	AINativeStoryboardAssetRoleAudioReference       = "audio_reference"
	AINativeStoryboardAssetRoleBrandElement         = "brand_element"

	AINativeStoryboardAssetSourceProductImport = "product_import"
	AINativeStoryboardAssetSourceProjectAsset  = "project_asset"
	AINativeStoryboardAssetSourceAIGenerated   = "ai_generated"

	AINativeStoryboardAssetReady      = "ready"
	AINativeStoryboardAssetPlanned    = "planned"
	AINativeStoryboardAssetGenerating = "generating"
	AINativeStoryboardAssetFailed     = "failed"

	aiNativeStoryboardContract = "creative.ai-native.storyboard/v1"
)

type AINativeStoryboardAsset struct {
	ID                   string                    `json:"id"`
	Role                 string                    `json:"role"`
	Name                 string                    `json:"name"`
	Source               string                    `json:"source"`
	AssetRef             *contract.AssetVersionRef `json:"asset_ref,omitempty"`
	GenerationBrief      string                    `json:"generation_brief,omitempty"`
	RegenerationFeedback string                    `json:"regeneration_feedback,omitempty"`
	Status               string                    `json:"status"`
	GenerationAttempt    int                       `json:"generation_attempt,omitempty"`
	ErrorCode            string                    `json:"error_code,omitempty"`
	ErrorMessage         string                    `json:"error_message,omitempty"`
}

type AINativeStoryboardShot struct {
	ID                      string   `json:"id"`
	StartMS                 int      `json:"start_ms"`
	EndMS                   int      `json:"end_ms"`
	DurationMS              int      `json:"duration_ms"`
	VisualContent           string   `json:"visual_content"`
	SubjectsProductsActions string   `json:"subjects_products_actions"`
	ShotSize                string   `json:"shot_size"`
	CameraMovement          string   `json:"camera_movement"`
	ReferenceAssetIDs       []string `json:"reference_asset_ids"`
	Voiceover               string   `json:"voiceover"`
	Subtitle                string   `json:"subtitle"`
	SoundEffect             string   `json:"sound_effect"`
	BGMDirection            string   `json:"bgm_direction"`
	Transition              string   `json:"transition"`
	ProductIdentityRequired bool     `json:"product_identity_required"`
}

type AINativeStoryboardGenerationMetadata struct {
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

type AINativeStoryboardRevision struct {
	ContractVersion            string                               `json:"contract_version"`
	Revision                   int64                                `json:"revision"`
	Status                     string                               `json:"status"`
	DurationSeconds            int                                  `json:"duration_seconds"`
	Assets                     []AINativeStoryboardAsset            `json:"assets"`
	Shots                      []AINativeStoryboardShot             `json:"shots"`
	ChannelProfileID           string                               `json:"channel_profile_id"`
	ChannelProfileHash         string                               `json:"channel_profile_hash"`
	BasedOnRequirementRevision int64                                `json:"based_on_requirement_revision"`
	BasedOnRequirementHash     string                               `json:"based_on_requirement_hash"`
	BasedOnScriptRevision      int64                                `json:"based_on_script_revision"`
	BasedOnScriptHash          string                               `json:"based_on_script_hash"`
	Generation                 AINativeStoryboardGenerationMetadata `json:"generation"`
	CreatedBy                  string                               `json:"created_by,omitempty"`
	ConfirmedBy                string                               `json:"confirmed_by,omitempty"`
	CreatedAt                  time.Time                            `json:"created_at,omitempty"`
	ConfirmedAt                *time.Time                           `json:"confirmed_at,omitempty"`
}

type GenerateAINativeStoryboardRequest struct {
	ExpectedWorkspaceVersion int64 `json:"expected_workspace_version"`
}

type RegenerateAINativeStoryboardAssetRequest struct {
	ExpectedWorkspaceVersion int64  `json:"expected_workspace_version"`
	Feedback                 string `json:"feedback,omitempty"`
}

type UpdateAINativeStoryboardRequest struct {
	ExpectedRevision int64                      `json:"expected_revision"`
	Storyboard       AINativeStoryboardRevision `json:"storyboard"`
}

type ConfirmAINativeStoryboardRequest struct {
	ExpectedRevision         int64 `json:"expected_revision"`
	ExpectedWorkspaceVersion int64 `json:"expected_workspace_version"`
}

func (s AINativeStoryboardRevision) ValidatePlanAgainst(requirement AINativeRequirementDraft, script AINativeScriptRevision) error {
	if s.ContractVersion != aiNativeStoryboardContract || s.Revision < 1 ||
		(s.Status != AINativeStoryboardDraftStatus && s.Status != AINativeStoryboardConfirmedStatus && s.Status != AINativeStoryboardSupersededStatus) ||
		s.DurationSeconds != requirement.DurationSeconds || s.BasedOnRequirementRevision != requirement.Revision ||
		s.BasedOnScriptRevision != script.Revision || len(s.BasedOnRequirementHash) != 64 || len(s.BasedOnScriptHash) != 64 ||
		strings.TrimSpace(s.ChannelProfileID) == "" || len(s.ChannelProfileHash) != 64 ||
		s.Generation.PromptVersion != aiNativeStoryboardPromptVersion || s.Generation.ProfileHash != s.ChannelProfileHash ||
		strings.TrimSpace(s.Generation.ModelAlias) == "" || strings.TrimSpace(s.Generation.ModelVersion) == "" || len(s.Assets) == 0 || len(s.Shots) == 0 {
		return fmt.Errorf("AI native storyboard is invalid")
	}
	assets := make(map[string]AINativeStoryboardAsset, len(s.Assets))
	roles := make(map[string]bool, 3)
	for index, asset := range s.Assets {
		if err := validateAINativeStoryboardAsset(asset); err != nil {
			return fmt.Errorf("AI native storyboard asset %d is invalid: %w", index, err)
		}
		if _, duplicate := assets[asset.ID]; duplicate {
			return fmt.Errorf("AI native storyboard asset %q is duplicated", asset.ID)
		}
		if asset.Role == AINativeStoryboardAssetRoleProductIdentity && asset.Source == AINativeStoryboardAssetSourceAIGenerated {
			return fmt.Errorf("AI-generated assets cannot represent product identity")
		}
		assets[asset.ID] = asset
		roles[asset.Role] = true
	}
	if !roles[AINativeStoryboardAssetRoleProductIdentity] || !roles[AINativeStoryboardAssetRolePersonIdentity] || !roles[AINativeStoryboardAssetRoleSceneReference] {
		return fmt.Errorf("AI native storyboard must include product, person and scene reference assets")
	}
	lastEnd := 0
	for index, shot := range s.Shots {
		if strings.TrimSpace(shot.ID) == "" || shot.StartMS != lastEnd || shot.EndMS <= shot.StartMS || shot.DurationMS != shot.EndMS-shot.StartMS ||
			strings.TrimSpace(shot.VisualContent) == "" || strings.TrimSpace(shot.SubjectsProductsActions) == "" ||
			strings.TrimSpace(shot.ShotSize) == "" || strings.TrimSpace(shot.CameraMovement) == "" ||
			strings.TrimSpace(shot.Voiceover) == "" || strings.TrimSpace(shot.Subtitle) == "" ||
			strings.TrimSpace(shot.SoundEffect) == "" || strings.TrimSpace(shot.BGMDirection) == "" || strings.TrimSpace(shot.Transition) == "" {
			return fmt.Errorf("AI native storyboard shot %d is incomplete or timeline is not contiguous", index)
		}
		hasProductIdentity := false
		for _, assetID := range shot.ReferenceAssetIDs {
			asset, ok := assets[assetID]
			if !ok {
				return fmt.Errorf("AI native storyboard shot %d references unknown asset %q", index, assetID)
			}
			if asset.Role == AINativeStoryboardAssetRoleProductIdentity && asset.Source != AINativeStoryboardAssetSourceAIGenerated && asset.AssetRef != nil {
				hasProductIdentity = true
			}
		}
		if shot.ProductIdentityRequired && !hasProductIdentity {
			return fmt.Errorf("AI native storyboard shot %d must reference a real product asset", index)
		}
		lastEnd = shot.EndMS
	}
	if lastEnd != requirement.DurationSeconds*1000 {
		return fmt.Errorf("AI native storyboard timeline must close at target duration")
	}
	return nil
}

func (s AINativeStoryboardRevision) ValidateReadyAgainst(requirement AINativeRequirementDraft, script AINativeScriptRevision) error {
	if err := s.ValidatePlanAgainst(requirement, script); err != nil {
		return err
	}
	for index, asset := range s.Assets {
		if asset.Status != AINativeStoryboardAssetReady || asset.AssetRef == nil || asset.AssetRef.Validate() != nil {
			return fmt.Errorf("AI native storyboard asset %d is not a stable ready AssetVersionRef", index)
		}
	}
	return nil
}

func validateAINativeStoryboardAsset(asset AINativeStoryboardAsset) error {
	if strings.TrimSpace(asset.ID) == "" || strings.TrimSpace(asset.Name) == "" || asset.GenerationAttempt < 0 {
		return fmt.Errorf("identity and name are required")
	}
	switch asset.Role {
	case AINativeStoryboardAssetRoleProductIdentity, AINativeStoryboardAssetRolePersonIdentity, AINativeStoryboardAssetRoleSceneReference,
		AINativeStoryboardAssetRoleCompositionReference, AINativeStoryboardAssetRoleAudioReference, AINativeStoryboardAssetRoleBrandElement:
	default:
		return fmt.Errorf("role is unsupported")
	}
	switch asset.Source {
	case AINativeStoryboardAssetSourceProductImport, AINativeStoryboardAssetSourceProjectAsset, AINativeStoryboardAssetSourceAIGenerated:
	default:
		return fmt.Errorf("source is unsupported")
	}
	switch asset.Status {
	case AINativeStoryboardAssetReady:
		if asset.AssetRef == nil || asset.AssetRef.Validate() != nil {
			return fmt.Errorf("ready asset requires AssetVersionRef")
		}
	case AINativeStoryboardAssetPlanned, AINativeStoryboardAssetGenerating:
		if asset.Source != AINativeStoryboardAssetSourceAIGenerated || strings.TrimSpace(asset.GenerationBrief) == "" || asset.AssetRef != nil {
			return fmt.Errorf("planned asset requires an AI generation brief without an AssetVersionRef")
		}
	case AINativeStoryboardAssetFailed:
		if asset.Source != AINativeStoryboardAssetSourceAIGenerated || strings.TrimSpace(asset.GenerationBrief) == "" || strings.TrimSpace(asset.ErrorMessage) == "" {
			return fmt.Errorf("failed asset requires its generation brief")
		}
	default:
		return fmt.Errorf("status is unsupported")
	}
	return nil
}
