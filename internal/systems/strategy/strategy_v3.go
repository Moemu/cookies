package strategy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const StrategyDraftContractV3 = "strategy-draft/v3"

// DecodeStrategyDocumentReadOnly is the only compatibility decoder for legacy
// Strategy documents. Callers may render or compare v1/v2, but write commands
// must require v3 and must never marshal a legacy document as a new revision.
func DecodeStrategyDocumentReadOnly(raw json.RawMessage) (StrategyDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document StrategyDocument
	if err := decoder.Decode(&document); err != nil {
		return StrategyDocument{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return StrategyDocument{}, fmt.Errorf("strategy document contains trailing JSON")
	}
	switch document.ContractVersion {
	case "strategy-draft/v1", "strategy-draft/v2", StrategyDraftContractV3:
		return document, nil
	default:
		return StrategyDocument{}, fmt.Errorf("unsupported strategy document contract %q", document.ContractVersion)
	}
}

func decodePackageSnapshotReadOnly(raw json.RawMessage) (PackageSnapshot, error) {
	var snapshot PackageSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return PackageSnapshot{}, err
	}
	expectedStrategyContract := map[string]string{
		"strategy-package/v1": "strategy-draft/v1",
		"strategy-package/v2": "strategy-draft/v2",
		"strategy-package/v3": StrategyDraftContractV3,
	}[snapshot.ContractVersion]
	if expectedStrategyContract == "" || snapshot.Strategy.ContractVersion != expectedStrategyContract {
		return PackageSnapshot{}, fmt.Errorf("strategy package contract does not match its strategy snapshot")
	}
	return snapshot, nil
}

// UpgradeStrategyDocumentToV3 creates a deterministic successor snapshot. It
// preserves the legacy decision fields and lineage, replaces only the removed
// creative_recommendations representation, and recomputes compliance against
// the exact Brief version that originally produced the Strategy.
func UpgradeStrategyDocumentToV3(legacy StrategyDocument, brief BriefVersion, checkedAt time.Time) (StrategyDocument, error) {
	if legacy.ContractVersion == StrategyDraftContractV3 {
		if err := legacy.Validate(); err != nil {
			return StrategyDocument{}, err
		}
		return legacy, nil
	}
	if legacy.ContractVersion != "strategy-draft/v1" && legacy.ContractVersion != "strategy-draft/v2" {
		return StrategyDocument{}, ErrStrategyUpgradeRequired
	}
	if legacy.Lineage.BriefID != brief.BriefID || legacy.Lineage.BriefVersion != brief.Version {
		return StrategyDocument{}, fmt.Errorf("%w: strategy and Brief lineage do not match", ErrInvalidState)
	}
	fallback := deterministicStrategy(brief, Draft{
		ProjectContextVersion: legacy.Lineage.ProjectContextVersion,
		SkillVersions:         legacy.Lineage.SkillVersions,
	})
	directions := append([]string(nil), legacy.CreativeRecommendations...)
	if len(directions) == 0 {
		directions = []string{strings.TrimSpace(legacy.Proposition)}
	}
	legacy.ContractVersion = StrategyDraftContractV3
	legacy.CreativeStrategy = deterministicCreativeStrategy(brief, directions, normalizePlatformPlans(
		legacy.PlatformPlans, fallback.PlatformPlans, brief.Snapshot.Channels,
	))
	legacy.CreativeRecommendations = nil
	if strings.TrimSpace(legacy.ExecutiveSummary) == "" {
		legacy.ExecutiveSummary = fallback.ExecutiveSummary
	}
	if strings.TrimSpace(legacy.CrossPlatformRole) == "" {
		legacy.CrossPlatformRole = fallback.CrossPlatformRole
	}
	legacy.PlatformPlans = normalizePlatformPlans(legacy.PlatformPlans, fallback.PlatformPlans, brief.Snapshot.Channels)
	compliance := evaluateCompliance(legacy, brief, checkedAt)
	legacy.Compliance = &compliance
	if err := legacy.Validate(); err != nil {
		return StrategyDocument{}, err
	}
	return legacy, nil
}
