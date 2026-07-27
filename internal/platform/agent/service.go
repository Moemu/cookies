package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/remix"
)

var (
	ErrNotFound     = errors.New("agent run not found")
	ErrInvalidState = errors.New("agent run state does not allow this operation")
)

type IDGenerator func(prefix string) (string, error)

type RenderJobReader interface {
	GetRenderJob(context.Context, contract.ActorContext, contract.ProjectID, string) (remix.RenderJob, error)
}

type Store interface {
	Save(context.Context, AgentRun) error
	Get(context.Context, contract.ActorContext, contract.ProjectID, string) (AgentRun, error)
	List(context.Context, contract.ActorContext, contract.ProjectID, int) ([]AgentRun, error)
}

type MemoryStore struct {
	mu   sync.RWMutex
	runs map[string]AgentRun
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{runs: make(map[string]AgentRun)}
}

func (s *MemoryStore) Save(ctx context.Context, run AgentRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := run.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = cloneRun(run)
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (AgentRun, error) {
	if err := ctx.Err(); err != nil {
		return AgentRun{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[id]
	if !ok || run.OrganizationID != actor.OrganizationID || run.ProjectID != projectID {
		return AgentRun{}, ErrNotFound
	}
	return cloneRun(run), nil
}

func (s *MemoryStore) List(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]AgentRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	runs := make([]AgentRun, 0, len(s.runs))
	for _, run := range s.runs {
		if run.OrganizationID == actor.OrganizationID && run.ProjectID == projectID {
			runs = append(runs, cloneRun(run))
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

type Service struct {
	store   Store
	tools   ToolRegistry
	renders RenderJobReader
	newID   IDGenerator
	nowUTC  func() time.Time
}

func NewMemoryService(renders RenderJobReader, newID IDGenerator) *Service {
	store := NewMemoryStore()
	return NewServiceWithStore(store, renders, newID)
}

func NewServiceWithStore(store Store, renders RenderJobReader, newID IDGenerator) *Service {
	if store == nil {
		store = NewMemoryStore()
	}
	service := &Service{
		store:   store,
		renders: renders,
		newID:   newID,
		nowUTC:  func() time.Time { return time.Now().UTC() },
	}
	service.tools = NewDefaultToolRegistry(renders)
	if service.newID == nil {
		service.newID = defaultID
	}
	return service
}

func (s *Service) CreateRun(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateRunRequest) (AgentRun, error) {
	if err := ctx.Err(); err != nil {
		return AgentRun{}, err
	}
	if err := request.Validate(); err != nil {
		return AgentRun{}, err
	}
	runID, err := s.newID("agentrun")
	if err != nil {
		return AgentRun{}, err
	}
	rootSpanID, err := s.newID("span")
	if err != nil {
		return AgentRun{}, err
	}
	now := s.nowUTC()
	run := AgentRun{
		ID:             runID,
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		Workflow:       request.Workflow,
		Status:         RunRunning,
		Target:         request.Target,
		CreatedBy:      actor.Principal,
		CreatedAt:      now,
		UpdatedAt:      now,
		TraceSpans: []TraceSpan{{
			ID:        rootSpanID,
			Name:      "render diagnosis agent",
			Kind:      SpanKindAgent,
			Status:    SpanRunning,
			StartedAt: now,
		}},
		Steps: []AgentStep{{
			ID:        "step_collect_render_error",
			Label:     "读取渲染错误并生成诊断",
			Status:    RunRunning,
			Summary:   "调用受控工具读取 RenderJob 错误。",
			StartedAt: now,
		}},
	}
	run = s.executeRenderDiagnosis(ctx, actor, projectID, run, rootSpanID)
	if err := s.store.Save(ctx, run); err != nil {
		return AgentRun{}, err
	}
	return cloneRun(run), nil
}

func (s *Service) GetRun(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (AgentRun, error) {
	return s.store.Get(ctx, actor, projectID, id)
}

func (s *Service) ListRuns(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]AgentRun, error) {
	return s.store.List(ctx, actor, projectID, limit)
}

func (s *Service) CancelRun(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (AgentRun, error) {
	run, err := s.store.Get(ctx, actor, projectID, id)
	if err != nil {
		return AgentRun{}, err
	}
	if run.Status != RunQueued && run.Status != RunRunning {
		return AgentRun{}, ErrInvalidState
	}
	now := s.nowUTC()
	run.Status = RunCancelled
	run.ErrorMessage = "agent run cancelled by user"
	run.UpdatedAt = now
	run.CompletedAt = &now
	for index := range run.TraceSpans {
		if run.TraceSpans[index].Status == SpanRunning {
			run.TraceSpans[index].Status = SpanCancelled
			run.TraceSpans[index].EndedAt = &now
		}
	}
	for index := range run.Steps {
		if run.Steps[index].Status == RunQueued || run.Steps[index].Status == RunRunning {
			run.Steps[index].Status = RunCancelled
			run.Steps[index].EndedAt = &now
		}
	}
	if err := s.store.Save(ctx, run); err != nil {
		return AgentRun{}, err
	}
	return cloneRun(run), nil
}

func (s *Service) executeRenderDiagnosis(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, run AgentRun, rootSpanID string) AgentRun {
	toolSpanID, toolCallID, modelSpanID, err := s.allocateExecutionIDs()
	now := s.nowUTC()
	if err != nil {
		return failRun(run, now, fmt.Errorf("allocate trace IDs: %w", err))
	}
	toolInput := map[string]any{"render_job_id": run.Target.RenderJobID}
	toolCall := ToolCall{
		ID:        toolCallID,
		Name:      RenderDiagnosisToolName,
		Status:    ToolRunning,
		Input:     toolInput,
		StartedAt: now,
	}
	run.ToolCalls = append(run.ToolCalls, toolCall)
	run.TraceSpans = append(run.TraceSpans, TraceSpan{
		ID:        toolSpanID,
		ParentID:  rootSpanID,
		Name:      RenderDiagnosisToolName,
		Kind:      SpanKindTool,
		Status:    SpanRunning,
		StartedAt: now,
	})

	result, err := s.tools.Execute(ctx, ToolExecutionContext{Actor: actor, ProjectID: projectID, RenderJobs: s.renders}, RenderDiagnosisToolName, toolInput)
	ended := s.nowUTC()
	toolIndex := len(run.ToolCalls) - 1
	spanIndex := len(run.TraceSpans) - 1
	run.ToolCalls[toolIndex].EndedAt = &ended
	run.TraceSpans[spanIndex].EndedAt = &ended
	if err != nil {
		run.ToolCalls[toolIndex].Status = ToolFailed
		run.ToolCalls[toolIndex].ErrorMessage = err.Error()
		run.TraceSpans[spanIndex].Status = SpanFailed
		run.TraceSpans[spanIndex].ErrorMessage = err.Error()
		return failRun(run, ended, err)
	}
	run.ToolCalls[toolIndex].Status = ToolSucceeded
	run.ToolCalls[toolIndex].Output = result.Output
	run.ToolCalls[toolIndex].References = result.References
	run.TraceSpans[spanIndex].Status = SpanSucceeded

	run.TraceSpans = append(run.TraceSpans, TraceSpan{
		ID:           modelSpanID,
		ParentID:     rootSpanID,
		Name:         "diagnosis-summary",
		Kind:         SpanKindModel,
		Status:       SpanSucceeded,
		Model:        "fake.render-diagnosis.v1",
		InputTokens:  64,
		OutputTokens: 96,
		StartedAt:    ended,
		EndedAt:      &ended,
	})
	run.Output = result.Output
	run.Status = RunSucceeded
	run.UpdatedAt = ended
	run.CompletedAt = &ended
	run.TraceSpans[0].Status = SpanSucceeded
	run.TraceSpans[0].EndedAt = &ended
	run.Steps[0].Status = RunSucceeded
	run.Steps[0].Summary = outputString(result.Output, "diagnosis")
	run.Steps[0].EndedAt = &ended
	return run
}

func (s *Service) allocateExecutionIDs() (string, string, string, error) {
	toolSpanID, err := s.newID("span")
	if err != nil {
		return "", "", "", err
	}
	toolCallID, err := s.newID("toolcall")
	if err != nil {
		return "", "", "", err
	}
	modelSpanID, err := s.newID("span")
	if err != nil {
		return "", "", "", err
	}
	return toolSpanID, toolCallID, modelSpanID, nil
}

func failRun(run AgentRun, now time.Time, err error) AgentRun {
	run.Status = RunFailed
	run.ErrorMessage = err.Error()
	run.UpdatedAt = now
	run.CompletedAt = &now
	if len(run.TraceSpans) > 0 {
		run.TraceSpans[0].Status = SpanFailed
		run.TraceSpans[0].ErrorMessage = err.Error()
		run.TraceSpans[0].EndedAt = &now
	}
	if len(run.Steps) > 0 {
		run.Steps[0].Status = RunFailed
		run.Steps[0].Summary = err.Error()
		run.Steps[0].EndedAt = &now
	}
	return run
}

func outputString(output map[string]any, key string) string {
	value, _ := output[key].(string)
	return value
}

func cloneRun(run AgentRun) AgentRun {
	run.Steps = append([]AgentStep(nil), run.Steps...)
	run.ToolCalls = cloneToolCalls(run.ToolCalls)
	run.TraceSpans = append([]TraceSpan(nil), run.TraceSpans...)
	run.Output = cloneMap(run.Output)
	return run
}

func cloneToolCalls(calls []ToolCall) []ToolCall {
	cloned := make([]ToolCall, len(calls))
	for index, call := range calls {
		cloned[index] = call
		cloned[index].Input = cloneMap(call.Input)
		cloned[index].Output = cloneMap(call.Output)
		cloned[index].References = append([]contract.ResourceRef(nil), call.References...)
	}
	return cloned
}

func cloneMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func defaultID(prefix string) (string, error) {
	if strings.TrimSpace(prefix) == "" {
		return "", fmt.Errorf("id prefix is required")
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano()), nil
}
