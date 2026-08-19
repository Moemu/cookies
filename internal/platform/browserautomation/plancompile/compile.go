package plancompile

import (
	"fmt"
	"slices"

	"github.com/shikanon/cookies/internal/platform/browserautomation"
	"github.com/shikanon/cookies/internal/platform/browserautomation/rparunner"
	"github.com/shikanon/cookies/internal/systems/delivery/calibrationmanifest"
)

// Field keys of the frozen OceanEngine calibration manifest used by the
// compiler. Locators never originate in the compiler itself.
const (
	dailyBudgetEditFieldKey = "promotion.daily_budget_edit"

	budgetListPageKind = "promotion_list"

	accountQueryKey = "aadvid"
	objectQueryKey  = "promotion_id"
)

// Compiler turns an authorized BrowserRpaRun into the deterministic
// Playwright plan executed by rparunner. All page locators come from the
// frozen calibration manifest; the site policy contributes only allowlists.
type Compiler struct {
	Manifest calibrationmanifest.Manifest
}

var _ rparunner.PlanCompiler = Compiler{}

func (c Compiler) CompilePrepare(run browserautomation.BrowserRpaRun, policy browserautomation.SitePolicy) (rparunner.RpaPlan, error) {
	field, err := c.requireBudgetEditField(run)
	if err != nil {
		return rparunner.RpaPlan{}, err
	}
	if err := requirePolicy(policy); err != nil {
		return rparunner.RpaPlan{}, err
	}
	return rparunner.RpaPlan{
		SchemaVersion:     rparunner.PlanSchemaV2,
		Browser:           "msedge",
		Mode:              "prepare",
		AccountID:         run.AccountID,
		AllowRemoteWrite:  false,
		RunID:             run.ID,
		AllowedProtocols:  policy.AllowedProtocols,
		AllowedHosts:      policy.AllowedHosts,
		ExpectedObjectID:  run.Authority.TargetPlatformObjectID,
		ObjectIDQueryKey:  objectQueryKey,
		AccountIDQueryKey: accountQueryKey,
		Steps: []rparunner.RpaStep{
			identifyStep(),
			budgetReadbackStep(field),
		},
	}, nil
}

func (c Compiler) CompileSubmit(run browserautomation.BrowserRpaRun, attempt browserautomation.ControlledActionAttempt, policy browserautomation.SitePolicy) (rparunner.RpaPlan, error) {
	field, err := c.requireBudgetEditField(run)
	if err != nil {
		return rparunner.RpaPlan{}, err
	}
	if err := requirePolicy(policy); err != nil {
		return rparunner.RpaPlan{}, err
	}
	mutation := run.Authority.PromotionMutation
	if mutation == nil {
		return rparunner.RpaPlan{}, fmt.Errorf("budget mutation binding is required for submit")
	}
	targetYuan, err := inputUnits(field, mutation.TargetDailyBudgetMinor)
	if err != nil {
		return rparunner.RpaPlan{}, err
	}
	boundary, err := finalWriteBoundary(field)
	if err != nil {
		return rparunner.RpaPlan{}, err
	}
	return rparunner.RpaPlan{
		SchemaVersion:     rparunner.PlanSchemaV2,
		Browser:           "msedge",
		Mode:              "submit",
		AccountID:         run.AccountID,
		AllowRemoteWrite:  true,
		RunID:             run.ID,
		AllowedProtocols:  policy.AllowedProtocols,
		AllowedHosts:      policy.AllowedHosts,
		ExpectedObjectID:  run.Authority.TargetPlatformObjectID,
		ObjectIDQueryKey:  objectQueryKey,
		AccountIDQueryKey: accountQueryKey,
		Steps: []rparunner.RpaStep{
			identifyStep(),
			budgetReadbackStep(field),
			{
				ID:          "fill_target_daily_budget",
				Kind:        "fill_money",
				PageKind:    budgetListPageKind,
				ScopeChecks: []rparunner.LocatorSpec{locator(field.PlaywrightRPA.Scope)},
				Fields: []rparunner.RpaField{{
					Key:     "daily_budget_yuan",
					Value:   targetYuan,
					Locator: locator(field.PlaywrightRPA.Target),
				}},
			},
			{
				ID:          "final_click_confirm_budget_change",
				Kind:        "final_click",
				PageKind:    budgetListPageKind,
				Locator:     &boundary,
				RemoteWrite: true,
			},
		},
	}, nil
}

// requireBudgetEditField enforces the minimal-link scope: only the calibrated
// update_promotion_budget action has an executable Playwright path today.
func (c Compiler) requireBudgetEditField(run browserautomation.BrowserRpaRun) (calibrationmanifest.Field, error) {
	if run.Authority.Action != "update_promotion_budget" {
		return calibrationmanifest.Field{}, fmt.Errorf("action %q has no calibrated browser-rpa execution path", run.Authority.Action)
	}
	for _, field := range c.Manifest.Fields {
		if field.Key == dailyBudgetEditFieldKey {
			if field.PlaywrightRPA.Target.Value == "" {
				return calibrationmanifest.Field{}, fmt.Errorf("calibration field %q carries no playwright_rpa target", field.Key)
			}
			return field, nil
		}
	}
	return calibrationmanifest.Field{}, fmt.Errorf("calibration manifest is missing field %q", dailyBudgetEditFieldKey)
}

func requirePolicy(policy browserautomation.SitePolicy) error {
	if !slices.Contains(policy.AllowedPageKinds, budgetListPageKind) {
		return fmt.Errorf("site policy does not allow page kind %q", budgetListPageKind)
	}
	if len(policy.AllowedProtocols) == 0 || len(policy.AllowedHosts) == 0 {
		return fmt.Errorf("site policy must bind protocols and hosts")
	}
	return nil
}

func identifyStep() rparunner.RpaStep {
	return rparunner.RpaStep{
		ID:       "identify_account_and_object",
		Kind:     "identify_page",
		PageKind: budgetListPageKind,
	}
}

func budgetReadbackStep(field calibrationmanifest.Field) rparunner.RpaStep {
	step := rparunner.RpaStep{
		ID:          "readback_promotion_daily_budget",
		Kind:        "readback",
		PageKind:    budgetListPageKind,
		ScopeChecks: []rparunner.LocatorSpec{locator(field.PlaywrightRPA.Scope)},
		Fields: []rparunner.RpaField{{
			Key:     "daily_budget_yuan",
			Locator: locator(field.PlaywrightRPA.Target),
		}},
	}
	if field.PlaywrightRPA.Readback.Value != "" {
		step.PresenceChecks = []rparunner.LocatorSpec{locator(field.PlaywrightRPA.Readback)}
	}
	return step
}

func locator(value calibrationmanifest.Locator) rparunner.LocatorSpec {
	return rparunner.LocatorSpec{Kind: value.Kind, Value: value.Value}
}

// inputUnits converts the authoritative minor-unit budget into the platform
// input unit declared by the frozen calibration (for the OceanEngine budget
// dialog: minor is CNY fen, input is CNY yuan).
func inputUnits(field calibrationmanifest.Field, minor int64) (int64, error) {
	perUnit, err := constraintInt(field, "minor_per_input_unit")
	if err != nil {
		return 0, err
	}
	if perUnit <= 0 {
		return 0, fmt.Errorf("calibration input constraint minor_per_input_unit must be positive")
	}
	if minor%perUnit != 0 {
		return 0, fmt.Errorf("budget %d minor is not representable in the calibrated input unit", minor)
	}
	return minor / perUnit, nil
}

func finalWriteBoundary(field calibrationmanifest.Field) (rparunner.LocatorSpec, error) {
	raw, ok := field.PlaywrightRPA.InputConstraints["final_write_boundary"].(string)
	if !ok || raw == "" {
		return rparunner.LocatorSpec{}, fmt.Errorf("calibration field %q declares no final_write_boundary", field.Key)
	}
	role, name, ok := cutRoleLocator(raw)
	if !ok {
		return rparunner.LocatorSpec{}, fmt.Errorf("final_write_boundary %q is not a role locator", raw)
	}
	return rparunner.LocatorSpec{Kind: "role_name", Value: role + ":" + name}, nil
}

func constraintInt(field calibrationmanifest.Field, key string) (int64, error) {
	value, ok := field.PlaywrightRPA.InputConstraints[key]
	if !ok {
		return 0, fmt.Errorf("calibration field %q is missing input constraint %q", field.Key, key)
	}
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int64:
		return typed, nil
	case float64:
		return int64(typed), nil
	default:
		return 0, fmt.Errorf("calibration input constraint %q is not numeric", key)
	}
}

func cutRoleLocator(value string) (role, name string, ok bool) {
	for i := 0; i < len(value); i++ {
		if value[i] == ':' {
			rolePart := value[:i]
			namePart := value[i+1:]
			if rolePart == "" || namePart == "" {
				return "", "", false
			}
			return rolePart, namePart, true
		}
	}
	return "", "", false
}
