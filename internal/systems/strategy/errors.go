package strategy

import "errors"

var (
	ErrNotFound               = errors.New("strategy resource not found")
	ErrScopeRequired          = errors.New("strategy scope required")
	ErrProjectAccessDenied    = errors.New("strategy project access denied")
	ErrInvalidRequest         = errors.New("invalid strategy request")
	ErrRevisionScopeAmbiguous = errors.New("strategy revision scope is ambiguous")
	ErrInvalidState           = errors.New("invalid strategy state")
	ErrVersionConflict        = errors.New("strategy version conflict")
	ErrIdempotencyConflict    = errors.New("strategy idempotency conflict")
	ErrBriefBlocked           = errors.New("brief is blocked")
	ErrReviewStale            = errors.New("strategy review is stale")
	ErrReviewAssignment       = errors.New("strategy review assignment required")
	ErrConcurrencyLimit       = errors.New("strategy task concurrency limit")
	ErrEventCursorExpired     = errors.New("strategy event cursor expired")
	ErrFeatureDisabled        = errors.New("strategy feature disabled")
	ErrGenerationUnavailable  = errors.New("strategy generation provider unavailable")
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
