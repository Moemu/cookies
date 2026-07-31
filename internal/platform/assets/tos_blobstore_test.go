package assets

import (
	"strings"
	"testing"
)

func TestNewTOSBlobStoreRejectsIncompleteConfig(t *testing.T) {
	t.Parallel()

	_, err := NewTOSBlobStore(TOSConfig{Endpoint: "tos.example.com", Region: "cn-test", AccessKey: "test-access-key"})
	if err == nil {
		t.Fatal("expected incomplete TOS configuration to fail closed")
	}
	if !strings.Contains(err.Error(), "secret key") {
		t.Fatalf("unexpected TOS config error: %v", err)
	}
}
