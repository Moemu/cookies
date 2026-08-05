package strategy

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// MessageReferenceValidator proves that every immutable attachment belongs to
// the current tenant/project and has not drifted since the composer selected
// it. It deliberately runs before the message, events, or AgentTask are
// persisted.
type MessageReferenceValidator interface {
	ValidateMessageReferences(context.Context, contract.ActorContext, contract.ProjectID, []MessageContentBlock) error
}

func (s Service) validateMessageReferences(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, blocks []MessageContentBlock) error {
	validator := s.MessageReferences
	if validator == nil {
		validator = MySQLMessageReferenceValidator{DB: s.DB}
	}
	return validator.ValidateMessageReferences(ctx, actor, projectID, blocks)
}

type MySQLMessageReferenceValidator struct {
	DB *sql.DB
}

func (v MySQLMessageReferenceValidator) ValidateMessageReferences(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, blocks []MessageContentBlock) error {
	hasReference := false
	for _, block := range blocks {
		if block.Type != "text" {
			hasReference = true
			break
		}
	}
	if !hasReference {
		return nil
	}
	if v.DB == nil {
		return ErrInvalidRequest
	}
	for _, block := range blocks {
		switch block.Type {
		case "text":
			continue
		case "document_ref":
			var contentSHA256, status string
			err := v.DB.QueryRowContext(ctx, `SELECT content_sha256, status
				FROM platform_knowledge_documents
				WHERE organization_id = ? AND project_id = ? AND id = ?`,
				actor.OrganizationID, projectID, block.DocumentID,
			).Scan(&contentSHA256, &status)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalidRequest
			}
			if err != nil {
				return err
			}
			if contentSHA256 != block.ExpectedContentSHA256 || !messageDocumentStatusAllowed(status) {
				return ErrInvalidRequest
			}
		case "asset_ref":
			var assetKind, assetStatus, versionStatus, projectAssetStatus string
			err := v.DB.QueryRowContext(ctx, `SELECT a.asset_kind, a.status, av.status, pa.status
				FROM project_assets pa
				JOIN assets a ON a.organization_id = pa.organization_id AND a.id = pa.asset_id
				JOIN asset_versions av ON av.organization_id = pa.organization_id
					AND av.asset_id = pa.asset_id AND av.version = pa.asset_version
				WHERE pa.organization_id = ? AND pa.project_id = ?
					AND pa.asset_id = ? AND pa.asset_version = ?`,
				actor.OrganizationID, projectID, block.AssetID, block.AssetVersion,
			).Scan(&assetKind, &assetStatus, &versionStatus, &projectAssetStatus)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalidRequest
			}
			if err != nil {
				return err
			}
			if assetKind != block.AssetKind || projectAssetStatus != "active" || !messageAssetStatusAllowed(assetStatus) || !messageAssetStatusAllowed(versionStatus) {
				return ErrInvalidRequest
			}
		case "research_ref":
			var contentHash, runStatus string
			err := v.DB.QueryRowContext(ctx, `SELECT artifact.content_hash, run.status
				FROM platform_research_artifacts artifact
				JOIN platform_research_runs run
					ON run.organization_id = artifact.organization_id
					AND run.project_id = artifact.project_id
					AND run.id = artifact.research_run_id
				WHERE artifact.organization_id = ? AND artifact.project_id = ? AND artifact.id = ?`,
				actor.OrganizationID, projectID, block.ResearchArtifactID,
			).Scan(&contentHash, &runStatus)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalidRequest
			}
			if err != nil {
				return err
			}
			if contentHash != block.ExpectedContentHash || runStatus != "succeeded" {
				return ErrInvalidRequest
			}
		default:
			return ErrInvalidRequest
		}
	}
	return nil
}

func messageDocumentStatusAllowed(status string) bool {
	return status == "parse_queued" || status == "parsing" || status == "ready"
}

func messageAssetStatusAllowed(status string) bool {
	return status == "processing" || status == "ready"
}
