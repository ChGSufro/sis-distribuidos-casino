package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User representa un usuario persistido en users_db.
type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLoginAt  *time.Time
}

// UserRepo encapsula el acceso a la tabla users.
type UserRepo struct {
	pool *pgxpool.Pool
}

const userColumns = `id::text, username, email, password_hash, status, created_at, updated_at, last_login_at`

// ErrEmailTaken / ErrUsernameTaken clasifican violaciones de unicidad.
var (
	ErrEmailTaken    = errors.New("email ya registrado")
	ErrUsernameTaken = errors.New("username ya registrado")
)

// Create inserta un nuevo usuario.
func (r *UserRepo) Create(ctx context.Context, u *User) error {
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, username, email, password_hash, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		u.ID, u.Username, u.Email, u.PasswordHash, u.Status, u.CreatedAt, u.UpdatedAt,
	)
	if isUniqueViolation(err) {
		if existing, getErr := r.GetByEmail(ctx, u.Email); getErr == nil && existing != nil {
			return ErrEmailTaken
		}
		return ErrUsernameTaken
	}
	return err
}

// GetByEmail busca un usuario por correo.
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)
	return scanUser(row)
}

// GetByID busca un usuario por identificador.
func (r *UserRepo) GetByID(ctx context.Context, id string) (*User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}

// UpdateLastLogin registra la fecha del ultimo inicio de sesion.
func (r *UserRepo) UpdateLastLogin(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_login_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
