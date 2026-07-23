// Package strategycreative contains the explicit composition adapter between
// two bounded contexts. It is intentionally not part of either domain package.
package strategycreative

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/creative"
	"github.com/shikanon/cookies/internal/systems/strategy"
)

// Reader converts only the approved Strategy package resource into Creative's
// input vocabulary. Strategy remains the authorization owner of the package;
// Creative retains ownership of the resulting Intake and all later work.
type Reader struct {
	Service strategy.Service
}

func (r Reader) ReadForCreative(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, reference creative.StrategyPackageReference) (creative.StrategyPackageSnapshot, error) {
	value, err := r.Service.GetPackage(ctx, actor, projectID, reference.PackageID, reference.PackageVersion)
	if err != nil {
		return creative.StrategyPackageSnapshot{}, err
	}
	if !strings.EqualFold(string(value.ContentHash), strings.TrimSpace(reference.ExpectedContentHash)) {
		return creative.StrategyPackageSnapshot{}, fmt.Errorf("strategy package content hash no longer matches the selected version")
	}
	document := value.Snapshot.Strategy
	concept := ""
	if len(document.CreativeRecommendations) > 0 {
		concept = document.CreativeRecommendations[0]
	}
	mandatory := append([]string{}, document.Constraints...)
	return creative.StrategyPackageSnapshot{
		PackageID: value.PackageID, PackageVersion: value.Version, ContentHash: string(value.ContentHash),
		CreativeReady: value.Snapshot.Readiness.CreativeReady,
		Objective:     document.Objective, Audience: document.Audience.Primary, CoreMessage: document.Proposition,
		Concept: concept, Tone: []string{"清晰", "可信"}, VisualKeywords: []string{"品牌主视觉", "真实使用场景"},
		Mandatory: mandatory, Prohibited: []string{},
	}, nil
}
