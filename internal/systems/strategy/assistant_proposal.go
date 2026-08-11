package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	ArtifactProposalContractV1         = "strategy-artifact-proposal/v1"
	ResearchAdoptionProposalContractV1 = "strategy-research-adoption-proposal/v1"
)

const artifactProposalSelect = `SELECT id, organization_id, project_id, workspace_id,
	conversation_id, proposal_kind, target_type, target_id, target_version,
	base_content_hash, operations, rationale, risk, status,
	COALESCE(source_message_id, ''), COALESCE(finding_ids, JSON_ARRAY()),
	COALESCE(source_research_run_id, ''), COALESCE(proposal_fingerprint, ''),
	stale_reason, COALESCE(supersedes_proposal_id, ''),
	created_by, COALESCE(applied_by, ''), applied_at,
	COALESCE(ignored_by, ''), ignored_at, version, created_at, updated_at
	FROM strategy_artifact_proposals`

// ArtifactProposal is a reviewable AI change set. It never mutates an
// authoritative artifact until a user explicitly applies it.
type ArtifactProposal struct {
	ContractVersion      string                  `json:"contract_version"`
	ID                   string                  `json:"id"`
	OrganizationID       contract.OrganizationID `json:"organization_id"`
	ProjectID            contract.ProjectID      `json:"project_id"`
	WorkspaceID          string                  `json:"workspace_id"`
	ConversationID       string                  `json:"conversation_id"`
	ProposalKind         string                  `json:"proposal_kind"`
	TargetType           string                  `json:"target_type"`
	TargetID             string                  `json:"target_id"`
	TargetVersion        int64                   `json:"target_version"`
	BaseContentHash      contract.ContentHash    `json:"base_content_hash"`
	Operations           []BriefPatchOperation   `json:"operations"`
	Rationale            string                  `json:"rationale"`
	Risk                 string                  `json:"risk"`
	Status               string                  `json:"status"`
	SourceMessageID      string                  `json:"source_message_id,omitempty"`
	FindingIDs           []string                `json:"finding_ids,omitempty"`
	SourceResearchRunID  string                  `json:"source_research_run_id,omitempty"`
	ProposalFingerprint  string                  `json:"-"`
	StaleReason          string                  `json:"stale_reason"`
	SupersedesProposalID string                  `json:"supersedes_proposal_id,omitempty"`
	CreatedBy            string                  `json:"created_by"`
	AppliedBy            string                  `json:"applied_by,omitempty"`
	AppliedAt            *time.Time              `json:"applied_at,omitempty"`
	IgnoredBy            string                  `json:"ignored_by,omitempty"`
	IgnoredAt            *time.Time              `json:"ignored_at,omitempty"`
	Version              int64                   `json:"version"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
}

type ApplyArtifactProposalRequest struct {
	ExpectedVersion int64                 `json:"expected_version"`
	Operations      []BriefPatchOperation `json:"operations,omitempty"`
}

type ApplyArtifactProposalResult struct {
	Proposal ArtifactProposal `json:"proposal"`
	Draft    BriefDraft       `json:"brief_draft"`
}

func briefDraftProposalContentHash(draft BriefDraft) (contract.ContentHash, error) {
	return contract.NewContentHash(struct {
		Document    BriefDocument         `json:"document"`
		FieldStates map[string]FieldState `json:"field_states"`
	}{draft.Document, draft.FieldStates})
}

func scanArtifactProposal(row rowScanner) (ArtifactProposal, error) {
	var value ArtifactProposal
	var operations json.RawMessage
	var findingIDs json.RawMessage
	var appliedAt, ignoredAt sql.NullTime
	if err := row.Scan(
		&value.ID, &value.OrganizationID, &value.ProjectID, &value.WorkspaceID,
		&value.ConversationID, &value.ProposalKind, &value.TargetType, &value.TargetID,
		&value.TargetVersion, &value.BaseContentHash, &operations, &value.Rationale,
		&value.Risk, &value.Status, &value.SourceMessageID, &findingIDs,
		&value.SourceResearchRunID, &value.ProposalFingerprint,
		&value.StaleReason, &value.SupersedesProposalID,
		&value.CreatedBy, &value.AppliedBy, &appliedAt, &value.IgnoredBy, &ignoredAt, &value.Version,
		&value.CreatedAt, &value.UpdatedAt,
	); err != nil {
		return ArtifactProposal{}, mapNotFound(err)
	}
	if err := json.Unmarshal(operations, &value.Operations); err != nil {
		return ArtifactProposal{}, err
	}
	if err := json.Unmarshal(findingIDs, &value.FindingIDs); err != nil {
		return ArtifactProposal{}, err
	}
	if appliedAt.Valid {
		value.AppliedAt = &appliedAt.Time
	}
	if ignoredAt.Valid {
		value.IgnoredAt = &ignoredAt.Time
	}
	if value.Operations == nil {
		value.Operations = []BriefPatchOperation{}
	}
	value.ContractVersion = ArtifactProposalContractV1
	if value.ProposalKind == "research" {
		value.ContractVersion = ResearchAdoptionProposalContractV1
	}
	return value, nil
}

func (s Service) createAssistantBriefProposal(
	ctx context.Context,
	tx *sql.Tx,
	agentTaskID string,
	workspaceID string,
	conversationID string,
	sourceMessageID string,
	draft BriefDraft,
	decision ConversationTurnDecision,
	now time.Time,
) (ArtifactProposal, error) {
	if len(decision.Patch.Operations) == 0 {
		return ArtifactProposal{}, ErrInvalidRequest
	}
	proposalID, err := s.newID("proposal")
	if err != nil {
		return ArtifactProposal{}, err
	}
	baseHash, err := briefDraftProposalContentHash(draft)
	if err != nil {
		return ArtifactProposal{}, err
	}
	risk := "low"
	for _, operation := range decision.Patch.Operations {
		if state, ok := draft.FieldStates[operation.FieldPath]; ok && state.Confirmation == "confirmed" {
			risk = "high"
			break
		}
	}
	rationale := strings.TrimSpace(decision.AssistantReply)
	if len([]rune(rationale)) > 500 {
		rationale = string([]rune(rationale)[:500])
	}
	if rationale == "" {
		rationale = "AI 根据当前项目上下文提出字段补充，采用前请核对。"
	}
	proposal := ArtifactProposal{
		ContractVersion: ArtifactProposalContractV1,
		ID:              proposalID, OrganizationID: draft.OrganizationID, ProjectID: draft.ProjectID,
		WorkspaceID: workspaceID, ConversationID: conversationID, ProposalKind: "assistant",
		TargetType: "brief_draft", TargetID: draft.ID, TargetVersion: draft.Version,
		BaseContentHash: baseHash, Operations: decision.Patch.Operations, Rationale: rationale,
		Risk: risk, Status: "proposed", SourceMessageID: sourceMessageID,
		CreatedBy: agentTaskID, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	operations, err := snapshotJSON(proposal.Operations)
	if err != nil {
		return ArtifactProposal{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO strategy_artifact_proposals
		(id, organization_id, project_id, workspace_id, conversation_id, proposal_kind,
		target_type, target_id, target_version, base_content_hash, operations, rationale,
		risk, status, source_message_id, created_by, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		proposal.ID, proposal.OrganizationID, proposal.ProjectID, proposal.WorkspaceID,
		proposal.ConversationID, proposal.ProposalKind, proposal.TargetType, proposal.TargetID,
		proposal.TargetVersion, proposal.BaseContentHash, operations, proposal.Rationale,
		proposal.Risk, proposal.Status, proposal.SourceMessageID, proposal.CreatedBy,
		proposal.Version, proposal.CreatedAt, proposal.UpdatedAt)
	return proposal, err
}

func (s Service) ListArtifactProposals(
	ctx context.Context,
	actor contract.ActorContext,
	workspaceID string,
	status string,
) ([]ArtifactProposal, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	workspace, err := s.GetWorkspace(ctx, actor, workspaceID)
	if err != nil {
		return nil, err
	}
	status = strings.TrimSpace(status)
	query := artifactProposalSelect + ` WHERE organization_id = ? AND project_id = ? AND workspace_id = ? AND proposal_kind = 'assistant'`
	args := []any{actor.OrganizationID, workspace.ProjectID, workspaceID}
	if status != "" {
		if status != "proposed" && status != "applied" && status != "edited" && status != "ignored" && status != "stale" {
			return nil, ErrInvalidRequest
		}
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT 100`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]ArtifactProposal, 0)
	for rows.Next() {
		value, scanErr := scanArtifactProposal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s Service) ApplyArtifactProposal(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	proposalID string,
	request ApplyArtifactProposalRequest,
) (ApplyArtifactProposalResult, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	if err := key.Validate(); err != nil || proposalID == "" || request.ExpectedVersion < 1 {
		return ApplyArtifactProposalResult{}, false, ErrInvalidRequest
	}
	known, err := scanArtifactProposal(s.DB.QueryRowContext(ctx, artifactProposalSelect+`
		WHERE organization_id = ? AND id = ?`, actor.OrganizationID, proposalID))
	if err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	if _, err := s.project(ctx, actor, known.ProjectID); err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	command := "assistant_proposal.apply"
	if known.ProposalKind == "research" {
		command = "research_proposal.apply"
	}
	hash, err := contract.CanonicalJSONHash(struct {
		ProposalID string                       `json:"proposal_id"`
		Request    ApplyArtifactProposalRequest `json:"request"`
	}{proposalID, request})
	if err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	var prior ApplyArtifactProposalResult
	found, err := s.loadReceipt(ctx, actor, known.ProjectID, command, key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	defer tx.Rollback()
	proposal, err := scanArtifactProposal(tx.QueryRowContext(ctx, artifactProposalSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, known.ProjectID, proposalID))
	if err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	if proposal.Version != request.ExpectedVersion || proposal.Status != "proposed" || proposal.TargetType != "brief_draft" {
		return ApplyArtifactProposalResult{}, false, ErrVersionConflict
	}
	draft, err := scanBriefDraft(tx.QueryRowContext(ctx, briefDraftSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, proposal.ProjectID, proposal.TargetID))
	if err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	currentHash, err := briefDraftProposalContentHash(draft)
	if err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	if draft.Version != proposal.TargetVersion || currentHash != proposal.BaseContentHash || draft.Status != "open" {
		now := s.now()
		staleReason := "目标 Brief 已产生新版本或不可再编辑"
		if _, updateErr := tx.ExecContext(ctx, `UPDATE strategy_artifact_proposals
			SET status = 'stale', stale_reason = ?, version = version + 1, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
			staleReason, now, actor.OrganizationID, proposal.ProjectID, proposal.ID, proposal.Version); updateErr != nil {
			return ApplyArtifactProposalResult{}, false, updateErr
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return ApplyArtifactProposalResult{}, false, commitErr
		}
		return ApplyArtifactProposalResult{}, false, ErrVersionConflict
	}
	operations := proposal.Operations
	status := "applied"
	if len(request.Operations) > 0 {
		if err := validateEditedProposalOperations(proposal.Operations, request.Operations); err != nil {
			return ApplyArtifactProposalResult{}, false, err
		}
		operations = request.Operations
		status = "edited"
	}
	now := s.now()
	updated, err := ApplyBriefPatch(draft, BriefPatch{
		ContractVersion: draft.Document.ContractVersion,
		ExpectedVersion: draft.Version,
		Operations:      operations,
	}, PatchFromUser, actor.Principal.ID, now)
	if err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	if err := updateBriefDraft(ctx, tx, draft.Version, updated); err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	revisionID, err := s.newID("briefrev")
	if err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	patchJSON, _ := snapshotJSON(BriefPatch{ExpectedVersion: draft.Version, Operations: operations})
	snapshotHash, err := contract.NewContentHash(updated.Document)
	if err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_brief_revisions
		(id, organization_id, project_id, draft_id, draft_version, patch, snapshot_hash, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, revisionID, actor.OrganizationID, proposal.ProjectID,
		updated.ID, updated.Version, patchJSON, snapshotHash, actor.Principal.ID, now); err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	if err := syncEvidenceReferences(ctx, tx, actor.OrganizationID, proposal.ProjectID,
		"brief_draft", updated.ID, updated.Version, "reference_ids", updated.Document.ReferenceIDs,
		actor.Principal.ID, now, true); err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	if proposal.ProposalKind == "research" {
		for _, operation := range operations {
			if err := insertResearchFindingEvidenceReferences(ctx, tx, proposal, "brief_draft", updated.ID, updated.Version, operation.FieldPath, actor.Principal.ID, now); err != nil {
				return ApplyArtifactProposalResult{}, false, err
			}
		}
	}
	storedOperations, _ := snapshotJSON(operations)
	result, err := tx.ExecContext(ctx, `UPDATE strategy_artifact_proposals SET operations = ?,
		status = ?, applied_by = ?, applied_at = ?, version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ? AND status = 'proposed'`,
		storedOperations, status, actor.Principal.ID, now, now, actor.OrganizationID,
		proposal.ProjectID, proposal.ID, proposal.Version)
	if err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	if changed, rowsErr := result.RowsAffected(); rowsErr != nil || changed != 1 {
		if rowsErr != nil {
			return ApplyArtifactProposalResult{}, false, rowsErr
		}
		return ApplyArtifactProposalResult{}, false, ErrVersionConflict
	}
	proposal.Operations = operations
	proposal.Status = status
	proposal.AppliedBy = actor.Principal.ID
	proposal.AppliedAt = &now
	proposal.Version++
	proposal.UpdatedAt = now
	taskStatus := "waiting_user"
	if updated.Completeness.Ready {
		taskStatus = "ready_to_confirm"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_tasks SET status = ?, version = version + 1,
		updated_at = ? WHERE organization_id = ? AND project_id = ? AND conversation_id = ? AND discarded_at IS NULL`,
		taskStatus, now, actor.OrganizationID, proposal.ProjectID, proposal.ConversationID); err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	eventID, _ := s.newID("stratevent")
	payload := mustJSON(map[string]any{"proposal_id": proposal.ID, "brief_draft_version": updated.Version, "proposal_kind": proposal.ProposalKind})
	eventType := "assistant.proposal.applied"
	if proposal.ProposalKind == "research" {
		eventType = "research.proposal.applied"
	}
	if err := insertConversationEvent(ctx, tx, eventID, actor.OrganizationID, proposal.ProjectID,
		proposal.ConversationID, eventType, payload, now); err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	response := ApplyArtifactProposalResult{Proposal: proposal, Draft: updated}
	if err := insertReceipt(ctx, tx, actor, proposal.ProjectID, command, key, hash, 200, response, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, proposal.ProjectID, command, key, hash, &prior)
			return prior, found, readErr
		}
		return ApplyArtifactProposalResult{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return ApplyArtifactProposalResult{}, false, err
	}
	return response, false, nil
}

func validateEditedProposalOperations(original, edited []BriefPatchOperation) error {
	if len(edited) == 0 || len(edited) != len(original) || len(edited) > 32 {
		return ErrInvalidRequest
	}
	allowed := make(map[string]struct{}, len(original))
	for _, operation := range original {
		allowed[operation.FieldPath] = struct{}{}
	}
	seen := make(map[string]struct{}, len(edited))
	for _, operation := range edited {
		if operation.Op != "set" || !json.Valid(operation.Value) {
			return ErrInvalidRequest
		}
		if _, ok := allowed[operation.FieldPath]; !ok {
			return ErrInvalidRequest
		}
		if _, duplicate := seen[operation.FieldPath]; duplicate {
			return ErrInvalidRequest
		}
		seen[operation.FieldPath] = struct{}{}
	}
	return nil
}

func (s Service) IgnoreArtifactProposal(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	proposalID string,
	expectedVersion int64,
) (ArtifactProposal, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return ArtifactProposal{}, false, err
	}
	if err := key.Validate(); err != nil || proposalID == "" || expectedVersion < 1 {
		return ArtifactProposal{}, false, ErrInvalidRequest
	}
	known, err := scanArtifactProposal(s.DB.QueryRowContext(ctx, artifactProposalSelect+`
		WHERE organization_id = ? AND id = ?`, actor.OrganizationID, proposalID))
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	if _, err := s.project(ctx, actor, known.ProjectID); err != nil {
		return ArtifactProposal{}, false, err
	}
	command := "assistant_proposal.ignore"
	eventType := "assistant.proposal.ignored"
	if known.ProposalKind == "research" {
		command = "research_proposal.ignore"
		eventType = "research.proposal.ignored"
	}
	hash, err := contract.CanonicalJSONHash(struct {
		ProposalID      string `json:"proposal_id"`
		ExpectedVersion int64  `json:"expected_version"`
	}{proposalID, expectedVersion})
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	var prior ArtifactProposal
	found, err := s.loadReceipt(ctx, actor, known.ProjectID, command, key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	defer tx.Rollback()
	proposal, err := scanArtifactProposal(tx.QueryRowContext(ctx, artifactProposalSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, known.ProjectID, proposalID))
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	if proposal.Version != expectedVersion || proposal.Status != "proposed" {
		return ArtifactProposal{}, false, ErrVersionConflict
	}
	now := s.now()
	result, err := tx.ExecContext(ctx, `UPDATE strategy_artifact_proposals SET status = 'ignored',
		ignored_by = ?, ignored_at = ?, version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ? AND status = 'proposed'`,
		actor.Principal.ID, now, now, actor.OrganizationID, proposal.ProjectID, proposal.ID, proposal.Version)
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	if changed, rowsErr := result.RowsAffected(); rowsErr != nil || changed != 1 {
		if rowsErr != nil {
			return ArtifactProposal{}, false, rowsErr
		}
		return ArtifactProposal{}, false, ErrVersionConflict
	}
	proposal.Status = "ignored"
	proposal.IgnoredBy = actor.Principal.ID
	proposal.IgnoredAt = &now
	proposal.Version++
	proposal.UpdatedAt = now
	eventID, err := s.newID("stratevent")
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	payload := mustJSON(map[string]any{"proposal_id": proposal.ID, "proposal_kind": proposal.ProposalKind})
	if err := insertConversationEvent(ctx, tx, eventID, actor.OrganizationID, proposal.ProjectID,
		proposal.ConversationID, eventType, payload, now); err != nil {
		return ArtifactProposal{}, false, err
	}
	if err := insertReceipt(ctx, tx, actor, proposal.ProjectID, command, key, hash, 200, proposal, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, proposal.ProjectID, command, key, hash, &prior)
			return prior, found, readErr
		}
		return ArtifactProposal{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return ArtifactProposal{}, false, err
	}
	return proposal, false, nil
}
