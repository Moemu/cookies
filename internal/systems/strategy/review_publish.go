package strategy

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/eventoutbox"
)

// publishReviewedStrategyInTx is the single publication seam for both formal
// approval and self confirmation. Callers must lock and validate the draft and
// review before entering this function. Keeping publication here prevents the
// two UX modes from drifting on package hashing, handoff freezing, events, or
// task completion.
func (s Service) publishReviewedStrategyInTx(
	ctx context.Context,
	tx *sql.Tx,
	actor contract.ActorContext,
	draft Draft,
	review Review,
	assignment ReviewAssignment,
	projectContext contract.ProjectContext,
) (PackageVersion, error) {
	revision, err := scanDraftRevision(tx.QueryRowContext(ctx, draftRevisionSelect+` WHERE organization_id = ?
		AND project_id = ? AND strategy_id = ? AND revision = ?`, actor.OrganizationID,
		draft.ProjectID, draft.ID, review.CandidateRevision))
	if err != nil {
		return PackageVersion{}, err
	}
	if !revision.ContentHash.Equal(review.CandidateContentHash) {
		return PackageVersion{}, ErrReviewStale
	}
	brief, err := scanBriefVersion(tx.QueryRowContext(ctx, briefVersionSelect+` WHERE organization_id = ?
		AND project_id = ? AND brief_id = ? AND version = ?`, actor.OrganizationID,
		draft.ProjectID, draft.BriefID, draft.BriefVersion))
	if err != nil {
		return PackageVersion{}, err
	}
	if problems := s.projectBriefCompatibilityProblems(ctx, actor, draft.ProjectID, brief.Snapshot); len(problems) > 0 {
		return PackageVersion{}, StrategyPublishBlockedError{Problems: problems}
	}
	var compliancePassed bool
	var complianceHash contract.ContentHash
	err = tx.QueryRowContext(ctx, `SELECT passed, candidate_content_hash
		FROM strategy_compliance_reports WHERE organization_id = ? AND project_id = ?
		AND strategy_id = ? AND strategy_revision = ?`,
		actor.OrganizationID, draft.ProjectID, draft.ID, revision.Revision).
		Scan(&compliancePassed, &complianceHash)
	if err == sql.ErrNoRows {
		compliance := evaluateCompliance(revision.Document, brief, s.now())
		compliancePassed = compliance.Passed
		complianceHash = revision.ContentHash
	} else if err != nil {
		return PackageVersion{}, err
	}
	if !complianceHash.Equal(revision.ContentHash) || !compliancePassed {
		return PackageVersion{}, StrategyPublishBlockedError{Problems: []ValidationError{{
			Field: "strategy.compliance", Reason: "策略合规检查存在阻断项或已过期",
		}}}
	}
	readiness := calculateReadiness(brief, revision.Document)
	if len(readiness.PublishBlockers) > 0 {
		return PackageVersion{}, StrategyPublishBlockedError{Problems: readiness.PublishBlockers}
	}
	packageID := ""
	latestVersion := int64(0)
	row := tx.QueryRowContext(ctx, `SELECT id, latest_version FROM strategy_packages
		WHERE organization_id = ? AND project_id = ? AND strategy_id = ? FOR UPDATE`,
		actor.OrganizationID, draft.ProjectID, draft.ID)
	err = row.Scan(&packageID, &latestVersion)
	if err == sql.ErrNoRows {
		packageID, err = s.newID("strategypackage")
		if err != nil {
			return PackageVersion{}, err
		}
	} else if err != nil {
		return PackageVersion{}, err
	}
	now := s.now()
	versionNumber := latestVersion + 1
	packageContractVersion := "strategy-package/v1"
	if revision.Document.ContractVersion == "strategy-draft/v2" {
		packageContractVersion = "strategy-package/v2"
	} else if revision.Document.ContractVersion == "strategy-draft/v3" {
		packageContractVersion = "strategy-package/v3"
	}
	snapshot := PackageSnapshot{
		ContractVersion: packageContractVersion, PackageID: packageID, PackageVersion: versionNumber,
		OrganizationID: actor.OrganizationID, ProjectID: draft.ProjectID, StrategyID: draft.ID,
		StrategyRevision: revision.Revision, Brief: brief, Strategy: revision.Document, Readiness: readiness,
		CreativeRoutes: creativeRoutesForPackage(brief, revision.Document),
		Approval:       PackageApproval{ReviewID: review.ID, ApprovedBy: actor.Principal.ID, ApprovedAt: now},
	}
	for _, route := range snapshot.CreativeRoutes {
		if err := route.Validate(); err != nil {
			return PackageVersion{}, err
		}
	}
	contentHash, err := PackageContentHash(snapshot)
	if err != nil {
		return PackageVersion{}, err
	}
	snapshot.Approval.ContentHash = contentHash
	if err := VerifyPackageContentHash(snapshot); err != nil {
		return PackageVersion{}, err
	}
	packageVersion := PackageVersion{
		PackageID: packageID, Version: versionNumber, OrganizationID: actor.OrganizationID,
		ProjectID: draft.ProjectID, Snapshot: snapshot, ContentHash: contentHash,
		Status: "published", PublishedBy: actor.Principal.ID, PublishedAt: now,
	}
	packageVersion, handoff, err := packageVersionWithHandoffReadiness(packageVersion, projectContext.ProductIDs)
	if err != nil {
		return PackageVersion{}, err
	}
	snapshot = packageVersion.Snapshot
	contentHash = packageVersion.ContentHash
	if latestVersion == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_packages
			(id, organization_id, project_id, strategy_id, latest_version, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'published', ?, ?)`, packageID, actor.OrganizationID,
			draft.ProjectID, draft.ID, versionNumber, now, now); err != nil {
			return PackageVersion{}, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE strategy_package_versions SET status = 'superseded'
			WHERE organization_id = ? AND project_id = ? AND package_id = ? AND version = ?`,
			actor.OrganizationID, draft.ProjectID, packageID, latestVersion); err != nil {
			return PackageVersion{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE strategy_packages SET latest_version = ?,
			status = 'published', updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ?`,
			versionNumber, now, actor.OrganizationID, draft.ProjectID, packageID); err != nil {
			return PackageVersion{}, err
		}
	}
	snapshotJSONValue, _ := snapshotJSON(snapshot)
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_package_versions
		(package_id, version, organization_id, project_id, strategy_id, strategy_revision,
		 review_id, snapshot, content_hash, published_by, published_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'published')`, packageID, versionNumber,
		actor.OrganizationID, draft.ProjectID, draft.ID, revision.Revision, review.ID,
		snapshotJSONValue, contentHash, actor.Principal.ID, now); err != nil {
		return PackageVersion{}, err
	}
	handoffSnapshot, err := json.Marshal(handoff)
	if err != nil {
		return PackageVersion{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_creative_handoffs
		(organization_id, project_id, package_id, package_version, contract_version,
		 snapshot, content_hash, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, actor.OrganizationID, draft.ProjectID,
		packageID, versionNumber, handoff.ContractVersion, handoffSnapshot,
		handoff.HandoffContentHash, now); err != nil {
		return PackageVersion{}, err
	}
	reviewResult, err := tx.ExecContext(ctx, `UPDATE strategy_reviews SET status = 'approved',
		decided_by = ?, decided_at = ?, updated_at = ? WHERE organization_id = ? AND project_id = ?
		AND id = ? AND status = 'open'`, actor.Principal.ID, now, now, actor.OrganizationID,
		draft.ProjectID, review.ID)
	if err != nil {
		return PackageVersion{}, err
	}
	if changed, _ := reviewResult.RowsAffected(); changed != 1 {
		return PackageVersion{}, ErrReviewStale
	}
	if err := updateReviewAssignmentDecision(ctx, tx, assignment, "approved", "", now); err != nil {
		return PackageVersion{}, err
	}
	draftResult, err := tx.ExecContext(ctx, `UPDATE strategy_drafts SET status = 'approved',
		current_review_id = ?, version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
		review.ID, now, actor.OrganizationID, draft.ProjectID, draft.ID, draft.Version)
	if err != nil {
		return PackageVersion{}, err
	}
	if changed, _ := draftResult.RowsAffected(); changed != 1 {
		return PackageVersion{}, ErrVersionConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_tasks SET status = 'completed',
		version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ?`,
		now, actor.OrganizationID, draft.ProjectID, draft.TaskID); err != nil {
		return PackageVersion{}, err
	}
	eventID, err := s.newID("event")
	if err != nil {
		return PackageVersion{}, err
	}
	traceID := eventID
	if requestContext, ok := contract.RequestContextFrom(ctx); ok && requestContext.TraceID != "" {
		traceID = requestContext.TraceID
	}
	if latestVersion > 0 {
		supersededEventID, eventErr := s.newID("event")
		if eventErr != nil {
			return PackageVersion{}, eventErr
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
			return PackageVersion{}, err
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
		return PackageVersion{}, err
	}
	return packageVersion, nil
}
