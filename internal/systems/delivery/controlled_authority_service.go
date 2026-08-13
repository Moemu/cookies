package delivery

import (
	"context"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/delivery/platformskills"
)

type controlledAuthorityRepository interface {
	CreateControlledChangeSet(context.Context, ControlledChangeSet) (ControlledChangeSet, bool, error)
	GetControlledChangeSet(context.Context, contract.OrganizationID, contract.ProjectID, string) (ControlledChangeSet, error)
	ApproveControlledChangeSet(context.Context, ControlledChangeSet, RemoteWriteApproval) (ControlledChangeSet, RemoteWriteApproval, error)
	CreateControlledExecution(context.Context, ControlledExecution) (ControlledExecution, error)
	GetControlledExecution(context.Context, contract.OrganizationID, contract.ProjectID, string) (ControlledExecution, error)
	AttachComputerUseRun(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (ControlledExecution, error)
	CreatePlatformEntityMapping(context.Context, PlatformEntityMapping) (PlatformEntityMapping, error)
	GetPlatformEntityMapping(context.Context, contract.OrganizationID, contract.ProjectID, string) (PlatformEntityMapping, error)
	ConfirmPlatformEntityMapping(context.Context, PlatformEntityMapping, int64) (PlatformEntityMapping, error)
}

type MappingReadback struct {
	PlatformObjectID string `json:"platform_object_id"`
	PlatformStatus   string `json:"platform_status"`
	EvidenceID       string `json:"evidence_id"`
}

type ConfirmPlatformEntityMappingRequest struct {
	ExpectedVersion int64           `json:"expected_version"`
	Result          MappingReadback `json:"result_readback"`
	List            MappingReadback `json:"list_readback"`
}

func (s Service) AttachComputerUseRun(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, executionID string, expectedVersion int64, runID string) (ControlledExecution, error) {
	if err := s.ready(actor, projectID, ScopeExecute); err != nil {
		return ControlledExecution{}, err
	}
	if expectedVersion < 1 || strings.TrimSpace(runID) == "" {
		return ControlledExecution{}, ErrInvalidRequest
	}
	repo, ok := s.Repository.(controlledAuthorityRepository)
	if !ok {
		return ControlledExecution{}, ErrUnsupportedConfigurationWorkflow
	}
	return repo.AttachComputerUseRun(ctx, actor.OrganizationID, projectID, executionID, expectedVersion, runID, s.now())
}

func (s Service) CreatePendingPlatformEntityMapping(ctx context.Context, actor contract.ActorContext, value PlatformEntityMapping) (PlatformEntityMapping, error) {
	if err := s.ready(actor, value.ProjectID, ScopeExecute); err != nil {
		return PlatformEntityMapping{}, err
	}
	value.OrganizationID = actor.OrganizationID
	value.SchemaVersion = PlatformEntityMappingV1
	value.Status = PlatformEntityMappingPending
	value.PlatformObjectID = ""
	value.PlatformStatus = ""
	value.ResultEvidenceID = ""
	value.ListEvidenceID = ""
	value.Version = 1
	value.CreatedAt = s.now()
	if err := value.Validate(); err != nil {
		return PlatformEntityMapping{}, err
	}
	repo, ok := s.Repository.(controlledAuthorityRepository)
	if !ok {
		return PlatformEntityMapping{}, ErrUnsupportedConfigurationWorkflow
	}
	return repo.CreatePlatformEntityMapping(ctx, value)
}

func (s Service) ConfirmPlatformEntityMapping(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string, expectedVersion int64, result, list MappingReadback) (PlatformEntityMapping, error) {
	if err := s.ready(actor, projectID, ScopeExecute); err != nil {
		return PlatformEntityMapping{}, err
	}
	if expectedVersion < 1 || result.PlatformObjectID == "" || result.PlatformStatus == "" || result.EvidenceID == "" || list.EvidenceID == "" || result.EvidenceID == list.EvidenceID {
		return PlatformEntityMapping{}, ErrInvalidRequest
	}
	if result.PlatformObjectID != list.PlatformObjectID || result.PlatformStatus != list.PlatformStatus {
		return PlatformEntityMapping{}, ErrApprovalContentMismatch
	}
	repo, ok := s.Repository.(controlledAuthorityRepository)
	if !ok {
		return PlatformEntityMapping{}, ErrUnsupportedConfigurationWorkflow
	}
	value, err := repo.GetPlatformEntityMapping(ctx, actor.OrganizationID, projectID, id)
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	value.PlatformObjectID = result.PlatformObjectID
	value.PlatformStatus = result.PlatformStatus
	value.ResultEvidenceID = result.EvidenceID
	value.ListEvidenceID = list.EvidenceID
	value.Status = PlatformEntityMappingConfirmed
	if err := value.Validate(); err != nil {
		return PlatformEntityMapping{}, err
	}
	return repo.ConfirmPlatformEntityMapping(ctx, value, expectedVersion)
}

func (s Service) GetPlatformEntityMapping(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (PlatformEntityMapping, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return PlatformEntityMapping{}, err
	}
	repo, ok := s.Repository.(controlledAuthorityRepository)
	if !ok {
		return PlatformEntityMapping{}, ErrUnsupportedConfigurationWorkflow
	}
	return repo.GetPlatformEntityMapping(ctx, actor.OrganizationID, projectID, id)
}

func (s Service) GetControlledChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (ControlledChangeSet, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return ControlledChangeSet{}, err
	}
	repo, ok := s.Repository.(controlledAuthorityRepository)
	if !ok {
		return ControlledChangeSet{}, ErrUnsupportedConfigurationWorkflow
	}
	return repo.GetControlledChangeSet(ctx, actor.OrganizationID, projectID, id)
}
func (s Service) GetControlledExecution(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (ControlledExecution, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return ControlledExecution{}, err
	}
	repo, ok := s.Repository.(controlledAuthorityRepository)
	if !ok {
		return ControlledExecution{}, ErrUnsupportedConfigurationWorkflow
	}
	return repo.GetControlledExecution(ctx, actor.OrganizationID, projectID, id)
}

type CompileControlledChangeSetRequest struct {
	ObservatoryRunID string `json:"observatory_run_id"`
}

func (s Service) CompileControlledChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CompileControlledChangeSetRequest) (ControlledChangeSet, bool, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return ControlledChangeSet{}, false, err
	}
	if strings.TrimSpace(request.ObservatoryRunID) == "" {
		return ControlledChangeSet{}, false, ErrInvalidRequest
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return ControlledChangeSet{}, false, err
	}
	observatory, err := s.observatory()
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	decisions, err := s.decisionWorkflows()
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	repo, ok := s.Repository.(controlledAuthorityRepository)
	if !ok {
		return ControlledChangeSet{}, false, ErrUnsupportedConfigurationWorkflow
	}
	run, err := observatory.GetObservatoryRun(ctx, actor.OrganizationID, projectID, request.ObservatoryRunID)
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	feedbacks, err := observatory.ListObservatoryFeedback(ctx, actor.OrganizationID, projectID, run.ID, 1)
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	if len(feedbacks) != 1 || (feedbacks[0].Disposition != ObservatoryFeedbackAccepted && feedbacks[0].Disposition != ObservatoryFeedbackModified) {
		return ControlledChangeSet{}, false, ErrInvalidState
	}
	feedback := feedbacks[0]
	if feedback.RunCanonicalHash != run.CanonicalHash {
		return ControlledChangeSet{}, false, ErrApprovalContentMismatch
	}
	selection, err := decisions.GetDecisionSelection(ctx, actor.OrganizationID, projectID, run.Binding.SelectionID)
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	decision, err := decisions.GetDecision(ctx, actor.OrganizationID, projectID, selection.DecisionID)
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	configuration := selection.Configuration
	if feedback.FinalConfiguration != nil {
		configuration = *feedback.FinalConfiguration
	}
	if configuration.Platform != DeliveryPlatformOceanEngine || configuration.Payload.OceanEngine == nil || configuration.Payload.OceanEngine.Project == nil {
		return ControlledChangeSet{}, false, ErrInvalidState
	}
	accountID := selection.Workflow.AccountReference.ID
	if accountID == "" {
		return ControlledChangeSet{}, false, ErrInvalidState
	}
	fingerprint, err := contract.CanonicalJSONHash(struct {
		AccountID         string           `json:"account_id"`
		Action            ControlledAction `json:"action"`
		ConfigurationHash string           `json:"configuration_hash"`
	}{accountID, ControlledActionCreateProjectAndPromotions, configuration.CanonicalHash})
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	skill, err := platformskills.Get(platformskills.OceanEngineEcommerceManualID, platformskills.OceanEngineEcommerceManualVersion)
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	// The immutable approval binds the stage B calibration definition. Its
	// definition remains non-executable until gate one revalidates live DOM
	// locators and a real Browser Driver is delivered.
	binding := ControlledAuthorityBinding{SelectionID: selection.ID, ObservatoryRunID: run.ID, ObservatoryRunCanonicalHash: run.CanonicalHash, OperatorFeedbackID: feedback.ID, OperatorFeedbackCanonicalHash: feedback.CanonicalHash, OperatorFeedbackDisposition: feedback.Disposition, PlanID: decision.Inputs.PlanID, PlanVersion: decision.Inputs.PlanVersion, PlanCanonicalHash: decision.Inputs.PlanCanonicalHash, IntentID: decision.Inputs.IntentID, IntentVersion: decision.Inputs.IntentVersion, IntentCanonicalHash: decision.Inputs.IntentCanonicalHash, DecisionID: decision.ID, DecisionCanonicalHash: decision.CanonicalHash, ConfigurationID: configuration.ConfigurationID, ConfigurationVersion: configuration.VersionNumber, ConfigurationCanonicalHash: configuration.CanonicalHash, WorkflowID: selection.Workflow.ID, WorkflowCanonicalHash: selection.Workflow.CanonicalHash, AccountReferenceID: accountID, ObjectFingerprint: fingerprint, SkillID: skill.ID, SkillVersion: skill.Version}
	id, err := s.idGenerator()("controlledchangeset")
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	now := s.now()
	change := ControlledChangeSet{SchemaVersion: ControlledChangeSetSchemaV1, ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, Binding: binding, Action: ControlledActionCreateProjectAndPromotions, BudgetLimitMinor: configuration.Payload.OceanEngine.Project.BudgetAndBidding.DailyBudgetMinor, Currency: "CNY", Status: ControlledChangeSetReady, Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now}
	hash, err := change.ComputeCanonicalHash()
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	change.CanonicalHash = hash
	if err := change.Validate(); err != nil {
		return ControlledChangeSet{}, false, err
	}
	return repo.CreateControlledChangeSet(ctx, change)
}

type ApproveControlledChangeSetRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}

func (s Service) ApproveControlledChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string, request ApproveControlledChangeSetRequest) (ControlledChangeSet, RemoteWriteApproval, error) {
	if err := s.ready(actor, projectID, ScopeApprove); err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	if request.ExpectedVersion < 1 {
		return ControlledChangeSet{}, RemoteWriteApproval{}, ErrInvalidRequest
	}
	repo, ok := s.Repository.(controlledAuthorityRepository)
	if !ok {
		return ControlledChangeSet{}, RemoteWriteApproval{}, ErrUnsupportedConfigurationWorkflow
	}
	change, err := repo.GetControlledChangeSet(ctx, actor.OrganizationID, projectID, id)
	if err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	if change.Version != request.ExpectedVersion {
		return ControlledChangeSet{}, RemoteWriteApproval{}, ErrVersionConflict
	}
	if change.Status != ControlledChangeSetReady {
		return ControlledChangeSet{}, RemoteWriteApproval{}, ErrInvalidState
	}
	approvalID, err := s.idGenerator()("remotewriteapproval")
	if err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	now := s.now()
	approval := RemoteWriteApproval{SchemaVersion: RemoteWriteApprovalSchemaV1, ID: approvalID, OrganizationID: actor.OrganizationID, ProjectID: projectID, ControlledChangeSetID: change.ID, ControlledChangeSetHash: change.CanonicalHash, Binding: change.Binding, Action: change.Action, Scope: "controlled_remote_write", BudgetLimitMinor: change.BudgetLimitMinor, Currency: change.Currency, ApprovedBy: actor.Principal.ID, ApprovedAt: now, ExpiresAt: now.Add(RemoteWriteApprovalTTL)}
	hash, err := approval.ComputeActionHash()
	if err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	approval.ActionHash = hash
	if err := approval.Validate(now); err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	return repo.ApproveControlledChangeSet(ctx, change, approval)
}

func (s Service) CreateControlledExecution(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string) (ControlledExecution, error) {
	if err := s.ready(actor, projectID, ScopeExecute); err != nil {
		return ControlledExecution{}, err
	}
	repo, ok := s.Repository.(controlledAuthorityRepository)
	if !ok {
		return ControlledExecution{}, ErrUnsupportedConfigurationWorkflow
	}
	change, err := repo.GetControlledChangeSet(ctx, actor.OrganizationID, projectID, changeSetID)
	if err != nil {
		return ControlledExecution{}, err
	}
	if change.Status != ControlledChangeSetApproved {
		return ControlledExecution{}, ErrApprovalRequired
	}
	// The repository transaction verifies and links the immutable approval.
	approvalRepo, ok := s.Repository.(interface {
		GetRemoteWriteApproval(context.Context, contract.OrganizationID, contract.ProjectID, string) (RemoteWriteApproval, error)
	})
	if !ok {
		return ControlledExecution{}, ErrUnsupportedConfigurationWorkflow
	}
	approval, err := approvalRepo.GetRemoteWriteApproval(ctx, actor.OrganizationID, projectID, changeSetID)
	if err != nil {
		return ControlledExecution{}, err
	}
	if err := approval.Validate(s.now()); err != nil {
		return ControlledExecution{}, err
	}
	id, err := s.idGenerator()("controlledexecution")
	if err != nil {
		return ControlledExecution{}, err
	}
	now := s.now()
	value := ControlledExecution{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, ControlledChangeSetID: change.ID, RemoteWriteApprovalID: approval.ID, Status: "pending", Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now}
	return repo.CreateControlledExecution(ctx, value)
}
