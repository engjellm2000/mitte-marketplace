package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrOutboxClaimLost = errors.New("outbox claim is no longer owned by this worker")

const outboxClaimDuration = 10 * time.Minute

func outboxFinalizeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func newOutboxClaimToken() (string, error) {
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return "", fmt.Errorf("generate outbox claim token: %w", err)
	}
	return hex.EncodeToString(token), nil
}

// OutboxEvent is an immutable domain event. Consumers must deduplicate by ID:
// delivery is at-least-once so a crash after Kafka acknowledges a message can
// cause a safe replay.
type OutboxEvent struct {
	ID         int64
	OrderID    string
	Type       string
	Payload    json.RawMessage
	OccurredAt time.Time
}

type OutboxPublisher interface {
	Publish(context.Context, OutboxEvent) error
}

func enqueueOrderEvent(ctx context.Context, q executor, orderID, eventType string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", eventType, err)
	}
	_, err = q.ExecContext(ctx, `INSERT INTO order_outbox_events (order_id, event_type, payload)
		VALUES ($1, $2, $3::jsonb)`, orderID, eventType, body)
	return err
}

// DispatchNextOutboxEvent publishes at most one event. It can be called by
// many relay replicas concurrently. It commits a durable lease before calling
// the external publisher, so network I/O never holds a database transaction.
// The NOT EXISTS predicate preserves event ordering within each order.
func (s *Store) DispatchNextOutboxEvent(ctx context.Context, publisher OutboxPublisher) (bool, error) {
	claimToken, err := newOutboxClaimToken()
	if err != nil {
		return false, err
	}

	var event OutboxEvent
	err = s.db.QueryRowContext(ctx, `WITH candidate AS (
			SELECT current.id
			FROM order_outbox_events AS current
			WHERE current.published_at IS NULL
			  AND current.available_at <= NOW()
			  AND (current.claimed_until IS NULL OR current.claimed_until <= NOW())
			  AND NOT EXISTS (
				SELECT 1 FROM order_outbox_events AS earlier
				WHERE earlier.order_id = current.order_id
				  AND earlier.id < current.id
				  AND earlier.published_at IS NULL
			  )
			ORDER BY current.id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE order_outbox_events AS claimed
		SET claimed_by = $1, claimed_until = NOW() + ($2::double precision * INTERVAL '1 second')
		FROM candidate
		WHERE claimed.id = candidate.id
		RETURNING claimed.id, claimed.order_id, claimed.event_type, claimed.payload, claimed.occurred_at`,
		claimToken, outboxClaimDuration.Seconds()).Scan(&event.ID, &event.OrderID, &event.Type, &event.Payload, &event.OccurredAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	publishCtx, cancelPublish := context.WithTimeout(ctx, 2*time.Minute)
	publishErr := publisher.Publish(publishCtx, event)
	cancelPublish()
	if publishErr != nil {
		finalizeCtx, cancelFinalize := outboxFinalizeContext(ctx)
		defer cancelFinalize()
		result, updateErr := s.db.ExecContext(finalizeCtx, `UPDATE order_outbox_events
			SET publish_attempts = publish_attempts + 1,
				last_error = LEFT($2, 1000),
				available_at = NOW() + (LEAST(300, POWER(2, publish_attempts + 1)) * INTERVAL '1 second'),
				claimed_by = NULL, claimed_until = NULL
			WHERE id = $1 AND claimed_by = $3`, event.ID, publishErr.Error(), claimToken)
		if updateErr != nil {
			return true, fmt.Errorf("publish outbox event %d: %v; persist retry: %w", event.ID, publishErr, updateErr)
		}
		if claimErr := requireOutboxClaim(result); claimErr != nil {
			return true, fmt.Errorf("publish outbox event %d: %v; %w", event.ID, publishErr, claimErr)
		}
		return true, fmt.Errorf("publish outbox event %d: %w", event.ID, publishErr)
	}
	finalizeCtx, cancelFinalize := outboxFinalizeContext(ctx)
	defer cancelFinalize()
	result, err := s.db.ExecContext(finalizeCtx, `UPDATE order_outbox_events
		SET published_at = NOW(), last_error = NULL, claimed_by = NULL, claimed_until = NULL
		WHERE id = $1 AND claimed_by = $2`, event.ID, claimToken)
	if err != nil {
		return true, err
	}
	if err := requireOutboxClaim(result); err != nil {
		return true, err
	}
	return true, nil
}

func requireOutboxClaim(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check outbox claim update: %w", err)
	}
	if rows != 1 {
		return ErrOutboxClaimLost
	}
	return nil
}
