package store

import (
	"context"
	"database/sql"
	"fmt"
)

type EmailMessage struct {
	ID                                  int64
	CampaignID                          int64
	Recipient, Subject, Preheader, Body string
	UnsubscribeURL                      string
}
type EmailSender interface {
	Send(context.Context, EmailMessage) error
}

func enqueueOrderEmail(ctx context.Context, q executor, orderID, subject, body string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO email_outbox_events (order_id, recipient, subject, body)
SELECT id, customer_email, $2, $3 FROM orders WHERE id = $1`, orderID, subject, body)
	return err
}

// DispatchNextEmail sends one durable outbox message. It is safe to run in
// multiple worker replicas; delivery is at-least-once, so providers should
// deduplicate using the Message-ID built from EmailMessage.ID.
func (s *Store) DispatchNextEmail(ctx context.Context, sender EmailSender) (bool, error) {
	claimToken, err := newOutboxClaimToken()
	if err != nil {
		return false, err
	}
	var message EmailMessage
	var campaignID sql.NullInt64
	err = s.db.QueryRowContext(ctx, `WITH candidate AS (
			SELECT id
			FROM email_outbox_events
			WHERE sent_at IS NULL
			  AND cancelled_at IS NULL
			  AND available_at <= NOW()
			  AND (claimed_until IS NULL OR claimed_until <= NOW())
			ORDER BY id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE email_outbox_events AS claimed
		SET claimed_by = $1, claimed_until = NOW() + ($2::double precision * INTERVAL '1 second')
		FROM candidate
		WHERE claimed.id = candidate.id
		RETURNING claimed.id, claimed.recipient, claimed.subject, claimed.preheader, claimed.body,
			claimed.unsubscribe_url, claimed.campaign_id`,
		claimToken, outboxClaimDuration.Seconds()).Scan(&message.ID, &message.Recipient, &message.Subject,
		&message.Preheader, &message.Body, &message.UnsubscribeURL, &campaignID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if campaignID.Valid {
		message.CampaignID = campaignID.Int64
	}
	if err := sender.Send(ctx, message); err != nil {
		finalizeCtx, cancelFinalize := outboxFinalizeContext(ctx)
		defer cancelFinalize()
		result, updateErr := s.db.ExecContext(finalizeCtx, `UPDATE email_outbox_events
			SET attempts = attempts + 1,
				last_error = LEFT($2, 1000),
				available_at = NOW() + (LEAST(300, POWER(2, attempts + 1)) * INTERVAL '1 second'),
				claimed_by = NULL, claimed_until = NULL
			WHERE id = $1 AND claimed_by = $3 AND cancelled_at IS NULL`, message.ID, err.Error(), claimToken)
		if updateErr != nil {
			return true, fmt.Errorf("send email %d: %v; persist retry: %w", message.ID, err, updateErr)
		}
		if claimErr := requireOutboxClaim(result); claimErr != nil {
			return true, fmt.Errorf("send email %d: %v; %w", message.ID, err, claimErr)
		}
		return true, fmt.Errorf("send email %d: %w", message.ID, err)
	}
	finalizeCtx, cancelFinalize := outboxFinalizeContext(ctx)
	defer cancelFinalize()
	finalizeTx, err := s.db.BeginTx(finalizeCtx, nil)
	if err != nil {
		return true, err
	}
	defer finalizeTx.Rollback()
	result, err := finalizeTx.ExecContext(finalizeCtx, `UPDATE email_outbox_events
		SET sent_at = NOW(), body = '', preheader = '', unsubscribe_url = '',
			last_error = NULL, claimed_by = NULL, claimed_until = NULL
		WHERE id = $1 AND claimed_by = $2 AND cancelled_at IS NULL`, message.ID, claimToken)
	if err != nil {
		return true, err
	}
	if err := requireOutboxClaim(result); err != nil {
		return true, err
	}
	if message.CampaignID != 0 {
		if _, err := finalizeTx.ExecContext(finalizeCtx, `UPDATE marketing_email_campaigns campaign
			SET status = 'sent', sent_at = NOW(), updated_at = NOW()
			WHERE campaign.id = $1 AND campaign.status = 'queued'
			  AND NOT EXISTS (
				SELECT 1 FROM email_outbox_events delivery
				WHERE delivery.campaign_id = campaign.id
				  AND delivery.sent_at IS NULL AND delivery.cancelled_at IS NULL
			  )`, message.CampaignID); err != nil {
			return true, err
		}
	}
	if err := finalizeTx.Commit(); err != nil {
		return true, err
	}
	return true, nil
}
