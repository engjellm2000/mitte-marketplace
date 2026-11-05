package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

func NewOperatorSessionToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("generate operator session token: %w", err)
	}
	return hex.EncodeToString(token[:]), nil
}

func (s *Store) CreateOperatorSession(ctx context.Context, token, principal string, expiresAt time.Time) error {
	if !validOperatorSessionToken(token) {
		return errors.New("operator session token must be 32-byte hex")
	}
	if len(principal) < 3 || len(principal) > 128 {
		return errors.New("operator session principal length must be between 3 and 128")
	}
	now := time.Now().UTC()
	if !expiresAt.After(now) {
		return errors.New("operator session expiry must be in the future")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM operator_sessions
		WHERE expires_at <= $1 OR revoked_at < $1 - INTERVAL '30 days'`, now); err != nil {
		return fmt.Errorf("prune expired operator sessions: %w", err)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO operator_sessions (token_hash, principal, expires_at)
		VALUES ($1, $2, $3)`, operatorSessionTokenHash(token), principal, expiresAt.UTC())
	if err != nil {
		return fmt.Errorf("create operator session: %w", err)
	}
	return nil
}

func (s *Store) OperatorSessionPrincipal(ctx context.Context, token string, now time.Time) (string, error) {
	if !validOperatorSessionToken(token) {
		return "", ErrInvalidSession
	}
	var principal string
	err := s.db.QueryRowContext(ctx, `UPDATE operator_sessions
		SET last_seen_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
		RETURNING principal`, operatorSessionTokenHash(token), now.UTC()).Scan(&principal)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInvalidSession
	}
	if err != nil {
		return "", fmt.Errorf("validate operator session: %w", err)
	}
	return principal, nil
}

func (s *Store) RevokeOperatorSession(ctx context.Context, token string, now time.Time) error {
	if !validOperatorSessionToken(token) {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE operator_sessions
		SET revoked_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL`, operatorSessionTokenHash(token), now.UTC())
	if err != nil {
		return fmt.Errorf("revoke operator session: %w", err)
	}
	return nil
}

func validOperatorSessionToken(token string) bool {
	decoded, err := hex.DecodeString(token)
	return err == nil && len(decoded) == 32
}

func operatorSessionTokenHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}
