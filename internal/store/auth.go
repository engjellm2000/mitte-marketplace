package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrEmailInUse = errors.New("email already in use")
var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidSession = errors.New("invalid session")
var ErrInvalidAuthToken = errors.New("invalid or expired authentication token")
var ErrRecommendationConsentRequired = errors.New("recommendation consent is required")

type User struct {
	ID                      int64
	Email                   string
	Profile                 CustomerProfile
	RecommendationConsent   bool
	MarketingEmailConsent   bool
	MarketingEmailConsentAt *time.Time
	EmailVerified           bool
}

type CustomerProfile struct {
	FullName       string
	Phone          string
	CustomerType   string
	CompanyName    string
	VATID          string
	BuyerReference string
	Address        Address
}

const sessionLifetime = 7 * 24 * time.Hour

func (s *Store) CreateUser(ctx context.Context, email, passwordHash, customerType string) (User, error) {
	email = normalizeEmail(email)
	if customerType != "b2c" && customerType != "b2b" {
		return User{}, errors.New("customer type must be b2c or b2b")
	}
	var user User
	user, err := scanUser(s.db.QueryRowContext(ctx, `INSERT INTO users (email, password_hash, customer_type)
		VALUES ($1, $2, $3)
		RETURNING `+userColumns, email, passwordHash, customerType))
	if isUniqueViolation(err) {
		return User{}, ErrEmailInUse
	}
	return user, err
}

func (s *Store) CreateUserWithVerification(ctx context.Context, email, passwordHash, customerType string, tokenHash []byte, expiresAt time.Time, subject, body string) (User, error) {
	email = normalizeEmail(email)
	if customerType != "b2c" && customerType != "b2b" {
		return User{}, errors.New("customer type must be b2c or b2b")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	user, err := scanUser(tx.QueryRowContext(ctx, `INSERT INTO users (email, password_hash, customer_type)
		VALUES ($1, $2, $3)
		RETURNING `+userColumns, email, passwordHash, customerType))
	if isUniqueViolation(err) {
		return User{}, ErrEmailInUse
	}
	if err != nil {
		return User{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_auth_tokens (user_id, purpose, token_hash, expires_at)
		VALUES ($1, 'verify_email', $2, $3)`, user.ID, tokenHash, expiresAt); err != nil {
		return User{}, err
	}
	if err := enqueueAccountEmail(ctx, tx, email, subject, body); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) IssueUserAuthToken(ctx context.Context, email, purpose string, tokenHash []byte, expiresAt time.Time, subject, body string) (bool, error) {
	if purpose != "verify_email" && purpose != "password_reset" {
		return false, errors.New("unsupported authentication token purpose")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var userID int64
	var emailVerified bool
	err = tx.QueryRowContext(ctx, `SELECT id, email_verified_at IS NOT NULL
		FROM users WHERE email = $1 FOR UPDATE`, normalizeEmail(email)).Scan(&userID, &emailVerified)
	if errors.Is(err, sql.ErrNoRows) || purpose == "verify_email" && emailVerified {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_auth_tokens WHERE user_id = $1 AND purpose = $2`, userID, purpose); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_auth_tokens (user_id, purpose, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, userID, purpose, tokenHash, expiresAt); err != nil {
		return false, err
	}
	if err := enqueueAccountEmail(ctx, tx, email, subject, body); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) VerifyUserEmail(ctx context.Context, tokenHash []byte, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	userID, err := lockUserAuthToken(ctx, tx, tokenHash, "verify_email", now)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET email_verified_at = COALESCE(email_verified_at, $2) WHERE id = $1`, userID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_auth_tokens SET consumed_at = $2
		WHERE user_id = $1 AND purpose = 'verify_email' AND consumed_at IS NULL`, userID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ResetUserPassword(ctx context.Context, tokenHash []byte, passwordHash string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	userID, err := lockUserAuthToken(ctx, tx, tokenHash, "password_reset", now)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = $2, password_changed_at = $3,
		email_verified_at = COALESCE(email_verified_at, $3) WHERE id = $1`, userID, passwordHash, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_auth_tokens SET consumed_at = $2
		WHERE user_id = $1 AND consumed_at IS NULL`, userID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_sessions SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL`, userID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AuthenticateUser(ctx context.Context, email string) (User, string, error) {
	var user User
	var passwordHash string
	err := s.db.QueryRowContext(ctx, `SELECT `+userColumns+`, password_hash
		FROM users WHERE email = $1`, normalizeEmail(email)).Scan(&user.ID, &user.Email, &user.Profile.FullName, &user.Profile.Phone,
		&user.Profile.CustomerType, &user.Profile.CompanyName, &user.Profile.VATID, &user.Profile.BuyerReference,
		&user.Profile.Address.AddressLine1, &user.Profile.Address.AddressLine2, &user.Profile.Address.PostalCode,
		&user.Profile.Address.City, &user.Profile.Address.CountryCode, &user.RecommendationConsent,
		&user.MarketingEmailConsent, &user.MarketingEmailConsentAt, &user.EmailVerified, &passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", err
	}
	return user, passwordHash, nil
}

func (s *Store) CreateUserSession(ctx context.Context, userID int64, token string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, userID, sessionTokenHash(token), now.Add(sessionLifetime))
	return err
}

func (s *Store) UserBySession(ctx context.Context, token string, now time.Time) (User, error) {
	var user User
	user, err := scanUser(s.db.QueryRowContext(ctx, `UPDATE user_sessions AS session
		SET last_seen_at = $3
		FROM users
		WHERE session.token_hash = $1
		  AND session.user_id = users.id
		  AND session.revoked_at IS NULL
		  AND session.expires_at > $2
		RETURNING `+qualifiedUserColumns("users"), sessionTokenHash(token), now, now))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrInvalidSession
	}
	return user, err
}

const userColumns = `id, email, full_name, phone, customer_type, company_name, vat_id, buyer_reference,
	address_line1, address_line2, postal_code, city, country_code, recommendation_consent,
	marketing_email_consent, marketing_email_consent_at, email_verified_at IS NOT NULL`

func qualifiedUserColumns(table string) string {
	return table + `.id, ` + table + `.email, ` + table + `.full_name, ` + table + `.phone, ` + table + `.customer_type, ` +
		table + `.company_name, ` + table + `.vat_id, ` + table + `.buyer_reference, ` + table + `.address_line1, ` +
		table + `.address_line2, ` + table + `.postal_code, ` + table + `.city, ` + table + `.country_code, ` +
		table + `.recommendation_consent, ` + table + `.marketing_email_consent, ` +
		table + `.marketing_email_consent_at, ` + table + `.email_verified_at IS NOT NULL`
}

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.Email, &user.Profile.FullName, &user.Profile.Phone, &user.Profile.CustomerType,
		&user.Profile.CompanyName, &user.Profile.VATID, &user.Profile.BuyerReference,
		&user.Profile.Address.AddressLine1, &user.Profile.Address.AddressLine2, &user.Profile.Address.PostalCode,
		&user.Profile.Address.City, &user.Profile.Address.CountryCode, &user.RecommendationConsent,
		&user.MarketingEmailConsent, &user.MarketingEmailConsentAt, &user.EmailVerified)
	return user, err
}

func (s *Store) UpdateUserProfile(ctx context.Context, userID int64, email string, profile CustomerProfile) (User, error) {
	email = normalizeEmail(email)
	user, err := scanUser(s.db.QueryRowContext(ctx, `UPDATE users SET email = $2, full_name = $3, phone = $4,
		customer_type = $5, company_name = $6, vat_id = $7, buyer_reference = $8, address_line1 = $9,
		address_line2 = $10, postal_code = $11, city = $12, country_code = $13
		WHERE id = $1 RETURNING `+userColumns, userID, email, profile.FullName, profile.Phone, profile.CustomerType,
		profile.CompanyName, profile.VATID, profile.BuyerReference, profile.Address.AddressLine1, profile.Address.AddressLine2,
		profile.Address.PostalCode, profile.Address.City, profile.Address.CountryCode))
	if isUniqueViolation(err) {
		return User{}, ErrEmailInUse
	}
	return user, err
}

func (s *Store) ChangeUserPassword(ctx context.Context, userID int64, passwordHash string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = $2, password_changed_at = $3 WHERE id = $1`, userID, passwordHash, now)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_auth_tokens SET consumed_at = $2
		WHERE user_id = $1 AND consumed_at IS NULL`, userID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_sessions SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL`, userID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RevokeUserSession(ctx context.Context, token string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE user_sessions SET revoked_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL`, sessionTokenHash(token), now)
	return err
}

func (s *Store) OrderByIDForCustomer(ctx context.Context, orderID string, customerID int64) (Order, error) {
	const query = `SELECT id, status, total_cents, COALESCE(subtotal_cents, total_cents), discount_cents, shipping_cents, tax_cents,
		customer_name, customer_email, COALESCE(promotion_code, ''),
		COALESCE(stripe_checkout_session_id, ''), COALESCE(stripe_payment_intent_id, ''), paid_at, payment_provider
		FROM orders WHERE id = $1 AND customer_id = $2`
	var order Order
	err := s.db.QueryRowContext(ctx, query, orderID, customerID).Scan(
		&order.ID, &order.Status, &order.TotalCents, &order.SubtotalCents, &order.DiscountCents, &order.ShippingCents, &order.TaxCents,
		&order.CustomerName, &order.CustomerEmail, &order.PromotionCode, &order.StripeCheckoutSessionID, &order.StripePaymentIntentID,
		&order.PaidAt, &order.PaymentProvider)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	if order.Shipment, err = shipmentForOrder(ctx, s.db, order.ID); err != nil {
		return Order{}, err
	}
	if order.Invoice, err = invoiceForOrder(ctx, s.db, order.ID); err != nil {
		return Order{}, err
	}
	return order, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func sessionTokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func enqueueAccountEmail(ctx context.Context, q executor, recipient, subject, body string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO email_outbox_events (order_id, recipient, subject, body)
		VALUES (NULL, $1, $2, $3)`, recipient, subject, body)
	return err
}

func lockUserAuthToken(ctx context.Context, tx *sql.Tx, tokenHash []byte, purpose string, now time.Time) (int64, error) {
	var userID int64
	err := tx.QueryRowContext(ctx, `SELECT user_id FROM user_auth_tokens
		WHERE token_hash = $1 AND purpose = $2 AND consumed_at IS NULL AND expires_at > $3`, tokenHash, purpose, now).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInvalidAuthToken
	}
	if err != nil {
		return 0, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&userID); errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInvalidAuthToken
	} else if err != nil {
		return 0, err
	}
	var tokenID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM user_auth_tokens
		WHERE user_id = $1 AND token_hash = $2 AND purpose = $3
		  AND consumed_at IS NULL AND expires_at > $4 FOR UPDATE`, userID, tokenHash, purpose, now).Scan(&tokenID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInvalidAuthToken
	}
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
