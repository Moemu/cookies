package knowledge

import (
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestTerminalKnowledgeJobMapping(t *testing.T) {
	tests := []struct {
		name       string
		mapState   func(terminalKnowledgeJob) (string, string, string)
		job        terminalKnowledgeJob
		wantStatus string
		wantCode   string
	}{
		{
			name: "cancelled document remains retryable", mapState: documentTerminalState,
			job:        terminalKnowledgeJob{Status: contract.JobCancelled},
			wantStatus: "parse_failed", wantCode: "DOCUMENT_PARSE_CANCELLED",
		},
		{
			name: "failed document preserves job diagnosis", mapState: documentTerminalState,
			job:        terminalKnowledgeJob{Status: contract.JobFailed, ErrorCode: "JOB_ATTEMPT_LIMIT_EXCEEDED"},
			wantStatus: "parse_failed", wantCode: "JOB_ATTEMPT_LIMIT_EXCEEDED",
		},
		{
			name: "cancelled research is explicit", mapState: researchTerminalState,
			job:        terminalKnowledgeJob{Status: contract.JobCancelled},
			wantStatus: "cancelled", wantCode: "RESEARCH_CANCELLED",
		},
		{
			name: "failed research has a stable fallback", mapState: researchTerminalState,
			job:        terminalKnowledgeJob{Status: contract.JobFailed},
			wantStatus: "failed", wantCode: "RESEARCH_JOB_FAILED",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, message := tt.mapState(tt.job)
			if status != tt.wantStatus || code != tt.wantCode || strings.TrimSpace(message) == "" {
				t.Fatalf("mapping=(%q,%q,%q)", status, code, message)
			}
		})
	}
}

func TestTerminalJobMessageIsBounded(t *testing.T) {
	message := terminalJobMessage(strings.Repeat("x", 2048), "fallback")
	if len(message) != 1024 {
		t.Fatalf("message length=%d", len(message))
	}
}
