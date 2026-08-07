package strategy

import (
	"context"
	"strings"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/knowledge"
)

// prepareConversationWebSearch turns the user's visible search policy into
// grounded input for the same AgentTask. It never mutates the immutable user
// message in storage; the durable ResearchRun keeps the message relationship,
// while these server-produced refs are used only for this answer generation.
func (s Service) prepareConversationWebSearch(ctx context.Context, task agent.Task, message Message) (Message, error) {
	if message.RequestedPolicy == nil || message.RequestedPolicy.WebSearch != "allowed" {
		return message, nil
	}
	if s.ConversationResearch == nil {
		return Message{}, conversationWebSearchError("conversation web search is unavailable")
	}
	queryParts := make([]string, 0, len(message.ContentBlocks))
	for _, block := range message.ContentBlocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			queryParts = append(queryParts, strings.TrimSpace(block.Text))
		}
	}
	query := strings.Join(queryParts, "\n")
	if query == "" {
		return Message{}, conversationWebSearchError("conversation web search requires a text query")
	}
	actor := contract.ActorContext{
		OrganizationID: task.OrganizationID,
		Principal:      task.CreatedBy,
		Scopes:         []contract.Scope{knowledge.ScopeRead, knowledge.ScopeWrite},
	}
	run, err := s.ConversationResearch.RunConversationWebSearch(
		ctx, actor, task.ProjectID, message.ID, query,
	)
	if err != nil || run.Status != "succeeded" || len(run.Artifacts) == 0 {
		return Message{}, conversationWebSearchError("conversation web search did not complete")
	}
	prepared := message
	prepared.ContentBlocks = append([]MessageContentBlock(nil), message.ContentBlocks...)
	for _, artifact := range run.Artifacts {
		if strings.TrimSpace(artifact.ID) == "" || strings.TrimSpace(artifact.ContentHash) == "" {
			return Message{}, conversationWebSearchError("conversation web search returned invalid evidence")
		}
		prepared.ContentBlocks = append(prepared.ContentBlocks, MessageContentBlock{
			Type: "research_ref", ResearchArtifactID: artifact.ID, ExpectedContentHash: artifact.ContentHash,
		})
	}
	return prepared, nil
}

func conversationWebSearchError(message string) error {
	return jobruntime.ExecutionError{JobError: contract.JobError{
		Code: "CONVERSATION_WEB_SEARCH_FAILED", Message: message, Retryable: true,
	}}
}
