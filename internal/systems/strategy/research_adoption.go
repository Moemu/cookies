package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/knowledge"
)

// OnResearchCompleted is the composition-root adapter from Knowledge into the
// Strategy-owned proposal lifecycle. It creates reviewable diffs only; it does
// not write a Brief or Strategy value.
func (s Service) OnResearchCompleted(ctx context.Context, run knowledge.ResearchRun) error {
	if run.Status != "completed" || run.RunMode != "deep" || run.SourceRef == nil ||
		run.SourceRef.Type != "strategy_workspace" || strings.TrimSpace(run.SourceRef.ID) == "" {
		return nil
	}
	workspace, err := scanWorkspace(s.DB.QueryRowContext(ctx, workspaceSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		run.OrganizationID, run.ProjectID, run.SourceRef.ID))
	if err != nil {
		return err
	}
	task, err := scanTask(s.DB.QueryRowContext(ctx, taskSelect+`
		WHERE organization_id = ? AND project_id = ? AND workspace_id = ?
		  AND discarded_at IS NULL ORDER BY created_at DESC LIMIT 1`,
		run.OrganizationID, run.ProjectID, workspace.ID))
	if err != nil {
		return err
	}
	created := 0
	for _, finding := range run.Findings {
		if (finding.Status != "verified" && finding.Status != "conflicting") ||
			len(finding.ProposedValue) == 0 || !json.Valid(finding.ProposedValue) {
			continue
		}
		var proposal ArtifactProposal
		switch finding.Target.Artifact {
		case "brief":
			draft, loadErr := scanBriefDraft(s.DB.QueryRowContext(ctx, briefDraftSelect+`
				WHERE organization_id = ? AND project_id = ? AND brief_id = ?
				ORDER BY created_at DESC LIMIT 1`,
				run.OrganizationID, run.ProjectID, task.BriefID))
			if loadErr != nil {
				return loadErr
			}
			if draft.Status != "open" {
				continue
			}
			baseHash, hashErr := briefDraftProposalContentHash(draft)
			if hashErr != nil {
				return hashErr
			}
			proposal, err = s.newResearchProposal(run, workspace, task, finding, "brief_draft", draft.ID, draft.Version, baseHash)
		case "strategy":
			if task.CurrentStrategyID == "" {
				continue
			}
			draft, loadErr := scanDraft(s.DB.QueryRowContext(ctx, draftSelect+`
				WHERE organization_id = ? AND project_id = ? AND id = ?`,
				run.OrganizationID, run.ProjectID, task.CurrentStrategyID))
			if loadErr != nil {
				return loadErr
			}
			if draft.ArchivedAt != nil || draft.CurrentRevision < 1 ||
				(draft.Status != "draft" && draft.Status != "ready_for_review" && draft.Status != "returned" && draft.Status != "approved") {
				continue
			}
			revision, loadErr := scanDraftRevision(s.DB.QueryRowContext(ctx, draftRevisionSelect+`
				WHERE organization_id = ? AND project_id = ? AND strategy_id = ? AND revision = ?`,
				run.OrganizationID, run.ProjectID, draft.ID, draft.CurrentRevision))
			if loadErr != nil {
				return loadErr
			}
			proposal, err = s.newResearchProposal(run, workspace, task, finding, "strategy_revision", draft.ID, draft.Version, revision.ContentHash)
		default:
			continue
		}
		if err != nil {
			return err
		}
		if _, insertErr := s.insertResearchProposal(ctx, proposal); insertErr != nil {
			return insertErr
		}
		created++
	}
	if created == 0 {
		return fmt.Errorf("research produced no proposal applicable to the current artifact versions")
	}
	return nil
}

func (s Service) newResearchProposal(
	run knowledge.ResearchRun,
	workspace Workspace,
	task Task,
	finding knowledge.ResearchFinding,
	targetType, targetID string,
	targetVersion int64,
	baseHash contract.ContentHash,
) (ArtifactProposal, error) {
	risk := "medium"
	if finding.Status == "conflicting" {
		risk = "high"
	}
	now := s.now()
	fingerprint, err := contract.NewContentHash(struct {
		RunID         string `json:"run_id"`
		FindingID     string `json:"finding_id"`
		TargetType    string `json:"target_type"`
		TargetID      string `json:"target_id"`
		TargetVersion int64  `json:"target_version"`
	}{run.ID, finding.ID, targetType, targetID, targetVersion})
	if err != nil {
		return ArtifactProposal{}, err
	}
	return ArtifactProposal{
		ContractVersion: ResearchAdoptionProposalContractV1,
		OrganizationID:  run.OrganizationID, ProjectID: run.ProjectID,
		WorkspaceID: workspace.ID, ConversationID: task.ConversationID,
		ProposalKind: "research", TargetType: targetType, TargetID: targetID,
		TargetVersion: targetVersion, BaseContentHash: baseHash,
		Operations: []BriefPatchOperation{{
			Op: "set", FieldPath: finding.Target.FieldPath,
			Value:      append(json.RawMessage(nil), finding.ProposedValue...),
			Source:     FieldSource{Type: "research_finding", ID: finding.ID},
			Confidence: finding.Confidence, Confirmation: "proposed",
		}},
		Rationale: finding.Implication, Risk: risk, Status: "proposed",
		FindingIDs: []string{finding.ID}, SourceResearchRunID: run.ID,
		ProposalFingerprint: string(fingerprint), CreatedBy: run.ID,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s Service) insertResearchProposal(ctx context.Context, proposal ArtifactProposal) (ArtifactProposal, error) {
	var err error
	proposal.ID, err = s.newID("proposal")
	if err != nil {
		return ArtifactProposal{}, err
	}
	operations, err := snapshotJSON(proposal.Operations)
	if err != nil {
		return ArtifactProposal{}, err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO strategy_artifact_proposals
		(id, organization_id, project_id, workspace_id, conversation_id, proposal_kind,
		 target_type, target_id, target_version, base_content_hash, operations, rationale,
		 risk, status, finding_ids, source_research_run_id, proposal_fingerprint,
		 created_by, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'research', ?, ?, ?, ?, ?, ?, ?, 'proposed', ?, ?, ?, ?, 1, ?, ?)
		ON DUPLICATE KEY UPDATE id = id`,
		proposal.ID, proposal.OrganizationID, proposal.ProjectID, proposal.WorkspaceID,
		proposal.ConversationID, proposal.TargetType, proposal.TargetID, proposal.TargetVersion,
		proposal.BaseContentHash, operations, proposal.Rationale, proposal.Risk,
		mustJSON(proposal.FindingIDs), proposal.SourceResearchRunID, proposal.ProposalFingerprint,
		proposal.CreatedBy, proposal.CreatedAt, proposal.UpdatedAt)
	if err != nil {
		return ArtifactProposal{}, err
	}
	return scanArtifactProposal(s.DB.QueryRowContext(ctx, artifactProposalSelect+`
		WHERE organization_id = ? AND project_id = ? AND proposal_fingerprint = ?`,
		proposal.OrganizationID, proposal.ProjectID, proposal.ProposalFingerprint))
}

func (s Service) ListResearchAdoptionProposals(
	ctx context.Context,
	actor contract.ActorContext,
	workspaceID, runID string,
) ([]ArtifactProposal, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	workspace, err := s.GetWorkspace(ctx, actor, workspaceID)
	if err != nil {
		return nil, err
	}
	query := artifactProposalSelect + ` WHERE organization_id = ? AND project_id = ?
		AND workspace_id = ? AND proposal_kind = 'research'`
	args := []any{actor.OrganizationID, workspace.ProjectID, workspaceID}
	if runID = strings.TrimSpace(runID); runID != "" {
		query += ` AND source_research_run_id = ?`
		args = append(args, runID)
	}
	query += ` ORDER BY created_at DESC LIMIT 100`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ArtifactProposal{}
	for rows.Next() {
		value, scanErr := scanArtifactProposal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

type ApplyResearchProposalResult struct {
	Proposal ArtifactProposal `json:"proposal"`
	Brief    *BriefDraft      `json:"brief_draft,omitempty"`
	Strategy *Draft           `json:"strategy_draft,omitempty"`
}

func (s Service) ApplyResearchAdoptionProposal(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	proposalID string,
	request ApplyArtifactProposalRequest,
) (ApplyResearchProposalResult, bool, error) {
	known, err := scanArtifactProposal(s.DB.QueryRowContext(ctx, artifactProposalSelect+`
		WHERE organization_id = ? AND id = ?`, actor.OrganizationID, proposalID))
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if known.ProposalKind != "research" {
		return ApplyResearchProposalResult{}, false, ErrInvalidRequest
	}
	if known.TargetType == "brief_draft" {
		value, duplicate, applyErr := s.ApplyArtifactProposal(ctx, actor, key, proposalID, request)
		if applyErr != nil {
			return ApplyResearchProposalResult{}, duplicate, applyErr
		}
		return ApplyResearchProposalResult{Proposal: value.Proposal, Brief: &value.Draft}, duplicate, nil
	}
	return s.applyResearchStrategyProposal(ctx, actor, key, known, request)
}

func (s Service) applyResearchStrategyProposal(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	known ArtifactProposal,
	request ApplyArtifactProposalRequest,
) (ApplyResearchProposalResult, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if err := key.Validate(); err != nil || request.ExpectedVersion < 1 {
		return ApplyResearchProposalResult{}, false, ErrInvalidRequest
	}
	if _, err := s.project(ctx, actor, known.ProjectID); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	hash, err := contract.CanonicalJSONHash(struct {
		ProposalID string                       `json:"proposal_id"`
		Request    ApplyArtifactProposalRequest `json:"request"`
	}{known.ID, request})
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	var prior ApplyResearchProposalResult
	found, err := s.loadReceipt(ctx, actor, known.ProjectID, "research_proposal.apply", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	defer tx.Rollback()
	proposal, err := scanArtifactProposal(tx.QueryRowContext(ctx, artifactProposalSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, known.ProjectID, known.ID))
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if proposal.ProposalKind != "research" || proposal.TargetType != "strategy_revision" ||
		proposal.Status != "proposed" || proposal.Version != request.ExpectedVersion {
		return ApplyResearchProposalResult{}, false, ErrVersionConflict
	}
	draft, err := scanDraft(tx.QueryRowContext(ctx, draftSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, proposal.ProjectID, proposal.TargetID))
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	current, err := scanDraftRevision(tx.QueryRowContext(ctx, draftRevisionSelect+`
		WHERE organization_id = ? AND project_id = ? AND strategy_id = ? AND revision = ?`,
		actor.OrganizationID, proposal.ProjectID, draft.ID, draft.CurrentRevision))
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if draft.Version != proposal.TargetVersion || current.ContentHash != proposal.BaseContentHash ||
		draft.ArchivedAt != nil || !researchStrategyStatusEditable(draft.Status) {
		if err := markResearchProposalStale(ctx, tx, proposal, "目标策略已产生新修订或不可再编辑", s.now()); err != nil {
			return ApplyResearchProposalResult{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return ApplyResearchProposalResult{}, false, err
		}
		return ApplyResearchProposalResult{}, false, ErrVersionConflict
	}
	operations := proposal.Operations
	status := "applied"
	if len(request.Operations) > 0 {
		if err := validateEditedProposalOperations(proposal.Operations, request.Operations); err != nil {
			return ApplyResearchProposalResult{}, false, err
		}
		operations = request.Operations
		status = "edited"
	}
	if len(operations) != 1 {
		return ApplyResearchProposalResult{}, false, ErrInvalidRequest
	}
	document := current.Document
	if err := setStrategySection(&document, operations[0].FieldPath, operations[0].Value); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	brief, err := scanBriefVersion(tx.QueryRowContext(ctx, briefVersionSelect+`
		WHERE organization_id = ? AND project_id = ? AND brief_id = ? AND version = ?`,
		actor.OrganizationID, proposal.ProjectID, draft.BriefID, draft.BriefVersion))
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	now := s.now()
	compliance := evaluateCompliance(document, brief, now)
	document.Compliance = &compliance
	if err := document.Validate(); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	next := DraftRevision{
		StrategyID: draft.ID, Revision: current.Revision + 1, BaseRevision: &current.Revision,
		Document: document, ChangedSections: []string{operations[0].FieldPath},
		CreatedBy: actor.Principal.ID, CreatedAt: now,
	}
	next.ContentHash, err = contract.NewContentHash(next.Document)
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if err := insertDraftRevision(ctx, tx, actor.OrganizationID, proposal.ProjectID, next); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if err := s.insertComplianceReport(ctx, tx, actor.OrganizationID, proposal.ProjectID, draft.ID, next, compliance); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_reviews SET status = 'invalidated', updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND strategy_id = ? AND status = 'open'`,
		now, actor.OrganizationID, proposal.ProjectID, draft.ID); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	draftUpdate, err := tx.ExecContext(ctx, `UPDATE strategy_drafts
		SET current_revision = ?, status = 'draft', current_review_id = NULL,
			version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
		next.Revision, now, actor.OrganizationID, proposal.ProjectID, draft.ID, draft.Version)
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if err := requireResearchMutation(draftUpdate); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	storedOperations, _ := snapshotJSON(operations)
	proposalUpdate, err := tx.ExecContext(ctx, `UPDATE strategy_artifact_proposals
		SET operations = ?, status = ?, applied_by = ?, applied_at = ?, version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ? AND status = 'proposed'`,
		storedOperations, status, actor.Principal.ID, now, now, actor.OrganizationID,
		proposal.ProjectID, proposal.ID, proposal.Version)
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if err := requireResearchMutation(proposalUpdate); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if err := insertResearchFindingEvidenceReferences(ctx, tx, proposal, "strategy_revision", draft.ID, next.Revision, operations[0].FieldPath, actor.Principal.ID, now); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	proposal.Operations = operations
	proposal.Status = status
	proposal.AppliedBy = actor.Principal.ID
	proposal.AppliedAt = &now
	proposal.Version++
	proposal.UpdatedAt = now
	draft.CurrentRevision = next.Revision
	draft.Version++
	draft.Status = "draft"
	draft.CurrentReviewID = ""
	draft.UpdatedAt = now
	draft.Revision = &next
	response := ApplyResearchProposalResult{Proposal: proposal, Strategy: &draft}
	eventID, err := s.newID("stratevent")
	if err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	payload := mustJSON(map[string]any{
		"proposal_id": proposal.ID, "strategy_revision": next.Revision, "proposal_kind": "research",
	})
	if err := insertConversationEvent(ctx, tx, eventID, actor.OrganizationID, proposal.ProjectID,
		proposal.ConversationID, "research.proposal.applied", payload, now); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	if err := insertReceipt(ctx, tx, actor, proposal.ProjectID, "research_proposal.apply", key, hash, 200, response, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, proposal.ProjectID, "research_proposal.apply", key, hash, &prior)
			return prior, found, readErr
		}
		return ApplyResearchProposalResult{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return ApplyResearchProposalResult{}, false, err
	}
	return response, false, nil
}

func markResearchProposalStale(ctx context.Context, tx *sql.Tx, proposal ArtifactProposal, reason string, now time.Time) error {
	result, err := tx.ExecContext(ctx, `UPDATE strategy_artifact_proposals
		SET status = 'stale', stale_reason = ?, version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ? AND status = 'proposed'`,
		reason, now, proposal.OrganizationID, proposal.ProjectID, proposal.ID, proposal.Version)
	if err != nil {
		return err
	}
	return requireResearchMutation(result)
}

func researchStrategyStatusEditable(status string) bool {
	switch status {
	case "draft", "ready_for_review", "returned", "approved":
		return true
	default:
		return false
	}
}

func requireResearchMutation(result sql.Result) error {
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrVersionConflict
	}
	return nil
}

func insertResearchFindingEvidenceReferences(
	ctx context.Context,
	tx *sql.Tx,
	proposal ArtifactProposal,
	targetType, targetID string,
	targetVersion int64,
	fieldPath, createdBy string,
	now time.Time,
) error {
	for _, findingID := range proposal.FindingIDs {
		var contentHash string
		err := tx.QueryRowContext(ctx, `SELECT content_hash FROM platform_research_findings
			WHERE organization_id = ? AND project_id = ? AND research_run_id = ? AND id = ?`,
			proposal.OrganizationID, proposal.ProjectID, proposal.SourceResearchRunID, findingID).Scan(&contentHash)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO strategy_evidence_references
			(organization_id, project_id, evidence_type, evidence_id, target_type, target_id,
			 target_version, field_path, content_hash, created_by, created_at)
			VALUES (?, ?, 'research_finding', ?, ?, ?, ?, ?, ?, ?, ?)`,
			proposal.OrganizationID, proposal.ProjectID, findingID, targetType, targetID,
			targetVersion, fieldPath, contentHash, createdBy, now); err != nil {
			return err
		}
	}
	return nil
}

type RemapResearchProposalRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}

func (s Service) RemapResearchAdoptionProposal(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	proposalID string,
	request RemapResearchProposalRequest,
) (ArtifactProposal, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return ArtifactProposal{}, false, err
	}
	if err := key.Validate(); err != nil || request.ExpectedVersion < 1 {
		return ArtifactProposal{}, false, ErrInvalidRequest
	}
	old, err := scanArtifactProposal(s.DB.QueryRowContext(ctx, artifactProposalSelect+`
		WHERE organization_id = ? AND id = ?`, actor.OrganizationID, proposalID))
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	if old.ProposalKind != "research" || old.Status != "stale" || old.Version != request.ExpectedVersion {
		return ArtifactProposal{}, false, ErrVersionConflict
	}
	if _, err := s.project(ctx, actor, old.ProjectID); err != nil {
		return ArtifactProposal{}, false, err
	}
	hash, err := contract.CanonicalJSONHash(struct {
		ProposalID string `json:"proposal_id"`
		Version    int64  `json:"version"`
	}{proposalID, request.ExpectedVersion})
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	var prior ArtifactProposal
	found, err := s.loadReceipt(ctx, actor, old.ProjectID, "research_proposal.remap", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	remapped := old
	remapped.ID, err = s.newID("proposal")
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	remapped.Status = "proposed"
	remapped.StaleReason = ""
	remapped.SupersedesProposalID = old.ID
	remapped.Version = 1
	remapped.AppliedBy, remapped.IgnoredBy = "", ""
	remapped.AppliedAt, remapped.IgnoredAt = nil, nil
	remapped.CreatedBy = actor.Principal.ID
	remapped.CreatedAt, remapped.UpdatedAt = s.now(), s.now()
	switch old.TargetType {
	case "brief_draft":
		draft, loadErr := s.GetBriefDraftByID(ctx, actor, old.TargetID)
		if loadErr != nil || draft.Status != "open" {
			return ArtifactProposal{}, false, ErrInvalidState
		}
		remapped.TargetVersion = draft.Version
		remapped.BaseContentHash, err = briefDraftProposalContentHash(draft)
	case "strategy_revision":
		draft, loadErr := s.GetDraft(ctx, actor, old.TargetID)
		if loadErr != nil || draft.Revision == nil || draft.ArchivedAt != nil || !researchStrategyStatusEditable(draft.Status) {
			return ArtifactProposal{}, false, ErrInvalidState
		}
		remapped.TargetVersion = draft.Version
		remapped.BaseContentHash = draft.Revision.ContentHash
	default:
		return ArtifactProposal{}, false, ErrInvalidRequest
	}
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	remapped.ProposalFingerprint = ""
	operations, err := snapshotJSON(remapped.Operations)
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ArtifactProposal{}, false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_artifact_proposals
		(id, organization_id, project_id, workspace_id, conversation_id, proposal_kind,
		 target_type, target_id, target_version, base_content_hash, operations, rationale,
		 risk, status, finding_ids, source_research_run_id, stale_reason,
		 supersedes_proposal_id, created_by, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'research', ?, ?, ?, ?, ?, ?, ?, 'proposed', ?, ?, '', ?, ?, 1, ?, ?)`,
		remapped.ID, remapped.OrganizationID, remapped.ProjectID, remapped.WorkspaceID,
		remapped.ConversationID, remapped.TargetType, remapped.TargetID, remapped.TargetVersion,
		remapped.BaseContentHash, operations, remapped.Rationale, remapped.Risk,
		mustJSON(remapped.FindingIDs), remapped.SourceResearchRunID, old.ID,
		remapped.CreatedBy, remapped.CreatedAt, remapped.UpdatedAt); err != nil {
		return ArtifactProposal{}, false, err
	}
	if err := insertReceipt(ctx, tx, actor, old.ProjectID, "research_proposal.remap", key, hash, 201, remapped, remapped.CreatedAt); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, old.ProjectID, "research_proposal.remap", key, hash, &prior)
			return prior, found, readErr
		}
		return ArtifactProposal{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return ArtifactProposal{}, false, err
	}
	return remapped, false, nil
}

func (s Service) GetBriefDraftByID(ctx context.Context, actor contract.ActorContext, draftID string) (BriefDraft, error) {
	var projectID contract.ProjectID
	if err := s.DB.QueryRowContext(ctx, `SELECT project_id FROM strategy_brief_drafts
		WHERE organization_id = ? AND id = ?`, actor.OrganizationID, draftID).Scan(&projectID); err != nil {
		return BriefDraft{}, mapNotFound(err)
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return BriefDraft{}, err
	}
	return scanBriefDraft(s.DB.QueryRowContext(ctx, briefDraftSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ?`, actor.OrganizationID, projectID, draftID))
}

var _ knowledge.ResearchCompletionSink = Service{}
