package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidMarketingEmailCampaign = errors.New("invalid marketing email campaign")
	ErrMarketingEmailCampaignLocked  = errors.New("marketing email campaign is no longer editable")
	ErrMarketingEmailAudienceChanged = errors.New("marketing email audience changed")
	ErrEmailMarketingNotAllowed      = errors.New("verified customer account required for marketing email")
	ErrInvalidEmailUnsubscribeToken  = errors.New("invalid email unsubscribe token")
	ErrInvalidEmailPublicURL         = errors.New("invalid public URL for email links")
)

type MarketingEmailCampaign struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	Subject         string     `json:"subject"`
	Preheader       string     `json:"preheader"`
	Body            string     `json:"body"`
	Status          string     `json:"status"`
	RecipientCount  int        `json:"recipientCount"`
	DeliveredCount  int        `json:"deliveredCount"`
	PendingCount    int        `json:"pendingCount"`
	SuppressedCount int        `json:"suppressedCount"`
	CreatedBy       string     `json:"createdBy"`
	CreatedAt       time.Time  `json:"createdAt"`
	SentAt          *time.Time `json:"sentAt"`
}

func (s *Store) MarketingEmailCampaignsForAdmin(ctx context.Context) ([]MarketingEmailCampaign, int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT campaign.id, campaign.name, campaign.subject, campaign.preheader,
		campaign.body, campaign.status, campaign.recipient_count, campaign.created_by, campaign.created_at, campaign.sent_at,
		(SELECT COUNT(*) FROM email_outbox_events delivery
			WHERE delivery.campaign_id = campaign.id AND delivery.sent_at IS NOT NULL),
		(SELECT COUNT(*) FROM email_outbox_events delivery
			WHERE delivery.campaign_id = campaign.id AND delivery.sent_at IS NULL AND delivery.cancelled_at IS NULL),
		(SELECT COUNT(*) FROM email_outbox_events delivery
			WHERE delivery.campaign_id = campaign.id AND delivery.cancelled_at IS NOT NULL)
		FROM marketing_email_campaigns campaign
		ORDER BY campaign.created_at DESC, campaign.id DESC`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	campaigns := make([]MarketingEmailCampaign, 0)
	for rows.Next() {
		var campaign MarketingEmailCampaign
		if err := rows.Scan(&campaign.ID, &campaign.Name, &campaign.Subject, &campaign.Preheader,
			&campaign.Body, &campaign.Status, &campaign.RecipientCount, &campaign.CreatedBy, &campaign.CreatedAt,
			&campaign.SentAt, &campaign.DeliveredCount, &campaign.PendingCount, &campaign.SuppressedCount); err != nil {
			return nil, 0, err
		}
		campaigns = append(campaigns, campaign)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var audienceSize int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users
		WHERE email_verified_at IS NOT NULL AND marketing_email_consent = TRUE`).Scan(&audienceSize); err != nil {
		return nil, 0, err
	}
	return campaigns, audienceSize, nil
}

func (s *Store) SaveMarketingEmailCampaign(ctx context.Context, campaign MarketingEmailCampaign, actor string) (MarketingEmailCampaign, error) {
	campaign.Name = strings.TrimSpace(campaign.Name)
	campaign.Subject = strings.TrimSpace(campaign.Subject)
	campaign.Preheader = strings.TrimSpace(campaign.Preheader)
	campaign.Body = strings.TrimSpace(strings.ReplaceAll(campaign.Body, "\r\n", "\n"))
	actor = strings.TrimSpace(actor)
	if utf8.RuneCountInString(campaign.Name) < 3 || utf8.RuneCountInString(campaign.Name) > 120 ||
		!validMarketingEmailContent(campaign, false) || actor == "" {
		return MarketingEmailCampaign{}, ErrInvalidMarketingEmailCampaign
	}

	var saved MarketingEmailCampaign
	var err error
	if campaign.ID == 0 {
		err = s.db.QueryRowContext(ctx, `INSERT INTO marketing_email_campaigns
			(name, subject, preheader, body, created_by)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, name, subject, preheader, body, status, recipient_count, created_by, created_at, sent_at`,
			campaign.Name, campaign.Subject, campaign.Preheader, campaign.Body, actor).
			Scan(&saved.ID, &saved.Name, &saved.Subject, &saved.Preheader, &saved.Body, &saved.Status,
				&saved.RecipientCount, &saved.CreatedBy, &saved.CreatedAt, &saved.SentAt)
	} else {
		err = s.db.QueryRowContext(ctx, `UPDATE marketing_email_campaigns
			SET name = $2, subject = $3, preheader = $4, body = $5, updated_at = NOW()
			WHERE id = $1 AND status = 'draft'
			RETURNING id, name, subject, preheader, body, status, recipient_count, created_by, created_at, sent_at`,
			campaign.ID, campaign.Name, campaign.Subject, campaign.Preheader, campaign.Body).
			Scan(&saved.ID, &saved.Name, &saved.Subject, &saved.Preheader, &saved.Body, &saved.Status,
				&saved.RecipientCount, &saved.CreatedBy, &saved.CreatedAt, &saved.SentAt)
	}
	if errors.Is(err, sql.ErrNoRows) && campaign.ID != 0 {
		return MarketingEmailCampaign{}, ErrMarketingEmailCampaignLocked
	}
	return saved, err
}

func (s *Store) QueueMarketingEmailCampaign(ctx context.Context, campaignID int64, publicBaseURL string, expectedAudienceSize int) (int, error) {
	baseURL, err := url.Parse(publicBaseURL)
	if err != nil || (baseURL.Scheme != "https" && baseURL.Scheme != "http") || baseURL.Host == "" ||
		baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return 0, ErrInvalidEmailPublicURL
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var campaign MarketingEmailCampaign
	err = tx.QueryRowContext(ctx, `SELECT id, name, subject, preheader, body, status
		FROM marketing_email_campaigns WHERE id = $1 FOR UPDATE`, campaignID).
		Scan(&campaign.ID, &campaign.Name, &campaign.Subject, &campaign.Preheader, &campaign.Body, &campaign.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if campaign.Status != "draft" {
		return 0, ErrMarketingEmailCampaignLocked
	}
	if !validMarketingEmailContent(campaign, true) {
		return 0, ErrInvalidMarketingEmailCampaign
	}

	var recipientCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users
		WHERE email_verified_at IS NOT NULL AND marketing_email_consent = TRUE`).Scan(&recipientCount); err != nil {
		return 0, err
	}
	if expectedAudienceSize != recipientCount {
		return recipientCount, ErrMarketingEmailAudienceChanged
	}

	const batchSize = 250
	queued := 0
	var afterID int64
	for {
		rows, err := tx.QueryContext(ctx, `SELECT id, email FROM users
			WHERE email_verified_at IS NOT NULL AND marketing_email_consent = TRUE AND id > $1
			ORDER BY id LIMIT $2 FOR SHARE`, afterID, batchSize)
		if err != nil {
			return 0, err
		}
		type recipient struct {
			id    int64
			email string
		}
		recipients := make([]recipient, 0, batchSize)
		for rows.Next() {
			var item recipient
			if err := rows.Scan(&item.id, &item.email); err != nil {
				rows.Close()
				return 0, err
			}
			recipients = append(recipients, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return 0, err
		}
		if err := rows.Close(); err != nil {
			return 0, err
		}
		if len(recipients) == 0 {
			break
		}

		for _, item := range recipients {
			token, tokenHash, err := newEmailUnsubscribeToken()
			if err != nil {
				return 0, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO email_marketing_unsubscribe_tokens (token_hash, user_id)
				VALUES ($1, $2)`, tokenHash, item.id); err != nil {
				return 0, err
			}
			tokenQuery := url.QueryEscape(token)
			basePath := strings.TrimRight(baseURL.String(), "/")
			browserURL := basePath + "/email/unsubscribe?token=" + tokenQuery
			unsubscribeURL := basePath + "/api/email/unsubscribe?token=" + tokenQuery
			body := campaign.Body + "\n\n—\nDu erhältst diese E-Mail, weil du Marketing-E-Mails in deinem MITTE-Kundenkonto aktiviert hast.\n" +
				"Abmelden: " + browserURL
			if _, err := tx.ExecContext(ctx, `INSERT INTO email_outbox_events
				(order_id, user_id, campaign_id, recipient, subject, body, preheader, unsubscribe_url)
				VALUES (NULL, $1, $2, $3, $4, $5, $6, $7)`,
				item.id, campaign.ID, item.email, campaign.Subject, body, campaign.Preheader, unsubscribeURL); err != nil {
				return 0, err
			}
			afterID = item.id
			queued++
		}
	}
	if queued != recipientCount {
		return queued, ErrMarketingEmailAudienceChanged
	}

	status := "queued"
	var sentAt any
	if queued == 0 {
		status = "sent"
		sentAt = time.Now().UTC()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE marketing_email_campaigns
		SET status = $2, recipient_count = $3, queued_at = NOW(), sent_at = $4, updated_at = NOW()
		WHERE id = $1`, campaign.ID, status, queued, sentAt); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return queued, nil
}

func (s *Store) SetMarketingEmailConsent(ctx context.Context, userID int64, enabled bool, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var verified, current bool
	err = tx.QueryRowContext(ctx, `SELECT email_verified_at IS NOT NULL, marketing_email_consent
		FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&verified, &current)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !verified {
		return ErrEmailMarketingNotAllowed
	}
	if current == enabled {
		return tx.Commit()
	}
	var consentAt any
	if enabled {
		consentAt = now
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users
		SET marketing_email_consent = $2, marketing_email_consent_at = $3
		WHERE id = $1`, userID, enabled, consentAt); err != nil {
		return err
	}
	event := "unsubscribed"
	if enabled {
		event = "subscribed"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO email_marketing_consent_events (user_id, event, source, occurred_at)
		VALUES ($1, $2, 'account', $3)`, userID, event, now); err != nil {
		return err
	}
	if !enabled {
		if _, err := tx.ExecContext(ctx, `UPDATE email_outbox_events
			SET cancelled_at = $2, body = '', preheader = '', unsubscribe_url = ''
			WHERE user_id = $1 AND campaign_id IS NOT NULL
			  AND sent_at IS NULL AND cancelled_at IS NULL`, userID, now); err != nil {
			return err
		}
		if err := markCompletedMarketingEmailCampaigns(ctx, tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UnsubscribeMarketingEmails(ctx context.Context, tokenHash []byte, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID int64
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM email_marketing_unsubscribe_tokens
		WHERE token_hash = $1 FOR UPDATE`, tokenHash).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidEmailUnsubscribeToken
	}
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE users
		SET marketing_email_consent = FALSE, marketing_email_consent_at = NULL
		WHERE id = $1 AND marketing_email_consent = TRUE`, userID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO email_marketing_consent_events (user_id, event, source, occurred_at)
			VALUES ($1, 'unsubscribed', 'unsubscribe', $2)`, userID, now); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE email_marketing_unsubscribe_tokens
		SET used_at = COALESCE(used_at, $2) WHERE token_hash = $1`, tokenHash, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE email_outbox_events
		SET cancelled_at = $2, body = '', preheader = '', unsubscribe_url = ''
		WHERE user_id = $1 AND campaign_id IS NOT NULL
		  AND sent_at IS NULL AND cancelled_at IS NULL`, userID, now); err != nil {
		return err
	}
	if err := markCompletedMarketingEmailCampaigns(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) EmailUnsubscribeTokenExists(ctx context.Context, tokenHash []byte) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM email_marketing_unsubscribe_tokens WHERE token_hash = $1
	)`, tokenHash).Scan(&exists)
	return exists, err
}

func markCompletedMarketingEmailCampaigns(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `UPDATE marketing_email_campaigns campaign
		SET status = 'sent', sent_at = COALESCE(sent_at, NOW()), updated_at = NOW()
		WHERE campaign.status = 'queued' AND NOT EXISTS (
			SELECT 1 FROM email_outbox_events delivery
			WHERE delivery.campaign_id = campaign.id
			  AND delivery.sent_at IS NULL AND delivery.cancelled_at IS NULL
		)`)
	return err
}

func newEmailUnsubscribeToken() (string, []byte, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", nil, fmt.Errorf("generate email unsubscribe token: %w", err)
	}
	digest := sha256.Sum256(value)
	return hex.EncodeToString(value), digest[:], nil
}

func EmailUnsubscribeTokenHash(token string) ([]byte, bool) {
	value, err := hex.DecodeString(token)
	if err != nil || len(value) != 32 {
		return nil, false
	}
	digest := sha256.Sum256(value)
	return digest[:], true
}

func validMarketingEmailContent(campaign MarketingEmailCampaign, forSending bool) bool {
	subjectLength := utf8.RuneCountInString(campaign.Subject)
	bodyLength := utf8.RuneCountInString(campaign.Body)
	if subjectLength > 120 || strings.ContainsAny(campaign.Subject, "\r\n") ||
		utf8.RuneCountInString(campaign.Preheader) > 180 || bodyLength > 12000 {
		return false
	}
	if subjectLength > 0 && subjectLength < 3 {
		return false
	}
	if forSending {
		return subjectLength >= 3 && bodyLength >= 20
	}
	return true
}
