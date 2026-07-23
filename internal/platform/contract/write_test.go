package contract

import "testing"

func TestIdempotencyKeyAndExpectedVersionValidation(t *testing.T) {
	t.Parallel()
	if err := IdempotencyKey("request-001").Validate(); err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
	if err := IdempotencyKey("contains spaces").Validate(); err == nil {
		t.Fatal("expected key with spaces to be rejected")
	}
	if err := ExpectedVersion(0).Validate(); err == nil {
		t.Fatal("expected zero version to be rejected")
	}
}
