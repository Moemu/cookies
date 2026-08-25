package delivery

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/shikanon/cookies/internal/platform/browserautomation"
	"github.com/shikanon/cookies/internal/platform/contract"
)

const browserRpaRemoteWriteStepID = "submit-platform-configuration"

type browserRpaAuthorityRepository interface {
	GetControlledExecution(context.Context, contract.OrganizationID, contract.ProjectID, string) (ControlledExecution, error)
	GetControlledChangeSet(context.Context, contract.OrganizationID, contract.ProjectID, string) (ControlledChangeSet, error)
	GetRemoteWriteApproval(context.Context, contract.OrganizationID, contract.ProjectID, string) (RemoteWriteApproval, error)
	AttachBrowserRpaRun(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (ControlledExecution, error)
}

// BrowserRpaAuthorityProvider projects the immutable Delivery authority into
// the shared Computer Use control plane. The browser client supplies only the
// business execution ID; it cannot construct or widen the authority binding.
type BrowserRpaAuthorityProvider struct {
	Repository browserRpaAuthorityRepository
}

func (p BrowserRpaAuthorityProvider) ResolveAuthority(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, executionID string, now time.Time) (browserautomation.AuthorityResolution, error) {
	execution, change, approval, err := p.load(ctx, organizationID, projectID, executionID, now)
	if err != nil {
		return browserautomation.AuthorityResolution{}, err
	}
	if execution.Status != "pending" && execution.Status != "running" {
		return browserautomation.AuthorityResolution{}, browserautomation.ErrInvalidContract
	}
	authority, err := p.authorityFromLoaded(execution, change, approval)
	if err != nil {
		return browserautomation.AuthorityResolution{}, err
	}
	return browserautomation.AuthorityResolution{Binding: authority, BoundRunID: execution.BrowserRpaRunID}, nil
}

func (p BrowserRpaAuthorityProvider) BindRun(ctx context.Context, authority browserautomation.AuthorityBinding, runID string, now time.Time) error {
	execution, err := p.Repository.GetControlledExecution(ctx, authority.OrganizationID, authority.ProjectID, authority.BusinessExecutionID)
	if err != nil {
		return mapBrowserRpaAuthorityError(err)
	}
	if execution.BrowserRpaRunID == runID && execution.Status == "running" {
		return nil
	}
	if execution.BrowserRpaRunID != "" || execution.Status != "pending" {
		return browserautomation.ErrInvalidContract
	}
	_, err = p.Repository.AttachBrowserRpaRun(ctx, authority.OrganizationID, authority.ProjectID, execution.ID, execution.Version, runID, now)
	return mapBrowserRpaAuthorityError(err)
}

func (p BrowserRpaAuthorityProvider) VerifyAuthority(ctx context.Context, authority browserautomation.AuthorityBinding, runID string, now time.Time) error {
	execution, change, approval, err := p.load(ctx, authority.OrganizationID, authority.ProjectID, authority.BusinessExecutionID, now)
	if err != nil {
		return err
	}
	if execution.BrowserRpaRunID != runID || execution.Status != "running" {
		return browserautomation.ErrInvalidContract
	}
	expected, err := p.authorityFromLoaded(execution, change, approval)
	if err != nil || !reflect.DeepEqual(expected, authority) {
		return browserautomation.ErrInvalidContract
	}
	return nil
}

func (p BrowserRpaAuthorityProvider) load(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, executionID string, now time.Time) (ControlledExecution, ControlledChangeSet, RemoteWriteApproval, error) {
	if p.Repository == nil || organizationID == "" || projectID == "" || executionID == "" {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, browserautomation.ErrInvalidContract
	}
	execution, err := p.Repository.GetControlledExecution(ctx, organizationID, projectID, executionID)
	if err != nil {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, mapBrowserRpaAuthorityError(err)
	}
	change, err := p.Repository.GetControlledChangeSet(ctx, organizationID, projectID, execution.ControlledChangeSetID)
	if err != nil {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, mapBrowserRpaAuthorityError(err)
	}
	approval, err := p.Repository.GetRemoteWriteApproval(ctx, organizationID, projectID, change.ID)
	if err != nil {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, mapBrowserRpaAuthorityError(err)
	}
	if change.Status != ControlledChangeSetExecuting || execution.RemoteWriteApprovalID != approval.ID || approval.ControlledChangeSetID != change.ID || approval.ControlledChangeSetHash != change.CanonicalHash || !reflect.DeepEqual(approval.Binding, change.Binding) {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, browserautomation.ErrInvalidContract
	}
	if err := change.Validate(); err != nil {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, browserautomation.ErrInvalidContract
	}
	if err := approval.Validate(now); err != nil {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, browserautomation.ErrInvalidContract
	}
	return execution, change, approval, nil
}

func (p BrowserRpaAuthorityProvider) authorityFromLoaded(execution ControlledExecution, change ControlledChangeSet, approval RemoteWriteApproval) (browserautomation.AuthorityBinding, error) {
	binding := approval.Binding
	value := browserautomation.AuthorityBinding{SchemaVersion: browserautomation.AuthoritySchemaV1, OrganizationID: execution.OrganizationID, ProjectID: execution.ProjectID, BusinessExecutionID: execution.ID, ChangeSetID: change.ID, ApprovalID: approval.ID, ApprovalActionHash: approval.ActionHash, AccountReferenceID: binding.AccountReferenceID, ParentPlatformProjectID: binding.ParentPlatformProjectID, TargetMappingID: binding.TargetMappingID, TargetMappingVersion: binding.TargetMappingVersion, TargetPlatformObjectID: binding.TargetPlatformObjectID, TargetPlatformObjectKind: binding.TargetPlatformObjectKind, OperatorPrincipalID: binding.OperatorPrincipalID, SupersedesControlledChangeSetID: binding.SupersedesControlledChangeSetID, ObjectFingerprint: binding.ObjectFingerprint, Action: string(approval.Action), PlanID: binding.PlanID, PlanVersion: binding.PlanVersion, ProjectBudgetMode: binding.ProjectBudgetMode, ProjectBudgetLimitMinor: binding.ProjectBudgetLimitMinor, PromotionBudgetLimitMinor: binding.PromotionBudgetLimitMinor, BudgetLimitMinor: approval.BudgetLimitMinor, Currency: approval.Currency, PlanCanonicalHash: binding.PlanCanonicalHash, IntentCanonicalHash: binding.IntentCanonicalHash, FeedbackCanonicalHash: binding.OperatorFeedbackCanonicalHash, DecisionCanonicalHash: binding.DecisionCanonicalHash, ConfigurationCanonicalHash: binding.ConfigurationCanonicalHash, WorkflowID: binding.WorkflowID, WorkflowCanonicalHash: binding.WorkflowCanonicalHash, WorkflowStepID: browserRpaRemoteWriteStepID, SkillID: binding.SkillID, SkillVersion: binding.SkillVersion}
	if binding.PromotionMutation != nil {
		value.PromotionMutation = toBrowserRpaPromotionMutation(*binding.PromotionMutation)
	}
	if binding.PromotionControl != nil {
		value.PromotionControl = &browserautomation.PromotionControlBinding{CurrentDailyBudgetMinor: binding.PromotionControl.CurrentDailyBudgetMinor, CurrentPlatformStatus: binding.PromotionControl.CurrentPlatformStatus, TargetPlatformStatus: binding.PromotionControl.TargetPlatformStatus, CurrentStateHash: binding.PromotionControl.CurrentStateHash, TargetStateHash: binding.PromotionControl.TargetStateHash}
	}
	if binding.PromotionRestart != nil {
		value.PromotionRestart = toBrowserRpaPromotionRestart(*binding.PromotionRestart)
	}
	return value, value.Validate()
}

func toBrowserRpaPromotionRestart(value ControlledPromotionRestart) *browserautomation.PromotionRestartBinding {
	converted := &browserautomation.PromotionRestartBinding{
		CurrentDailyBudgetMinor:  value.CurrentDailyBudgetMinor,
		ApprovedDailyBudgetMinor: value.ApprovedDailyBudgetMinor,
		CurrentPlatformStatus:    value.CurrentPlatformStatus,
		TargetPlatformStatus:     value.TargetPlatformStatus,
		Schedule:                 browserautomation.PromotionScheduleWindow{StartAt: value.Schedule.StartAt, EndAt: value.Schedule.EndAt, Timezone: value.Schedule.Timezone},
		LandingPage:              browserautomation.PromotionLandingPageReference{ReferenceID: value.LandingPage.ReferenceID, AuthorizationEvidenceID: value.LandingPage.AuthorizationEvidenceID},
		CurrentStateHash:         value.CurrentStateHash,
		TargetStateHash:          value.TargetStateHash,
	}
	for _, reference := range value.Materials {
		converted.Materials = append(converted.Materials, browserautomation.PromotionMaterialReference{ReferenceID: reference.ReferenceID, AuthorizationEvidenceID: reference.AuthorizationEvidenceID})
	}
	return converted
}

func toBrowserRpaPromotionMutation(value ControlledPromotionMutation) *browserautomation.PromotionMutationBinding {
	converted := &browserautomation.PromotionMutationBinding{CurrentDailyBudgetMinor: value.CurrentDailyBudgetMinor, TargetDailyBudgetMinor: value.TargetDailyBudgetMinor, CurrentStateHash: value.CurrentStateHash, TargetStateHash: value.TargetStateHash}
	for _, reference := range value.CurrentMaterials {
		converted.CurrentMaterials = append(converted.CurrentMaterials, browserautomation.PromotionMaterialReference{ReferenceID: reference.ReferenceID, AuthorizationEvidenceID: reference.AuthorizationEvidenceID})
	}
	for _, reference := range value.TargetMaterials {
		converted.TargetMaterials = append(converted.TargetMaterials, browserautomation.PromotionMaterialReference{ReferenceID: reference.ReferenceID, AuthorizationEvidenceID: reference.AuthorizationEvidenceID})
	}
	return converted
}

func mapBrowserRpaAuthorityError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return browserautomation.ErrNotFound
	}
	if errors.Is(err, ErrVersionConflict) {
		return browserautomation.ErrVersionConflict
	}
	return err
}
