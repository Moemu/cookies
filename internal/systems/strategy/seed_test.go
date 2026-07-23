package strategy

import (
	"context"
	"fmt"
	"testing"
)

func TestSeedPolarisFreshProposalIsIdempotent(t *testing.T) {
	t.Parallel()
	store := newMemoryStore()
	next := 0
	service := Service{Store: store, NewID: func(prefix string) (string, error) {
		next++
		return fmt.Sprintf("%s_%d", prefix, next), nil
	}}
	firstProposal, firstStrategy, created, err := SeedPolarisFresh(context.Background(), service, testActor(), testProject(), nil)
	if err != nil || !created || !firstStrategy.Approved() {
		t.Fatalf("first seed proposal=%#v strategy=%#v created=%v err=%v", firstProposal, firstStrategy, created, err)
	}
	secondProposal, secondStrategy, created, err := SeedPolarisFresh(context.Background(), service, testActor(), testProject(), nil)
	if err != nil || created || secondProposal.ID != firstProposal.ID || secondStrategy.ID != firstStrategy.ID {
		t.Fatalf("second seed proposal=%#v strategy=%#v created=%v err=%v", secondProposal, secondStrategy, created, err)
	}
}
