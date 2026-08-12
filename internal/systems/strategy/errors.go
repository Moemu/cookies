package strategy

import "errors"

var (
	ErrNotFound                = errors.New("strategy resource not found")
	ErrScopeRequired           = errors.New("strategy scope required")
	ErrProjectAccessDenied     = errors.New("strategy project access denied")
	ErrInvalidRequest          = errors.New("invalid strategy request")
	ErrRevisionScopeAmbiguous  = errors.New("strategy revision scope is ambiguous")
	ErrStrategyUpgradeRequired = errors.New("strategy draft must be upgraded to v3")
	ErrInvalidState            = errors.New("invalid strategy state")
	ErrVersionConflict         = errors.New("strategy version conflict")
	ErrIdempotencyConflict     = errors.New("strategy idempotency conflict")
	ErrBriefBlocked            = errors.New("brief is blocked")
	ErrStrategyPublishBlocked  = errors.New("strategy publish is blocked")
	ErrReviewStale             = errors.New("strategy review is stale")
	ErrReviewAssignment        = errors.New("strategy review assignment required")
	ErrConcurrencyLimit        = errors.New("strategy task concurrency limit")
	ErrEventCursorExpired      = errors.New("strategy event cursor expired")
	ErrFeatureDisabled         = errors.New("strategy feature disabled")
	ErrGenerationUnavailable   = errors.New("strategy generation provider unavailable")
	ErrCatalogChanged          = errors.New("strategy creative business catalog changed")
	ErrBusinessNotSelectable   = errors.New("strategy creative business is not selectable")
	ErrTaskPlanBlocked         = errors.New("strategy creative task plan is blocked")
	ErrReservedOutputField     = errors.New("strategy creative task output contains a reserved field")
	ErrProfileSkillMismatch    = errors.New("strategy creative business profile and skill mismatch")
)

type ValidationError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

type BlockedError struct {
	Problems []ValidationError
}

func (e BlockedError) Error() string { return ErrBriefBlocked.Error() }
func (e BlockedError) Unwrap() error { return ErrBriefBlocked }

type StrategyPublishBlockedError struct {
	Problems []ValidationError
}

func (e StrategyPublishBlockedError) Error() string { return ErrStrategyPublishBlocked.Error() }
func (e StrategyPublishBlockedError) Unwrap() error { return ErrStrategyPublishBlocked }

type TaskPlanBlockedError struct {
	Problems []ValidationError
}

func (e TaskPlanBlockedError) Error() string { return ErrTaskPlanBlocked.Error() }
func (e TaskPlanBlockedError) Unwrap() error { return ErrTaskPlanBlocked }
