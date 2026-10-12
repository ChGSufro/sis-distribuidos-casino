package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Session representa una sesion activa con sus tokens hasheados.
type Session struct {
	ID               string
	UserID           string
	TokenHash        string
	RefreshTokenHash *string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
}

// SessionRepo encapsula el acceso a la tabla sessions.
type SessionRepo struct {
	pool *pgxpool.Pool
}

const sessionColumns = `id::text, user_id::text, token_hash, refresh_token_hash, expires_at, revoked_at, created_at`

// Create inserta una nueva sesion.
func (r *SessionRepo) Create(ctx context.Context, s *Session) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sessions (id, user_id, token_hash, refresh_token_hash, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())`,
		s.ID, s.UserID, s.TokenHash, s.RefreshTokenHash, s.ExpiresAt,
	)
	return err
}

// GetByTokenHash busca la sesion asociada a un access token.
func (r *SessionRepo) GetByTokenHash(ctx context.Context, hash string) (*Session, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE token_hash = $1`, hash)
	return scanSession(row)
}

// GetByRefreshHash busca la sesion asociada a un refresh token.
func (r *SessionRepo) GetByRefreshHash(ctx context.Context, hash string) (*Session, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE refresh_token_hash = $1`, hash)
	return scanSession(row)
}

// Revoke marca una sesion como revocada.
func (r *SessionRepo) Revoke(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

// Rotate actualiza los hashes de tokens de una sesion (usado en RefreshToken).
func (r *SessionRepo) Rotate(ctx context.Context, id, tokenHash, refreshHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sessions SET token_hash = $2, refresh_token_hash = $3, expires_at = $4 WHERE id = $1`,
		id, tokenHash, refreshHash, expiresAt,
	)
	return err
}

func scanSession(row pgx.Row) (*Session, error) {
	var s Session
	err := row.Scan(&s.ID, &s.UserID, &s.TokenHash, &s.RefreshTokenHash, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
