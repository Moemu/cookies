package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/remix"
)

const RenderDiagnosisToolName = "remix.render.diagnose"

type ToolExecutionContext struct {
	Actor      contract.ActorContext
	ProjectID  contract.ProjectID
	RenderJobs RenderJobReader
}

type ToolResult struct {
	Output     map[string]any
	References []contract.ResourceRef
}

type Tool interface {
	Name() string
	RequiredScopes() []contract.Scope
	Execute(context.Context, ToolExecutionContext, map[string]any) (ToolResult, error)
}

type ToolRegistry struct {
	tools map[string]Tool
}

func NewDefaultToolRegistry(renders RenderJobReader) ToolRegistry {
	registry := ToolRegistry{tools: map[string]Tool{}}
	registry.Register(RenderDiagnosisTool{renders: renders})
	return registry
}

func (r ToolRegistry) Register(tool Tool) {
	if r.tools == nil {
		r.tools = make(map[string]Tool)
	}
	r.tools[tool.Name()] = tool
}

func (r ToolRegistry) Execute(ctx context.Context, exec ToolExecutionContext, name string, input map[string]any) (ToolResult, error) {
	tool, ok := r.tools[name]
	if !ok {
		return ToolResult{}, fmt.Errorf("tool %q is not registered", name)
	}
	for _, scope := range tool.RequiredScopes() {
		if !exec.Actor.HasScope(scope) {
			return ToolResult{}, fmt.Errorf("tool %s requires scope %s", name, scope)
		}
	}
	return tool.Execute(ctx, exec, input)
}

type RenderDiagnosisTool struct {
	renders RenderJobReader
}

func (RenderDiagnosisTool) Name() string {
	return RenderDiagnosisToolName
}

func (RenderDiagnosisTool) RequiredScopes() []contract.Scope {
	return []contract.Scope{remix.ScopePlanRead}
}

func (t RenderDiagnosisTool) Execute(ctx context.Context, exec ToolExecutionContext, input map[string]any) (ToolResult, error) {
	renders := t.renders
	if renders == nil {
		renders = exec.RenderJobs
	}
	if renders == nil {
		return ToolResult{}, fmt.Errorf("render job reader is not configured")
	}
	renderJobID, _ := input["render_job_id"].(string)
	if strings.TrimSpace(renderJobID) == "" {
		return ToolResult{}, fmt.Errorf("render_job_id is required")
	}
	job, err := renders.GetRenderJob(ctx, exec.Actor, exec.ProjectID, renderJobID)
	if err != nil {
		return ToolResult{}, err
	}
	diagnosis, recommendation := diagnoseRenderJob(job)
	return ToolResult{
		Output: map[string]any{
			"render_job_id":  job.ID,
			"status":         string(job.Status),
			"error_code":     job.ErrorCode,
			"error_message":  job.ErrorMessage,
			"diagnosis":      diagnosis,
			"recommendation": recommendation,
		},
		References: []contract.ResourceRef{{Type: "remix_render_job", ID: job.ID}},
	}, nil
}

func diagnoseRenderJob(job remix.RenderJob) (string, string) {
	switch {
	case job.Status == remix.RenderFailed && strings.TrimSpace(job.ErrorMessage) != "":
		return "渲染任务失败，错误来自 RenderJob 持久状态。", "检查输入 RemixPlan 的素材可用性、目标格式和渲染器日志后重试。"
	case job.Status == remix.RenderFailed:
		return "渲染任务失败，但缺少可读错误信息。", "补充渲染器错误上报，并从最近一次执行日志定位失败阶段。"
	case job.Status == remix.RenderSucceeded:
		return "渲染任务已成功，当前不需要失败诊断。", "查看 output_asset 和质量报告，确认是否需要人工复核。"
	case job.Status == remix.RenderRunning || job.Status == remix.RenderQueued:
		return "渲染任务仍在执行队列或运行中。", "等待状态推进；如长时间无进度，再检查调度器 lease 和 worker 日志。"
	default:
		return "渲染任务状态不可识别。", "确认 RenderJob 状态机是否写入了受支持状态。"
	}
}
