// Package oceanengineskill owns the versioned manual-path browser plan for
// Delivery. It deliberately exposes no submit/click-final-write capability.
package oceanengineskill

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/shikanon/cookies/internal/systems/delivery"
)

const (
	SkillID           = "oceanengine-ecommerce-manual"
	SkillVersionV1    = "2026-08-12"
	SelectorVersionV1 = "oceanengine-manual-selector/v1"
	ActionVersionV1   = "oceanengine-manual-action/v1"
)

var (
	ErrScopeNotConfirmed = errors.New("gate-one browser scope is not explicitly confirmed")
	ErrPageDrift         = errors.New("oceanengine page identity drift")
	ErrAccountMismatch   = errors.New("oceanengine account mismatch")
	ErrProjectNotAllowed = errors.New("oceanengine project is not allowlisted")
	ErrFieldReadback     = errors.New("oceanengine field readback mismatch")
)

type ScopeConfirmation struct {
	Confirmed                 bool
	AccountReferenceID        string
	AllowedPlatformProjectIDs []string
	BudgetLimitMinor          int64
	ForbiddenActions          []string
}

type PageIdentity struct{ URL, Host, PageKind, AccountReferenceID, PlatformProjectID string }

// BrowserDriver is intentionally write-incomplete. It can navigate, inspect,
// fill an unsubmitted form, and clear local draft state. It cannot submit,
// save, enable, pause, delete, or alter a remote object.
type BrowserDriver interface {
	Navigate(context.Context, string) (PageIdentity, error)
	Inspect(context.Context) (PageIdentity, error)
	FillLocalField(context.Context, string, any) error
	ReadLocalField(context.Context, string) (any, error)
	DiscardLocalDraft(context.Context) error
}

type FieldReadback struct {
	StepID, Key        string
	Approved, Observed any
	Matches            bool
}
type GateOneResult struct {
	Page                PageIdentity
	Readbacks           []FieldReadback
	DiffKeys            []string
	StoppedBeforeAction string
	SafeExit            bool
}

type Skill struct{}

func (Skill) PrepareUnsubmitted(ctx context.Context, driver BrowserDriver, workflow delivery.CompiledDeliveryWorkflow, scope ScopeConfirmation, entryURL string) (GateOneResult, error) {
	if !scope.Confirmed || strings.TrimSpace(scope.AccountReferenceID) == "" || len(scope.AllowedPlatformProjectIDs) == 0 || scope.BudgetLimitMinor <= 0 || !containsForbiddenSubmit(scope.ForbiddenActions) {
		return GateOneResult{}, ErrScopeNotConfirmed
	}
	if err := workflow.Validate(); err != nil {
		return GateOneResult{}, err
	}
	if workflow.AccountReference.ID != scope.AccountReferenceID {
		return GateOneResult{}, ErrAccountMismatch
	}
	page, err := driver.Navigate(ctx, entryURL)
	if err != nil {
		return GateOneResult{}, err
	}
	if err := validatePage(page, scope); err != nil {
		return GateOneResult{}, err
	}
	result := GateOneResult{Page: page, Readbacks: []FieldReadback{}, DiffKeys: []string{}, StoppedBeforeAction: "submit_platform_configuration"}
	for _, step := range workflow.Steps {
		if step.Risk == delivery.WorkflowRiskRemoteWrite {
			if !step.Blocked || step.BlockReason != "PHASE_C_REMOTE_WRITE_PROHIBITED" {
				return GateOneResult{}, ErrPageDrift
			}
			break
		}
		if step.Risk != delivery.WorkflowRiskPrepareLocalForm {
			continue
		}
		current, inspectErr := driver.Inspect(ctx)
		if inspectErr != nil {
			return result, inspectErr
		}
		if err := validatePage(current, scope); err != nil {
			return result, err
		}
		for _, field := range step.Fields {
			if err := driver.FillLocalField(ctx, field.Key, field.Value); err != nil {
				return result, err
			}
			observed, readErr := driver.ReadLocalField(ctx, field.Key)
			if readErr != nil {
				return result, readErr
			}
			matches := fmt.Sprint(observed) == fmt.Sprint(field.ExpectedReadback)
			result.Readbacks = append(result.Readbacks, FieldReadback{StepID: step.ID, Key: field.Key, Approved: field.ExpectedReadback, Observed: observed, Matches: matches})
			if !matches {
				result.DiffKeys = append(result.DiffKeys, step.ID+"."+field.Key)
			}
		}
	}
	if len(result.DiffKeys) > 0 {
		_ = driver.DiscardLocalDraft(ctx)
		return result, ErrFieldReadback
	}
	if err := driver.DiscardLocalDraft(ctx); err != nil {
		return result, err
	}
	result.SafeExit = true
	return result, nil
}

func validatePage(page PageIdentity, scope ScopeConfirmation) error {
	if page.Host != "ad.oceanengine.com" || page.PageKind == "" {
		return ErrPageDrift
	}
	if page.AccountReferenceID != scope.AccountReferenceID {
		return ErrAccountMismatch
	}
	if !slices.Contains(scope.AllowedPlatformProjectIDs, page.PlatformProjectID) {
		return ErrProjectNotAllowed
	}
	return nil
}
func containsForbiddenSubmit(values []string) bool {
	for _, required := range []string{"save", "create", "submit", "enable", "modify"} {
		if !slices.Contains(values, required) {
			return false
		}
	}
	return true
}
