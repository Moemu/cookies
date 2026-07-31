package strategy

import (
	"context"

	"github.com/shikanon/cookies/internal/platform/contract"
	strategyskills "github.com/shikanon/cookies/internal/systems/strategy/skills"
)

func (s Service) ListSkills(ctx context.Context, actor contract.ActorContext, includeInstructions bool) ([]strategyskills.Descriptor, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	registry, err := strategyskills.DefaultRegistry()
	if err != nil {
		return nil, err
	}
	return registry.List(includeInstructions), nil
}
