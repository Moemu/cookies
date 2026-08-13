package creative

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

func isSupportedAINativeProductSource(source string) bool {
	switch source {
	case "douyin_mall", "taobao", "tmall", "xiaohongshu", "1688":
		return true
	default:
		return false
	}
}

const (
	aiNativeRequirementContractV1 = "creative.ai-native.requirement/v1"
	aiNativeRequirementContractV2 = "creative.ai-native.requirement/v2"
	aiNativeRequirementContract   = aiNativeRequirementContractV2

	AINativeProductResolutionRecognized     = "recognized"
	AINativeProductResolutionPartial        = "partial"
	AINativeProductResolutionManualRequired = "manual_required"
	AINativeProductResourceProduct          = "product"
	AINativeProductResourceNote             = "note"

	AINativeOutputPresetDouyinFeed9x16V1 = "douyin_feed_9x16_v1"

	AINativeDeliveryPresetFullAd        = "full_ad"
	AINativeDeliveryPresetNoVoiceover   = "no_voiceover"
	AINativeDeliveryPresetCleanMaterial = "clean_material"
	AINativeDeliveryPresetCustom        = "custom"
	AINativeVoiceoverGenerated          = "generated"
	AINativeVoiceoverNone               = "none"
	AINativeCaptionFromVoiceover        = "from_voiceover"
	AINativeCaptionEditorial            = "editorial"
	AINativeCaptionNone                 = "none"
	AINativeSalesOverlayKeyPoints       = "key_points"
	AINativeSalesOverlayMinimal         = "minimal"
	AINativeSalesOverlayNone            = "none"
	AINativeMusicSFXAuto                = "auto"
	AINativeMusicSFXNone                = "none"
)

type AINativeProductResolution struct {
	Status        string   `json:"status"`
	Source        string   `json:"source"`
	ResourceType  string   `json:"resource_type"`
	ExternalID    string   `json:"external_id,omitempty"`
	SourceURL     string   `json:"source_url"`
	MissingFields []string `json:"missing_fields"`
}

func (r AINativeProductResolution) Validate() error {
	if r.Status != AINativeProductResolutionRecognized && r.Status != AINativeProductResolutionPartial && r.Status != AINativeProductResolutionManualRequired {
		return fmt.Errorf("AI native product resolution status is invalid")
	}
	if !isSupportedAINativeProductSource(r.Source) || (r.ResourceType != AINativeProductResourceProduct && r.ResourceType != AINativeProductResourceNote) {
		return fmt.Errorf("AI native product resolution identity is invalid")
	}
	parsed, err := url.Parse(strings.TrimSpace(r.SourceURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return fmt.Errorf("AI native product resolution URL is invalid")
	}
	seen := map[string]struct{}{}
	for _, field := range r.MissingFields {
		field = strings.TrimSpace(field)
		if field == "" {
			return fmt.Errorf("AI native product resolution missing field is invalid")
		}
		if _, exists := seen[field]; exists {
			return fmt.Errorf("AI native product resolution missing field is duplicated")
		}
		seen[field] = struct{}{}
	}
	return nil
}

type AINativeOutputSafeZone struct {
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
}

type AINativeOutputPresetSnapshot struct {
	ID             string                 `json:"id"`
	Label          string                 `json:"label"`
	Channel        string                 `json:"channel"`
	Placement      string                 `json:"placement"`
	AspectRatio    string                 `json:"aspect_ratio"`
	Width          int                    `json:"width"`
	Height         int                    `json:"height"`
	Resolution     string                 `json:"resolution"`
	ProfileID      string                 `json:"profile_id"`
	ProfileVersion string                 `json:"profile_version"`
	ProfileHash    string                 `json:"profile_hash"`
	SafeZone       AINativeOutputSafeZone `json:"safe_zone"`
}

func (p AINativeOutputPresetSnapshot) Validate() error {
	hash, hashErr := hex.DecodeString(p.ProfileHash)
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Label) == "" || strings.TrimSpace(p.Channel) == "" ||
		strings.TrimSpace(p.Placement) == "" || strings.TrimSpace(p.AspectRatio) == "" || p.Width < 1 || p.Height < 1 ||
		strings.TrimSpace(p.Resolution) == "" || strings.TrimSpace(p.ProfileID) == "" || strings.TrimSpace(p.ProfileVersion) == "" ||
		hashErr != nil || len(hash) != 32 || p.SafeZone.Top < 0 || p.SafeZone.Right < 0 || p.SafeZone.Bottom < 0 || p.SafeZone.Left < 0 {
		return fmt.Errorf("AI native output preset snapshot is invalid")
	}
	return nil
}

func DefaultAINativeOutputPreset() AINativeOutputPresetSnapshot {
	preset, _ := NewOutputPresetRegistry(NewChannelCreativeProfileRegistry()).Resolve(AINativeOutputPresetDouyinFeed9x16V1)
	return preset
}

type AINativeDeliveryTreatment struct {
	Preset           string `json:"preset"`
	VoiceoverMode    string `json:"voiceover_mode"`
	CaptionMode      string `json:"caption_mode"`
	SalesOverlayMode string `json:"sales_overlay_mode"`
	MusicSFXMode     string `json:"music_sfx_mode"`
}

func DefaultAINativeDeliveryTreatment() AINativeDeliveryTreatment {
	value, _ := AINativeDeliveryTreatmentForPreset(AINativeDeliveryPresetFullAd)
	return value
}

func AINativeDeliveryTreatmentForPreset(preset string) (AINativeDeliveryTreatment, error) {
	switch strings.TrimSpace(preset) {
	case AINativeDeliveryPresetFullAd:
		return AINativeDeliveryTreatment{Preset: AINativeDeliveryPresetFullAd, VoiceoverMode: AINativeVoiceoverGenerated, CaptionMode: AINativeCaptionFromVoiceover, SalesOverlayMode: AINativeSalesOverlayKeyPoints, MusicSFXMode: AINativeMusicSFXAuto}, nil
	case AINativeDeliveryPresetNoVoiceover:
		return AINativeDeliveryTreatment{Preset: AINativeDeliveryPresetNoVoiceover, VoiceoverMode: AINativeVoiceoverNone, CaptionMode: AINativeCaptionEditorial, SalesOverlayMode: AINativeSalesOverlayKeyPoints, MusicSFXMode: AINativeMusicSFXAuto}, nil
	case AINativeDeliveryPresetCleanMaterial:
		return AINativeDeliveryTreatment{Preset: AINativeDeliveryPresetCleanMaterial, VoiceoverMode: AINativeVoiceoverNone, CaptionMode: AINativeCaptionNone, SalesOverlayMode: AINativeSalesOverlayNone, MusicSFXMode: AINativeMusicSFXNone}, nil
	default:
		return AINativeDeliveryTreatment{}, fmt.Errorf("AI native delivery preset is invalid")
	}
}

func effectiveAINativeDeliveryTreatment(requirement AINativeRequirementDraft) AINativeDeliveryTreatment {
	if requirement.DeliveryTreatment.Validate() == nil {
		return requirement.DeliveryTreatment
	}
	return DefaultAINativeDeliveryTreatment()
}

func (t AINativeDeliveryTreatment) Validate() error {
	if t.Preset != AINativeDeliveryPresetFullAd && t.Preset != AINativeDeliveryPresetNoVoiceover && t.Preset != AINativeDeliveryPresetCleanMaterial && t.Preset != AINativeDeliveryPresetCustom {
		return fmt.Errorf("AI native delivery preset is invalid")
	}
	if t.VoiceoverMode != AINativeVoiceoverGenerated && t.VoiceoverMode != AINativeVoiceoverNone {
		return fmt.Errorf("AI native voiceover mode is invalid")
	}
	if t.CaptionMode != AINativeCaptionFromVoiceover && t.CaptionMode != AINativeCaptionEditorial && t.CaptionMode != AINativeCaptionNone {
		return fmt.Errorf("AI native caption mode is invalid")
	}
	if t.SalesOverlayMode != AINativeSalesOverlayKeyPoints && t.SalesOverlayMode != AINativeSalesOverlayMinimal && t.SalesOverlayMode != AINativeSalesOverlayNone {
		return fmt.Errorf("AI native sales overlay mode is invalid")
	}
	if t.MusicSFXMode != AINativeMusicSFXAuto && t.MusicSFXMode != AINativeMusicSFXNone {
		return fmt.Errorf("AI native music and sound effect mode is invalid")
	}
	if t.CaptionMode == AINativeCaptionFromVoiceover && t.VoiceoverMode != AINativeVoiceoverGenerated {
		return fmt.Errorf("AI native voiceover captions require generated voiceover")
	}
	expected, fixedErr := AINativeDeliveryTreatmentForPreset(t.Preset)
	if fixedErr == nil && t != expected {
		return fmt.Errorf("AI native delivery treatment does not match preset %s", t.Preset)
	}
	return nil
}

type AINativeRequirementFieldIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AINativeRequirementConfirmationError struct {
	Issues []AINativeRequirementFieldIssue
}

func (e AINativeRequirementConfirmationError) Error() string {
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, issue.Field+": "+issue.Message)
	}
	return "AI native requirement is incomplete: " + strings.Join(parts, "; ")
}

func (AINativeRequirementConfirmationError) Unwrap() error { return ErrInvalidAINativeRequirement }

func upgradeAINativeRequirementV1(d AINativeRequirementDraft) AINativeRequirementDraft {
	if d.ContractVersion != aiNativeRequirementContractV1 {
		return d
	}
	status := AINativeProductResolutionRecognized
	missing := []string{}
	if strings.TrimSpace(d.ProductName) == "" {
		missing = append(missing, "product_name")
	}
	if len(d.Media) == 0 {
		missing = append(missing, "images")
	}
	if len(missing) > 0 {
		status = AINativeProductResolutionManualRequired
	}
	d.ContractVersion = aiNativeRequirementContractV2
	d.ProductResolution = AINativeProductResolution{
		Status: status, Source: d.Product.Source, ResourceType: AINativeProductResourceProduct,
		ExternalID: d.Product.ProductID, SourceURL: d.Product.SourceURL, MissingFields: missing,
	}
	d.OutputPreset = DefaultAINativeOutputPreset()
	d.DeliveryTreatment = DefaultAINativeDeliveryTreatment()
	return d
}
