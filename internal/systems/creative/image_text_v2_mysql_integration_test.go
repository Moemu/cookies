package creative

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/project"
)

func TestImageTextMySQLConcurrencyAndRecovery(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}
	for _, table := range []string{
		"creative_image_prompt_packages",
		"creative_image_generation_attempts",
		"creative_image_slot_selections",
	} {
		var found string
		if err := db.QueryRowContext(ctx, `SELECT TABLE_NAME FROM information_schema.TABLES
			WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`, table).Scan(&found); err != nil {
			t.Fatalf("creative image-text migrations must be applied before integration test (%s): %v", table, err)
		}
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	organizationID := contract.OrganizationID("org_creative_it_" + suffix)
	projectID := contract.ProjectID("project_creative_it_" + suffix)
	userID := "user_creative_it_" + suffix
	intakeID := "intake_creative_it_" + suffix
	taskID := "task_creative_it_" + suffix
	actor := contract.ActorContext{
		OrganizationID: organizationID,
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: userID},
		Scopes:         []contract.Scope{"project.read", "project.write", ScopeRead, ScopeWrite},
	}
	t.Cleanup(func() {
		cleanupImageTextIntegration(t, db, organizationID, userID)
	})
	if err := (identity.MySQLStore{DB: db}).EnsureLocalActor(ctx, actor); err != nil {
		t.Fatalf("seed actor: %v", err)
	}
	if err := (project.MySQLStore{DB: db}).EnsureLocalProject(ctx, actor, projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	repository := MySQLRepository{DB: db}
	now := time.Now().UTC().Truncate(time.Microsecond)
	inputIdentityHash := "sha256:" + fmt.Sprintf("%064x", 1)
	directionHash := "sha256:" + fmt.Sprintf("%064x", 2)
	intake := CreativeIntake{
		ContractVersion: CreativeIntakeV3ContractVersion,
		ID:              intakeID,
		OrganizationID:  organizationID,
		ProjectID:       projectID,
		Source:          IntakeSourceManual,
		Status:          IntakeReady,
		Request: CreateIntakeRequest{
			ContractVersion: CreativeIntakeCreateV3ContractVersion,
			Source:          IntakeSourceManual,
			Channel:         ChannelXiaohongshu,
			Objective:       "验证图文并发和恢复",
			Audience:        "研发负责人",
			CoreMessage:     "制造能力可验证",
			Tone:            []string{"专业"},
			VisualKeywords:  []string{"精密制造"},
			Mandatory:       []string{},
			Prohibited:      []string{},
		},
		MissingFields:     []string{},
		Warnings:          []string{},
		ConfirmedBy:       userID,
		Principal:         actor.Principal,
		IdempotencyKey:    contract.IdempotencyKey("intake_" + suffix),
		RequestHash:       fmt.Sprintf("%064x", 3),
		InputIdentityHash: inputIdentityHash,
		Version:           1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if _, duplicate, err := repository.CreateIntake(ctx, intake); err != nil || duplicate {
		t.Fatalf("create intake: duplicate=%v err=%v", duplicate, err)
	}
	direction := CreativeDirection{
		DirectionVersionID: "direction_" + suffix,
		InputIdentityHash:  inputIdentityHash,
		ContentType:        ContentTypeCustom,
		Focus:              "制造证据",
		Audience:           "研发负责人",
		CoreMessage:        "制造能力可验证",
		Concept:            "看得见的精度",
		Tone:               []string{"专业"},
		VisualKeywords:     []string{"精密制造"},
	}
	task := CreativeTask{
		ID:             taskID,
		OrganizationID: organizationID,
		ProjectID:      projectID,
		IntakeID:       intakeID,
		Format:         FormatImageText,
		Channel:        ChannelXiaohongshu,
		Status:         TaskDraft,
		Direction:      direction,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	initialDraft := imageTextIntegrationDraft(taskID, 1, direction.DirectionVersionID, directionHash, inputIdentityHash, now)
	if _, err := repository.CreateTask(ctx, task, initialDraft); err != nil {
		t.Fatalf("create image-text task: %v", err)
	}

	nextDraft := imageTextIntegrationDraft(taskID, 2, direction.DirectionVersionID, directionHash, inputIdentityHash, now.Add(time.Second))
	type saveResult struct {
		task CreativeTask
		err  error
	}
	start := make(chan struct{})
	results := make(chan saveResult, 2)
	var writers sync.WaitGroup
	for range 2 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			<-start
			storedTask, _, saveErr := repository.SaveImageTextDraft(
				ctx, organizationID, projectID, taskID, 1, 1,
				nextDraft, TaskInProgress, now.Add(time.Second),
			)
			results <- saveResult{task: storedTask, err: saveErr}
		}()
	}
	close(start)
	writers.Wait()
	close(results)
	successes := 0
	conflicts := 0
	for result := range results {
		switch {
		case result.err == nil:
			successes++
			if result.task.Version != 2 || result.task.Status != TaskInProgress {
				t.Fatalf("concurrent winner task=%+v", result.task)
			}
		case errors.Is(result.err, ErrVersionConflict):
			conflicts++
		default:
			t.Fatalf("concurrent save returned unexpected error: %v", result.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent saves: successes=%d conflicts=%d", successes, conflicts)
	}
	assertImageTextDraftCount(t, ctx, db, organizationID, taskID, 2, 1)

	attemptIDs := make([]string, 0, 3)
	for order := 1; order <= 3; order++ {
		prompt := imageTextIntegrationPrompt(
			suffix, order, organizationID, projectID, taskID, direction.DirectionVersionID,
			directionHash, inputIdentityHash, userID, now.Add(time.Duration(order)*time.Minute),
		)
		storedPrompt, duplicate, err := repository.CreateImagePromptPackage(ctx, prompt)
		if err != nil || duplicate {
			t.Fatalf("create prompt %d: duplicate=%v err=%v", order, duplicate, err)
		}
		if order == 1 {
			replay := prompt
			replay.ID += "_replay"
			replayed, replayDuplicate, replayErr := repository.CreateImagePromptPackage(ctx, replay)
			if replayErr != nil || !replayDuplicate || replayed.ID != storedPrompt.ID {
				t.Fatalf("prompt replay: duplicate=%v prompt=%+v err=%v", replayDuplicate, replayed, replayErr)
			}
		}

		taskVersion, _ := imageTextIntegrationTaskState(t, ctx, db, organizationID, projectID, taskID)
		attempt := imageTextIntegrationAttempt(
			suffix, order, 1, organizationID, projectID, taskID, prompt.ID,
			taskVersion, userID, now.Add(time.Duration(order)*time.Minute),
		)
		storedAttempt, duplicate, err := repository.CreateImageGenerationAttempt(ctx, attempt)
		if err != nil || duplicate {
			t.Fatalf("create attempt %d: duplicate=%v err=%v", order, duplicate, err)
		}
		attemptIDs = append(attemptIDs, storedAttempt.ID)
		if order == 1 {
			replayed, replayDuplicate, replayErr := repository.CreateImageGenerationAttempt(ctx, attempt)
			if replayErr != nil || !replayDuplicate || replayed.ID != storedAttempt.ID {
				t.Fatalf("attempt replay: duplicate=%v attempt=%+v err=%v", replayDuplicate, replayed, replayErr)
			}
			conflict := attempt
			conflict.RequestHash = fmt.Sprintf("%064x", 9001)
			if _, _, conflictErr := repository.CreateImageGenerationAttempt(ctx, conflict); !errors.Is(conflictErr, ErrIdempotencyConflict) {
				t.Fatalf("attempt idempotency conflict error=%v", conflictErr)
			}
			if _, scopeErr := repository.GetImageGenerationAttempt(
				ctx, organizationID, contract.ProjectID("other_"+suffix), storedAttempt.ID,
			); !errors.Is(scopeErr, ErrNotFound) {
				t.Fatalf("cross-project attempt read error=%v, want ErrNotFound", scopeErr)
			}
		}

		providerJobID := fmt.Sprintf("provider_%s_%d", suffix, order)
		attached, err := repository.AttachImageProviderJob(
			ctx, organizationID, projectID, storedAttempt.ID, providerJobID, now.Add(time.Duration(order)*time.Minute),
		)
		if err != nil || attached.Status != ImageAttemptRunning {
			t.Fatalf("attach provider job %d: attempt=%+v err=%v", order, attached, err)
		}
		if _, err := repository.AttachImageProviderJob(
			ctx, organizationID, projectID, storedAttempt.ID, providerJobID, now.Add(time.Duration(order)*time.Minute),
		); err != nil {
			t.Fatalf("duplicate provider callback %d: %v", order, err)
		}
		if order == 1 {
			if _, err := repository.AttachImageProviderJob(
				ctx, organizationID, projectID, storedAttempt.ID, "other_provider_"+suffix, now,
			); !errors.Is(err, ErrProviderJobConflict) {
				t.Fatalf("conflicting provider callback error=%v", err)
			}
		}

		baseRef := contract.AssetVersionRef{
			AssetID: contract.AssetID(fmt.Sprintf("asset_base_%s_%d", suffix, order)),
			Version: 1,
		}
		if _, err := repository.MarkImageAttemptBaseReady(
			ctx, organizationID, projectID, storedAttempt.ID, baseRef, now.Add(time.Duration(order)*time.Minute),
		); err != nil {
			t.Fatalf("base asset ready %d: %v", order, err)
		}
		finalRef := contract.AssetVersionRef{
			AssetID: contract.AssetID(fmt.Sprintf("asset_final_%s_%d", suffix, order)),
			Version: 1,
		}
		succeeded, err := repository.MarkImageAttemptFinalReady(
			ctx, organizationID, projectID, storedAttempt.ID,
			fmt.Sprintf("render_%s_%d", suffix, order), finalRef, now.Add(time.Duration(order)*time.Minute),
		)
		if err != nil || succeeded.Status != ImageAttemptSucceeded || succeeded.FinalAssetRef == nil ||
			*succeeded.FinalAssetRef != finalRef {
			t.Fatalf("final asset ready %d: attempt=%+v err=%v", order, succeeded, err)
		}
		if _, err := repository.MarkImageAttemptFinalReady(
			ctx, organizationID, projectID, storedAttempt.ID,
			fmt.Sprintf("render_%s_%d", suffix, order), finalRef, now.Add(time.Duration(order)*time.Minute),
		); err != nil {
			t.Fatalf("duplicate final callback %d: %v", order, err)
		}
		if order == 1 {
			wrongRef := contract.AssetVersionRef{AssetID: contract.AssetID("wrong_" + suffix), Version: 1}
			if _, err := repository.MarkImageAttemptFinalReady(
				ctx, organizationID, projectID, storedAttempt.ID, "wrong_render_"+suffix, wrongRef, now,
			); !errors.Is(err, ErrInvalidState) {
				t.Fatalf("old callback overwrote final result: %v", err)
			}
		}
		recoverable, err := repository.ListActiveImageGenerationAttempts(ctx, 100)
		if err != nil {
			t.Fatalf("list recoverable attempts before adoption %d: %v", order, err)
		}
		if !containsImageAttempt(recoverable, storedAttempt.ID) {
			t.Fatalf("succeeded attempt %s disappeared before first adoption", storedAttempt.ID)
		}

		taskVersion, _ = imageTextIntegrationTaskState(t, ctx, db, organizationID, projectID, taskID)
		selection, err := repository.AdoptImageGenerationAttempt(
			ctx, organizationID, projectID, taskID, taskVersion, int64(order),
			storedAttempt.ID, 0, userID, now.Add(time.Duration(order)*time.Minute),
		)
		if err != nil || selection.Version != 1 || selection.AdoptedAttemptID != storedAttempt.ID {
			t.Fatalf("adopt attempt %d: selection=%+v err=%v", order, selection, err)
		}
		if order < 3 {
			recoverable, err = repository.ListActiveImageGenerationAttempts(ctx, 100)
			if err != nil {
				t.Fatalf("list recoverable attempts after adoption %d: %v", order, err)
			}
			if containsImageAttempt(recoverable, storedAttempt.ID) {
				t.Fatalf("adopted attempt %s remained in recovery queue before all slots were selected", storedAttempt.ID)
			}
		}
		if order == 1 {
			if _, err := repository.AdoptImageGenerationAttempt(
				ctx, organizationID, contract.ProjectID("other_"+suffix), taskID,
				selection.Version, 1, storedAttempt.ID, 0, userID, now,
			); err == nil {
				t.Fatal("cross-project attempt adoption succeeded")
			}
			taskVersion, _ = imageTextIntegrationTaskState(t, ctx, db, organizationID, projectID, taskID)
			staleAttempt := imageTextIntegrationAttempt(
				suffix, 1, 2, organizationID, projectID, taskID, prompt.ID,
				taskVersion, userID, now.Add(10*time.Minute),
			)
			staleAttempt.ID += "_stale"
			staleAttempt.IdempotencyKey = contract.IdempotencyKey("attempt_stale_" + suffix)
			staleAttempt.RequestHash = fmt.Sprintf("%064x", 8001)
			if _, _, err := repository.CreateImageGenerationAttempt(ctx, staleAttempt); err != nil {
				t.Fatalf("create stale retry: %v", err)
			}
			stale, err := repository.MarkImageAttemptStale(
				ctx, organizationID, projectID, staleAttempt.ID, "draft_revision_changed", now.Add(11*time.Minute),
			)
			if err != nil || stale.Status != ImageAttemptStale {
				t.Fatalf("mark retry stale: attempt=%+v err=%v", stale, err)
			}
			if _, err := repository.MarkImageAttemptFinalReady(
				ctx, organizationID, projectID, staleAttempt.ID, "late_render_"+suffix,
				contract.AssetVersionRef{AssetID: contract.AssetID("late_" + suffix), Version: 1},
				now.Add(12*time.Minute),
			); !errors.Is(err, ErrInvalidState) {
				t.Fatalf("stale callback error=%v, want ErrInvalidState", err)
			}
		}

		if order == 2 {
			beforeVersion, beforeStatus := imageTextIntegrationTaskState(t, ctx, db, organizationID, projectID, taskID)
			if _, _, err := repository.FinalizeImageTextDraftAssets(
				ctx, organizationID, projectID, taskID, beforeVersion, 2, userID, now.Add(20*time.Minute),
			); !errors.Is(err, ErrInvalidState) {
				t.Fatalf("two-slot materialization error=%v, want ErrInvalidState", err)
			}
			afterVersion, afterStatus := imageTextIntegrationTaskState(t, ctx, db, organizationID, projectID, taskID)
			if afterVersion != beforeVersion || afterStatus != beforeStatus {
				t.Fatalf("failed materialization changed task: before=%d/%s after=%d/%s",
					beforeVersion, beforeStatus, afterVersion, afterStatus)
			}
			assertImageTextDraftCount(t, ctx, db, organizationID, taskID, 3, 0)
			assertImageTextOutboxCount(t, ctx, db, organizationID, taskID, 0)
		}
	}

	taskVersion, _ := imageTextIntegrationTaskState(t, ctx, db, organizationID, projectID, taskID)
	readyTask, materialized, err := repository.FinalizeImageTextDraftAssets(
		ctx, organizationID, projectID, taskID, taskVersion, 2, userID, now.Add(30*time.Minute),
	)
	if err != nil || readyTask.Status != TaskReady || materialized.Version != 3 ||
		materialized.GenerationSourceVersion == nil || *materialized.GenerationSourceVersion != 2 {
		t.Fatalf("materialize three slots: task=%+v draft=%+v err=%v", readyTask, materialized, err)
	}
	for index, item := range materialized.ImagePlan {
		if item.AssetRef == nil || item.AssetRef.AssetID != contract.AssetID(fmt.Sprintf("asset_final_%s_%d", suffix, index+1)) {
			t.Fatalf("materialized slot %d asset=%+v", index+1, item.AssetRef)
		}
	}
	assertImageTextDraftCount(t, ctx, db, organizationID, taskID, 3, 1)
	assertImageTextOutboxCount(t, ctx, db, organizationID, taskID, 1)

	replayedTask, replayedDraft, err := repository.FinalizeImageTextDraftAssets(
		ctx, organizationID, projectID, taskID, readyTask.Version, 2, userID, now.Add(31*time.Minute),
	)
	if err != nil || replayedTask.Version != readyTask.Version || replayedDraft.Version != materialized.Version {
		t.Fatalf("materialization replay: task=%+v draft=%+v err=%v", replayedTask, replayedDraft, err)
	}
	assertImageTextDraftCount(t, ctx, db, organizationID, taskID, 3, 1)
	assertImageTextOutboxCount(t, ctx, db, organizationID, taskID, 1)

	selections, err := repository.ListImageSlotSelections(ctx, organizationID, projectID, taskID, 2)
	if err != nil || len(selections) != 3 {
		t.Fatalf("slot selections=%+v err=%v", selections, err)
	}
	for index, selection := range selections {
		if selection.AdoptedAttemptID != attemptIDs[index] {
			t.Fatalf("selection %d adopted=%s, want=%s", index+1, selection.AdoptedAttemptID, attemptIDs[index])
		}
	}
}

func containsImageAttempt(values []ImageGenerationAttempt, attemptID string) bool {
	for _, value := range values {
		if value.ID == attemptID {
			return true
		}
	}
	return false
}

func imageTextIntegrationDraft(
	taskID string,
	version int64,
	directionID string,
	directionHash string,
	inputIdentityHash string,
	now time.Time,
) ImageTextDraft {
	return ImageTextDraft{
		ContractVersion:   ImageTextDraftV2Contract,
		TaskID:            taskID,
		Version:           version,
		DirectionRef:      &ImageTextDirectionRef{DirectionID: directionID, ContentHash: directionHash},
		InputIdentityHash: inputIdentityHash,
		Status:            "draft",
		TitleCandidates:   []string{"看得见的精度", "制造能力如何成为证据"},
		SelectedTitle:     "看得见的精度",
		Body:              "从产品细节切入，用可验证的信息解释品牌的制造能力。",
		Topics:            []string{"#精密制造", "#品牌故事"},
		CoverCopy:         "看得见的精度",
		ImagePlan: []ImagePlanItem{
			{Order: 1, Role: "cover", Purpose: "建立主张", VisualBrief: "产品细节", Caption: "封面", OverlayCopy: "看得见的精度", LayoutPreset: "cover_center_v1"},
			{Order: 2, Role: "proof", Purpose: "展示证据", VisualBrief: "制造现场", Caption: "证据", OverlayCopy: "每一步都有依据", LayoutPreset: "proof_lower_left_v1"},
			{Order: 3, Role: "cta", Purpose: "引导行动", VisualBrief: "产品全貌", Caption: "行动", OverlayCopy: "了解更多制造细节", LayoutPreset: "cta_bottom_v1"},
		},
		CreatedAt: now,
	}
}

func imageTextIntegrationPrompt(
	suffix string,
	order int,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	taskID string,
	directionID string,
	directionHash string,
	inputIdentityHash string,
	userID string,
	now time.Time,
) ImagePromptPackage {
	return ImagePromptPackage{
		ContractVersion:      ImagePromptPackageV1Contract,
		ID:                   fmt.Sprintf("prompt_%s_%d", suffix, order),
		OrganizationID:       organizationID,
		ProjectID:            projectID,
		TaskID:               taskID,
		DraftRevision:        2,
		ImagePlanOrder:       order,
		DirectionID:          directionID,
		DirectionContentHash: directionHash,
		InputIdentityHash:    inputIdentityHash,
		CompiledPrompt:       fmt.Sprintf("portrait image slot %d", order),
		NegativeConstraints:  []string{"text", "watermark"},
		SourceAssetRefs:      []contract.AssetVersionRef{},
		CompilerVersion:      ImagePromptCompilerV1,
		ContentHash:          "sha256:" + fmt.Sprintf("%064x", 100+order),
		CreatedBy:            userID,
		CreatedAt:            now,
	}
}

func imageTextIntegrationAttempt(
	suffix string,
	order int,
	attemptNo int,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	taskID string,
	promptPackageID string,
	expectedTaskVersion int64,
	userID string,
	now time.Time,
) ImageGenerationAttempt {
	return ImageGenerationAttempt{
		ContractVersion:     ImageGenerationAttemptV1,
		ID:                  fmt.Sprintf("attempt_%s_%d_%d", suffix, order, attemptNo),
		OrganizationID:      organizationID,
		ProjectID:           projectID,
		TaskID:              taskID,
		DraftRevision:       2,
		ImagePlanOrder:      order,
		AttemptNo:           attemptNo,
		PromptPackageID:     promptPackageID,
		GenerationSpec:      DefaultImageGenerationSpec(""),
		GenerationSpecHash:  "sha256:" + fmt.Sprintf("%064x", 200+order+attemptNo),
		Status:              ImageAttemptQueued,
		IdempotencyKey:      contract.IdempotencyKey(fmt.Sprintf("attempt_%s_%d_%d", suffix, order, attemptNo)),
		RequestHash:         fmt.Sprintf("%064x", 300+order+attemptNo),
		ExpectedTaskVersion: expectedTaskVersion,
		CreatedByKind:       contract.PrincipalUser,
		CreatedBy:           userID,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func imageTextIntegrationTaskState(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	taskID string,
) (int64, TaskStatus) {
	t.Helper()
	var version int64
	var status TaskStatus
	if err := db.QueryRowContext(ctx, `SELECT version, status FROM creative_tasks
		WHERE organization_id=? AND project_id=? AND id=?`,
		organizationID, projectID, taskID).Scan(&version, &status); err != nil {
		t.Fatal(err)
	}
	return version, status
}

func assertImageTextDraftCount(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	organizationID contract.OrganizationID,
	taskID string,
	version int64,
	want int,
) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM creative_image_text_drafts
		WHERE organization_id=? AND task_id=? AND version=?`,
		organizationID, taskID, version).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("draft version %d count=%d, want=%d", version, count, want)
	}
}

func assertImageTextOutboxCount(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	organizationID contract.OrganizationID,
	taskID string,
	want int,
) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM event_outbox
		WHERE organization_id=? AND event_type='creative.image_text.materialized.v1'
		  AND subject_type='creative_image_text_draft' AND subject_id=?`,
		organizationID, taskID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("image-text materialized outbox count=%d, want=%d", count, want)
	}
}

func cleanupImageTextIntegration(
	t *testing.T,
	db *sql.DB,
	organizationID contract.OrganizationID,
	userID string,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	statements := []string{
		"DELETE FROM event_consumptions WHERE event_id IN (SELECT event_id FROM event_outbox WHERE organization_id=?)",
		"DELETE FROM event_outbox WHERE organization_id=?",
		"DELETE FROM creative_image_slot_selections WHERE organization_id=?",
		"DELETE FROM creative_image_generation_attempts WHERE organization_id=?",
		"DELETE FROM creative_image_prompt_packages WHERE organization_id=?",
		"DELETE FROM creative_image_text_drafts WHERE organization_id=?",
		"DELETE FROM creative_tasks WHERE organization_id=?",
		"DELETE FROM creative_intakes WHERE organization_id=?",
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
