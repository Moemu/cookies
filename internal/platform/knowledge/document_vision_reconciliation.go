package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const DocumentVisionReconciliationContractVersion = "platform-document-vision-reconciliation/v1"

var ErrDocumentVisionReconciliationForbidden = errors.New("document vision reconciliation requires an authorized human operator")
var ErrDocumentVisionReconciliationInvalid = errors.New("invalid document vision reconciliation request")
var ErrDocumentVisionReconciliationConflict = errors.New("document vision reconciliation is not in the expected state")
var ErrDocumentVisionReconciliationSameActor = errors.New("document vision reconciliation requires a second operator")

type DocumentVisionReconciliation struct {
	ContractVersion string                  `json:"contract_version"`
	ID              string                  `json:"id"`
	OrganizationID  contract.OrganizationID `json:"organization_id"`
	ProjectID       contract.ProjectID      `json:"project_id"`
	DocumentID      string                  `json:"document_id"`
	AttemptID       string                  `json:"attempt_id"`
	TaskIndex       int                     `json:"task_index"`
	IntentID        string                  `json:"intent_id"`
	Decision        string                  `json:"decision"`
	ExternalTaskID  string                  `json:"external_task_id,omitempty"`
	EvidenceRef     string                  `json:"evidence_ref"`
	Status          string                  `json:"status"`
	ProposedBy      string                  `json:"proposed_by"`
	ProposedAt      time.Time               `json:"proposed_at"`
	ConfirmedBy     string                  `json:"confirmed_by,omitempty"`
	ConfirmedAt     *time.Time              `json:"confirmed_at,omitempty"`
	AppliedAt       *time.Time              `json:"applied_at,omitempty"`
	ScheduledAt     *time.Time              `json:"scheduled_at,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

const DocumentVisionReconciliationCandidateContractVersion = "platform-document-vision-reconciliation-candidate/v1"

type DocumentVisionReconciliationCandidate struct {
	ContractVersion string                  `json:"contract_version"`
	OrganizationID  contract.OrganizationID `json:"organization_id"`
	ProjectID       contract.ProjectID      `json:"project_id"`
	DocumentID      string                  `json:"document_id"`
	DocumentTitle   string                  `json:"document_title"`
	Filename        string                  `json:"filename"`
	AttemptID       string                  `json:"attempt_id"`
	TaskIndex       int                     `json:"task_index"`
	PageNumbers     []int                   `json:"page_numbers"`
	Status          string                  `json:"status"`
	IntentID        string                  `json:"intent_id"`
	ProviderCode    string                  `json:"provider_code"`
	ModelAlias      string                  `json:"model_alias"`
	ModelVersion    string                  `json:"model_version"`
	RouteRevisionID string                  `json:"route_revision_id"`
	ErrorCode       string                  `json:"error_code"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type ProposeDocumentVisionReconciliationRequest struct {
	TaskIndex        int    `json:"task_index"`
	ExpectedIntentID string `json:"expected_intent_id"`
	Decision         string `json:"decision"`
	ExternalTaskID   string `json:"external_task_id,omitempty"`
	EvidenceRef      string `json:"evidence_ref"`
}

type ConfirmDocumentVisionReconciliationRequest struct {
	Approve *bool `json:"approve"`
}

func (s Service) ListDocumentVisionReconciliationCandidates(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	limit int,
) ([]DocumentVisionReconciliationCandidate, error) {
	if err := validateVisionReconciliationActor(actor); err != nil {
		return nil, err
	}
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, ErrDocumentVisionReconciliationInvalid
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT task.organization_id, task.project_id, task.document_id,
		document.title, document.filename, task.attempt_id, task.task_index, task.page_numbers,
		task.status, task.intent_id, task.provider_code, task.model_alias, task.model_version,
		task.route_revision_id, task.error_code, task.updated_at
		FROM platform_knowledge_document_vision_tasks AS task
		INNER JOIN platform_knowledge_documents AS document
		  ON document.organization_id = task.organization_id
		 AND document.project_id = task.project_id
		 AND document.id = task.document_id
		 AND document.vision_attempt_id = task.attempt_id
		WHERE task.organization_id = ? AND task.project_id = ?
		  AND task.status IN ('submitting', 'unknown')
		  AND task.intent_id <> ''
		  AND document.vision_fallback_status = 'failed'
		  AND document.vision_error_code IN (
			'DOCUMENT_VISION_SUBMISSION_UNKNOWN',
			'DOCUMENT_VISION_SUBMISSION_INVALID',
			'DOCUMENT_VISION_CHECKPOINT_FAILED'
		  )
		ORDER BY task.updated_at, task.document_id, task.task_index
		LIMIT ?`, actor.OrganizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []DocumentVisionReconciliationCandidate{}
	for rows.Next() {
		value := DocumentVisionReconciliationCandidate{ContractVersion: DocumentVisionReconciliationCandidateContractVersion}
		var encodedPages []byte
		if err := rows.Scan(
			&value.OrganizationID, &value.ProjectID, &value.DocumentID, &value.DocumentTitle,
			&value.Filename, &value.AttemptID, &value.TaskIndex, &encodedPages, &value.Status,
			&value.IntentID, &value.ProviderCode, &value.ModelAlias, &value.ModelVersion,
			&value.RouteRevisionID, &value.ErrorCode, &value.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(encodedPages, &value.PageNumbers); err != nil || len(value.PageNumbers) == 0 {
			return nil, ErrDocumentVisionReconciliationConflict
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s Service) GetDocumentVisionReconciliation(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	reconciliationID string,
) (DocumentVisionReconciliation, error) {
	if err := validateVisionReconciliationActor(actor); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	value, err := s.getDocumentVisionReconciliation(ctx, actor.OrganizationID, projectID, strings.TrimSpace(reconciliationID))
	if errors.Is(err, sql.ErrNoRows) {
		return DocumentVisionReconciliation{}, ErrNotFound
	}
	return value, err
}

func (s Service) ProposeDocumentVisionReconciliation(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	documentID string,
	request ProposeDocumentVisionReconciliationRequest,
) (DocumentVisionReconciliation, error) {
	if err := validateVisionReconciliationActor(actor); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	request.ExpectedIntentID = strings.TrimSpace(request.ExpectedIntentID)
	request.Decision = strings.TrimSpace(request.Decision)
	request.ExternalTaskID = strings.TrimSpace(request.ExternalTaskID)
	request.EvidenceRef = strings.TrimSpace(request.EvidenceRef)
	if request.TaskIndex < 0 || !validVisionIntentID(request.ExpectedIntentID) || !validVisionReconciliationDecision(request) {
		return DocumentVisionReconciliation{}, ErrDocumentVisionReconciliationInvalid
	}
	reconciliationID, err := s.newID("visionreconciliation")
	if err != nil {
		return DocumentVisionReconciliation{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return DocumentVisionReconciliation{}, err
	}
	defer tx.Rollback()
	var attemptID, taskStatus, intentID, fallbackStatus, fallbackErrorCode string
	if err := tx.QueryRowContext(ctx, `SELECT document.vision_attempt_id, task.status, task.intent_id,
		document.vision_fallback_status, document.vision_error_code
		FROM platform_knowledge_documents AS document
		INNER JOIN platform_knowledge_document_vision_tasks AS task
		  ON task.organization_id = document.organization_id
		 AND task.project_id = document.project_id
		 AND task.document_id = document.id
		 AND task.attempt_id = document.vision_attempt_id
		WHERE document.organization_id = ? AND document.project_id = ? AND document.id = ?
		  AND task.task_index = ? FOR UPDATE`, actor.OrganizationID, projectID, documentID, request.TaskIndex,
	).Scan(&attemptID, &taskStatus, &intentID, &fallbackStatus, &fallbackErrorCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DocumentVisionReconciliation{}, ErrNotFound
		}
		return DocumentVisionReconciliation{}, err
	}
	if (taskStatus != "unknown" && taskStatus != "submitting") || intentID != request.ExpectedIntentID ||
		fallbackStatus != "failed" || !documentVisionRequiresReconciliation(fallbackErrorCode) {
		return DocumentVisionReconciliation{}, ErrDocumentVisionReconciliationConflict
	}
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_knowledge_document_vision_reconciliations
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?
		  AND task_index = ? AND status IN ('proposed', 'applied')`,
		actor.OrganizationID, projectID, documentID, attemptID, request.TaskIndex,
	).Scan(&active); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	if active != 0 {
		return DocumentVisionReconciliation{}, ErrDocumentVisionReconciliationConflict
	}
	if request.Decision == "accepted" {
		var conflictingBinding int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*)
			FROM platform_knowledge_document_vision_tasks
			WHERE organization_id = ? AND external_task_id = ?
			  AND NOT (project_id = ? AND document_id = ? AND attempt_id = ? AND task_index = ?)`,
			actor.OrganizationID, request.ExternalTaskID, projectID, documentID, attemptID, request.TaskIndex,
		).Scan(&conflictingBinding); err != nil {
			return DocumentVisionReconciliation{}, err
		}
		if conflictingBinding != 0 {
			return DocumentVisionReconciliation{}, ErrDocumentVisionReconciliationConflict
		}
	}
	now := s.now()
	if _, err := tx.ExecContext(ctx, `INSERT INTO platform_knowledge_document_vision_reconciliations
		(id, organization_id, project_id, document_id, attempt_id, task_index, intent_id,
		 decision, external_task_id, evidence_ref, status, proposed_by, proposed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'proposed', ?, ?, ?, ?)`,
		reconciliationID, actor.OrganizationID, projectID, documentID, attemptID, request.TaskIndex,
		request.ExpectedIntentID, request.Decision, request.ExternalTaskID, request.EvidenceRef,
		actor.Principal.ID, now, now, now,
	); err != nil {
		var mysqlError *mysqlDriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return DocumentVisionReconciliation{}, ErrDocumentVisionReconciliationConflict
		}
		return DocumentVisionReconciliation{}, err
	}
	if err := tx.Commit(); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	return s.getDocumentVisionReconciliation(ctx, actor.OrganizationID, projectID, reconciliationID)
}

func (s Service) ConfirmDocumentVisionReconciliation(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	reconciliationID string,
	request ConfirmDocumentVisionReconciliationRequest,
) (DocumentVisionReconciliation, error) {
	if err := validateVisionReconciliationActor(actor); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	if request.Approve == nil {
		return DocumentVisionReconciliation{}, ErrDocumentVisionReconciliationInvalid
	}
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return DocumentVisionReconciliation{}, err
	}
	defer tx.Rollback()
	value, err := scanDocumentVisionReconciliation(tx.QueryRowContext(ctx, documentVisionReconciliationSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, projectID, reconciliationID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return DocumentVisionReconciliation{}, ErrNotFound
	}
	if err != nil {
		return DocumentVisionReconciliation{}, err
	}
	if value.Status != "proposed" {
		return DocumentVisionReconciliation{}, ErrDocumentVisionReconciliationConflict
	}
	if value.ProposedBy == actor.Principal.ID {
		return DocumentVisionReconciliation{}, ErrDocumentVisionReconciliationSameActor
	}
	now := s.now()
	if !*request.Approve {
		if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_reconciliations
			SET status = 'rejected', confirmed_by = ?, confirmed_at = ?, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'proposed'`,
			actor.Principal.ID, now, now, actor.OrganizationID, projectID, reconciliationID,
		); err != nil {
			return DocumentVisionReconciliation{}, err
		}
		if err := tx.Commit(); err != nil {
			return DocumentVisionReconciliation{}, err
		}
		return s.getDocumentVisionReconciliation(ctx, actor.OrganizationID, projectID, reconciliationID)
	}
	var taskStatus, intentID, fallbackStatus, fallbackErrorCode string
	if err := tx.QueryRowContext(ctx, `SELECT task.status, task.intent_id,
		document.vision_fallback_status, document.vision_error_code
		FROM platform_knowledge_document_vision_tasks AS task
		INNER JOIN platform_knowledge_documents AS document
		  ON document.organization_id = task.organization_id
		 AND document.project_id = task.project_id
		 AND document.id = task.document_id
		 AND document.vision_attempt_id = task.attempt_id
		WHERE task.organization_id = ? AND task.project_id = ? AND task.document_id = ?
		  AND task.attempt_id = ? AND task.task_index = ?
		FOR UPDATE`, value.OrganizationID, value.ProjectID, value.DocumentID, value.AttemptID, value.TaskIndex,
	).Scan(&taskStatus, &intentID, &fallbackStatus, &fallbackErrorCode); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	if (taskStatus != "unknown" && taskStatus != "submitting") || intentID != value.IntentID ||
		fallbackStatus != "failed" || !documentVisionRequiresReconciliation(fallbackErrorCode) {
		return DocumentVisionReconciliation{}, ErrDocumentVisionReconciliationConflict
	}
	if value.Decision == "accepted" {
		if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_tasks
			SET status = 'submitted', external_task_id = ?, error_code = '', error_message = '', updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ? AND task_index = ?`,
			value.ExternalTaskID, now, value.OrganizationID, value.ProjectID, value.DocumentID, value.AttemptID, value.TaskIndex,
		); err != nil {
			var mysqlError *mysqlDriver.MySQLError
			if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
				return DocumentVisionReconciliation{}, ErrDocumentVisionReconciliationConflict
			}
			return DocumentVisionReconciliation{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_documents
			SET status = 'parsing', parse_strategy = 'hybrid', parse_phase = 'visual_fallback',
				parse_progress = GREATEST(COALESCE(parse_progress, 72), 72),
				vision_fallback_status = 'queued', vision_error_code = '', vision_error_message = '',
				heartbeat_at = ?, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND vision_attempt_id = ?`,
			now, now, value.OrganizationID, value.ProjectID, value.DocumentID, value.AttemptID,
		); err != nil {
			return DocumentVisionReconciliation{}, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_tasks
			SET status = 'failed', error_code = 'DOCUMENT_VISION_RECONCILED_NOT_ACCEPTED',
				error_message = 'Two operators confirmed that LAS did not accept this submission', updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ? AND task_index = ?`,
			now, value.OrganizationID, value.ProjectID, value.DocumentID, value.AttemptID, value.TaskIndex,
		); err != nil {
			return DocumentVisionReconciliation{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_documents
			SET status = 'partial', parse_phase = 'partial', parse_progress = 100, preview_status = 'partial',
				vision_fallback_status = 'failed', vision_error_code = 'DOCUMENT_VISION_RECONCILED_NOT_ACCEPTED',
				vision_error_message = '两名操作员已确认 LAS 未接收该任务；原文本仍可使用，可由用户明确发起新尝试。',
				vision_completed_at = ?, heartbeat_at = ?, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND vision_attempt_id = ?`,
			now, now, now, value.OrganizationID, value.ProjectID, value.DocumentID, value.AttemptID,
		); err != nil {
			return DocumentVisionReconciliation{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_document_pages
			SET status = 'failed', error_code = 'DOCUMENT_VISION_RECONCILED_NOT_ACCEPTED',
				error_message = 'LAS did not accept the prior submission', updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND document_id = ? AND status IN ('selected', 'processing')`,
			now, value.OrganizationID, value.ProjectID, value.DocumentID,
		); err != nil {
			return DocumentVisionReconciliation{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_reconciliations
		SET status = 'applied', confirmed_by = ?, confirmed_at = ?, applied_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'proposed'`,
		actor.Principal.ID, now, now, now, actor.OrganizationID, projectID, reconciliationID,
	); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	if err := tx.Commit(); err != nil {
		return DocumentVisionReconciliation{}, err
	}
	if value.Decision == "accepted" {
		_ = s.scheduleAppliedDocumentVisionReconciliation(ctx, value.OrganizationID, value.ProjectID, reconciliationID)
	}
	return s.getDocumentVisionReconciliation(ctx, actor.OrganizationID, projectID, reconciliationID)
}

func validateVisionReconciliationActor(actor contract.ActorContext) error {
	if actor.Principal.Kind != contract.PrincipalUser || strings.TrimSpace(actor.Principal.ID) == "" || !actor.HasScope(ScopeDocumentVisionReconcile) {
		return ErrDocumentVisionReconciliationForbidden
	}
	return nil
}

func validVisionReconciliationDecision(request ProposeDocumentVisionReconciliationRequest) bool {
	if !validVisionEvidenceRef(request.EvidenceRef) {
		return false
	}
	switch request.Decision {
	case "accepted":
		return validVisionExternalTaskID(request.ExternalTaskID)
	case "not_accepted":
		return request.ExternalTaskID == ""
	default:
		return false
	}
}

func validVisionExternalTaskID(value string) bool {
	if value == "" || len(value) > 160 || strings.ContainsAny(value, " \t\r\n") || strings.Contains(value, "://") {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func validVisionEvidenceRef(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 || strings.ContainsAny(value, "\r\n") || strings.Contains(value, "://") {
		return false
	}
	lower := strings.ToLower(value)
	for _, forbidden := range []string{"authorization", "api_key", "apikey", "secret", "token", "password"} {
		if strings.Contains(lower, forbidden) {
			return false
		}
	}
	return strings.Contains(value, ":")
}

func validVisionIntentID(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

const documentVisionReconciliationSelect = `SELECT id, organization_id, project_id, document_id,
	attempt_id, task_index, intent_id, decision, external_task_id, evidence_ref, status,
	proposed_by, proposed_at, confirmed_by, confirmed_at, applied_at, scheduled_at, created_at, updated_at
	FROM platform_knowledge_document_vision_reconciliations `

func (s Service) getDocumentVisionReconciliation(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, reconciliationID string) (DocumentVisionReconciliation, error) {
	return scanDocumentVisionReconciliation(s.DB.QueryRowContext(ctx, documentVisionReconciliationSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, reconciliationID))
}

type reconciliationRow interface{ Scan(...any) error }

func scanDocumentVisionReconciliation(row reconciliationRow) (DocumentVisionReconciliation, error) {
	value := DocumentVisionReconciliation{ContractVersion: DocumentVisionReconciliationContractVersion}
	var confirmedBy string
	var confirmedAt, appliedAt, scheduledAt sql.NullTime
	err := row.Scan(
		&value.ID, &value.OrganizationID, &value.ProjectID, &value.DocumentID,
		&value.AttemptID, &value.TaskIndex, &value.IntentID, &value.Decision,
		&value.ExternalTaskID, &value.EvidenceRef, &value.Status, &value.ProposedBy,
		&value.ProposedAt, &confirmedBy, &confirmedAt, &appliedAt, &scheduledAt,
		&value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return DocumentVisionReconciliation{}, err
	}
	value.ConfirmedBy = confirmedBy
	if confirmedAt.Valid {
		value.ConfirmedAt = &confirmedAt.Time
	}
	if appliedAt.Valid {
		value.AppliedAt = &appliedAt.Time
	}
	if scheduledAt.Valid {
		value.ScheduledAt = &scheduledAt.Time
	}
	return value, nil
}

func (s Service) scheduleAppliedDocumentVisionReconciliation(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, reconciliationID string) error {
	if s.VisionScheduler == nil {
		return ErrKnowledgeControlUnavailable
	}
	value, err := s.getDocumentVisionReconciliation(ctx, organizationID, projectID, reconciliationID)
	if err != nil {
		return err
	}
	if value.Status != "applied" || value.Decision != "accepted" || value.ScheduledAt != nil {
		return nil
	}
	document, err := scanDocument(s.DB.QueryRowContext(ctx, documentSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ? AND vision_attempt_id = ?`,
		organizationID, projectID, value.DocumentID, value.AttemptID,
	))
	if err != nil {
		return err
	}
	if err := s.VisionScheduler.ScheduleDocumentVisionFallback(ctx, document, document.VisionSelectedPages, value.ID); err != nil {
		return err
	}
	now := s.now()
	_, err = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_reconciliations
		SET scheduled_at = COALESCE(scheduled_at, ?), updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'applied' AND decision = 'accepted'`,
		now, now, organizationID, projectID, reconciliationID)
	return err
}

func (s Service) recoverOnePendingVisionReconciliation(ctx context.Context) (bool, error) {
	if s.VisionScheduler == nil {
		return false, nil
	}
	var organizationID contract.OrganizationID
	var projectID contract.ProjectID
	var reconciliationID string
	err := s.DB.QueryRowContext(ctx, `SELECT organization_id, project_id, id
		FROM platform_knowledge_document_vision_reconciliations
		WHERE status = 'applied' AND decision = 'accepted' AND scheduled_at IS NULL
		ORDER BY updated_at, id LIMIT 1`).Scan(&organizationID, &projectID, &reconciliationID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := s.scheduleAppliedDocumentVisionReconciliation(ctx, organizationID, projectID, reconciliationID); err != nil {
		return false, err
	}
	return true, nil
}
