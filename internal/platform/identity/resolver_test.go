package identity

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Cecillia803/cookies/internal/platform/contract"
)

func TestRejectingResolverFailsClosed(t *testing.T) {
	t.Parallel()
	_, err := (RejectingResolver{}).Authenticate(context.Background(), httptest.NewRequest("GET", "/", nil))
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate() error = %v, want ErrUnauthenticated", err)
	}
}

func TestStaticResolverRejectsInvalidActor(t *testing.T) {
	t.Parallel()
	_, err := NewStaticResolver(contract.ActorContext{})
	if err == nil {
		t.Fatal("expected invalid actor to be rejected")
	}
}
