package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const ProjectContextManifestContractV1 = "strategy-project-context-manifest/v1"

type VersionedContextRef struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

type SnapshotContextRef struct {
	Type        string               `json:"type"`
	ID          string               `json:"id"`
	Version     int64                `json:"version,omitempty"`
	ContentHash contract.ContentHash `json:"content_hash"`
}

type ProjectContextManifest struct {
	ContractVersion    string               `json:"contract_version"`
	WorkspaceRef       VersionedContextRef  `json:"workspace_ref"`
	ProjectContextRef  VersionedContextRef  `json:"project_context_ref"`
	Stage              string               `json:"stage"`
	BriefRef           *SnapshotContextRef  `json:"brief_ref"`
	StrategyRef        *SnapshotContextRef  `json:"strategy_ref"`
	SelectedSourceRefs []SnapshotContextRef `json:"selected_source_refs"`
	MemoryVersion      int64                `json:"memory_version"`
	GeneratedAt        time.Time            `json:"generated_at"`
}

func (m ProjectContextManifest) Validate() error {
	if m.ContractVersion != ProjectContextManifestContractV1 || !validStrategyStage(m.Stage) ||
		m.WorkspaceRef.Type != "strategy_workspace" || strings.TrimSpace(m.WorkspaceRef.ID) == "" || m.WorkspaceRef.Version < 1 ||
		m.ProjectContextRef.Type != "project_context" || strings.TrimSpace(m.ProjectContextRef.ID) == "" || m.ProjectContextRef.Version < 1 ||
		m.MemoryVersion < 0 || m.GeneratedAt.IsZero() || len(m.SelectedSourceRefs) > 32 {
		return ErrInvalidRequest
	}
	for _, ref := range append(append([]SnapshotContextRef{}, m.SelectedSourceRefs...), optionalSnapshotRefs(m.BriefRef, m.StrategyRef)...) {
		if strings.TrimSpace(ref.Type) == "" || strings.TrimSpace(ref.ID) == "" || ref.ContentHash.Validate() != nil {
			return ErrInvalidRequest
		}
	}
	encoded, err := json.Marshal(m)
	if err != nil || len(encoded) > 64<<10 {
		return ErrInvalidRequest
	}
	return nil
}

func optionalSnapshotRefs(refs ...*SnapshotContextRef) []SnapshotContextRef {
	result := make([]SnapshotContextRef, 0, len(refs))
	for _, ref := range refs {
		if ref != nil {
			result = append(result, *ref)
		}
	}
	return result
}

func contextSourcesWithout(refs []SnapshotContextRef, excludedIDs []string) []SnapshotContextRef {
	if len(refs) == 0 || len(excludedIDs) == 0 {
		return refs
	}
	excluded := make(map[string]struct{}, len(excludedIDs))
	for _, id := range excludedIDs {
		excluded[id] = struct{}{}
	}
	result := make([]SnapshotContextRef, 0, len(refs))
	for _, ref := range refs {
		if _, remove := excluded[ref.ID]; !remove {
			result = append(result, ref)
		}
	}
	return result
}

func validStrategyStage(stage string) bool {
	switch strings.TrimSpace(stage) {
	case "intake", "brief", "strategy", "review", "handoff":
		return true
	default:
		return false
	}
}

func (s Service) BuildProjectContextManifest(
	ctx context.Context,
	actor contract.ActorContext,
	workspaceID string,
	stage string,
) (ProjectContextManifest, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return ProjectContextManifest{}, err
	}
	if !validStrategyStage(stage) {
		return ProjectContextManifest{}, ErrInvalidRequest
	}
	detail, err := s.GetWorkspaceDetail(ctx, actor, workspaceID)
	if err != nil {
		return ProjectContextManifest{}, err
	}
	projectContext, err := s.Projects.GetContext(ctx, actor, detail.Workspace.ProjectID)
	if err != nil {
		return ProjectContextManifest{}, err
	}
	manifest := ProjectContextManifest{
		ContractVersion:   ProjectContextManifestContractV1,
		WorkspaceRef:      VersionedContextRef{Type: "strategy_workspace", ID: detail.Workspace.ID, Version: detail.Workspace.Version},
		ProjectContextRef: VersionedContextRef{Type: "project_context", ID: string(detail.Workspace.ProjectID), Version: projectContext.ProjectContextVersion},
		Stage:             stage, SelectedSourceRefs: []SnapshotContextRef{}, GeneratedAt: s.now(),
	}
	if detail.CurrentConversation != nil {
		err := s.DB.QueryRowContext(ctx, `SELECT version FROM strategy_conversation_memories
			WHERE organization_id = ? AND project_id = ? AND conversation_id = ?`,
			actor.OrganizationID, detail.Workspace.ProjectID, detail.CurrentConversation.ID).Scan(&manifest.MemoryVersion)
		if err != nil && err != sql.ErrNoRows {
			return ProjectContextManifest{}, err
		}
	}
	if detail.CurrentTask == nil {
		return manifest, manifest.Validate()
	}

	briefDraft, err := s.GetTaskBriefDraft(ctx, actor, detail.CurrentTask.ID)
	if err != nil {
		return ProjectContextManifest{}, err
	}
	var latestBriefVersion int64
	err = s.DB.QueryRowContext(ctx, `SELECT latest_version FROM strategy_briefs
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		actor.OrganizationID, detail.Workspace.ProjectID, detail.CurrentTask.BriefID).Scan(&latestBriefVersion)
	if err != nil {
		return ProjectContextManifest{}, err
	}
	if latestBriefVersion > 0 {
		brief, err := s.GetBriefVersion(ctx, actor, detail.CurrentTask.BriefID, latestBriefVersion)
		if err != nil {
			return ProjectContextManifest{}, err
		}
		manifest.BriefRef = &SnapshotContextRef{Type: "brief_version", ID: brief.BriefID, Version: brief.Version, ContentHash: brief.ContentHash}
	} else {
		hash, err := contract.NewContentHash(struct {
			Document    BriefDocument         `json:"document"`
			FieldStates map[string]FieldState `json:"field_states"`
		}{briefDraft.Document, briefDraft.FieldStates})
		if err != nil {
			return ProjectContextManifest{}, err
		}
		manifest.BriefRef = &SnapshotContextRef{Type: "brief_draft", ID: briefDraft.ID, Version: briefDraft.Version, ContentHash: hash}
	}

	if strategyID := strings.TrimSpace(detail.CurrentTask.CurrentStrategyID); strategyID != "" {
		draft, err := s.GetDraft(ctx, actor, strategyID)
		if err != nil {
			return ProjectContextManifest{}, err
		}
		if draft.CurrentRevision > 0 {
			revision, err := s.GetDraftRevision(ctx, actor, draft.ID, draft.CurrentRevision)
			if err != nil {
				return ProjectContextManifest{}, err
			}
			manifest.StrategyRef = &SnapshotContextRef{Type: "strategy_revision", ID: draft.ID, Version: revision.Revision, ContentHash: revision.ContentHash}
		}
	}

	manifest.SelectedSourceRefs, err = s.contextSourceRefs(ctx, actor, detail.Workspace.ProjectID, briefDraft.Document.ReferenceIDs)
	if err != nil {
		return ProjectContextManifest{}, err
	}
	return manifest, manifest.Validate()
}

func (s Service) contextSourceRefs(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	ids []string,
) ([]SnapshotContextRef, error) {
	if s.Knowledge == nil || len(ids) == 0 {
		return []SnapshotContextRef{}, nil
	}
	result := make([]SnapshotContextRef, 0, min(len(ids), 32))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		if len(result) == 32 {
			break
		}
		value, err := s.Knowledge.GetReference(ctx, actor, projectID, id)
		if err != nil {
			return nil, fmt.Errorf("resolve context source %q: %w", id, err)
		}
		hash, err := contextContentHash(value.ContentHash)
		if err != nil {
			return nil, fmt.Errorf("resolve context source hash %q: %w", id, err)
		}
		kind := "knowledge_document"
		if value.Kind == "research_artifact" {
			kind = "knowledge_research_artifact"
		}
		result = append(result, SnapshotContextRef{Type: kind, ID: value.ID, ContentHash: hash})
	}
	return result, nil
}

func contextContentHash(value string) (contract.ContentHash, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "sha256:") {
		value = "sha256:" + value
	}
	return contract.ParseContentHash(value)
}

func (s Service) workspaceIDForTask(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID string) (string, error) {
	var workspaceID string
	err := s.DB.QueryRowContext(ctx, `SELECT workspace_id FROM strategy_tasks
		WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, taskID).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return workspaceID, err
}
