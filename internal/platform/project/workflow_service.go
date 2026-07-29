package project

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/ids"
)

var ErrVersionConflict = errors.New("project workflow version conflict")
var ErrInvalidState = errors.New("project workflow invalid state")

func (s Service) GetDetail(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (ProjectDetail, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return ProjectDetail{}, err
	}
	projectValue, err := s.Store.GetProject(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return ProjectDetail{}, err
	}
	tasks, err := s.Store.ListBusinessTasks(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return ProjectDetail{}, err
	}
	operations, err := s.Store.ListOperationalRecords(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return ProjectDetail{}, err
	}
	changeSets, err := s.Store.ListChangeSets(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return ProjectDetail{}, err
	}
	return ProjectDetail{
		Project:    projectValue,
		Runtime:    runtimeForProject(projectValue, actor),
		Artifacts:  []ProjectArtifactSummary{},
		Tasks:      tasks,
		Operations: operations,
		ChangeSets: changeSets,
	}, nil
}

func (s Service) CreateBusinessTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateBusinessTaskRequest) (BusinessTask, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return BusinessTask{}, err
	}
	if err := validateCreateBusinessTask(request); err != nil {
		return BusinessTask{}, err
	}
	id, err := s.newID("task")
	if err != nil {
		return BusinessTask{}, err
	}
	now := time.Now().UTC()
	task := BusinessTask{
		ID:                id,
		OrganizationID:    actor.OrganizationID,
		ProjectID:         projectID,
		Type:              request.Type,
		Name:              strings.TrimSpace(request.Name),
		Objective:         strings.TrimSpace(request.Objective),
		Status:            BusinessTaskDraft,
		SourceTaskIDs:     compactStrings(request.SourceTaskIDs),
		SourceArtifactIDs: compactStrings(request.SourceArtifactIDs),
		OutputArtifactIDs: []string{},
		Version:           1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.Store.CreateBusinessTask(ctx, task); err != nil {
		return BusinessTask{}, err
	}
	return s.Store.GetBusinessTask(ctx, actor.OrganizationID, projectID, task.ID)
}

func (s Service) ListBusinessTasks(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]BusinessTask, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Store.ListBusinessTasks(ctx, actor.OrganizationID, projectID)
}

func (s Service) GetBusinessTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string) (BusinessTask, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return BusinessTask{}, err
	}
	return s.Store.GetBusinessTask(ctx, actor.OrganizationID, projectID, strings.TrimSpace(taskID))
}

func (s Service) UpdateBusinessTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request UpdateBusinessTaskRequest) (BusinessTask, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return BusinessTask{}, err
	}
	task, err := s.Store.GetBusinessTask(ctx, actor.OrganizationID, projectID, strings.TrimSpace(taskID))
	if err != nil {
		return BusinessTask{}, err
	}
	if request.ExpectedVersion != nil && *request.ExpectedVersion != task.Version {
		return BusinessTask{}, ErrVersionConflict
	}
	if request.Name != nil {
		task.Name = strings.TrimSpace(*request.Name)
	}
	if request.Objective != nil {
		task.Objective = strings.TrimSpace(*request.Objective)
	}
	if request.Status != nil {
		task.Status = *request.Status
	}
	if request.SourceTaskIDs != nil {
		task.SourceTaskIDs = compactStrings(request.SourceTaskIDs)
	}
	if request.SourceArtifactIDs != nil {
		task.SourceArtifactIDs = compactStrings(request.SourceArtifactIDs)
	}
	if request.OutputArtifactIDs != nil {
		task.OutputArtifactIDs = compactStrings(request.OutputArtifactIDs)
	}
	if err := validateBusinessTask(task); err != nil {
		return BusinessTask{}, err
	}
	task.UpdatedAt = time.Now().UTC()
	if err := s.Store.UpdateBusinessTask(ctx, task); err != nil {
		return BusinessTask{}, err
	}
	return s.Store.GetBusinessTask(ctx, actor.OrganizationID, projectID, task.ID)
}

func (s Service) CreateOperationalRecord(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request UpsertOperationalRecordRequest) (OperationalRecord, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return OperationalRecord{}, err
	}
	if err := validateOperationalRecordRequest(request); err != nil {
		return OperationalRecord{}, err
	}
	id, err := s.newID("operation")
	if err != nil {
		return OperationalRecord{}, err
	}
	record := operationalRecordFromRequest(actor.OrganizationID, projectID, id, request)
	if err := s.Store.CreateOperationalRecord(ctx, record); err != nil {
		return OperationalRecord{}, err
	}
	return s.Store.GetOperationalRecord(ctx, actor.OrganizationID, projectID, record.ID)
}

func (s Service) ListOperationalRecords(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]OperationalRecord, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Store.ListOperationalRecords(ctx, actor.OrganizationID, projectID)
}

func (s Service) GetOperationalRecord(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, recordID string) (OperationalRecord, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return OperationalRecord{}, err
	}
	return s.Store.GetOperationalRecord(ctx, actor.OrganizationID, projectID, strings.TrimSpace(recordID))
}

func (s Service) UpsertOperationalRecord(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, recordID string, request UpsertOperationalRecordRequest) (OperationalRecord, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return OperationalRecord{}, err
	}
	if strings.TrimSpace(recordID) == "" {
		return OperationalRecord{}, fmt.Errorf("operation id is required")
	}
	if err := validateOperationalRecordRequest(request); err != nil {
		return OperationalRecord{}, err
	}
	record := operationalRecordFromRequest(actor.OrganizationID, projectID, strings.TrimSpace(recordID), request)
	if _, err := s.Store.GetOperationalRecord(ctx, actor.OrganizationID, projectID, record.ID); errors.Is(err, ErrNotFound) {
		if err := s.Store.CreateOperationalRecord(ctx, record); err != nil {
			return OperationalRecord{}, err
		}
	} else if err != nil {
		return OperationalRecord{}, err
	} else if err := s.Store.UpdateOperationalRecord(ctx, record); err != nil {
		return OperationalRecord{}, err
	}
	return s.Store.GetOperationalRecord(ctx, actor.OrganizationID, projectID, record.ID)
}

func (s Service) CreateChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateChangeSetRequest) (ChangeSet, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return ChangeSet{}, err
	}
	artifactRefs, err := normalizeChangeSetArtifactRefs(projectID, request.ArtifactRefs)
	if err != nil {
		return ChangeSet{}, err
	}
	if err := validateCreateChangeSet(request); err != nil {
		return ChangeSet{}, err
	}
	id, err := s.newID("changeset")
	if err != nil {
		return ChangeSet{}, err
	}
	now := time.Now().UTC()
	changeSet := ChangeSet{
		ID:             id,
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		Name:           strings.TrimSpace(request.Name),
		Status:         ChangeSetDraft,
		ArtifactRefs:   artifactRefs,
		BudgetLimit:    request.BudgetLimit,
		AuditEvents:    []AuditEvent{},
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.Store.CreateChangeSet(ctx, changeSet); err != nil {
		return ChangeSet{}, err
	}
	if err := s.appendChangeSetAudit(ctx, actor, changeSet, "change_set.created", map[string]any{"name": changeSet.Name}); err != nil {
		return ChangeSet{}, err
	}
	return s.Store.GetChangeSet(ctx, actor.OrganizationID, projectID, changeSet.ID)
}

func (s Service) ListChangeSets(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]ChangeSet, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Store.ListChangeSets(ctx, actor.OrganizationID, projectID)
}

func (s Service) GetChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string) (ChangeSet, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return ChangeSet{}, err
	}
	return s.Store.GetChangeSet(ctx, actor.OrganizationID, projectID, strings.TrimSpace(changeSetID))
}

func (s Service) PreflightChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string) (ChangeSet, error) {
	changeSet, err := s.workflowChangeSet(ctx, actor, projectID, changeSetID)
	if err != nil {
		return ChangeSet{}, err
	}
	if changeSet.Status != ChangeSetDraft && changeSet.Status != ChangeSetPreflightFailed {
		return ChangeSet{}, ErrInvalidState
	}
	now := time.Now().UTC()
	checks := []PreflightCheck{
		{Code: "confirmed_brief", Passed: true, Message: "Project brief is available for simulation.", Repair: ""},
		{Code: "ready_creative", Passed: len(changeSet.ArtifactRefs) > 0, Message: "At least one ready asset reference is attached.", Repair: "Attach one project asset reference before preflight."},
		{Code: "budget_boundary", Passed: changeSet.BudgetLimit == nil || (*changeSet.BudgetLimit >= 0 && *changeSet.BudgetLimit <= 1000000), Message: "Budget limit is inside the configured simulation boundary.", Repair: "Keep budget_limit between 0 and 1000000."},
	}
	passed := true
	for _, check := range checks {
		if !check.Passed {
			passed = false
			break
		}
	}
	changeSet.Preflight = &ChangeSetPreflight{Passed: passed, Checks: checks, CheckedAt: now}
	changeSet.Status = ChangeSetPreflightPassed
	if !passed {
		changeSet.Status = ChangeSetPreflightFailed
	}
	changeSet.UpdatedAt = now
	if err := s.Store.UpdateChangeSet(ctx, changeSet); err != nil {
		return ChangeSet{}, err
	}
	if err := s.appendChangeSetEvent(ctx, actor, changeSet, "preflight", map[string]any{"passed": passed}); err != nil {
		return ChangeSet{}, err
	}
	if err := s.appendChangeSetAudit(ctx, actor, changeSet, "change_set.preflight", map[string]any{"passed": passed}); err != nil {
		return ChangeSet{}, err
	}
	return s.Store.GetChangeSet(ctx, actor.OrganizationID, projectID, changeSet.ID)
}

func (s Service) ApproveChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string, request ChangeSetApprovalRequest) (ChangeSet, error) {
	changeSet, err := s.workflowChangeSet(ctx, actor, projectID, changeSetID)
	if err != nil {
		return ChangeSet{}, err
	}
	request.Actor = strings.TrimSpace(request.Actor)
	request.Role = strings.TrimSpace(request.Role)
	if request.Actor == "" || request.Role == "" {
		return ChangeSet{}, fmt.Errorf("approval actor and role are required")
	}
	if changeSet.Status != ChangeSetPreflightPassed {
		return ChangeSet{}, ErrInvalidState
	}
	changeSet.Status = ChangeSetApproved
	changeSet.UpdatedAt = time.Now().UTC()
	if err := s.Store.UpdateChangeSet(ctx, changeSet); err != nil {
		return ChangeSet{}, err
	}
	metadata := map[string]any{"actor": request.Actor, "role": request.Role, "note": strings.TrimSpace(request.Note)}
	if err := s.appendChangeSetEvent(ctx, actor, changeSet, "approve", metadata); err != nil {
		return ChangeSet{}, err
	}
	if err := s.appendChangeSetAudit(ctx, actor, changeSet, "change_set.approved", metadata); err != nil {
		return ChangeSet{}, err
	}
	return s.Store.GetChangeSet(ctx, actor.OrganizationID, projectID, changeSet.ID)
}

func (s Service) ExecuteChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string) (ChangeSet, error) {
	changeSet, err := s.workflowChangeSet(ctx, actor, projectID, changeSetID)
	if err != nil {
		return ChangeSet{}, err
	}
	if changeSet.Status != ChangeSetApproved {
		return ChangeSet{}, ErrInvalidState
	}
	now := time.Now().UTC()
	changeSet.Status = ChangeSetExecuted
	changeSet.Execution = &ChangeSetExecution{Simulated: true, Evidence: []ChangeSetEvidence{{
		Step:       "delivery_simulation",
		Status:     "succeeded",
		Message:    "ChangeSet execution was simulated without mutating delivery systems.",
		RecordedAt: now,
	}}, ExecutedAt: now}
	changeSet.UpdatedAt = now
	if err := s.Store.UpdateChangeSet(ctx, changeSet); err != nil {
		return ChangeSet{}, err
	}
	if err := s.appendChangeSetEvent(ctx, actor, changeSet, "execute", map[string]any{"simulated": true}); err != nil {
		return ChangeSet{}, err
	}
	if err := s.appendChangeSetAudit(ctx, actor, changeSet, "change_set.executed", map[string]any{"simulated": true}); err != nil {
		return ChangeSet{}, err
	}
	return s.Store.GetChangeSet(ctx, actor.OrganizationID, projectID, changeSet.ID)
}

func (s Service) RollbackChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string, request RollbackChangeSetRequest) (ChangeSet, error) {
	changeSet, err := s.workflowChangeSet(ctx, actor, projectID, changeSetID)
	if err != nil {
		return ChangeSet{}, err
	}
	request.Actor = strings.TrimSpace(request.Actor)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Actor == "" || request.Reason == "" {
		return ChangeSet{}, fmt.Errorf("rollback actor and reason are required")
	}
	if changeSet.Status != ChangeSetExecuted {
		return ChangeSet{}, ErrInvalidState
	}
	now := time.Now().UTC()
	changeSet.Status = ChangeSetRolledBack
	changeSet.Rollback = &ChangeSetRollback{Simulated: true, Reason: request.Reason, RolledBackAt: now}
	changeSet.UpdatedAt = now
	if err := s.Store.UpdateChangeSet(ctx, changeSet); err != nil {
		return ChangeSet{}, err
	}
	metadata := map[string]any{"actor": request.Actor, "reason": request.Reason}
	if err := s.appendChangeSetEvent(ctx, actor, changeSet, "rollback", metadata); err != nil {
		return ChangeSet{}, err
	}
	if err := s.appendChangeSetAudit(ctx, actor, changeSet, "change_set.rolled_back", metadata); err != nil {
		return ChangeSet{}, err
	}
	return s.Store.GetChangeSet(ctx, actor.OrganizationID, projectID, changeSet.ID)
}

func (s Service) ListAuditEvents(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]AuditEvent, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Store.ListAuditEvents(ctx, actor.OrganizationID, projectID)
}

func (s Service) workflowChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string) (ChangeSet, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return ChangeSet{}, err
	}
	return s.Store.GetChangeSet(ctx, actor.OrganizationID, projectID, strings.TrimSpace(changeSetID))
}

func (s Service) authorizeWorkflow(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) error {
	if s.Store == nil {
		return fmt.Errorf("project store is required")
	}
	if s.Authorizer == nil {
		return identity.ErrProjectAccessDenied
	}
	if strings.TrimSpace(string(projectID)) == "" {
		return ErrNotFound
	}
	if err := actor.Validate(); err != nil {
		return err
	}
	return s.Authorizer.AuthorizeProject(ctx, actor, projectID)
}

func (s Service) newID(prefix string) (string, error) {
	newID := s.NewID
	if newID == nil {
		newID = ids.New
	}
	return newID(prefix)
}

func (s Service) appendChangeSetEvent(ctx context.Context, actor contract.ActorContext, changeSet ChangeSet, eventType string, payload map[string]any) error {
	id, err := s.newID("event")
	if err != nil {
		return err
	}
	return s.Store.AppendChangeSetEvent(ctx, ChangeSetEvent{
		ID:             id,
		OrganizationID: actor.OrganizationID,
		ProjectID:      changeSet.ProjectID,
		ChangeSetID:    changeSet.ID,
		EventType:      eventType,
		Actor:          actorLabel(actor),
		Payload:        payload,
		CreatedAt:      time.Now().UTC(),
	})
}

func (s Service) appendChangeSetAudit(ctx context.Context, actor contract.ActorContext, changeSet ChangeSet, action string, metadata map[string]any) error {
	id, err := s.newID("audit")
	if err != nil {
		return err
	}
	return s.Store.AppendAuditEvent(ctx, AuditEvent{
		ID:             id,
		OrganizationID: actor.OrganizationID,
		ProjectID:      changeSet.ProjectID,
		Actor:          actorLabel(actor),
		Action:         action,
		EntityType:     AuditEntityChangeSet,
		EntityID:       changeSet.ID,
		Metadata:       metadata,
		CreatedAt:      time.Now().UTC(),
	})
}

func runtimeForProject(projectValue Project, actor contract.ActorContext) ProjectRuntime {
	status := "active"
	if projectValue.Status == StatusArchived {
		status = "completed"
	}
	if projectValue.Status == StatusDraft {
		status = "blocked"
	}
	progress := 60
	if projectValue.Status == StatusDraft {
		progress = 10
	}
	if projectValue.Status == StatusArchived {
		progress = 100
	}
	return ProjectRuntime{
		Code:      string(projectValue.ID),
		Stage:     string(projectValue.Status),
		Progress:  progress,
		Status:    status,
		Owner:     actorLabel(actor),
		Budget:    0,
		Currency:  "CNY",
		Timezone:  "Asia/Shanghai",
		UpdatedAt: projectValue.UpdatedAt,
	}
}

func operationalRecordFromRequest(organizationID contract.OrganizationID, projectID contract.ProjectID, id string, request UpsertOperationalRecordRequest) OperationalRecord {
	now := time.Now().UTC()
	return OperationalRecord{
		ID:             id,
		OrganizationID: organizationID,
		ProjectID:      projectID,
		Kind:           request.Kind,
		Title:          strings.TrimSpace(request.Title),
		Status:         strings.TrimSpace(request.Status),
		OccurredAt:     request.OccurredAt,
		Fields:         request.Fields,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func validateCreateBusinessTask(request CreateBusinessTaskRequest) error {
	task := BusinessTask{
		Type:              request.Type,
		Name:              strings.TrimSpace(request.Name),
		Objective:         strings.TrimSpace(request.Objective),
		Status:            BusinessTaskDraft,
		SourceTaskIDs:     compactStrings(request.SourceTaskIDs),
		SourceArtifactIDs: compactStrings(request.SourceArtifactIDs),
		OutputArtifactIDs: []string{},
		Version:           1,
	}
	return validateBusinessTask(task)
}

func validateBusinessTask(task BusinessTask) error {
	if !validBusinessTaskType(task.Type) {
		return fmt.Errorf("business task type is invalid")
	}
	if strings.TrimSpace(task.Name) == "" || len(strings.TrimSpace(task.Name)) > 255 {
		return fmt.Errorf("business task name must be between 1 and 255 characters")
	}
	if strings.TrimSpace(task.Objective) == "" {
		return fmt.Errorf("business task objective is required")
	}
	if !validBusinessTaskStatus(task.Status) {
		return fmt.Errorf("business task status is invalid")
	}
	if hasEmptyString(task.SourceTaskIDs) || hasEmptyString(task.SourceArtifactIDs) || hasEmptyString(task.OutputArtifactIDs) {
		return fmt.Errorf("business task references must not be empty")
	}
	return nil
}

func validateOperationalRecordRequest(request UpsertOperationalRecordRequest) error {
	if !validOperationalKind(request.Kind) {
		return fmt.Errorf("operational record kind is invalid")
	}
	if strings.TrimSpace(request.Title) == "" || strings.TrimSpace(request.Status) == "" {
		return fmt.Errorf("operational record title and status are required")
	}
	if request.OccurredAt.IsZero() {
		return fmt.Errorf("operational record occurred_at is required")
	}
	if request.Fields == nil {
		return fmt.Errorf("operational record fields must be an object")
	}
	for key, value := range request.Fields {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("operational field key is required")
		}
		switch typed := value.(type) {
		case string, float64:
		case int, int64, uint64, jsonNumber:
		default:
			return fmt.Errorf("operational field %q has unsupported value type %T", key, typed)
		}
	}
	return nil
}

type jsonNumber interface{ String() string }

func validateCreateChangeSet(request CreateChangeSetRequest) error {
	if strings.TrimSpace(request.Name) == "" || len(strings.TrimSpace(request.Name)) > 255 {
		return fmt.Errorf("change set name must be between 1 and 255 characters")
	}
	if request.ArtifactRefs == nil {
		return fmt.Errorf("artifact_refs must be an array")
	}
	if request.BudgetLimit != nil && (*request.BudgetLimit < 0 || math.IsNaN(*request.BudgetLimit) || math.IsInf(*request.BudgetLimit, 0)) {
		return fmt.Errorf("budget_limit must be non-negative")
	}
	return nil
}

func normalizeChangeSetArtifactRefs(projectID contract.ProjectID, refs []contract.ProjectAssetRef) ([]contract.ProjectAssetRef, error) {
	if refs == nil {
		return nil, nil
	}
	normalized := make([]contract.ProjectAssetRef, len(refs))
	for index, ref := range refs {
		if ref.ProjectID == "" {
			ref.ProjectID = projectID
		}
		if ref.ProjectID != projectID || ref.Validate() != nil {
			return nil, fmt.Errorf("change set artifact_refs must belong to the project")
		}
		normalized[index] = ref
	}
	return normalized, nil
}

func validBusinessTaskType(value BusinessTaskType) bool {
	switch value {
	case BusinessTaskStrategy, BusinessTaskCreative, BusinessTaskVideo, BusinessTaskBrandVideo, BusinessTaskShortDramaPreroll, BusinessTaskGamePreroll, BusinessTaskCommercePreroll, BusinessTaskViralRemake, BusinessTaskVideoEdit:
		return true
	default:
		return false
	}
}

func validBusinessTaskStatus(value BusinessTaskStatus) bool {
	switch value {
	case BusinessTaskDraft, BusinessTaskInProgress, BusinessTaskReady, BusinessTaskCompleted, BusinessTaskFailed:
		return true
	default:
		return false
	}
}

func validOperationalKind(value OperationalRecordKind) bool {
	switch value {
	case OperationalRecordWorkItem, OperationalRecordEvidence, OperationalRecordActivity, OperationalRecordMetric, OperationalRecordPerformanceAd, OperationalRecordAudienceMix, OperationalRecordMethod, OperationalRecordDeliveryDiagnostic, OperationalRecordDeliveryAction, OperationalRecordUnifiedRecord:
		return true
	default:
		return false
	}
}

func compactStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strings.TrimSpace(value))
	}
	return result
}

func hasEmptyString(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func actorLabel(actor contract.ActorContext) string {
	return string(actor.Principal.Kind) + ":" + actor.Principal.ID
}
