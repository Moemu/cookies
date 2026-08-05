package strategy

import (
	"errors"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

func TestGenerationReadinessReasonCategorizesProviderFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "missing route",
			err:  errors.New("no enabled adapter gateway route for alias cookies.text.standard"),
			want: "MODEL_ROUTE_NOT_FOUND",
		},
		{
			name: "credential",
			err: provider.ExecutionError{JobError: contract.JobError{
				Code: "MODEL_AUTH_UNAVAILABLE",
			}},
			want: "PROVIDER_CREDENTIAL_UNAVAILABLE",
		},
		{
			name: "invalid route policy",
			err:  errors.New("unsupported response mode xml"),
			want: "MODEL_ROUTE_POLICY_INVALID",
		},
		{
			name: "unknown preflight failure",
			err:  errors.New("provider control plane unavailable"),
			want: "PROVIDER_PREFLIGHT_FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generationReadinessReason(tt.err); got != tt.want {
				t.Fatalf("generationReadinessReason() = %q, want %q", got, tt.want)
			}
		})
	}
}
