package creative

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
)

type ActiveProjectResolver interface {
	RequireActiveContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
}

type Service struct {
	Repository Repository
	Projects   ActiveProjectResolver
	NewID      ids.Generator
	Now        func() time.Time
}

func (s Service) CreateIntake(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, key contract.IdempotencyKey, request CreateIntakeRequest) (CreativeIntake, error) {
	if s.Repository == nil || s.Projects == nil {
		return CreativeIntake{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if err := requestContext.Validate(); err != nil {
		return CreativeIntake{}, err
	}
	if !requestContext.Actor.HasScope(ScopeWrite) {
		return CreativeIntake{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if err := key.Validate(); err != nil {
		return CreativeIntake{}, err
	}
	if err := request.Validate(); err != nil {
		return CreativeIntake{}, err
	}
	project, err := s.Projects.RequireActiveContext(ctx, requestContext.Actor, projectID)
	if err != nil {
		return CreativeIntake{}, err
	}
	if project.OrganizationID != requestContext.Actor.OrganizationID || project.ProjectID != projectID {
		return CreativeIntake{}, fmt.Errorf("resolved project context does not match request scope")
	}
	hash, err := contract.CanonicalJSONHash(request)
	if err != nil {
		return CreativeIntake{}, fmt.Errorf("canonicalize creative intake: %w", err)
	}
	intakeID, err := s.idGenerator()("creativeintake")
	if err != nil {
		return CreativeIntake{}, err
	}
	now := s.now()
	missing := request.missingFields()
	status := IntakeReady
	confirmedBy := requestContext.Actor.Principal.ID
	if len(missing) > 0 {
		status, confirmedBy = IntakeNeedsClarification, ""
	}
	value := CreativeIntake{
		ID: intakeID, OrganizationID: requestContext.Actor.OrganizationID, ProjectID: projectID, Source: request.Source, Status: status,
		Request: request, MissingFields: missing, Warnings: []string{}, ConfirmedBy: confirmedBy, Principal: requestContext.Actor.Principal,
		IdempotencyKey: key, RequestHash: hash, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	stored, _, err := s.Repository.CreateIntake(ctx, value)
	return stored, err
}

func (s Service) ListIntakes(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]CreativeIntake, error) {
	if s.Repository == nil || s.Projects == nil {
		return nil, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return nil, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListIntakes(ctx, actor.OrganizationID, projectID, normalizedLimit(limit))
}

func (s Service) CreateTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, intakeID string) (CreativeTask, error) {
	if s.Repository == nil || s.Projects == nil {
		return CreativeTask{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return CreativeTask{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return CreativeTask{}, err
	}
	intake, err := s.Repository.GetIntake(ctx, actor.OrganizationID, projectID, intakeID)
	if err != nil {
		return CreativeTask{}, err
	}
	if intake.Status != IntakeReady {
		return CreativeTask{}, ErrIntakeNotReady
	}
	id, err := s.idGenerator()("creativetask")
	if err != nil {
		return CreativeTask{}, err
	}
	now := s.now()
	direction := CreativeDirection{Concept: strings.TrimSpace(intake.Request.Concept), Tone: append([]string{}, intake.Request.Tone...), VisualKeywords: append([]string{}, intake.Request.VisualKeywords...)}
	task := CreativeTask{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, IntakeID: intake.ID, Format: FormatImageText, Channel: intake.Request.Channel, Status: TaskDraft, Direction: direction, Version: 1, CreatedAt: now, UpdatedAt: now}
	draft := composeXiaohongshuDraft(task.ID, intake, now)
	stored, err := s.Repository.CreateTask(ctx, task, draft)
	if err != nil {
		return CreativeTask{}, err
	}
	return stored, nil
}

func (s Service) ListTasks(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]CreativeTask, error) {
	if s.Repository == nil || s.Projects == nil {
		return nil, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return nil, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListTasks(ctx, actor.OrganizationID, projectID, normalizedLimit(limit))
}

func (s Service) GetTaskDetail(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string) (TaskDetail, error) {
	if s.Repository == nil || s.Projects == nil {
		return TaskDetail{}, fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return TaskDetail{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) RegisterCoverImageJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID, providerJobID string) error {
	if s.Repository == nil || s.Projects == nil {
		return fmt.Errorf("creative dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return err
	}
	if strings.TrimSpace(providerJobID) == "" {
		return fmt.Errorf("provider job ID is required")
	}
	if _, err := s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID); err != nil {
		return err
	}
	return s.Repository.RegisterProductionJob(ctx, actor.OrganizationID, projectID, taskID, ProductionJob{TaskID: taskID, Kind: "cover_image", ProviderJobID: providerJobID, CreatedAt: s.now()})
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s Service) idGenerator() ids.Generator {
	if s.NewID != nil {
		return s.NewID
	}
	return ids.New
}
func normalizedLimit(limit int) int {
	if limit < 1 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func composeXiaohongshuDraft(taskID string, intake CreativeIntake, now time.Time) ImageTextDraft {
	r := intake.Request
	message := strings.TrimSpace(r.CoreMessage)
	concept := strings.TrimSpace(r.Concept)
	if concept == "" {
		concept = "把产品价值放进真实使用场景"
	}
	titles := []string{message + "：这次想认真说清楚", "不只是一句卖点，" + message, "给" + strings.TrimSpace(r.Audience) + "的一份实用说明"}
	cover := message
	if len([]rune(cover)) > 18 {
		cover = string([]rune(cover)[:18])
	}
	body := fmt.Sprintf("最近在为%s整理一个更清楚的表达：%s。\n\n%s\n\n先从真实场景出发，再把值得被看见的细节说具体。%s", strings.TrimSpace(r.Audience), message, concept, ctaSentence(r.CallToAction))
	topics := []string{"#品牌内容", "#创意灵感", "#好内容值得被看见"}
	return ImageTextDraft{TaskID: taskID, Version: 1, Status: "draft", TitleCandidates: titles, Body: body, Topics: topics, CoverCopy: cover, ImagePlan: []ImagePlanItem{
		{Order: 1, Purpose: "封面", VisualBrief: concept + "，突出核心信息：" + message, Caption: cover},
		{Order: 2, Purpose: "场景", VisualBrief: "目标人群的真实使用场景，画面自然可信", Caption: "从一个具体场景开始"},
		{Order: 3, Purpose: "价值", VisualBrief: "以细节或产品体验证明核心信息", Caption: message},
		{Order: 4, Purpose: "行动", VisualBrief: "干净的品牌收束画面，留出行动引导区域", Caption: strings.TrimSpace(r.CallToAction)},
	}, CreatedAt: now}
}

func ctaSentence(value string) string {
	if strings.TrimSpace(value) == "" {
		return "欢迎把你的想法留在评论区。"
	}
	return "如果你也在关注这件事，" + strings.TrimSpace(value) + "。"
}
