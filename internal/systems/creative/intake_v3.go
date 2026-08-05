package creative

import (
	"encoding/json"
	"fmt"
)

type CreativeIntakeReadinessV3 struct {
	PlanningReady   bool `json:"planning_ready"`
	GenerationReady bool `json:"generation_ready"`
	ProductionReady bool `json:"production_ready"`
}

type CreativeIntakeV3 struct {
	ContractVersion    string                           `json:"contract_version"`
	ID                 string                           `json:"id"`
	OrganizationID     string                           `json:"organization_id"`
	ProjectID          string                           `json:"project_id"`
	Source             IntakeSource                     `json:"source"`
	Status             IntakeStatus                     `json:"status"`
	StrategyPackageRef StrategyPackageContractReference `json:"strategy_package_ref"`
	SelectedRouteID    string                           `json:"selected_route_id"`
	BaseHandoff        json.RawMessage                  `json:"base_handoff"`
	TaskOverlay        json.RawMessage                  `json:"task_overlay,omitempty"`
	InputIdentityHash  string                           `json:"input_identity_hash"`
	Readiness          CreativeIntakeReadinessV3        `json:"readiness"`
	Blockers           []json.RawMessage                `json:"blockers"`
	Warnings           []json.RawMessage                `json:"warnings"`
	Assumptions        []string                         `json:"assumptions"`
	ConfirmedBy        string                           `json:"confirmed_by,omitempty"`
	Version            int64                            `json:"version"`
	CreatedAt          string                           `json:"created_at"`
	UpdatedAt          string                           `json:"updated_at"`
}

func (value CreativeIntake) V3View() (CreativeIntakeV3, error) {
	if value.ContractVersion != CreativeIntakeV3ContractVersion ||
		value.Request.StrategyPackage == nil ||
		len(value.Request.StrategyHandoffInput) == 0 ||
		value.Request.SelectedRouteID == "" {
		return CreativeIntakeV3{}, fmt.Errorf("creative intake is not a complete v3 snapshot")
	}
	var handoff struct {
		Routes []struct {
			RouteID        string `json:"route_id"`
			RouteReadiness struct {
				Status string `json:"status"`
			} `json:"route_readiness"`
		} `json:"routes"`
		UpstreamReadiness struct {
			Blockers []json.RawMessage `json:"blockers"`
			Warnings []json.RawMessage `json:"warnings"`
		} `json:"upstream_readiness"`
	}
	if err := json.Unmarshal(value.Request.StrategyHandoffInput, &handoff); err != nil {
		return CreativeIntakeV3{}, fmt.Errorf("decode frozen Strategy handoff: %w", err)
	}
	generationReady := false
	for _, route := range handoff.Routes {
		if route.RouteID == value.Request.SelectedRouteID {
			generationReady = route.RouteReadiness.Status == "ready"
			break
		}
	}
	var taskOverlay json.RawMessage
	if value.Request.TaskOverlayInput != nil {
		taskOverlay = append(json.RawMessage(nil), value.Request.TaskOverlayInput.RawSnapshot...)
	}
	reference := value.Request.StrategyPackage
	return CreativeIntakeV3{
		ContractVersion: CreativeIntakeV3ContractVersion, ID: value.ID,
		OrganizationID: string(value.OrganizationID), ProjectID: string(value.ProjectID),
		Source: value.Source, Status: value.Status,
		StrategyPackageRef: StrategyPackageContractReference{
			PackageID: reference.PackageID, PackageVersion: reference.PackageVersion,
			PackageContentHash:     reference.ExpectedContentHash,
			HandoffContractVersion: reference.HandoffContractVersion,
			HandoffContentHash:     reference.ExpectedHandoffHash,
		},
		SelectedRouteID: value.Request.SelectedRouteID,
		BaseHandoff:     append(json.RawMessage(nil), value.Request.StrategyHandoffInput...),
		TaskOverlay:     taskOverlay, InputIdentityHash: value.InputIdentityHash,
		Readiness: CreativeIntakeReadinessV3{
			PlanningReady:   value.Status == IntakeReady,
			GenerationReady: value.Status == IntakeReady && generationReady,
			ProductionReady: false,
		},
		Blockers:    append([]json.RawMessage{}, handoff.UpstreamReadiness.Blockers...),
		Warnings:    append([]json.RawMessage{}, handoff.UpstreamReadiness.Warnings...),
		Assumptions: []string{}, ConfirmedBy: value.ConfirmedBy, Version: value.Version,
		CreatedAt: value.CreatedAt.Format(timeFormatRFC3339),
		UpdatedAt: value.UpdatedAt.Format(timeFormatRFC3339),
	}, nil
}

const timeFormatRFC3339 = "2006-01-02T15:04:05Z07:00"
