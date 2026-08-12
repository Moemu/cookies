package oceanengineskill

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/systems/delivery"
)

type fakeBrowser struct {
	page      PageIdentity
	values    map[string]any
	driftKey  string
	discarded bool
}

func (b *fakeBrowser) Navigate(context.Context, string) (PageIdentity, error) { return b.page, nil }
func (b *fakeBrowser) Inspect(context.Context) (PageIdentity, error)          { return b.page, nil }
func (b *fakeBrowser) FillLocalField(_ context.Context, key string, value any) error {
	b.values[key] = value
	return nil
}
func (b *fakeBrowser) ReadLocalField(_ context.Context, key string) (any, error) {
	if key == b.driftKey {
		return "drift", nil
	}
	return b.values[key], nil
}
func (b *fakeBrowser) DiscardLocalDraft(context.Context) error { b.discarded = true; return nil }

func TestGateOneReadsEveryFieldAndSafelyDiscardsTheDraft(t *testing.T) {
	workflow := validWorkflow(t)
	browser := &fakeBrowser{page: PageIdentity{Host: "ad.oceanengine.com", PageKind: "project_create", AccountReferenceID: "account-6391", PlatformProjectID: "test-project-1"}, values: map[string]any{}}
	result, err := (Skill{}).PrepareUnsubmitted(context.Background(), browser, workflow, confirmedScope(), "https://ad.oceanengine.com/project/create")
	if err != nil {
		t.Fatal(err)
	}
	if !result.SafeExit || !browser.discarded || len(result.Readbacks) == 0 || result.StoppedBeforeAction != "submit_platform_configuration" {
		t.Fatalf("result=%#v browser=%#v", result, browser)
	}
}
func TestGateOneFailsClosedOnReadbackAccountAndScopeDrift(t *testing.T) {
	workflow := validWorkflow(t)
	for _, test := range []struct {
		name    string
		browser *fakeBrowser
		scope   ScopeConfirmation
		want    error
	}{{"readback", &fakeBrowser{page: validPage(), values: map[string]any{}, driftKey: "project_name"}, confirmedScope(), ErrFieldReadback}, {"account", &fakeBrowser{page: PageIdentity{Host: "ad.oceanengine.com", PageKind: "project_create", AccountReferenceID: "wrong", PlatformProjectID: "test-project-1"}, values: map[string]any{}}, confirmedScope(), ErrAccountMismatch}, {"project", &fakeBrowser{page: PageIdentity{Host: "ad.oceanengine.com", PageKind: "project_create", AccountReferenceID: "account-6391", PlatformProjectID: "other"}, values: map[string]any{}}, confirmedScope(), ErrProjectNotAllowed}, {"unconfirmed", &fakeBrowser{page: validPage(), values: map[string]any{}}, ScopeConfirmation{}, ErrScopeNotConfirmed}} {
		t.Run(test.name, func(t *testing.T) {
			_, err := (Skill{}).PrepareUnsubmitted(context.Background(), test.browser, workflow, test.scope, "https://ad.oceanengine.com/project/create")
			if err != test.want {
				t.Fatalf("err=%v want=%v", err, test.want)
			}
		})
	}
}

func validPage() PageIdentity {
	return PageIdentity{Host: "ad.oceanengine.com", PageKind: "project_create", AccountReferenceID: "account-6391", PlatformProjectID: "test-project-1"}
}
func confirmedScope() ScopeConfirmation {
	return ScopeConfirmation{Confirmed: true, AccountReferenceID: "account-6391", AllowedPlatformProjectIDs: []string{"test-project-1"}, BudgetLimitMinor: 10000, ForbiddenActions: []string{"save", "create", "submit", "enable", "modify"}}
}
func validWorkflow(t *testing.T) delivery.CompiledDeliveryWorkflow {
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	workflow := delivery.CompiledDeliveryWorkflow{SchemaVersion: delivery.CompiledDeliveryWorkflowSchemaV1, ID: "workflow-1", OrganizationID: "org-1", ProjectID: "project-1", DecisionID: "decision-1", DecisionCanonicalHash: hash, SelectedCandidateID: "balanced", ConfigurationCanonicalHash: hash, ConfigurationID: "configuration-1", ConfigurationVersion: 1, Platform: delivery.DeliveryPlatformOceanEngine, ProfileVersion: delivery.OceanEngineConfigurationProfileV1, AccountReference: delivery.StableReference{Namespace: "oceanengine", ObjectKind: "account", Scope: "organization", ID: "account-6391", State: delivery.ReferenceResolved}, CapabilityContractVersion: delivery.OceanEngineCapabilityContractV01, SelectorContractVersion: delivery.OceanEngineSelectorContractV01, ActionContractVersion: delivery.OceanEngineActionContractV01, CompilerVersion: delivery.DeliveryWorkflowCompilerV1, Status: "ready_for_final_approval", RemoteWriteEnabled: false, Steps: []delivery.CompiledWorkflowStep{{ID: "prepare", Sequence: 1, Page: "oceanengine/project", Action: "prepare_project_local_form", Risk: delivery.WorkflowRiskPrepareLocalForm, Fields: []delivery.WorkflowField{{Key: "project_name", Value: "test project", ExpectedReadback: "test project", EvidenceRef: "configuration://project/name"}}, TimeoutSeconds: 60, Recovery: "discard local form"}, {ID: "submit", Sequence: 2, Page: "oceanengine/review", Action: "submit_platform_configuration", Risk: delivery.WorkflowRiskRemoteWrite, Fields: []delivery.WorkflowField{}, Blocked: true, BlockReason: "PHASE_C_REMOTE_WRITE_PROHIBITED"}}, CreatedBy: "user-1", CreatedAt: time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)}
	var err error
	workflow.CanonicalHash, err = workflow.ComputeCanonicalHash()
	if err != nil {
		t.Fatal(err)
	}
	if err = workflow.Validate(); err != nil {
		t.Fatal(err)
	}
	return workflow
}
