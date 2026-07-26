package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/remix"
)

func TestRenderDiagnosisRunPersistsTraceAndToolCalls(t *testing.T) {
	t.Parallel()
	service := newTestService(remix.RenderJob{
		ID: "remixrender_1", OrganizationID: "org_1", ProjectID: "project_1", PlanID: "remixplan_1",
		Status: remix.RenderFailed, TargetFormat: "mp4", TargetQuality: "draft", ErrorCode: "RENDERER_TIMEOUT", ErrorMessage: "renderer timed out",
	})
	actor := agentActor(remix.ScopePlanRead, ScopeRunWrite, ScopeRunRead)

	run, err := service.CreateRun(context.Background(), actor, "project_1", CreateRunRequest{
		Workflow: WorkflowRenderDiagnosis,
		Target:   DiagnosisTarget{RenderJobID: "remixrender_1"},
	})
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	if run.Status != RunSucceeded || outputString(run.Output, "diagnosis") == "" {
		t.Fatalf("run did not succeed with diagnosis: %#v", run)
	}
	if len(run.ToolCalls) != 1 || run.ToolCalls[0].Status != ToolSucceeded || len(run.ToolCalls[0].References) != 1 {
		t.Fatalf("tool calls were not persisted: %#v", run.ToolCalls)
	}
	if len(run.TraceSpans) != 3 {
		t.Fatalf("trace spans = %#v", run.TraceSpans)
	}
	rootID := run.TraceSpans[0].ID
	if run.TraceSpans[1].ParentID != rootID || run.TraceSpans[2].ParentID != rootID {
		t.Fatalf("trace spans do not preserve parent-child links: %#v", run.TraceSpans)
	}

	got, err := service.GetRun(context.Background(), actor, "project_1", run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	got.Output["diagnosis"] = "mutated"
	again, err := service.GetRun(context.Background(), actor, "project_1", run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if outputString(again.Output, "diagnosis") == "mutated" {
		t.Fatal("GetRun() returned mutable output map")
	}
}

func TestRenderDiagnosisToolPermissionFailureIsCaptured(t *testing.T) {
	t.Parallel()
	service := newTestService(remix.RenderJob{ID: "remixrender_1", OrganizationID: "org_1", ProjectID: "project_1", PlanID: "remixplan_1", Status: remix.RenderFailed, TargetFormat: "mp4", TargetQuality: "draft"})
	actor := agentActor(ScopeRunWrite)

	run, err := service.CreateRun(context.Background(), actor, "project_1", CreateRunRequest{
		Workflow: WorkflowRenderDiagnosis,
		Target:   DiagnosisTarget{RenderJobID: "remixrender_1"},
	})
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	if run.Status != RunFailed || len(run.ToolCalls) != 1 || run.ToolCalls[0].Status != ToolFailed {
		t.Fatalf("permission failure was not persisted: %#v", run)
	}
	if !strings.Contains(run.ErrorMessage, string(remix.ScopePlanRead)) {
		t.Fatalf("error does not explain missing tool scope: %q", run.ErrorMessage)
	}
}

func TestRenderDiagnosisFailureWhenRenderJobMissing(t *testing.T) {
	t.Parallel()
	service := newTestService(remix.RenderJob{})
	service.renders = fakeRenderJobs{err: remix.ErrNotFound}
	service.tools = NewDefaultToolRegistry(service.renders)
	actor := agentActor(remix.ScopePlanRead, ScopeRunWrite)

	run, err := service.CreateRun(context.Background(), actor, "project_1", CreateRunRequest{
		Workflow: WorkflowRenderDiagnosis,
		Target:   DiagnosisTarget{RenderJobID: "missing"},
	})
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if run.Status != RunFailed || !errors.Is(service.renders.(fakeRenderJobs).err, remix.ErrNotFound) {
		t.Fatalf("missing render job did not fail run: %#v", run)
	}
}

func TestCancelQueuedRun(t *testing.T) {
	t.Parallel()
	service := newTestService(remix.RenderJob{})
	actor := agentActor(ScopeRunWrite, ScopeRunRead)
	queued := AgentRun{
		ID:             "agentrun_queued",
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		Workflow:       WorkflowRenderDiagnosis,
		Status:         RunQueued,
		Target:         DiagnosisTarget{RenderJobID: "remixrender_1"},
		CreatedBy:      actor.Principal,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		TraceSpans: []TraceSpan{{
			ID:        "span_root",
			Name:      "queued agent",
			Kind:      SpanKindAgent,
			Status:    SpanRunning,
			StartedAt: time.Now().UTC(),
		}},
	}
	if err := service.store.Save(context.Background(), queued); err != nil {
		t.Fatal(err)
	}

	cancelled, err := service.CancelRun(context.Background(), actor, "project_1", queued.ID)
	if err != nil {
		t.Fatalf("CancelRun() error = %v", err)
	}
	if cancelled.Status != RunCancelled || cancelled.TraceSpans[0].Status != SpanCancelled || cancelled.CompletedAt == nil {
		t.Fatalf("cancelled run = %#v", cancelled)
	}
	if _, err := service.CancelRun(context.Background(), actor, "project_1", queued.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("second CancelRun() error = %v, want ErrInvalidState", err)
	}
}

func TestAgentRunValidateRejectsMissingTraceParent(t *testing.T) {
	t.Parallel()
	run := AgentRun{
		ID:             "agentrun_1",
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		Workflow:       WorkflowRenderDiagnosis,
		Status:         RunSucceeded,
		Target:         DiagnosisTarget{RenderJobID: "remixrender_1"},
		TraceSpans: []TraceSpan{{
			ID:        "span_child",
			ParentID:  "span_missing",
			Name:      "child",
			Kind:      SpanKindTool,
			Status:    SpanSucceeded,
			StartedAt: time.Now().UTC(),
		}},
	}

	if err := run.Validate(); err == nil {
		t.Fatal("Validate() succeeded with missing trace parent")
	}
}

func newTestService(job remix.RenderJob) *Service {
	next := 0
	service := NewMemoryService(fakeRenderJobs{job: job}, func(prefix string) (string, error) {
		next++
		return prefix + "_" + string(rune('0'+next)), nil
	})
	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	service.nowUTC = func() time.Time {
		now = now.Add(time.Second)
		return now
	}
	return service
}

func agentActor(scopes ...contract.Scope) contract.ActorContext {
	return contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"},
		Scopes:         scopes,
	}
}

type fakeRenderJobs struct {
	job remix.RenderJob
	err error
}

func (f fakeRenderJobs) GetRenderJob(context.Context, contract.ActorContext, contract.ProjectID, string) (remix.RenderJob, error) {
	if f.err != nil {
		return remix.RenderJob{}, f.err
	}
	return f.job, nil
}
