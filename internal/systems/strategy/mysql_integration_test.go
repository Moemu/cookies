package strategy_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/shikanon/cookies/internal/systems/creative"
	"github.com/shikanon/cookies/internal/systems/strategy"
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
	stored, err := service.GetPackage(ctx, actor, projectID, published.PackageID, published.Version)
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	if !stored.ContentHash.Equal(published.ContentHash) || stored.Snapshot.StrategyRevision != 2 {
		t.Fatalf("unexpected package: %#v", stored)
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
	changedDraft, duplicate, err := service.PatchStrategy(ctx, actor, contract.IdempotencyKey("strategy_patch_v2_"+suffix), approvedDraft.ID, strategy.StrategySectionPatch{
		ExpectedVersion: approvedDraft.Version, BaseRevision: approvedDraft.CurrentRevision,
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
	handler := agent.RuntimeHandler(agent.MySQLStore{DB: db}, func(ctx context.Context, task agent.Task) (*contract.ResourceRef, error) {
		ref, err := service.HandleAgentTask(ctx, task)
		if err != nil {
			domainErr = err
		}
		return ref, err
	}, runtimeStore)
	worker := jobruntime.Worker{
		Store: runtimeStore, Handlers: map[string]jobruntime.Handler{task.Kind: handler},
		Canceller: runtimeStore,
	}
	deadline := time.Now().Add(8 * time.Second)
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
		"DELETE FROM strategy_package_versions WHERE organization_id=?",
		"DELETE FROM strategy_packages WHERE organization_id=?",
		"DELETE FROM strategy_reviews WHERE organization_id=?",
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
