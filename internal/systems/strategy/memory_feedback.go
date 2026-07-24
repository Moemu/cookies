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

type ConversationMemory struct {
	ConversationID string                  `json:"conversation_id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Summary        string                  `json:"summary"`
	OpenQuestions  []string                `json:"open_questions"`
	LastMessageID  string                  `json:"last_message_id"`
	Version        int64                   `json:"version"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

func upsertConversationMemory(ctx context.Context, tx *sql.Tx, conversation Conversation, draft BriefDraft, questions []string, lastMessageID string, now time.Time) error {
	summary := briefMemorySummary(draft.Document)
	_, err := tx.ExecContext(ctx, `INSERT INTO strategy_conversation_memories
		(conversation_id, organization_id, project_id, summary, open_questions, last_message_id, version, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?)
		ON DUPLICATE KEY UPDATE summary = VALUES(summary), open_questions = VALUES(open_questions),
		last_message_id = VALUES(last_message_id), version = version + 1, updated_at = VALUES(updated_at)`,
		conversation.ID, conversation.OrganizationID, conversation.ProjectID, summary,
		mustJSON(questions), lastMessageID, now)
	return err
}

func briefMemorySummary(document BriefDocument) string {
	parts := make([]string, 0, 8)
	add := func(label, value string) {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, label+"："+value)
		}
	}
	add("品牌", document.Brand.Name)
	add("产品", document.Product.Name)
	add("目标", document.Campaign.Objective)
	add("受众", document.Audience.Primary)
	add("主张", document.Proposition)
	if len(document.Channels) > 0 {
		add("平台", strings.Join(document.Channels, "、"))
	}
	add("预算", document.Budget.Total)
	add("周期", document.Schedule.Window)
	if len(parts) == 0 {
		return "尚未形成稳定需求信息"
	}
	return strings.Join(parts, "；")
}

func (s Service) GetConversationMemory(ctx context.Context, actor contract.ActorContext, conversationID string) (ConversationMemory, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return ConversationMemory{}, err
	}
	conversation, err := s.GetConversation(ctx, actor, conversationID)
	if err != nil {
		return ConversationMemory{}, err
	}
	var value ConversationMemory
	var questions []byte
	err = s.DB.QueryRowContext(ctx, `SELECT conversation_id, organization_id, project_id, summary,
		open_questions, last_message_id, version, updated_at
		FROM strategy_conversation_memories WHERE organization_id = ? AND project_id = ? AND conversation_id = ?`,
		actor.OrganizationID, conversation.ProjectID, conversationID).
		Scan(&value.ConversationID, &value.OrganizationID, &value.ProjectID, &value.Summary,
			&questions, &value.LastMessageID, &value.Version, &value.UpdatedAt)
	if err != nil {
		return ConversationMemory{}, err
	}
	if json.Unmarshal(questions, &value.OpenQuestions) != nil {
		return ConversationMemory{}, fmt.Errorf("stored conversation memory is invalid")
	}
	return value, nil
}

type Feedback struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	TargetType     string                  `json:"target_type"`
	TargetID       string                  `json:"target_id"`
	TargetVersion  int64                   `json:"target_version"`
	Rating         string                  `json:"rating"`
	Comment        string                  `json:"comment,omitempty"`
	CreatedBy      string                  `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
}

type CreateFeedbackRequest struct {
	TargetType    string `json:"target_type"`
	TargetID      string `json:"target_id"`
	TargetVersion int64  `json:"target_version"`
	Rating        string `json:"rating"`
	Comment       string `json:"comment"`
}

func (s Service) CreateFeedback(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, key contract.IdempotencyKey, request CreateFeedbackRequest) (Feedback, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return Feedback{}, false, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return Feedback{}, false, err
	}
	request.TargetType = strings.TrimSpace(request.TargetType)
	request.TargetID = strings.TrimSpace(request.TargetID)
	request.Rating = strings.TrimSpace(request.Rating)
	request.Comment = strings.TrimSpace(request.Comment)
	if key.Validate() != nil || (request.TargetType != "strategy_revision" && request.TargetType != "strategy_package") ||
		request.TargetID == "" || request.TargetVersion < 1 ||
		(request.Rating != "useful" && request.Rating != "partly_useful" && request.Rating != "not_useful") ||
		len(request.Comment) > 2000 {
		return Feedback{}, false, ErrInvalidRequest
	}
	if err := s.validateFeedbackTarget(ctx, actor, projectID, request); err != nil {
		return Feedback{}, false, err
	}
	hash, err := contract.CanonicalJSONHash(request)
	if err != nil {
		return Feedback{}, false, err
	}
	var prior Feedback
	err = scanFeedback(s.DB.QueryRowContext(ctx, feedbackSelect+` WHERE organization_id = ? AND project_id = ?
		AND created_by = ? AND idempotency_key = ?`, actor.OrganizationID, projectID, actor.Principal.ID, key), &prior)
	if err == nil {
		var priorHash string
		if scanErr := s.DB.QueryRowContext(ctx, `SELECT request_hash FROM strategy_feedback
			WHERE organization_id = ? AND project_id = ? AND created_by = ? AND idempotency_key = ?`,
			actor.OrganizationID, projectID, actor.Principal.ID, key).Scan(&priorHash); scanErr != nil {
			return Feedback{}, false, scanErr
		}
		if priorHash != hash {
			return Feedback{}, false, ErrIdempotencyConflict
		}
		return prior, true, nil
	}
	if err != sql.ErrNoRows {
		return Feedback{}, false, err
	}
	id, err := s.newID("strategyfeedback")
	if err != nil {
		return Feedback{}, false, err
	}
	value := Feedback{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		TargetType: request.TargetType, TargetID: request.TargetID, TargetVersion: request.TargetVersion,
		Rating: request.Rating, Comment: request.Comment, CreatedBy: actor.Principal.ID, CreatedAt: s.now(),
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO strategy_feedback
		(id, organization_id, project_id, target_type, target_id, target_version, rating,
		 comment, created_by, idempotency_key, request_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.TargetType, value.TargetID,
		value.TargetVersion, value.Rating, nullableString(value.Comment), value.CreatedBy, key, hash, value.CreatedAt)
	if err != nil {
		return Feedback{}, false, err
	}
	return value, false, nil
}

func (s Service) validateFeedbackTarget(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateFeedbackRequest) error {
	var exists int
	var err error
	switch request.TargetType {
	case "strategy_revision":
		err = s.DB.QueryRowContext(ctx, `SELECT 1 FROM strategy_draft_revisions
			WHERE organization_id = ? AND project_id = ? AND strategy_id = ? AND revision = ?`,
			actor.OrganizationID, projectID, request.TargetID, request.TargetVersion).Scan(&exists)
	case "strategy_package":
		err = s.DB.QueryRowContext(ctx, `SELECT 1 FROM strategy_package_versions
			WHERE organization_id = ? AND project_id = ? AND package_id = ? AND version = ?`,
			actor.OrganizationID, projectID, request.TargetID, request.TargetVersion).Scan(&exists)
	default:
		return ErrInvalidRequest
	}
	if err == sql.ErrNoRows {
		return ErrInvalidRequest
	}
	return err
}

func (s Service) ListFeedback(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, targetType, targetID string, targetVersion int64) ([]Feedback, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, feedbackSelect+` WHERE organization_id = ? AND project_id = ?
		AND target_type = ? AND target_id = ? AND target_version = ? ORDER BY created_at DESC`,
		actor.OrganizationID, projectID, targetType, targetID, targetVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []Feedback{}
	for rows.Next() {
		var value Feedback
		if err := scanFeedback(rows, &value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

const feedbackSelect = `SELECT id, organization_id, project_id, target_type, target_id,
	target_version, rating, comment, created_by, created_at FROM strategy_feedback`

type feedbackScanner interface {
	Scan(...any) error
}

func scanFeedback(row feedbackScanner, value *Feedback) error {
	var comment sql.NullString
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.TargetType,
		&value.TargetID, &value.TargetVersion, &value.Rating, &comment, &value.CreatedBy, &value.CreatedAt)
	value.Comment = comment.String
	return err
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
