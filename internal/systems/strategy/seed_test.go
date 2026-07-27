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
	if firstProposal.SourceObjectURI != DefaultVolcadProposalObjectURI || firstProposal.SourceType != "tos" {
		t.Fatalf("seed must persist remote proposal source: %#v", firstProposal)
	}
	if firstProposal.Input.ProposalPackage == nil || firstProposal.Input.ProposalPackage.CampaignName != "极地鲜生 618 新品推广" {
		t.Fatalf("seed must persist full VolcAd proposal package: %#v", firstProposal.Input)
	}
	if firstProposal.Input.ProposalPackage.Options.VideoRatio != "9:16" || firstProposal.Input.ProposalPackage.Options.VideoDuration != 6 {
		t.Fatalf("seed lost VolcAd media requirements: %#v", firstProposal.Input.ProposalPackage.Options)
	}
	secondProposal, secondStrategy, created, err := SeedPolarisFresh(context.Background(), service, testActor(), testProject(), nil)
	if err != nil || created || secondProposal.ID != firstProposal.ID || secondStrategy.ID != firstStrategy.ID {
		t.Fatalf("second seed proposal=%#v strategy=%#v created=%v err=%v", secondProposal, secondStrategy, created, err)
	}
}

func TestSeedPolarisFreshCompletesWorkflowAfterProposalOnlySeed(t *testing.T) {
	t.Parallel()
	store := newMemoryStore()
	next := 0
	service := Service{Store: store, NewID: func(prefix string) (string, error) {
		next++
		return fmt.Sprintf("%s_%d", prefix, next), nil
	}}
	proposalOnly, duplicate, err := SeedPolarisFreshProposal(context.Background(), service, testActor(), testProject())
	if err != nil || duplicate {
		t.Fatalf("proposal-only seed proposal=%#v duplicate=%v err=%v", proposalOnly, duplicate, err)
	}
	proposal, strategyOutput, created, err := SeedPolarisFresh(context.Background(), service, testActor(), testProject(), nil)
	if err != nil || !created || proposal.ID != proposalOnly.ID || !strategyOutput.Approved() {
		t.Fatalf("workflow seed proposal=%#v strategy=%#v created=%v err=%v", proposal, strategyOutput, created, err)
	}
	_, secondStrategy, created, err := SeedPolarisFresh(context.Background(), service, testActor(), testProject(), nil)
	if err != nil || created || secondStrategy.ID != strategyOutput.ID {
		t.Fatalf("second workflow seed strategy=%#v created=%v err=%v", secondStrategy, created, err)
	}
}
