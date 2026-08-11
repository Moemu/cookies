package strategy

import (
	"context"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type rejectingMessageReferenceValidator struct {
	called bool
}

func (v *rejectingMessageReferenceValidator) ValidateMessageReferences(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, _ []MessageContentBlock) error {
	v.called = true
	return ErrInvalidRequest
}

func TestMessageReferenceValidationUsesInjectedGate(t *testing.T) {
	t.Parallel()
	validator := &rejectingMessageReferenceValidator{}
	service := Service{MessageReferences: validator}
	actor := contract.ActorContext{OrganizationID: "org_1"}
	err := service.validateMessageReferences(
		context.Background(), actor, "project_1",
		[]MessageContentBlock{{Type: "asset_ref", AssetKind: "image", AssetID: "asset_1", AssetVersion: 1}},
	)
	if err == nil || !validator.called {
		t.Fatalf("reference gate was not enforced: called=%v err=%v", validator.called, err)
	}
}

func TestMessageReferenceStatusPolicies(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"parse_queued", "parsing", "ready"} {
		if !messageDocumentStatusAllowed(status) {
			t.Fatalf("document status %q should be allowed", status)
		}
	}
	for _, status := range []string{"parse_failed", "deleted", ""} {
		if messageDocumentStatusAllowed(status) {
			t.Fatalf("document status %q should be rejected", status)
		}
	}
	for _, status := range []string{"processing", "ready"} {
		if !messageAssetStatusAllowed(status) {
			t.Fatalf("asset status %q should be allowed", status)
		}
	}
	for _, status := range []string{"failed", "quarantined", "archived", ""} {
		if messageAssetStatusAllowed(status) {
			t.Fatalf("asset status %q should be rejected", status)
		}
	}
}
