package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type ConfirmRequest struct {
	ExpectedVersion   int64 `json:"expected_version"`
	CandidateRevision int64 `json:"candidate_revision"`
}

// ConfirmStrategy performs the self-confirmation audit record and package
// publication in one transaction. It intentionally keeps an approved Review
// and Assignment for lineage, while removing the user-visible "submit to
// myself" intermediate state.
func (s Service) ConfirmStrategy(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	strategyID string,
	request ConfirmRequest,
) (PackageVersion, bool, error) {
	if s.DisableApproval {
		return PackageVersion{}, false, ErrFeatureDisabled
	}
	if !actor.HasScope(ScopeConfirm) && !actor.HasScope(ScopeApprove) {
		return PackageVersion{}, false, ErrScopeRequired
	}
	if err := key.Validate(); err != nil || request.ExpectedVersion < 1 || request.CandidateRevision < 1 {
		return PackageVersion{}, false, ErrInvalidRequest
	}
	draft, err := s.GetDraft(ctx, actor, strategyID)
	if err != nil {
		return PackageVersion{}, false, err
	}
	if draft.ArchivedAt != nil {
		return PackageVersion{}, false, ErrInvalidState
	}
	requestHash, _ := contract.CanonicalJSONHash(request)
	var prior PackageVersion
	found, err := s.loadReceipt(ctx, actor, draft.ProjectID, "strategy.confirm", key, requestHash, &prior)
	if found || err != nil {
		return prior, found, err
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return PackageVersion{}, false, err
	}
	defer tx.Rollback()
	locked, err := scanDraft(tx.QueryRowContext(ctx, draftSelect+` WHERE organization_id = ?
		AND project_id = ? AND id = ? FOR UPDATE`, actor.OrganizationID, draft.ProjectID, strategyID))
	if err != nil {
		return PackageVersion{}, false, err
	}
	if locked.Version != request.ExpectedVersion || locked.CurrentRevision != request.CandidateRevision {
		return PackageVersion{}, false, ErrVersionConflict
	}
	if locked.Status != "draft" && locked.Status != "returned" {
		tx.Rollback()
		found, readErr := s.loadReceipt(ctx, actor, draft.ProjectID, "strategy.confirm", key, requestHash, &prior)
		if found || readErr != nil {
			return prior, found, readErr
		}
		return PackageVersion{}, false, ErrInvalidState
	}
	// Re-read Project context after locking the Draft. A context snapshot read
	// before the lock could become stale while a concurrent Project update is
	// advancing its version, and must not be used to freeze Handoff product IDs.
	projectContext, err := s.project(ctx, actor, locked.ProjectID)
	if err != nil {
		return PackageVersion{}, false, err
	}
	if projectContext.ProjectContextVersion != locked.ProjectContextVersion {
		return PackageVersion{}, false, ErrReviewStale
	}
	policy, err := reviewPolicyForUpdate(ctx, tx, actor, locked.ProjectID)
	if err != nil {
		return PackageVersion{}, false, err
	}
	if policy.Mode != ReviewModeSelfConfirmation {
		return PackageVersion{}, false, ErrInvalidState
	}
	revision, err := scanDraftRevision(tx.QueryRowContext(ctx, draftRevisionSelect+` WHERE organization_id = ?
		AND project_id = ? AND strategy_id = ? AND revision = ?`, actor.OrganizationID,
		locked.ProjectID, locked.ID, request.CandidateRevision))
	if err != nil {
		return PackageVersion{}, false, err
	}
	reviewID, err := s.newID("strategyreview")
	if err != nil {
		return PackageVersion{}, false, err
	}
	now := s.now()
	review := Review{
		ID: reviewID, OrganizationID: actor.OrganizationID, ProjectID: locked.ProjectID,
		StrategyID: locked.ID, CandidateRevision: revision.Revision,
		CandidateContentHash: revision.ContentHash, BriefID: locked.BriefID,
		BriefVersion: locked.BriefVersion, ProjectContextVersion: locked.ProjectContextVersion,
		Status: "open", CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_reviews
		(id, organization_id, project_id, strategy_id, candidate_revision,
		 candidate_content_hash, brief_id, brief_version, project_context_version,
		 status, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'open', ?, ?, ?)`, review.ID, review.OrganizationID,
		review.ProjectID, review.StrategyID, review.CandidateRevision, review.CandidateContentHash,
		review.BriefID, review.BriefVersion, review.ProjectContextVersion, review.CreatedBy, now, now); err != nil {
		return PackageVersion{}, false, err
	}
	if err := s.createReviewAssignments(ctx, tx, actor, &review, policy, now); err != nil {
		return PackageVersion{}, false, err
	}
	if len(review.Assignments) != 1 || review.Assignments[0].ReviewerUserID != actor.Principal.ID {
		return PackageVersion{}, false, ErrInvalidState
	}
	locked.CurrentReviewID = review.ID
	packageVersion, err := s.publishReviewedStrategyInTx(
		ctx, tx, actor, locked, review, review.Assignments[0], projectContext,
	)
	if err != nil {
		return PackageVersion{}, false, err
	}
	if err := insertReceipt(
		ctx, tx, actor, locked.ProjectID, "strategy.confirm", key, requestHash,
		201, packageVersion, packageVersion.PublishedAt,
	); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, locked.ProjectID, "strategy.confirm", key, requestHash, &prior)
			return prior, found, readErr
		}
		return PackageVersion{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return PackageVersion{}, false, err
	}
	return packageVersion, false, nil
}

func reviewPolicyForUpdate(
	ctx context.Context,
	tx *sql.Tx,
	actor contract.ActorContext,
	projectID contract.ProjectID,
) (ReviewPolicy, error) {
	var value ReviewPolicy
	var approversJSON []byte
	err := tx.QueryRowContext(ctx, `SELECT organization_id, project_id, mode, approver_user_ids,
		allow_self_approval, version, updated_by, created_at, updated_at
		FROM strategy_review_policies WHERE organization_id = ? AND project_id = ? FOR UPDATE`,
		actor.OrganizationID, projectID).Scan(
		&value.OrganizationID, &value.ProjectID, &value.Mode, &approversJSON,
		&value.AllowSelfApproval, &value.Version, &value.UpdatedBy, &value.CreatedAt, &value.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultReviewPolicy(actor, projectID), nil
	}
	if err != nil {
		return ReviewPolicy{}, err
	}
	if err := json.Unmarshal(approversJSON, &value.ApproverUserIDs); err != nil {
		return ReviewPolicy{}, err
	}
	return value, nil
}
