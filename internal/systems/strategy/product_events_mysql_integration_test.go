package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMySQLProductEventWriterPersistsOnlyBoundedMetadata(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("COOKIES_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	eventID := "productevent_integration_" + suffix
	commandEventID := "productevent_command_" + suffix
	meaningfulEventID := "productevent_meaningful_" + suffix
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM strategy_product_events
			WHERE id IN (?, ?, ?) AND organization_id = ? AND project_id = ?`,
			eventID, commandEventID, meaningfulEventID, "org_product_event_test", "project_product_event_test")
	}()

	actor := contract.ActorContext{
		OrganizationID: "org_product_event_test",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "private-user-id"},
		Scopes:         []contract.Scope{ScopeRead},
	}
	duration := int64(734)
	event, err := NewProductEvent(NewProductEventInput{
		ID: eventID, Actor: actor, ProjectID: "project_product_event_test",
		WorkspaceID: "workspace_product_event_test", EventType: ProductEventAssistantFirstAck,
		Stage: "brief", DurationMS: &duration, Outcome: "accepted",
		Resource:   ProductEventResource{Type: "conversation_message", ID: "message_product_event_test"},
		Attributes: map[string]any{"mode": "standard"}, OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := (MySQLProductEventWriter{DB: db}).AppendProductEvent(ctx, event); err != nil {
		t.Fatalf("append product event: %v", err)
	}

	var actorHash, eventType, stage, outcome string
	var storedDuration int64
	var attributesJSON []byte
	if err := db.QueryRowContext(ctx, `SELECT actor_id_hash, event_type, stage, duration_ms, outcome, attributes_json
		FROM strategy_product_events WHERE id = ? AND organization_id = ? AND project_id = ?`,
		eventID, actor.OrganizationID, "project_product_event_test",
	).Scan(&actorHash, &eventType, &stage, &storedDuration, &outcome, &attributesJSON); err != nil {
		t.Fatalf("read product event: %v", err)
	}
	var attributes map[string]any
	if err := json.Unmarshal(attributesJSON, &attributes); err != nil {
		t.Fatalf("decode attributes: %v", err)
	}
	if actorHash == actor.Principal.ID || strings.Contains(actorHash, actor.Principal.ID) ||
		eventType != ProductEventAssistantFirstAck || stage != "brief" || storedDuration != duration ||
		outcome != "accepted" || attributes["mode"] != "standard" {
		t.Fatalf("stored event crossed its privacy or contract boundary: hash=%q type=%q stage=%q duration=%d outcome=%q attributes=%#v",
			actorHash, eventType, stage, storedDuration, outcome, attributes)
	}

	command, err := NewProductEvent(NewProductEventInput{
		ID: commandEventID, Actor: actor, ProjectID: "project_product_event_test",
		WorkspaceID: "workspace_product_event_test", EventType: ProductEventAssistantCommandSubmitted,
		Stage: "brief", Resource: ProductEventResource{Type: "conversation_message", ID: "message_product_event_test"},
		Outcome: "accepted", Attributes: map[string]any{"capability": "assistant.command"}, OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	meaningfulDuration := int64(1430)
	meaningful, err := NewProductEvent(NewProductEventInput{
		ID: meaningfulEventID, Actor: actor, ProjectID: "project_product_event_test",
		WorkspaceID: "workspace_product_event_test", EventType: ProductEventAssistantFirstMeaningfulUpdate,
		Stage: "brief", Resource: ProductEventResource{Type: "conversation_message", ID: "message_product_event_test"},
		DurationMS: &meaningfulDuration, Outcome: "succeeded",
		Attributes: map[string]any{"capability": "assistant.command"}, OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	transactionalWriter := MySQLProductEventWriter{DB: db}
	if err := transactionalWriter.AppendProductEventIn(ctx, tx, command); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := transactionalWriter.AppendProductEventIn(ctx, tx, meaningful); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	service := Service{
		DB: db, Projects: productEventProjectReader{},
		Now: func() time.Time { return time.Now().UTC().Add(time.Minute) },
	}
	metrics, err := service.GetWorkspaceUXMetrics(ctx, actor, "project_product_event_test", 1)
	if err != nil {
		t.Fatalf("read workspace UX metrics: %v", err)
	}
	if metrics.Assistant.Commands != 1 || metrics.Assistant.MissingAcknowledgements != 0 ||
		metrics.Assistant.MissingMeaningfulUpdates != 0 || metrics.Assistant.FirstAcknowledgement.P50MS == nil ||
		*metrics.Assistant.FirstAcknowledgement.P50MS != duration || metrics.TimeSavingMeasured {
		t.Fatalf("workspace UX metrics = %#v", metrics)
	}
}

type productEventProjectReader struct{}

func (productEventProjectReader) GetContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	brandID := contract.BrandID("brand_product_event_test")
	return contract.ProjectContext{
		OrganizationID: actor.OrganizationID, ProjectID: projectID, BrandID: &brandID,
		ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1,
	}, nil
}
