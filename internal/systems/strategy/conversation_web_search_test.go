package strategy

import (
	"context"
	"errors"
	"testing"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/knowledge"
)

type conversationResearchFixture struct {
	run       knowledge.ResearchRun
	err       error
	messageID string
	query     string
	calls     int
}

func (fixture *conversationResearchFixture) RunConversationWebSearch(
	_ context.Context,
	_ contract.ActorContext,
	_ contract.ProjectID,
	messageID string,
	query string,
) (knowledge.ResearchRun, error) {
	fixture.calls++
	fixture.messageID = messageID
	fixture.query = query
	return fixture.run, fixture.err
}

func TestConversationWebSearchGroundsTheSameTurn(t *testing.T) {
	t.Parallel()
	fixture := &conversationResearchFixture{run: knowledge.ResearchRun{
		Status:    "completed",
		Artifacts: []knowledge.ResearchArtifact{{ID: "research_1", ContentHash: "abc123"}},
	}}
	service := Service{ConversationResearch: fixture}
	message := Message{
		ID: "message_1", RequestedPolicy: &MessageRequestedPolicy{WebSearch: "allowed"},
		ContentBlocks: []MessageContentBlock{
			{Type: "text", Text: " 最近有哪些行业变化？ "},
			{Type: "document_ref", DocumentID: "document_1", ExpectedContentSHA256: "hash"},
		},
	}
	prepared, err := service.prepareConversationWebSearch(context.Background(), agent.Task{
		OrganizationID: "org_1", ProjectID: "project_1", CreatedBy: contract.Principal{Kind: "user", ID: "user_1"},
	}, message)
	if err != nil {
		t.Fatalf("prepare search: %v", err)
	}
	if fixture.calls != 1 || fixture.messageID != message.ID || fixture.query != "最近有哪些行业变化？" {
		t.Fatalf("search invocation = calls:%d message:%q query:%q", fixture.calls, fixture.messageID, fixture.query)
	}
	if len(message.ContentBlocks) != 2 {
		t.Fatal("immutable source message was modified")
	}
	if len(prepared.ContentBlocks) != 3 || prepared.ContentBlocks[2].Type != "research_ref" ||
		prepared.ContentBlocks[2].ResearchArtifactID != "research_1" ||
		prepared.ContentBlocks[2].ExpectedContentHash != "abc123" {
		t.Fatalf("prepared blocks=%#v", prepared.ContentBlocks)
	}
}

func TestConversationWebSearchFailureStopsUngroundedAnswer(t *testing.T) {
	t.Parallel()
	fixture := &conversationResearchFixture{err: errors.New("provider unavailable")}
	service := Service{ConversationResearch: fixture}
	_, err := service.prepareConversationWebSearch(context.Background(), agent.Task{}, Message{
		ID: "message_1", RequestedPolicy: &MessageRequestedPolicy{WebSearch: "allowed"},
		ContentBlocks: []MessageContentBlock{{Type: "text", Text: "查一下最新变化"}},
	})
	var execution jobruntime.ExecutionError
	if !errors.As(err, &execution) || execution.JobError.Code != "CONVERSATION_WEB_SEARCH_FAILED" || !execution.JobError.Retryable {
		t.Fatalf("error=%T %v", err, err)
	}
}

func TestConversationWithoutWebSearchSkipsResearch(t *testing.T) {
	t.Parallel()
	fixture := &conversationResearchFixture{}
	service := Service{ConversationResearch: fixture}
	message := Message{ID: "message_1", ContentBlocks: []MessageContentBlock{{Type: "text", Text: "普通对话"}}}
	prepared, err := service.prepareConversationWebSearch(context.Background(), agent.Task{}, message)
	if err != nil || fixture.calls != 0 || len(prepared.ContentBlocks) != 1 {
		t.Fatalf("prepared=%#v calls=%d err=%v", prepared, fixture.calls, err)
	}
}
