package strategy_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/integrations/strategycreative"
	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/eventoutbox"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/project"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
	"github.com/shikanon/cookies/internal/systems/strategy"
	strategyhttp "github.com/shikanon/cookies/internal/systems/strategy/httpapi"
)

func TestStrategyMySQLVerticalSlice(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	organizationID := contract.OrganizationID("org_strategy_it_" + suffix)
	projectID := contract.ProjectID("project_strategy_it_" + suffix)
	userID := "user_strategy_it_" + suffix
	actor := contract.ActorContext{
		OrganizationID: organizationID,
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: userID},
		Scopes: []contract.Scope{
			"project.read", "project.write", strategy.ScopeRead, strategy.ScopeWrite,
			strategy.ScopeConfirm, strategy.ScopeReview, strategy.ScopeApprove, strategy.ScopePackageRead,
			creative.ScopeWrite,
		},
	}
	t.Cleanup(func() { cleanupStrategyIntegration(t, db, organizationID, userID) })
	identityStore := identity.MySQLStore{DB: db}
	if err := identityStore.EnsureLocalActor(ctx, actor); err != nil {
		t.Fatal(err)
	}
	projectStore := project.MySQLStore{DB: db}
	if err := projectStore.EnsureLocalProject(ctx, actor, projectID); err != nil {
		t.Fatal(err)
	}
	projectService := &project.Service{Store: projectStore, Authorizer: projectStore}
	service := strategy.Service{DB: db, Projects: projectService, Agents: agent.MySQLStore{DB: db}}

	createdTask, duplicate, err := service.CreateTask(
		ctx, actor, contract.IdempotencyKey("atomic_task_"+suffix), projectID,
		strategy.CreateTaskRequest{Name: "原子策略任务", Objective: "验证新品认知"},
	)
	if err != nil || duplicate || createdTask.BriefDraft.Document.Campaign.Objective != "验证新品认知" {
		t.Fatalf("create atomic task: duplicate=%v bundle=%#v err=%v", duplicate, createdTask, err)
	}
	replayedTask, duplicate, err := service.CreateTask(
		ctx, actor, contract.IdempotencyKey("atomic_task_"+suffix), projectID,
		strategy.CreateTaskRequest{Name: "原子策略任务", Objective: "验证新品认知"},
	)
	if err != nil || !duplicate || replayedTask.Task.ID != createdTask.Task.ID {
		t.Fatalf("replay atomic task: duplicate=%v bundle=%#v err=%v", duplicate, replayedTask, err)
	}
	taskItems, err := service.ListTasks(ctx, actor, projectID)
	if err != nil || len(taskItems) != 1 || taskItems[0].Name != "原子策略任务" {
		t.Fatalf("list atomic tasks: items=%#v err=%v", taskItems, err)
	}
	discardedTask, duplicate, err := service.DiscardTask(
		ctx, actor, contract.IdempotencyKey("discard_task_"+suffix), createdTask.Task.ID,
		strategy.LifecycleRequest{ExpectedVersion: createdTask.Task.Version, Reason: "集成测试废弃"},
	)
	if err != nil || duplicate || discardedTask.DiscardedAt == nil {
		t.Fatalf("discard task: duplicate=%v task=%#v err=%v", duplicate, discardedTask, err)
	}
	replayedDiscard, duplicate, err := service.DiscardTask(
		ctx, actor, contract.IdempotencyKey("discard_task_"+suffix), createdTask.Task.ID,
		strategy.LifecycleRequest{ExpectedVersion: createdTask.Task.Version, Reason: "集成测试废弃"},
	)
	if err != nil || !duplicate || replayedDiscard.Version != discardedTask.Version {
		t.Fatalf("replay discard task: duplicate=%v task=%#v err=%v", duplicate, replayedDiscard, err)
	}
	taskItems, err = service.ListTasks(ctx, actor, projectID)
	if err != nil || len(taskItems) != 0 {
		t.Fatalf("discarded task leaked into active list: items=%#v err=%v", taskItems, err)
	}
	archivedTaskItems, err := service.ListTasksByLifecycle(ctx, actor, projectID, "archived")
	if err != nil || len(archivedTaskItems) != 1 || archivedTaskItems[0].Task.ID != createdTask.Task.ID {
		t.Fatalf("list discarded tasks: items=%#v err=%v", archivedTaskItems, err)
	}
	restoredTask, duplicate, err := service.RestoreTask(
		ctx, actor, contract.IdempotencyKey("restore_task_"+suffix), createdTask.Task.ID,
		strategy.LifecycleRequest{ExpectedVersion: discardedTask.Version},
	)
	if err != nil || duplicate || restoredTask.DiscardedAt != nil {
		t.Fatalf("restore task: duplicate=%v task=%#v err=%v", duplicate, restoredTask, err)
	}

	workspace, duplicate, err := service.CreateWorkspace(ctx, actor, contract.IdempotencyKey("workspace_"+suffix), projectID, "Integration Workspace")
	if err != nil || duplicate {
		t.Fatalf("create workspace: duplicate=%v err=%v", duplicate, err)
	}
	bundle, duplicate, err := service.CreateConversation(ctx, actor, contract.IdempotencyKey("conversation_"+suffix), projectID, workspace.ID)
	if err != nil || duplicate {
		t.Fatalf("create conversation: duplicate=%v err=%v", duplicate, err)
	}
	messageResult, duplicate, err := service.SendMessage(ctx, actor, contract.IdempotencyKey("message_"+suffix), bundle.Conversation.ID, "目标：新品认知；受众：研发负责人；卖点：缩短研发周期")
	if err != nil || duplicate {
		t.Fatalf("send message: duplicate=%v err=%v", duplicate, err)
	}
	messages, err := service.ListMessages(ctx, actor, bundle.Conversation.ID, messageResult.Message.ID, 50)
	if err != nil || len(messages) != 0 {
		t.Fatalf("list messages after cursor: messages=%d err=%v", len(messages), err)
	}
	if _, err := service.ListMessages(ctx, actor, bundle.Conversation.ID, "expired_cursor", 50); !errors.Is(err, strategy.ErrEventCursorExpired) {
		t.Fatalf("expired cursor error = %v", err)
	}
	if err := runAgentTaskThroughRuntime(ctx, db, service, messageResult.AgentTask); err != nil {
		t.Fatalf("extract brief: %v", err)
	}
	draft, err := service.GetTaskBriefDraft(ctx, actor, bundle.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	patch := strategy.BriefPatch{ExpectedVersion: draft.Version, Operations: []strategy.BriefPatchOperation{
		{Op: "set", FieldPath: "campaign.objective", Value: json.RawMessage(`"新品认知"`)},
		{Op: "set", FieldPath: "audience.primary", Value: json.RawMessage(`"研发负责人"`)},
		{Op: "set", FieldPath: "proposition", Value: json.RawMessage(`"缩短研发周期"`)},
		{Op: "set", FieldPath: "channels", Value: json.RawMessage(`["xiaohongshu"]`)},
	}}
	draft, _, err = service.PatchBriefDraft(ctx, actor, contract.IdempotencyKey("briefpatch_"+suffix), bundle.Task.ID, patch)
	if err != nil || !draft.Completeness.Ready {
		t.Fatalf("patch brief: ready=%v err=%v", draft.Completeness.Ready, err)
	}
	briefVersion, duplicate, err := service.ConfirmBrief(ctx, actor, contract.IdempotencyKey("confirm_"+suffix), bundle.Task.ID, draft.Version)
	if err != nil || duplicate {
		t.Fatalf("confirm brief: duplicate=%v err=%v", duplicate, err)
	}
	briefConfirmedTask, err := service.GetTask(ctx, actor, bundle.Task.ID)
	if err != nil || briefConfirmedTask.Status != "active" {
		t.Fatalf("task after brief confirmation=%#v err=%v", briefConfirmedTask, err)
	}
	created, duplicate, err := service.CreateStrategy(ctx, actor, contract.IdempotencyKey("strategy_"+suffix), bundle.Task.ID, briefVersion.BriefID, briefVersion.Version)
	if err != nil || duplicate {
		t.Fatalf("create strategy: duplicate=%v err=%v", duplicate, err)
	}
	if err := runAgentTaskThroughRuntime(ctx, db, service, created.AgentTask); err != nil {
		t.Fatalf("generate strategy: %v", err)
	}
	strategyDraft, err := service.GetDraft(ctx, actor, created.Draft.ID)
	if err != nil || strategyDraft.CurrentRevision != 1 {
		t.Fatalf("get generated draft: revision=%d err=%v", strategyDraft.CurrentRevision, err)
	}
	versionedTask, err := service.GetTask(ctx, actor, bundle.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.DiscardTask(
		ctx, actor, contract.IdempotencyKey("discard_versioned_task_"+suffix), bundle.Task.ID,
		strategy.LifecycleRequest{ExpectedVersion: versionedTask.Version, Reason: "已有版本不可废弃"},
	); !errors.Is(err, strategy.ErrInvalidState) {
		t.Fatalf("discard task with strategy revision error=%v", err)
	}
	archivedDraft, duplicate, err := service.ArchiveStrategy(
		ctx, actor, contract.IdempotencyKey("archive_strategy_"+suffix), strategyDraft.ID,
		strategy.LifecycleRequest{ExpectedVersion: strategyDraft.Version, Reason: "集成测试归档"},
	)
	if err != nil || duplicate || archivedDraft.ArchivedAt == nil {
		t.Fatalf("archive strategy: duplicate=%v draft=%#v err=%v", duplicate, archivedDraft, err)
	}
	replayedArchive, duplicate, err := service.ArchiveStrategy(
		ctx, actor, contract.IdempotencyKey("archive_strategy_"+suffix), strategyDraft.ID,
		strategy.LifecycleRequest{ExpectedVersion: strategyDraft.Version, Reason: "集成测试归档"},
	)
	if err != nil || !duplicate || replayedArchive.Version != archivedDraft.Version {
		t.Fatalf("replay archive strategy: duplicate=%v draft=%#v err=%v", duplicate, replayedArchive, err)
	}
	activeItems, err := service.ListTasks(ctx, actor, projectID)
	if err != nil || len(activeItems) != 1 || activeItems[0].Task.ID != createdTask.Task.ID {
		t.Fatalf("archived strategy leaked into active list: items=%#v err=%v", activeItems, err)
	}
	strategyDraft, duplicate, err = service.RestoreStrategy(
		ctx, actor, contract.IdempotencyKey("restore_strategy_"+suffix), strategyDraft.ID,
		strategy.LifecycleRequest{ExpectedVersion: archivedDraft.Version},
	)
	if err != nil || duplicate || strategyDraft.ArchivedAt != nil {
		t.Fatalf("restore strategy: duplicate=%v draft=%#v err=%v", duplicate, strategyDraft, err)
	}
	readiness, err := service.GetGenerationReadiness(ctx, actor, projectID)
	if err != nil || !readiness.Ready || readiness.GenerationMode != "deterministic" {
		t.Fatalf("generation readiness=%#v err=%v", readiness, err)
	}
	metadata, err := service.GetGenerationMetadata(ctx, actor, strategyDraft.ID)
	if err != nil {
		t.Fatalf("get generation metadata: %v", err)
	}
	if metadata.GenerationMode != "deterministic" ||
		metadata.PromptVersion != "strategy.generate.v2" ||
		len(metadata.SkillVersions) < 3 ||
		len(metadata.SkillSnapshotHashes) < 2 ||
		len(metadata.GenerationContextHash) != 64 ||
		len(metadata.OutputHash) != 64 ||
		metadata.QualityReport == nil || !metadata.QualityReport.Passed {
		t.Fatalf("generation metadata=%#v", metadata)
	}
	review, duplicate, err := service.SubmitStrategy(ctx, actor, contract.IdempotencyKey("submit_"+suffix), strategyDraft.ID, strategyDraft.Version, strategyDraft.CurrentRevision)
	if err != nil || duplicate {
		t.Fatalf("submit strategy: duplicate=%v err=%v", duplicate, err)
	}
	service.Text = &provider.Service{TextAdapter: &deepReviewTextAdapter{failFirst: true}}
	service.DeepReviewModelAlias = "cookies.text.deep_review"
	deep, duplicate, err := service.StartDeepReview(
		ctx, actor, contract.IdempotencyKey("deep_review_"+suffix), review.ID,
		strategy.StartDeepReviewRequest{ExpectedReviewStatus: "open"},
	)
	if err != nil || duplicate || deep.Analysis.Status != "pending" {
		t.Fatalf("start deep review: result=%#v duplicate=%v err=%v", deep, duplicate, err)
	}
	if err := runAgentTaskThroughRuntime(ctx, db, service, deep.AgentTask); err != nil {
		t.Fatalf("run deep review: %v", err)
	}
	deepResult, err := service.GetLatestDeepReview(ctx, actor, review.ID)
	if err != nil || deepResult.Status != "succeeded" || len(deepResult.Findings) != 1 ||
		deepResult.APIMode != provider.TextAPIResponses || !deepResult.Background {
		t.Fatalf("deep review result=%#v err=%v", deepResult, err)
	}
	service.Text = &provider.Service{TextAdapter: &deepReviewTextAdapter{alwaysFail: true}}
	failedDeep, duplicate, err := service.StartDeepReview(
		ctx, actor, contract.IdempotencyKey("deep_review_fail_"+suffix), review.ID,
		strategy.StartDeepReviewRequest{ExpectedReviewStatus: "open"},
	)
	if err != nil || duplicate {
		t.Fatalf("start failing deep review: duplicate=%v err=%v", duplicate, err)
	}
	if err := runAgentTaskThroughRuntime(ctx, db, service, failedDeep.AgentTask); err == nil {
		t.Fatal("failing deep review unexpectedly succeeded")
	}
	failedDeepResult, err := service.GetLatestDeepReview(ctx, actor, review.ID)
	if err != nil || failedDeepResult.ID != failedDeep.Analysis.ID || failedDeepResult.Status != "failed" {
		t.Fatalf("failed deep review result=%#v err=%v", failedDeepResult, err)
	}
	service.Text = nil
	strategyDraft, err = service.GetDraft(ctx, actor, strategyDraft.ID)
	if err != nil {
		t.Fatal(err)
	}
	revisionTask, duplicate, err := service.ReviseStrategy(
		ctx, actor, contract.IdempotencyKey("revise_"+suffix), strategyDraft.ID,
		strategy.ReviseRequest{
			ExpectedVersion: strategyDraft.Version,
			BaseRevision:    strategyDraft.CurrentRevision,
			Instruction:     "补充假设与缺口说明",
		},
	)
	if err != nil || duplicate {
		t.Fatalf("create revision task: duplicate=%v err=%v", duplicate, err)
	}
	if err := runAgentTaskThroughRuntime(ctx, db, service, revisionTask); err != nil {
		t.Fatalf("revise strategy: %v", err)
	}
	invalidatedReview, err := service.GetReview(ctx, actor, review.ID)
	if err != nil || invalidatedReview.Status != "invalidated" {
		t.Fatalf("review after revision=%#v err=%v", invalidatedReview, err)
	}
	strategyDraft, err = service.GetDraft(ctx, actor, strategyDraft.ID)
	if err != nil || strategyDraft.CurrentRevision != 2 ||
		strategyDraft.Revision == nil ||
		len(strategyDraft.Revision.ChangedSections) != 1 ||
		strategyDraft.Revision.ChangedSections[0] != "assumptions_and_gaps" {
		t.Fatalf("revised draft=%#v err=%v", strategyDraft, err)
	}
	review, duplicate, err = service.SubmitStrategy(
		ctx, actor, contract.IdempotencyKey("submit_revised_"+suffix),
		strategyDraft.ID, strategyDraft.Version, strategyDraft.CurrentRevision,
	)
	if err != nil || duplicate {
		t.Fatalf("submit revised strategy: duplicate=%v err=%v", duplicate, err)
	}
	strategyDraft, err = service.GetDraft(ctx, actor, strategyDraft.ID)
	if err != nil {
		t.Fatal(err)
	}
	wrongHash := contract.ContentHash("sha256:" + strings.Repeat("0", 64))
	_, _, err = service.ApproveStrategy(ctx, actor, contract.IdempotencyKey("approve_stale_"+suffix), strategyDraft.ID, strategy.ApproveRequest{
		ReviewID: review.ID, CandidateContentHash: wrongHash, ExpectedVersion: strategyDraft.Version,
	})
	if !errors.Is(err, strategy.ErrReviewStale) {
		t.Fatalf("approve stale hash error = %v", err)
	}
	published, duplicate, err := service.ApproveStrategy(ctx, actor, contract.IdempotencyKey("approve_"+suffix), strategyDraft.ID, strategy.ApproveRequest{
		ReviewID: review.ID, CandidateContentHash: review.CandidateContentHash, ExpectedVersion: strategyDraft.Version,
	})
	if err != nil || duplicate {
		t.Fatalf("approve strategy: duplicate=%v err=%v", duplicate, err)
	}
	completedTask, err := service.GetTask(ctx, actor, bundle.Task.ID)
	if err != nil || completedTask.Status != "completed" {
		t.Fatalf("task after strategy approval=%#v err=%v", completedTask, err)
	}
	stored, err := service.GetPackage(ctx, actor, projectID, published.PackageID, published.Version)
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	if !stored.ContentHash.Equal(published.ContentHash) || stored.Snapshot.StrategyRevision != 2 {
		t.Fatalf("unexpected package: %#v", stored)
	}
	handoff, err := service.GetCreativeHandoff(ctx, actor, projectID, published.PackageID, published.Version)
	if err != nil {
		t.Fatalf("get creative handoff: %v", err)
	}
	if handoff.ContractVersion != strategy.CreativeHandoffContractVersion ||
		handoff.PackageRef.PackageID != published.PackageID ||
		handoff.PackageRef.PackageVersion != published.Version ||
		!handoff.PackageRef.PackageContentHash.Equal(published.ContentHash) ||
		handoff.HandoffContentHash.Validate() != nil {
		t.Fatalf("unexpected creative handoff: %#v", handoff)
	}
	if _, err := service.GetCreativeHandoff(ctx, actor, contract.ProjectID("other_project"), published.PackageID, published.Version); !errors.Is(err, strategy.ErrProjectAccessDenied) {
		t.Fatalf("cross-project handoff read error = %v", err)
	}
	handoffPath := fmt.Sprintf(
		"/api/strategy/v1/projects/%s/strategy-packages/%s/versions/%d/creative-handoff",
		projectID, published.PackageID, published.Version,
	)
	strategyServer := strategyhttp.New(service, agent.MySQLStore{DB: db}, jobruntime.MySQLStore{DB: db})
	handoffRequest := httptest.NewRequest(http.MethodGet, handoffPath, nil)
	handoffRequest = handoffRequest.WithContext(contract.WithRequestContext(handoffRequest.Context(), contract.RequestContext{
		RequestID: "handoff_get_" + suffix, Actor: actor,
	}))
	handoffResponse := httptest.NewRecorder()
	strategyServer.ServeHTTP(handoffResponse, handoffRequest)
	if handoffResponse.Code != http.StatusOK ||
		handoffResponse.Header().Get("ETag") != `"`+string(handoff.HandoffContentHash)+`"` {
		t.Fatalf("handoff HTTP status=%d headers=%v body=%s", handoffResponse.Code, handoffResponse.Header(), handoffResponse.Body.String())
	}
	notModifiedRequest := httptest.NewRequest(http.MethodGet, handoffPath, nil)
	notModifiedRequest.Header.Set("If-None-Match", `"other", W/"`+string(handoff.HandoffContentHash)+`"`)
	notModifiedRequest = notModifiedRequest.WithContext(contract.WithRequestContext(notModifiedRequest.Context(), contract.RequestContext{
		RequestID: "handoff_not_modified_" + suffix, Actor: actor,
	}))
	notModifiedResponse := httptest.NewRecorder()
	strategyServer.ServeHTTP(notModifiedResponse, notModifiedRequest)
	if notModifiedResponse.Code != http.StatusNotModified ||
		notModifiedResponse.Header().Get("ETag") != `"`+string(handoff.HandoffContentHash)+`"` ||
		notModifiedResponse.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("handoff 304 status=%d headers=%v", notModifiedResponse.Code, notModifiedResponse.Header())
	}
	limitedActor := actor
	limitedActor.Scopes = []contract.Scope{"project.read", strategy.ScopeRead}
	forbiddenRequest := httptest.NewRequest(http.MethodGet, handoffPath, nil)
	forbiddenRequest = forbiddenRequest.WithContext(contract.WithRequestContext(forbiddenRequest.Context(), contract.RequestContext{
		RequestID: "handoff_forbidden_" + suffix, Actor: limitedActor,
	}))
	forbiddenResponse := httptest.NewRecorder()
	strategyServer.ServeHTTP(forbiddenResponse, forbiddenRequest)
	if forbiddenResponse.Code != http.StatusForbidden {
		t.Fatalf("handoff forbidden status=%d body=%s", forbiddenResponse.Code, forbiddenResponse.Body.String())
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM strategy_creative_handoffs
		WHERE organization_id = ? AND project_id = ? AND package_id = ? AND package_version = ?`,
		organizationID, projectID, published.PackageID, published.Version); err != nil {
		t.Fatal(err)
	}
	backfilled, err := strategy.BackfillCreativeHandoffsForProject(ctx, db, organizationID, projectID)
	if err != nil || backfilled != 1 {
		t.Fatalf("backfill handoff: inserted=%d err=%v", backfilled, err)
	}
	backfilledHandoff, err := service.GetCreativeHandoff(ctx, actor, projectID, published.PackageID, published.Version)
	if err != nil || !backfilledHandoff.HandoffContentHash.Equal(handoff.HandoffContentHash) {
		t.Fatalf("backfilled handoff=%#v err=%v", backfilledHandoff, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE strategy_creative_handoffs
		SET snapshot = JSON_SET(snapshot, '$.creative_view.objective.statement', 'tampered')
		WHERE organization_id = ? AND project_id = ? AND package_id = ? AND package_version = ?`,
		organizationID, projectID, published.PackageID, published.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetCreativeHandoff(ctx, actor, projectID, published.PackageID, published.Version); !errors.Is(err, strategy.ErrInvalidState) {
		t.Fatalf("tampered handoff read error = %v", err)
	}
	restoredHandoffJSON, err := json.Marshal(backfilledHandoff)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE strategy_creative_handoffs SET snapshot = ?
		WHERE organization_id = ? AND project_id = ? AND package_id = ? AND package_version = ?`,
		restoredHandoffJSON, organizationID, projectID, published.PackageID, published.Version); err != nil {
		t.Fatal(err)
	}
	feedback, duplicate, err := service.CreateFeedback(
		ctx, actor, projectID, contract.IdempotencyKey("feedback_"+suffix),
		strategy.CreateFeedbackRequest{
			TargetType: "strategy_package", TargetID: published.PackageID,
			TargetVersion: published.Version, Rating: "useful", Comment: "策略结构可执行",
		},
	)
	if err != nil || duplicate || feedback.TargetID != published.PackageID {
		t.Fatalf("create feedback: feedback=%#v duplicate=%v err=%v", feedback, duplicate, err)
	}
	if _, _, err := service.CreateFeedback(
		ctx, actor, projectID, contract.IdempotencyKey("feedback_invalid_"+suffix),
		strategy.CreateFeedbackRequest{
			TargetType: "strategy_package", TargetID: "missing_package",
			TargetVersion: 1, Rating: "not_useful",
		},
	); !errors.Is(err, strategy.ErrInvalidRequest) {
		t.Fatalf("invalid feedback target error = %v", err)
	}
	creativeService := creative.Service{
		Repository: creative.MySQLRepository{DB: db},
		Projects:   projectService,
		StrategyPackages: strategycreative.Reader{
			Service: service,
		},
	}
	intake, err := creativeService.CreateIntake(
		ctx,
		contract.RequestContext{RequestID: "creative_request_" + suffix, TraceID: "creative_trace_" + suffix, Actor: actor},
		projectID,
		contract.IdempotencyKey("creative_intake_"+suffix),
		creative.CreateIntakeRequest{
			Source: creative.IntakeSourceStrategyPackage,
			StrategyPackage: &creative.StrategyPackageReference{
				PackageID:           published.PackageID,
				PackageVersion:      published.Version,
				ExpectedContentHash: string(published.ContentHash),
			},
		},
	)
	if err != nil || intake.Status != creative.IntakeReady ||
		intake.Request.StrategyPackage == nil ||
		intake.Request.StrategyPackage.PackageID != published.PackageID {
		t.Fatalf("Strategy package handoff: intake=%#v err=%v", intake, err)
	}
	replayed, duplicate, err := service.ApproveStrategy(ctx, actor, contract.IdempotencyKey("approve_"+suffix), strategyDraft.ID, strategy.ApproveRequest{
		ReviewID: review.ID, CandidateContentHash: review.CandidateContentHash, ExpectedVersion: strategyDraft.Version,
	})
	if err != nil || !duplicate || !replayed.ContentHash.Equal(published.ContentHash) {
		t.Fatalf("idempotent approve: duplicate=%v package=%#v err=%v", duplicate, replayed, err)
	}
	if _, err := service.GetPackage(ctx, actor, contract.ProjectID("other_project"), published.PackageID, 1); !errors.Is(err, strategy.ErrProjectAccessDenied) {
		t.Fatalf("cross-project package read error = %v", err)
	}

	approvedDraft, err := service.GetDraft(ctx, actor, strategyDraft.ID)
	if err != nil {
		t.Fatal(err)
	}
	invalidObjective, _ := json.Marshal(strings.Repeat("目", 1001))
	invalidDraft, duplicate, err := service.PatchStrategy(ctx, actor, contract.IdempotencyKey("strategy_patch_invalid_handoff_"+suffix), approvedDraft.ID, strategy.StrategySectionPatch{
		ExpectedVersion: approvedDraft.Version, BaseRevision: approvedDraft.CurrentRevision,
		Section: "objective", Value: invalidObjective,
	})
	if err != nil || duplicate {
		t.Fatalf("patch invalid handoff candidate: duplicate=%v err=%v", duplicate, err)
	}
	invalidReview, duplicate, err := service.SubmitStrategy(ctx, actor, contract.IdempotencyKey("submit_invalid_handoff_"+suffix), invalidDraft.ID, invalidDraft.Version, invalidDraft.CurrentRevision)
	if err != nil || duplicate {
		t.Fatalf("submit invalid handoff candidate: duplicate=%v err=%v", duplicate, err)
	}
	invalidReady, err := service.GetDraft(ctx, actor, invalidDraft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.ApproveStrategy(ctx, actor, contract.IdempotencyKey("approve_invalid_handoff_"+suffix), invalidReady.ID, strategy.ApproveRequest{
		ReviewID: invalidReview.ID, CandidateContentHash: invalidReview.CandidateContentHash, ExpectedVersion: invalidReady.Version,
	}); !errors.Is(err, strategy.ErrInvalidRequest) {
		t.Fatalf("invalid handoff approval error = %v", err)
	}
	if _, err := service.GetPackage(ctx, actor, projectID, published.PackageID, 2); !errors.Is(err, strategy.ErrNotFound) {
		t.Fatalf("failed handoff approval committed package v2: %v", err)
	}
	stillOpen, err := service.GetReview(ctx, actor, invalidReview.ID)
	if err != nil || stillOpen.Status != "open" {
		t.Fatalf("failed handoff approval changed review: %#v err=%v", stillOpen, err)
	}
	if _, err := service.ReturnReview(ctx, actor, invalidReview.ID, "修正交接字段长度"); err != nil {
		t.Fatalf("return invalid handoff review: %v", err)
	}
	returnedDraft, err := service.GetDraft(ctx, actor, invalidDraft.ID)
	if err != nil {
		t.Fatal(err)
	}
	changedDraft, duplicate, err := service.PatchStrategy(ctx, actor, contract.IdempotencyKey("strategy_patch_v2_"+suffix), returnedDraft.ID, strategy.StrategySectionPatch{
		ExpectedVersion: returnedDraft.Version, BaseRevision: returnedDraft.CurrentRevision,
		Section: "objective", Value: json.RawMessage(`"updated objective"`),
	})
	if err != nil || duplicate {
		t.Fatalf("patch approved strategy: duplicate=%v err=%v", duplicate, err)
	}
	reviewV2, duplicate, err := service.SubmitStrategy(ctx, actor, contract.IdempotencyKey("submit_v2_"+suffix), changedDraft.ID, changedDraft.Version, changedDraft.CurrentRevision)
	if err != nil || duplicate {
		t.Fatalf("submit strategy v2: duplicate=%v err=%v", duplicate, err)
	}
	readyV2, err := service.GetDraft(ctx, actor, changedDraft.ID)
	if err != nil {
		t.Fatal(err)
	}
	publishedV2, duplicate, err := service.ApproveStrategy(ctx, actor, contract.IdempotencyKey("approve_v2_"+suffix), readyV2.ID, strategy.ApproveRequest{
		ReviewID: reviewV2.ID, CandidateContentHash: reviewV2.CandidateContentHash, ExpectedVersion: readyV2.Version,
	})
	if err != nil || duplicate {
		t.Fatalf("approve strategy v2: duplicate=%v err=%v", duplicate, err)
	}
	if publishedV2.PackageID != published.PackageID || publishedV2.Version != 2 {
		t.Fatalf("package series changed: v1=%s v2=%s/%d", published.PackageID, publishedV2.PackageID, publishedV2.Version)
	}
	storedV1, err := service.GetPackage(ctx, actor, projectID, published.PackageID, 1)
	if err != nil || storedV1.Status != "superseded" || !storedV1.ContentHash.Equal(published.ContentHash) {
		t.Fatalf("immutable v1 after supersession: %#v err=%v", storedV1, err)
	}
	handoffV1, err := service.GetCreativeHandoff(ctx, actor, projectID, published.PackageID, 1)
	if err != nil || !handoffV1.HandoffContentHash.Equal(handoff.HandoffContentHash) {
		t.Fatalf("immutable handoff v1 after supersession: %#v err=%v", handoffV1, err)
	}
	handoffV2, err := service.GetCreativeHandoff(ctx, actor, projectID, published.PackageID, 2)
	if err != nil || handoffV2.PackageRef.PackageVersion != 2 ||
		handoffV2.HandoffContentHash.Equal(handoffV1.HandoffContentHash) {
		t.Fatalf("isolated handoff v2: %#v err=%v", handoffV2, err)
	}
	var outboxCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM event_outbox WHERE organization_id = ?
		AND event_type = 'strategy.approved.v1'`, organizationID).Scan(&outboxCount); err != nil || outboxCount != 2 {
		t.Fatalf("outbox count=%d err=%v", outboxCount, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM event_outbox WHERE organization_id = ?
		AND event_type = 'strategy.superseded.v1'`, organizationID).Scan(&outboxCount); err != nil || outboxCount != 1 {
		t.Fatalf("superseded outbox count=%d err=%v", outboxCount, err)
	}
	publisher := &recordingPublisher{}
	dispatcher := eventoutbox.Dispatcher{DB: db, Publisher: publisher}
	for range 3 {
		processed, err := dispatcher.RunOnce(ctx)
		if err != nil || !processed {
			t.Fatalf("dispatch outbox: processed=%v err=%v", processed, err)
		}
	}
	if len(publisher.events) != 3 {
		t.Fatalf("published event count = %d", len(publisher.events))
	}
	consumeTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := eventoutbox.ConsumeOnce(ctx, consumeTx, "strategy_it_consumer", publisher.events[0].ID, time.Now())
	if err != nil || !first {
		consumeTx.Rollback()
		t.Fatalf("first consume: first=%v err=%v", first, err)
	}
	first, err = eventoutbox.ConsumeOnce(ctx, consumeTx, "strategy_it_consumer", publisher.events[0].ID, time.Now())
	if err != nil || first {
		consumeTx.Rollback()
		t.Fatalf("duplicate consume: first=%v err=%v", first, err)
	}
	if err := consumeTx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func runAgentTaskThroughRuntime(ctx context.Context, db *sql.DB, service strategy.Service, task agent.Task) error {
	runtimeStore := jobruntime.MySQLStore{DB: db}
	dispatcher := agent.Dispatcher{DB: db, Jobs: runtimeStore}
	processed, err := dispatcher.RunOnce(ctx)
	if err != nil {
		return err
	}
	if !processed {
		return errors.New("agent dispatch was not processed")
	}
	var domainErr error
	handler := agent.RuntimeHandlerWithFinalFailure(agent.MySQLStore{DB: db}, func(ctx context.Context, task agent.Task) (*contract.ResourceRef, error) {
		ref, err := service.HandleAgentTask(ctx, task)
		if err != nil {
			domainErr = err
		}
		return ref, err
	}, service.HandleAgentTaskFinalFailure, runtimeStore)
	worker := jobruntime.Worker{
		Store: runtimeStore, Handlers: map[string]jobruntime.Handler{task.Kind: handler},
		Canceller: runtimeStore,
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		processed, err = worker.RunOnce(ctx, "strategy-integration")
		if err != nil {
			return err
		}
		if !processed {
			time.Sleep(250 * time.Millisecond)
			continue
		}
		completed, err := (agent.MySQLStore{DB: db}).Get(ctx, task.OrganizationID, task.ProjectID, task.ID)
		if err != nil {
			return err
		}
		if completed.Status == agent.TaskSucceeded {
			return nil
		}
		if completed.Status == agent.TaskFailed || completed.Status == agent.TaskCancelled {
			return fmt.Errorf("agent task did not succeed: status=%s error=%#v domain_error=%v", completed.Status, completed.Error, domainErr)
		}
	}
	completed, err := (agent.MySQLStore{DB: db}).Get(ctx, task.OrganizationID, task.ProjectID, task.ID)
	if err != nil {
		return err
	}
	return fmt.Errorf("agent task did not complete: status=%s error=%#v", completed.Status, completed.Error)
}

type recordingPublisher struct {
	events []eventoutbox.Event
}

type deepReviewTextAdapter struct {
	failFirst  bool
	alwaysFail bool
	calls      int
}

func (*deepReviewTextAdapter) InspectTextRoute(
	context.Context, contract.OrganizationID, string,
) (provider.TextRouteInspection, error) {
	return provider.TextRouteInspection{
		ModelAlias: "cookies.text.deep_review", UpstreamModel: "gpt-5.5-pro",
		RouteRevisionID: "deep-review-r1", ResponseMode: provider.TextResponseJSONSchema,
		APIMode: provider.TextAPIResponses, Background: true, Ready: true,
	}, nil
}

func (a *deepReviewTextAdapter) GenerateText(
	context.Context, provider.TextAdapterRequest,
) (provider.SynchronousResult, error) {
	a.calls++
	if a.alwaysFail || (a.failFirst && a.calls == 1) {
		return provider.SynchronousResult{}, provider.ExecutionError{JobError: contract.JobError{
			Code: "MODEL_RATE_LIMITED", Message: "retry deep review", Retryable: true,
		}}
	}
	output := json.RawMessage(`{"summary":"候选策略具备基础可执行性。","findings":[{"severity":"warning","section":"measurement","title":"指标需要收敛","detail":"核心指标与渠道指标的归因窗口尚未明确。","recommendation":"补充统一归因窗口与渠道分解口径。"}]}`)
	return provider.SynchronousResult{
		ProviderCode: "adapter_gateway", ModelVersion: "gpt-5.5-pro",
		Text: string(output), StructuredOutput: output,
		Usage: &provider.TokenUsage{InputTokens: 100, OutputTokens: 40, TotalTokens: 140},
		RouteSnapshot: &provider.GatewayRouteSnapshot{
			RouteRevisionID: "deep-review-r1", TextResponseMode: provider.TextResponseJSONSchema,
			TextAPIMode: provider.TextAPIResponses, Background: true,
		},
	}, nil
}

func (p *recordingPublisher) Publish(_ context.Context, event eventoutbox.Event) error {
	p.events = append(p.events, event)
	return nil
}

func cleanupStrategyIntegration(t *testing.T, db *sql.DB, organizationID contract.OrganizationID, userID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	statements := []string{
		"DELETE FROM event_consumptions WHERE event_id IN (SELECT event_id FROM event_outbox WHERE organization_id=?)",
		"DELETE FROM creative_intakes WHERE organization_id=?",
		"DELETE FROM strategy_feedback WHERE organization_id=?",
		"DELETE FROM strategy_compliance_reports WHERE organization_id=?",
		"DELETE FROM strategy_conversation_memories WHERE organization_id=?",
		"DELETE FROM strategy_review_comments WHERE organization_id=?",
		"DELETE FROM strategy_review_analyses WHERE organization_id=?",
		"DELETE FROM strategy_review_assignments WHERE organization_id=?",
		"DELETE FROM strategy_creative_handoffs WHERE organization_id=?",
		"DELETE FROM strategy_package_versions WHERE organization_id=?",
		"DELETE FROM strategy_packages WHERE organization_id=?",
		"DELETE FROM strategy_reviews WHERE organization_id=?",
		"DELETE FROM strategy_review_policies WHERE organization_id=?",
		"DELETE FROM strategy_draft_revisions WHERE organization_id=?",
		"DELETE FROM strategy_drafts WHERE organization_id=?",
		"DELETE FROM strategy_brief_versions WHERE organization_id=?",
		"DELETE FROM strategy_brief_revisions WHERE organization_id=?",
		"DELETE FROM strategy_brief_drafts WHERE organization_id=?",
		"DELETE FROM strategy_briefs WHERE organization_id=?",
		"DELETE FROM strategy_messages WHERE organization_id=?",
		"DELETE FROM strategy_conversation_events WHERE organization_id=?",
		"DELETE FROM strategy_tasks WHERE organization_id=?",
		"DELETE FROM strategy_conversations WHERE organization_id=?",
		"DELETE FROM strategy_workspaces WHERE organization_id=?",
		"DELETE FROM strategy_write_receipts WHERE organization_id=?",
		"DELETE FROM platform_skill_runs WHERE organization_id=?",
		"DELETE FROM platform_agent_dispatches WHERE organization_id=?",
		"DELETE FROM platform_agent_tasks WHERE organization_id=?",
		"DELETE FROM event_outbox WHERE organization_id=?",
		"DELETE FROM platform_jobs WHERE organization_id=?",
		"DELETE FROM project_context_versions WHERE organization_id=?",
		"DELETE FROM project_products WHERE organization_id=?",
		"DELETE FROM project_memberships WHERE organization_id=?",
		"DELETE FROM platform_project_workbench_material_confirmations WHERE organization_id=?",
		"DELETE FROM platform_project_workbench_quality_checks WHERE organization_id=?",
		"DELETE FROM platform_project_workbench_asset_pointers WHERE organization_id=?",
		"DELETE FROM platform_project_workbench_ad_accounts WHERE organization_id=?",
		"DELETE FROM platform_project_workbenches WHERE organization_id=?",
		"DELETE FROM platform_project_runtimes WHERE organization_id=?",
		"DELETE FROM projects WHERE organization_id=?",
		"DELETE FROM brand_guideline_versions WHERE organization_id=?",
		"DELETE FROM products WHERE organization_id=?",
		"DELETE FROM brands WHERE organization_id=?",
		"DELETE FROM service_identity_scopes WHERE organization_id=?",
		"DELETE FROM service_identities WHERE organization_id=?",
		"DELETE FROM organization_memberships WHERE organization_id=?",
		"DELETE FROM organizations WHERE id=?",
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement, organizationID); err != nil {
			t.Errorf("cleanup %q: %v", statement, err)
		}
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id=?", userID); err != nil {
		t.Errorf("cleanup user: %v", err)
	}
}
