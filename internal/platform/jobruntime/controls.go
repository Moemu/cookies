package jobruntime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrJobNotFound = errors.New("job not found")
var ErrJobVersionConflict = errors.New("job version conflict")
var ErrJobNotCancellable = errors.New("job is not cancellable")

// Reader, ProgressReporter and Canceller are optional control-plane seams.
// They intentionally do not expand Store, keeping lightweight workers and
// existing test doubles compatible.
type Reader interface {
	Get(context.Context, contract.OrganizationID, contract.ProjectID, string) (contract.Job, error)
}

type ProgressReporter interface {
	UpdateProgress(context.Context, Claim, int, string, time.Time) error
}

type Canceller interface {
	RequestCancel(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, time.Time) (contract.Job, error)
	IsCancelRequested(context.Context, contract.OrganizationID, string) (bool, error)
}

type ClaimCanceller interface {
	IsCancelRequested(context.Context, contract.OrganizationID, string) (bool, error)
	CancelClaim(context.Context, Claim, time.Time) error
}

func validateProgress(progress int, message string) error {
	if progress < 0 || progress > 99 {
		return fmt.Errorf("running progress must be between 0 and 99")
	}
	if len(strings.TrimSpace(message)) > 512 {
		return fmt.Errorf("progress message must not exceed 512 characters")
	}
	return nil
}
