package delivery

import (
	"context"
	"errors"
	"time"

	"github.com/shikanon/cookies/internal/platform/computeruse"
	"github.com/shikanon/cookies/internal/platform/contract"
)

const computerUseRemoteWriteStepID = "submit-platform-configuration"

type computerUseAuthorityRepository interface {
	GetControlledExecution(context.Context, contract.OrganizationID, contract.ProjectID, string) (ControlledExecution, error)
	GetControlledChangeSet(context.Context, contract.OrganizationID, contract.ProjectID, string) (ControlledChangeSet, error)
	GetRemoteWriteApproval(context.Context, contract.OrganizationID, contract.ProjectID, string) (RemoteWriteApproval, error)
	AttachComputerUseRun(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (ControlledExecution, error)
}

// ComputerUseAuthorityProvider projects the immutable Delivery authority into
// the shared Computer Use control plane. The browser client supplies only the
// business execution ID; it cannot construct or widen the authority binding.
type ComputerUseAuthorityProvider struct {
	Repository computerUseAuthorityRepository
}

func (p ComputerUseAuthorityProvider) ResolveAuthority(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, executionID string, now time.Time) (computeruse.AuthorityResolution, error) {
	execution, change, approval, err := p.load(ctx, organizationID, projectID, executionID, now)
	if err != nil {
		return computeruse.AuthorityResolution{}, err
	}
	if execution.Status != "pending" && execution.Status != "running" {
		return computeruse.AuthorityResolution{}, computeruse.ErrInvalidContract
	}
	binding := approval.Binding
	return computeruse.AuthorityResolution{Binding: computeruse.AuthorityBinding{
		SchemaVersion:              computeruse.AuthoritySchemaV1,
		OrganizationID:             organizationID,
		ProjectID:                  projectID,
		BusinessExecutionID:        execution.ID,
		ChangeSetID:                change.ID,
		ApprovalID:                 approval.ID,
		ApprovalActionHash:         approval.ActionHash,
		AccountReferenceID:         binding.AccountReferenceID,
		ParentPlatformProjectID:    binding.ParentPlatformProjectID,
		ObjectFingerprint:          binding.ObjectFingerprint,
		Action:                     string(approval.Action),
		ProjectBudgetMode:          binding.ProjectBudgetMode,
		ProjectBudgetLimitMinor:    binding.ProjectBudgetLimitMinor,
		PromotionBudgetLimitMinor:  binding.PromotionBudgetLimitMinor,
		BudgetLimitMinor:           approval.BudgetLimitMinor,
		Currency:                   approval.Currency,
		PlanCanonicalHash:          binding.PlanCanonicalHash,
		IntentCanonicalHash:        binding.IntentCanonicalHash,
		FeedbackCanonicalHash:      binding.OperatorFeedbackCanonicalHash,
		DecisionCanonicalHash:      binding.DecisionCanonicalHash,
		ConfigurationCanonicalHash: binding.ConfigurationCanonicalHash,
		WorkflowID:                 binding.WorkflowID,
		WorkflowCanonicalHash:      binding.WorkflowCanonicalHash,
		WorkflowStepID:             computerUseRemoteWriteStepID,
		SkillID:                    binding.SkillID,
		SkillVersion:               binding.SkillVersion,
	}, BoundRunID: execution.ComputerUseRunID}, nil
}

func (p ComputerUseAuthorityProvider) BindRun(ctx context.Context, authority computeruse.AuthorityBinding, runID string, now time.Time) error {
	execution, err := p.Repository.GetControlledExecution(ctx, authority.OrganizationID, authority.ProjectID, authority.BusinessExecutionID)
	if err != nil {
		return mapComputerUseAuthorityError(err)
	}
	if execution.ComputerUseRunID == runID && execution.Status == "running" {
		return nil
	}
	if execution.ComputerUseRunID != "" || execution.Status != "pending" {
		return computeruse.ErrInvalidContract
	}
	_, err = p.Repository.AttachComputerUseRun(ctx, authority.OrganizationID, authority.ProjectID, execution.ID, execution.Version, runID, now)
	return mapComputerUseAuthorityError(err)
}

func (p ComputerUseAuthorityProvider) VerifyAuthority(ctx context.Context, authority computeruse.AuthorityBinding, runID string, now time.Time) error {
	execution, change, approval, err := p.load(ctx, authority.OrganizationID, authority.ProjectID, authority.BusinessExecutionID, now)
	if err != nil {
		return err
	}
	if execution.ComputerUseRunID != runID || execution.Status != "running" {
		return computeruse.ErrInvalidContract
	}
	expected, err := p.authorityFromLoaded(execution, change, approval)
	if err != nil || expected != authority {
		return computeruse.ErrInvalidContract
	}
	return nil
}

func (p ComputerUseAuthorityProvider) load(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, executionID string, now time.Time) (ControlledExecution, ControlledChangeSet, RemoteWriteApproval, error) {
	if p.Repository == nil || organizationID == "" || projectID == "" || executionID == "" {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, computeruse.ErrInvalidContract
	}
	execution, err := p.Repository.GetControlledExecution(ctx, organizationID, projectID, executionID)
	if err != nil {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, mapComputerUseAuthorityError(err)
	}
	change, err := p.Repository.GetControlledChangeSet(ctx, organizationID, projectID, execution.ControlledChangeSetID)
	if err != nil {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, mapComputerUseAuthorityError(err)
	}
	approval, err := p.Repository.GetRemoteWriteApproval(ctx, organizationID, projectID, change.ID)
	if err != nil {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, mapComputerUseAuthorityError(err)
	}
	if change.Status != ControlledChangeSetExecuting || execution.RemoteWriteApprovalID != approval.ID || approval.ControlledChangeSetID != change.ID || approval.ControlledChangeSetHash != change.CanonicalHash || approval.Binding != change.Binding {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, computeruse.ErrInvalidContract
	}
	if err := change.Validate(); err != nil {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, computeruse.ErrInvalidContract
	}
	if err := approval.Validate(now); err != nil {
		return ControlledExecution{}, ControlledChangeSet{}, RemoteWriteApproval{}, computeruse.ErrInvalidContract
	}
	return execution, change, approval, nil
}

func (p ComputerUseAuthorityProvider) authorityFromLoaded(execution ControlledExecution, change ControlledChangeSet, approval RemoteWriteApproval) (computeruse.AuthorityBinding, error) {
	binding := approval.Binding
	value := computeruse.AuthorityBinding{SchemaVersion: computeruse.AuthoritySchemaV1, OrganizationID: execution.OrganizationID, ProjectID: execution.ProjectID, BusinessExecutionID: execution.ID, ChangeSetID: change.ID, ApprovalID: approval.ID, ApprovalActionHash: approval.ActionHash, AccountReferenceID: binding.AccountReferenceID, ParentPlatformProjectID: binding.ParentPlatformProjectID, ObjectFingerprint: binding.ObjectFingerprint, Action: string(approval.Action), ProjectBudgetMode: binding.ProjectBudgetMode, ProjectBudgetLimitMinor: binding.ProjectBudgetLimitMinor, PromotionBudgetLimitMinor: binding.PromotionBudgetLimitMinor, BudgetLimitMinor: approval.BudgetLimitMinor, Currency: approval.Currency, PlanCanonicalHash: binding.PlanCanonicalHash, IntentCanonicalHash: binding.IntentCanonicalHash, FeedbackCanonicalHash: binding.OperatorFeedbackCanonicalHash, DecisionCanonicalHash: binding.DecisionCanonicalHash, ConfigurationCanonicalHash: binding.ConfigurationCanonicalHash, WorkflowID: binding.WorkflowID, WorkflowCanonicalHash: binding.WorkflowCanonicalHash, WorkflowStepID: computerUseRemoteWriteStepID, SkillID: binding.SkillID, SkillVersion: binding.SkillVersion}
	return value, value.Validate()
}

func mapComputerUseAuthorityError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return computeruse.ErrNotFound
	}
	if errors.Is(err, ErrVersionConflict) {
		return computeruse.ErrVersionConflict
	}
	return err
}
