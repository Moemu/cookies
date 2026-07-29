package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	ReviewModeSelfConfirmation    = "self_confirmation"
	ReviewModeLeaderApproval      = "leader_approval"
	ReviewModeDesignatedApprovers = "designated_approvers"
)

type reviewQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func defaultReviewPolicy(actor contract.ActorContext, projectID contract.ProjectID) ReviewPolicy {
	return ReviewPolicy{
		OrganizationID:    actor.OrganizationID,
		ProjectID:         projectID,
		Mode:              ReviewModeSelfConfirmation,
		ApproverUserIDs:   []string{},
		AllowSelfApproval: true,
		Version:           0,
	}
}

func (s Service) GetReviewPolicy(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (ReviewPolicy, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return ReviewPolicy{}, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return ReviewPolicy{}, err
	}
	var value ReviewPolicy
	var approversJSON []byte
	err := s.DB.QueryRowContext(ctx, `SELECT organization_id, project_id, mode, approver_user_ids,
		allow_self_approval, version, updated_by, created_at, updated_at
		FROM strategy_review_policies WHERE organization_id = ? AND project_id = ?`,
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

func (s Service) UpdateReviewPolicy(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request UpdateReviewPolicyRequest) (ReviewPolicy, error) {
	if err := requireScope(actor, ScopeApprove); err != nil {
		return ReviewPolicy{}, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return ReviewPolicy{}, err
	}
	request.Mode = strings.TrimSpace(request.Mode)
	request.ApproverUserIDs = uniqueNonEmpty(request.ApproverUserIDs)
	switch request.Mode {
	case ReviewModeSelfConfirmation:
		request.ApproverUserIDs = []string{}
		request.AllowSelfApproval = true
	case ReviewModeLeaderApproval:
		if len(request.ApproverUserIDs) == 0 {
			rows, err := s.DB.QueryContext(ctx, `SELECT user_id FROM organization_memberships
				WHERE organization_id = ? AND status = 'active' AND role IN ('owner', 'admin')
				ORDER BY CASE role WHEN 'owner' THEN 0 ELSE 1 END, created_at`,
				actor.OrganizationID)
			if err != nil {
				return ReviewPolicy{}, err
			}
			defer rows.Close()
			for rows.Next() {
				var userID string
				if err := rows.Scan(&userID); err != nil {
					return ReviewPolicy{}, err
				}
				request.ApproverUserIDs = append(request.ApproverUserIDs, userID)
			}
			if err := rows.Err(); err != nil {
				return ReviewPolicy{}, err
			}
		}
		if len(request.ApproverUserIDs) == 0 {
			return ReviewPolicy{}, ErrInvalidRequest
		}
	case ReviewModeDesignatedApprovers:
		if len(request.ApproverUserIDs) == 0 {
			return ReviewPolicy{}, ErrInvalidRequest
		}
	default:
		return ReviewPolicy{}, ErrInvalidRequest
	}
	for _, userID := range request.ApproverUserIDs {
		var exists int
		err := s.DB.QueryRowContext(ctx, `SELECT 1 FROM organization_memberships
			WHERE organization_id = ? AND user_id = ? AND status = 'active'`,
			actor.OrganizationID, userID).Scan(&exists)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ReviewPolicy{}, ErrInvalidRequest
			}
			return ReviewPolicy{}, err
		}
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ReviewPolicy{}, err
	}
	defer tx.Rollback()
	var currentVersion int64
	var createdAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT version, created_at FROM strategy_review_policies
		WHERE organization_id = ? AND project_id = ? FOR UPDATE`,
		actor.OrganizationID, projectID).Scan(&currentVersion, &createdAt)
	now := s.now()
	nextVersion := int64(1)
	approversJSON, _ := json.Marshal(request.ApproverUserIDs)
	if errors.Is(err, sql.ErrNoRows) {
		if request.ExpectedVersion != 0 {
			return ReviewPolicy{}, ErrVersionConflict
		}
		createdAt = now
		_, err = tx.ExecContext(ctx, `INSERT INTO strategy_review_policies
			(organization_id, project_id, mode, approver_user_ids, allow_self_approval,
			 version, updated_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?)`,
			actor.OrganizationID, projectID, request.Mode, approversJSON,
			request.AllowSelfApproval, actor.Principal.ID, now, now)
	} else if err != nil {
		return ReviewPolicy{}, err
	} else {
		if request.ExpectedVersion != currentVersion {
			return ReviewPolicy{}, ErrVersionConflict
		}
		nextVersion = currentVersion + 1
		_, err = tx.ExecContext(ctx, `UPDATE strategy_review_policies SET mode = ?,
			approver_user_ids = ?, allow_self_approval = ?, version = ?, updated_by = ?,
			updated_at = ? WHERE organization_id = ? AND project_id = ? AND version = ?`,
			request.Mode, approversJSON, request.AllowSelfApproval, nextVersion,
			actor.Principal.ID, now, actor.OrganizationID, projectID, currentVersion)
	}
	if err != nil {
		return ReviewPolicy{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReviewPolicy{}, err
	}
	return ReviewPolicy{
		OrganizationID:    actor.OrganizationID,
		ProjectID:         projectID,
		Mode:              request.Mode,
		ApproverUserIDs:   request.ApproverUserIDs,
		AllowSelfApproval: request.AllowSelfApproval,
		Version:           nextVersion,
		UpdatedBy:         actor.Principal.ID,
		CreatedAt:         createdAt,
		UpdatedAt:         now,
	}, nil
}

func (s Service) createReviewAssignments(ctx context.Context, tx *sql.Tx, actor contract.ActorContext, review *Review, policy ReviewPolicy, now time.Time) error {
	reviewerIDs := append([]string(nil), policy.ApproverUserIDs...)
	if policy.Mode == ReviewModeSelfConfirmation {
		reviewerIDs = []string{review.CreatedBy}
	}
	policySnapshot, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	review.ReviewMode = policy.Mode
	review.RequiredApprovals = 1
	review.Assignments = make([]ReviewAssignment, 0, len(reviewerIDs))
	for _, reviewerID := range reviewerIDs {
		id, err := s.newID("reviewassignment")
		if err != nil {
			return err
		}
		assignment := ReviewAssignment{
			ID: id, OrganizationID: actor.OrganizationID, ProjectID: review.ProjectID,
			ReviewID: review.ID, ReviewerUserID: reviewerID, ReviewMode: policy.Mode,
			Status: "pending", CreatedAt: now, UpdatedAt: now,
			AllowSelfApproval: policy.AllowSelfApproval,
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_review_assignments
			(id, organization_id, project_id, review_id, reviewer_user_id, review_mode,
			 status, policy_snapshot, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?, ?)`,
			assignment.ID, assignment.OrganizationID, assignment.ProjectID, assignment.ReviewID,
			assignment.ReviewerUserID, assignment.ReviewMode, policySnapshot, now, now); err != nil {
			return err
		}
		review.Assignments = append(review.Assignments, assignment)
	}
	return nil
}

func (s Service) decorateReview(ctx context.Context, review Review) (Review, error) {
	assignments, err := listReviewAssignments(ctx, s.DB, review.OrganizationID, review.ProjectID, review.ID)
	if err != nil {
		return Review{}, err
	}
	review.Assignments = assignments
	review.RequiredApprovals = 1
	for _, assignment := range assignments {
		if review.ReviewMode == "" {
			review.ReviewMode = assignment.ReviewMode
		}
		if assignment.Status == "approved" {
			review.ApprovalCount++
		}
	}
	return review, nil
}

func listReviewAssignments(ctx context.Context, queryer reviewQueryer, organizationID contract.OrganizationID, projectID contract.ProjectID, reviewID string) ([]ReviewAssignment, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT id, organization_id, project_id, review_id,
		reviewer_user_id, review_mode, status, COALESCE(decision_reason, ''), decided_at,
		policy_snapshot, created_at, updated_at FROM strategy_review_assignments
		WHERE organization_id = ? AND project_id = ? AND review_id = ?
		ORDER BY created_at ASC`, organizationID, projectID, reviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ReviewAssignment{}
	for rows.Next() {
		var value ReviewAssignment
		var decidedAt sql.NullTime
		var policySnapshot []byte
		if err := rows.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.ReviewID,
			&value.ReviewerUserID, &value.ReviewMode, &value.Status, &value.DecisionReason,
			&decidedAt, &policySnapshot, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		var policy ReviewPolicy
		if err := json.Unmarshal(policySnapshot, &policy); err != nil {
			return nil, err
		}
		value.AllowSelfApproval = policy.AllowSelfApproval
		if decidedAt.Valid {
			value.DecidedAt = &decidedAt.Time
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s Service) authorizeReviewDecision(ctx context.Context, actor contract.ActorContext, review Review, approve bool) (ReviewAssignment, error) {
	assignments, err := listReviewAssignments(ctx, s.DB, review.OrganizationID, review.ProjectID, review.ID)
	if err != nil {
		return ReviewAssignment{}, err
	}
	if len(assignments) == 0 {
		if approve {
			if err := requireScope(actor, ScopeApprove); err != nil {
				return ReviewAssignment{}, err
			}
		} else if err := requireScope(actor, ScopeReview); err != nil {
			return ReviewAssignment{}, err
		}
		return ReviewAssignment{}, nil
	}
	for _, assignment := range assignments {
		if assignment.ReviewerUserID != actor.Principal.ID || assignment.Status != "pending" {
			continue
		}
		if assignment.ReviewMode != ReviewModeSelfConfirmation &&
			actor.Principal.ID == review.CreatedBy && !assignment.AllowSelfApproval {
			return ReviewAssignment{}, ErrReviewAssignment
		}
		if approve && assignment.ReviewMode == ReviewModeSelfConfirmation {
			if !actor.HasScope(ScopeConfirm) && !actor.HasScope(ScopeApprove) {
				return ReviewAssignment{}, ErrScopeRequired
			}
		} else if approve {
			if err := requireScope(actor, ScopeApprove); err != nil {
				return ReviewAssignment{}, err
			}
		} else if err := requireScope(actor, ScopeReview); err != nil {
			return ReviewAssignment{}, err
		}
		return assignment, nil
	}
	return ReviewAssignment{}, ErrReviewAssignment
}

func updateReviewAssignmentDecision(ctx context.Context, tx *sql.Tx, assignment ReviewAssignment, status, reason string, now time.Time) error {
	if assignment.ID == "" {
		return nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE strategy_review_assignments SET status = ?,
		decision_reason = ?, decided_at = ?, updated_at = ?
		WHERE id = ? AND status = 'pending'`, status, nullableReviewReason(reason), now, now, assignment.ID)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return ErrInvalidState
	}
	return nil
}

func (s Service) ListReviews(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, filter, status string) ([]Review, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	filter = strings.TrimSpace(filter)
	status = strings.TrimSpace(status)
	args := []any{actor.OrganizationID}
	query := reviewSelect + ` WHERE organization_id = ?`
	if projectID != "" {
		if _, err := s.project(ctx, actor, projectID); err != nil {
			return nil, err
		}
		query += ` AND project_id = ?`
		args = append(args, projectID)
	}
	switch filter {
	case "", "all":
	case "requested_by_me":
		query += ` AND created_by = ?`
		args = append(args, actor.Principal.ID)
	case "assigned_to_me":
		query += ` AND id IN (SELECT review_id FROM strategy_review_assignments
			WHERE organization_id = ? AND reviewer_user_id = ?)`
		args = append(args, actor.OrganizationID, actor.Principal.ID)
	default:
		return nil, ErrInvalidRequest
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC LIMIT 100`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []Review{}
	for rows.Next() {
		value, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		value, err = s.decorateReview(ctx, value)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func nullableReviewReason(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
