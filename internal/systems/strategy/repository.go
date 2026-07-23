package strategy

import (
	"context"
	"errors"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var (
	ErrProposalNotFound = errors.New("strategy proposal not found")
	ErrStrategyNotFound = errors.New("strategy output not found")
	ErrNotApproved      = errors.New("strategy output is not approved")
)

// Store keeps Strategy's project-owned records. The platform only supplies
// authorization and project context; it does not own proposal business state.
type Store interface {
	CreateProposal(context.Context, Proposal) (Proposal, bool, error)
	GetProposal(context.Context, contract.OrganizationID, contract.ProjectID, string) (Proposal, error)
	CreateStrategy(context.Context, StrategyOutput) (StrategyOutput, error)
	GetStrategy(context.Context, contract.OrganizationID, contract.ProjectID, string) (StrategyOutput, error)
	GetStrategyByProposal(context.Context, contract.OrganizationID, contract.ProjectID, string) (StrategyOutput, error)
	ApproveStrategy(context.Context, contract.OrganizationID, contract.ProjectID, string) (StrategyOutput, error)
}
