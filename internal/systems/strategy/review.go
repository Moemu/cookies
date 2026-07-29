package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/eventoutbox"
)

func (s Service) SubmitStrategy(ctx context.Context, actor contract.ActorContext, key contract.IdempotencyKey, strategyID string, expectedVersion, candidateRevision int64) (Review, bool, error) {
	if err := requireScope(actor, ScopeReview); err != nil {
		return Review{}, false, err
	}
	if err := key.Validate(); err != nil || expectedVersion < 1 || candidateRevision < 1 {
		return Review{}, false, ErrInvalidRequest
	}
	draft, err := s.GetDraft(ctx, actor, strategyID)
	if err != nil {
		return Review{}, false, err
	}
	policy, err := s.GetReviewPolicy(ctx, actor, draft.ProjectID)
	if err != nil {
		return Review{}, false, err
	}
	request := struct {
		ExpectedVersion   int64 `json:"expected_version"`
		CandidateRevision int64 `json:"candidate_revision"`
	}{expectedVersion, candidateRevision}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior Review
	found, err := s.loadReceipt(ctx, actor, draft.ProjectID, "strategy.submit", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Review{}, false, err
	}
	defer tx.Rollback()
	locked, err := scanDraft(tx.QueryRowContext(ctx, draftSelect+` WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, draft.ProjectID, strategyID))
	if err != nil {
		return Review{}, false, err
	}
	if locked.Version != expectedVersion || locked.CurrentRevision != candidateRevision {
		return Review{}, false, ErrVersionConflict
	}
	if locked.Status != "draft" && locked.Status != "returned" {
		return Review{}, false, ErrInvalidState
	}
	revision, err := scanDraftRevision(tx.QueryRowContext(ctx, draftRevisionSelect+` WHERE organization_id = ?
		AND project_id = ? AND strategy_id = ? AND revision = ?`, actor.OrganizationID,
		draft.ProjectID, strategyID, candidateRevision))
	if err != nil {
		return Review{}, false, err
	}
	reviewID, err := s.newID("strategyreview")
	if err != nil {
		return Review{}, false, err
	}
	now := s.now()
	review := Review{
		ID: reviewID, OrganizationID: actor.OrganizationID, ProjectID: draft.ProjectID,
		StrategyID: strategyID, CandidateRevision: candidateRevision,
		CandidateContentHash: revision.ContentHash, BriefID: draft.BriefID,
		BriefVersion: draft.BriefVersion, ProjectContextVersion: draft.ProjectContextVersion,
		Status: "open", CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_reviews
		(id, organization_id, project_id, strategy_id, candidate_revision,
		 candidate_content_hash, brief_id, brief_version, project_context_version,
		 status, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, review.ID, review.OrganizationID,
		review.ProjectID, review.StrategyID, review.CandidateRevision, review.CandidateContentHash,
		review.BriefID, review.BriefVersion, review.ProjectContextVersion, review.Status,
		review.CreatedBy, now, now); err != nil {
		return Review{}, false, err
	}
	if err := s.createReviewAssignments(ctx, tx, actor, &review, policy, now); err != nil {
		return Review{}, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE strategy_drafts SET status = 'ready_for_review',
		current_review_id = ?, version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
		review.ID, now, actor.OrganizationID, draft.ProjectID, strategyID, expectedVersion)
	if err != nil {
		return Review{}, false, err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return Review{}, false, ErrVersionConflict
	}
	if err := insertReceipt(ctx, tx, actor, draft.ProjectID, "strategy.submit", key, hash, 201, review, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, draft.ProjectID, "strategy.submit", key, hash, &prior)
			return prior, found, readErr
		}
		return Review{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Review{}, false, err
	}
	return review, false, nil
}

func (s Service) GetReview(ctx context.Context, actor contract.ActorContext, reviewID string) (Review, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return Review{}, err
	}
	review, err := scanReview(s.DB.QueryRowContext(ctx, reviewSelect+` WHERE organization_id = ? AND id = ?`, actor.OrganizationID, reviewID))
	if err != nil {
		return Review{}, err
	}
	if _, err := s.project(ctx, actor, review.ProjectID); err != nil {
		return Review{}, err
	}
	return s.decorateReview(ctx, review)
}

func (s Service) AddReviewComment(ctx context.Context, actor contract.ActorContext, reviewID, body string) (ReviewComment, error) {
	if err := requireScope(actor, ScopeReview); err != nil {
		return ReviewComment{}, err
	}
	if strings.TrimSpace(body) == "" || len(body) > 16<<10 {
		return ReviewComment{}, ErrInvalidRequest
	}
	review, err := s.GetReview(ctx, actor, reviewID)
	if err != nil {
		return ReviewComment{}, err
	}
	if review.Status != "open" {
		return ReviewComment{}, ErrInvalidState
	}
	id, err := s.newID("reviewcomment")
	if err != nil {
		return ReviewComment{}, err
	}
	comment := ReviewComment{ID: id, OrganizationID: actor.OrganizationID, ProjectID: review.ProjectID, ReviewID: reviewID, AuthorID: actor.Principal.ID, Body: strings.TrimSpace(body), CreatedAt: s.now()}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO strategy_review_comments
		(id, organization_id, project_id, review_id, author_id, body, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, comment.ID, comment.OrganizationID, comment.ProjectID,
		comment.ReviewID, comment.AuthorID, comment.Body, comment.CreatedAt)
	return comment, err
}

func (s Service) ListReviewComments(ctx context.Context, actor contract.ActorContext, reviewID string) ([]ReviewComment, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	review, err := s.GetReview(ctx, actor, reviewID)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, organization_id, project_id, review_id,
		author_id, body, created_at FROM strategy_review_comments
		WHERE organization_id = ? AND project_id = ? AND review_id = ? ORDER BY created_at ASC`,
		actor.OrganizationID, review.ProjectID, reviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ReviewComment{}
	for rows.Next() {
		var value ReviewComment
		if err := rows.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.ReviewID,
			&value.AuthorID, &value.Body, &value.CreatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s Service) ReturnReview(ctx context.Context, actor contract.ActorContext, reviewID, reason string) (Review, error) {
	if strings.TrimSpace(reason) == "" || len(reason) > 16<<10 {
		return Review{}, ErrInvalidRequest
	}
	review, err := s.GetReview(ctx, actor, reviewID)
	if err != nil {
		return Review{}, err
	}
	assignment, err := s.authorizeReviewDecision(ctx, actor, review, false)
	if err != nil {
		return Review{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback()
	now := s.now()
	result, err := tx.ExecContext(ctx, `UPDATE strategy_reviews SET status = 'returned',
		decision_reason = ?, decided_by = ?, decided_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'open'`,
		strings.TrimSpace(reason), actor.Principal.ID, now, now, actor.OrganizationID,
		review.ProjectID, reviewID)
	if err != nil {
		return Review{}, err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return Review{}, ErrInvalidState
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_drafts SET status = 'returned',
		version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ?
		AND id = ? AND current_review_id = ?`, now, actor.OrganizationID, review.ProjectID,
		review.StrategyID, review.ID); err != nil {
		return Review{}, err
	}
	if err := updateReviewAssignmentDecision(ctx, tx, assignment, "returned", reason, now); err != nil {
		return Review{}, err
	}
	if err := tx.Commit(); err != nil {
		return Review{}, err
	}
	review.Status = "returned"
	review.DecisionReason = strings.TrimSpace(reason)
	review.DecidedBy = actor.Principal.ID
	review.DecidedAt = &now
	review.UpdatedAt = now
	if assignment.ID != "" {
		assignment.Status = "returned"
		assignment.DecisionReason = strings.TrimSpace(reason)
		assignment.DecidedAt = &now
		assignment.UpdatedAt = now
		review.Assignments = []ReviewAssignment{assignment}
	}
	return review, nil
}

type ApproveRequest struct {
	ReviewID             string               `json:"review_id"`
	CandidateContentHash contract.ContentHash `json:"candidate_content_hash"`
	ExpectedVersion      int64                `json:"expected_version"`
}

func (s Service) ApproveStrategy(ctx context.Context, actor contract.ActorContext, key contract.IdempotencyKey, strategyID string, request ApproveRequest) (PackageVersion, bool, error) {
	if s.DisableApproval {
		return PackageVersion{}, false, ErrFeatureDisabled
	}
	if err := key.Validate(); err != nil || request.ReviewID == "" || request.ExpectedVersion < 1 ||
		request.CandidateContentHash.Validate() != nil {
		return PackageVersion{}, false, ErrInvalidRequest
	}
	draft, err := s.GetDraft(ctx, actor, strategyID)
	if err != nil {
		return PackageVersion{}, false, err
	}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior PackageVersion
	found, err := s.loadReceipt(ctx, actor, draft.ProjectID, "strategy.approve", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	reviewForAuthorization, err := s.GetReview(ctx, actor, request.ReviewID)
	if err != nil {
		return PackageVersion{}, false, err
	}
	assignment, err := s.authorizeReviewDecision(ctx, actor, reviewForAuthorization, true)
	if err != nil {
		return PackageVersion{}, false, err
	}
	projectContext, err := s.project(ctx, actor, draft.ProjectID)
	if err != nil {
		return PackageVersion{}, false, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return PackageVersion{}, false, err
	}
	defer tx.Rollback()
	lockedDraft, err := scanDraft(tx.QueryRowContext(ctx, draftSelect+` WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, draft.ProjectID, strategyID))
	if err != nil {
		return PackageVersion{}, false, err
	}
	review, err := scanReview(tx.QueryRowContext(ctx, reviewSelect+` WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, draft.ProjectID, request.ReviewID))
	if err != nil {
		return PackageVersion{}, false, err
	}
	if lockedDraft.Version != request.ExpectedVersion {
		return PackageVersion{}, false, ErrVersionConflict
	}
	if lockedDraft.Status != "ready_for_review" || lockedDraft.CurrentReviewID != review.ID || review.Status != "open" ||
		review.StrategyID != strategyID || review.CandidateRevision != lockedDraft.CurrentRevision ||
		!review.CandidateContentHash.Equal(request.CandidateContentHash) {
		return PackageVersion{}, false, ErrReviewStale
	}
	if projectContext.ProjectContextVersion != lockedDraft.ProjectContextVersion {
		return PackageVersion{}, false, ErrReviewStale
	}
	revision, err := scanDraftRevision(tx.QueryRowContext(ctx, draftRevisionSelect+` WHERE organization_id = ?
		AND project_id = ? AND strategy_id = ? AND revision = ?`, actor.OrganizationID,
		draft.ProjectID, strategyID, review.CandidateRevision))
	if err != nil {
		return PackageVersion{}, false, err
	}
	if !revision.ContentHash.Equal(review.CandidateContentHash) {
		return PackageVersion{}, false, ErrReviewStale
	}
	brief, err := scanBriefVersion(tx.QueryRowContext(ctx, briefVersionSelect+` WHERE organization_id = ?
		AND project_id = ? AND brief_id = ? AND version = ?`, actor.OrganizationID,
		draft.ProjectID, draft.BriefID, draft.BriefVersion))
	if err != nil {
		return PackageVersion{}, false, err
	}
	var compliancePassed bool
	var complianceHash contract.ContentHash
	err = tx.QueryRowContext(ctx, `SELECT passed, candidate_content_hash
		FROM strategy_compliance_reports WHERE organization_id = ? AND project_id = ?
		AND strategy_id = ? AND strategy_revision = ?`,
		actor.OrganizationID, draft.ProjectID, strategyID, revision.Revision).
		Scan(&compliancePassed, &complianceHash)
	if err == sql.ErrNoRows {
		compliance := evaluateCompliance(revision.Document, brief, s.now())
		compliancePassed = compliance.Passed
		complianceHash = revision.ContentHash
	} else if err != nil {
		return PackageVersion{}, false, err
	}
	if !complianceHash.Equal(revision.ContentHash) || !compliancePassed {
		return PackageVersion{}, false, BlockedError{Problems: []ValidationError{{
			Field: "strategy.compliance", Reason: "策略合规检查存在阻断项或已过期",
		}}}
	}
	readiness := calculateReadiness(brief, revision.Document)
	if len(readiness.PublishBlockers) > 0 {
		return PackageVersion{}, false, BlockedError{Problems: readiness.PublishBlockers}
	}
	packageID := ""
	latestVersion := int64(0)
	row := tx.QueryRowContext(ctx, `SELECT id, latest_version FROM strategy_packages
		WHERE organization_id = ? AND project_id = ? AND strategy_id = ? FOR UPDATE`,
		actor.OrganizationID, draft.ProjectID, strategyID)
	err = row.Scan(&packageID, &latestVersion)
	if err == sql.ErrNoRows {
		packageID, err = s.newID("strategypackage")
		if err != nil {
			return PackageVersion{}, false, err
		}
	} else if err != nil {
		return PackageVersion{}, false, err
	}
	now := s.now()
	versionNumber := latestVersion + 1
	packageContractVersion := "strategy-package/v1"
	if revision.Document.ContractVersion == "strategy-draft/v2" {
		packageContractVersion = "strategy-package/v2"
	}
	snapshot := PackageSnapshot{
		ContractVersion: packageContractVersion, PackageID: packageID, PackageVersion: versionNumber,
		OrganizationID: actor.OrganizationID, ProjectID: draft.ProjectID, StrategyID: strategyID,
		StrategyRevision: revision.Revision, Brief: brief, Strategy: revision.Document, Readiness: readiness,
		CreativeRoutes: creativeRoutesForPackage(brief, revision.Document),
		Approval:       PackageApproval{ReviewID: review.ID, ApprovedBy: actor.Principal.ID, ApprovedAt: now},
	}
	for _, route := range snapshot.CreativeRoutes {
		if err := route.Validate(); err != nil {
			return PackageVersion{}, false, err
		}
	}
	contentHash, err := PackageContentHash(snapshot)
	if err != nil {
		return PackageVersion{}, false, err
	}
	snapshot.Approval.ContentHash = contentHash
	if err := VerifyPackageContentHash(snapshot); err != nil {
		return PackageVersion{}, false, err
	}
	packageVersion := PackageVersion{
		PackageID: packageID, Version: versionNumber, OrganizationID: actor.OrganizationID,
		ProjectID: draft.ProjectID, Snapshot: snapshot, ContentHash: contentHash,
		Status: "published", PublishedBy: actor.Principal.ID, PublishedAt: now,
	}
	if latestVersion == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_packages
			(id, organization_id, project_id, strategy_id, latest_version, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'published', ?, ?)`, packageID, actor.OrganizationID,
			draft.ProjectID, strategyID, versionNumber, now, now); err != nil {
			return PackageVersion{}, false, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE strategy_package_versions SET status = 'superseded'
			WHERE organization_id = ? AND project_id = ? AND package_id = ? AND version = ?`,
			actor.OrganizationID, draft.ProjectID, packageID, latestVersion); err != nil {
			return PackageVersion{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE strategy_packages SET latest_version = ?,
			status = 'published', updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ?`,
			versionNumber, now, actor.OrganizationID, draft.ProjectID, packageID); err != nil {
			return PackageVersion{}, false, err
		}
	}
	snapshotJSONValue, _ := snapshotJSON(snapshot)
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_package_versions
		(package_id, version, organization_id, project_id, strategy_id, strategy_revision,
		 review_id, snapshot, content_hash, published_by, published_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'published')`, packageID, versionNumber,
		actor.OrganizationID, draft.ProjectID, strategyID, revision.Revision, review.ID,
		snapshotJSONValue, contentHash, actor.Principal.ID, now); err != nil {
		return PackageVersion{}, false, err
	}
	handoff, err := BuildCreativeHandoff(packageVersion, projectContext.ProductIDs)
	if err != nil {
		return PackageVersion{}, false, err
	}
	handoffSnapshot, err := json.Marshal(handoff)
	if err != nil {
		return PackageVersion{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_creative_handoffs
		(organization_id, project_id, package_id, package_version, contract_version,
		 snapshot, content_hash, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, actor.OrganizationID, draft.ProjectID,
		packageID, versionNumber, handoff.ContractVersion, handoffSnapshot,
		handoff.HandoffContentHash, now); err != nil {
		return PackageVersion{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_reviews SET status = 'approved',
		decided_by = ?, decided_at = ?, updated_at = ? WHERE organization_id = ? AND project_id = ?
		AND id = ? AND status = 'open'`, actor.Principal.ID, now, now, actor.OrganizationID,
		draft.ProjectID, review.ID); err != nil {
		return PackageVersion{}, false, err
	}
	if err := updateReviewAssignmentDecision(ctx, tx, assignment, "approved", "", now); err != nil {
		return PackageVersion{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_drafts SET status = 'approved',
		version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ?
		AND id = ? AND version = ?`, now, actor.OrganizationID, draft.ProjectID, strategyID,
		lockedDraft.Version); err != nil {
		return PackageVersion{}, false, err
	}
	eventID, err := s.newID("event")
	if err != nil {
		return PackageVersion{}, false, err
	}
	traceID := eventID
	if requestContext, ok := contract.RequestContextFrom(ctx); ok && requestContext.TraceID != "" {
		traceID = requestContext.TraceID
	}
	if latestVersion > 0 {
		supersededEventID, eventErr := s.newID("event")
		if eventErr != nil {
			return PackageVersion{}, false, eventErr
		}
		supersededPayload := mustJSON(map[string]any{
			"event_id": supersededEventID, "event_type": "strategy.superseded.v1", "occurred_at": now,
			"producer": "strategy", "organization_id": actor.OrganizationID, "project_id": draft.ProjectID,
			"subject": map[string]any{"type": "strategy_package", "id": packageID, "version": latestVersion},
			"data": map[string]any{
				"superseded_version": latestVersion, "replacement_version": versionNumber,
			},
			"trace_id": traceID,
		})
		if err := (eventoutbox.MySQLStore{}).AppendIn(ctx, tx, eventoutbox.Event{
			ID: supersededEventID, OrganizationID: actor.OrganizationID, ProjectID: draft.ProjectID,
			Type: "strategy.superseded.v1", SubjectType: "strategy_package", SubjectID: packageID,
			SubjectVersion: latestVersion, Payload: supersededPayload, CreatedAt: now,
		}); err != nil {
			return PackageVersion{}, false, err
		}
	}
	eventPayload := mustJSON(map[string]any{
		"event_id": eventID, "event_type": "strategy.approved.v1", "occurred_at": now,
		"producer": "strategy", "organization_id": actor.OrganizationID, "project_id": draft.ProjectID,
		"subject":  map[string]any{"type": "strategy_package", "id": packageID, "version": versionNumber},
		"data":     map[string]any{"package_id": packageID, "package_version": versionNumber, "content_hash": contentHash},
		"trace_id": traceID,
	})
	if err := (eventoutbox.MySQLStore{}).AppendIn(ctx, tx, eventoutbox.Event{
		ID: eventID, OrganizationID: actor.OrganizationID, ProjectID: draft.ProjectID,
		Type: "strategy.approved.v1", SubjectType: "strategy_package", SubjectID: packageID,
		SubjectVersion: versionNumber, Payload: eventPayload, CreatedAt: now,
	}); err != nil {
		return PackageVersion{}, false, err
	}
	if err := insertReceipt(ctx, tx, actor, draft.ProjectID, "strategy.approve", key, hash, 201, packageVersion, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, draft.ProjectID, "strategy.approve", key, hash, &prior)
			return prior, found, readErr
		}
		return PackageVersion{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return PackageVersion{}, false, err
	}
	return packageVersion, false, nil
}

func calculateReadiness(brief BriefVersion, document StrategyDocument) Readiness {
	result := Readiness{PublishBlockers: []ValidationError{}}
	if err := document.Validate(); err != nil {
		result.PublishBlockers = append(result.PublishBlockers, ValidationError{Field: "strategy", Reason: err.Error()})
	}
	xiaohongshuReady := false
	for _, channel := range document.ChannelStrategy {
		if channel.Platform == "xiaohongshu" && len(channel.Formats) > 0 {
			xiaohongshuReady = true
			break
		}
	}
	videoRouteReady := len(creativeRoutesForPackage(brief, document)) > 0
	result.CreativeReady = len(document.CreativeRecommendations) > 0 && (xiaohongshuReady || videoRouteReady)
	result.DeliveryReady = strings.TrimSpace(brief.Snapshot.Budget.Total) != "" && strings.TrimSpace(brief.Snapshot.Schedule.Window) != ""
	result.InsightsReady = strings.TrimSpace(brief.Snapshot.Measurement.PrimaryKPI) != ""
	return result
}

func (s Service) ListPackages(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]PackageVersion, error) {
	if err := requireScope(actor, ScopePackageRead); err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, packageVersionSelect+` WHERE organization_id = ? AND project_id = ?
		ORDER BY published_at DESC`, actor.OrganizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PackageVersion
	for rows.Next() {
		value, err := scanPackageVersion(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if result == nil {
		result = []PackageVersion{}
	}
	return result, rows.Err()
}

func (s Service) GetPackage(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, packageID string, version int64) (PackageVersion, error) {
	if err := requireScope(actor, ScopePackageRead); err != nil {
		return PackageVersion{}, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return PackageVersion{}, err
	}
	value, err := scanPackageVersion(s.DB.QueryRowContext(ctx, packageVersionSelect+` WHERE organization_id = ?
		AND project_id = ? AND package_id = ? AND version = ?`, actor.OrganizationID,
		projectID, packageID, version))
	if err != nil {
		return PackageVersion{}, err
	}
	if value.PackageID != packageID || value.Version != version || value.ProjectID != projectID ||
		value.Snapshot.PackageID != packageID || value.Snapshot.PackageVersion != version ||
		value.Snapshot.ProjectID != projectID || !value.ContentHash.Equal(value.Snapshot.Approval.ContentHash) {
		return PackageVersion{}, fmt.Errorf("stored strategy package identity is inconsistent")
	}
	if err := VerifyPackageContentHash(value.Snapshot); err != nil {
		return PackageVersion{}, err
	}
	return value, nil
}

func ExportPackageMarkdown(value PackageVersion) string {
	doc := value.Snapshot.Strategy
	var builder strings.Builder
	fmt.Fprintf(&builder, "# 广告策略 v%d\n\n", value.Version)
	fmt.Fprintf(&builder, "## 目标\n\n%s\n\n", doc.Objective)
	fmt.Fprintf(&builder, "## 核心受众\n\n%s\n\n", doc.Audience.Primary)
	fmt.Fprintf(&builder, "## 核心主张\n\n%s\n\n", doc.Proposition)
	builder.WriteString("## 渠道策略\n\n")
	for _, channel := range doc.ChannelStrategy {
		fmt.Fprintf(&builder, "- %s：%s（%s）\n", channel.Platform, channel.Role, strings.Join(channel.Formats, "、"))
	}
	builder.WriteString("\n## 创意建议\n\n")
	for _, item := range doc.CreativeRecommendations {
		fmt.Fprintf(&builder, "- %s\n", item)
	}
	fmt.Fprintf(&builder, "\n## 版本证据\n\n- Package: `%s` v%d\n- Content hash: `%s`\n",
		value.PackageID, value.Version, value.ContentHash)
	return builder.String()
}
