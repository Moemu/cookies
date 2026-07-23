package eventoutbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type MySQLStore struct{ DB *sql.DB }

func (MySQLStore) AppendIn(ctx context.Context, executor DBTX, event Event) error {
	if executor == nil {
		return fmt.Errorf("database executor is required")
	}
	if err := event.Validate(); err != nil {
		return err
	}
	_, err := executor.ExecContext(ctx, `INSERT INTO event_outbox (
		event_id, organization_id, project_id, event_type, subject_type, subject_id,
		subject_version, payload, status, available_at, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)`,
		event.ID, event.OrganizationID, event.ProjectID, event.Type, event.SubjectType,
		event.SubjectID, event.SubjectVersion, event.Payload, event.CreatedAt, event.CreatedAt)
	return err
}

type Dispatcher struct {
	DB        *sql.DB
	Publisher Publisher
	Now       func() time.Time
	RetryIn   time.Duration
}

func (d Dispatcher) RunOnce(ctx context.Context) (bool, error) {
	if d.DB == nil || d.Publisher == nil {
		return false, fmt.Errorf("outbox database and publisher are required")
	}
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	var event Event
	err := d.DB.QueryRowContext(ctx, `SELECT event_id, organization_id, project_id,
		event_type, subject_type, subject_id, subject_version, payload, created_at
		FROM event_outbox WHERE status = 'pending' AND available_at <= ?
		ORDER BY available_at, created_at LIMIT 1`,
		now).Scan(&event.ID, &event.OrganizationID, &event.ProjectID, &event.Type,
		&event.SubjectType, &event.SubjectID, &event.SubjectVersion, &event.Payload, &event.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := d.Publisher.Publish(ctx, event); err != nil {
		delay := d.RetryIn
		if delay <= 0 {
			delay = 5 * time.Second
		}
		_, updateErr := d.DB.ExecContext(ctx, `UPDATE event_outbox SET attempt_count = attempt_count + 1,
			available_at = ?, last_error = ? WHERE event_id = ? AND status = 'pending'`,
			now.Add(delay), trimError(err.Error()), event.ID)
		return true, updateErr
	}
	result, err := d.DB.ExecContext(ctx, `UPDATE event_outbox SET status = 'published',
		attempt_count = attempt_count + 1, published_at = ?, last_error = NULL
		WHERE event_id = ? AND status = 'pending'`, now, event.ID)
	if err != nil {
		return true, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return true, err
	}
	if changed != 1 {
		return true, fmt.Errorf("outbox event %s changed during publish", event.ID)
	}
	return true, nil
}

func trimError(value string) string {
	if len(value) <= 1024 {
		return value
	}
	return value[:1024]
}

// ConsumeOnce records a consumer's durable idempotency guard in the same
// transaction as its business mutation.
func ConsumeOnce(ctx context.Context, executor DBTX, consumerName, eventID string, now time.Time) (bool, error) {
	result, err := executor.ExecContext(ctx, `INSERT IGNORE INTO event_consumptions
		(consumer_name, event_id, consumed_at) VALUES (?, ?, ?)`, consumerName, eventID, now.UTC())
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	return changed == 1, err
}
