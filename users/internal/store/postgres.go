package store

import (
	"context"
	"errors"

	"github.com/ChGSufro/sis-distribuidos-casino/users/migrations"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound indica que no existe el registro solicitado.
var ErrNotFound = errors.New("registro no encontrado")

// Store agrupa el pool de conexiones y los repositorios de users_db.
type Store struct {
	pool     *pgxpool.Pool
	Users    *UserRepo
	Sessions *SessionRepo
}

// New abre un pool contra users_db y verifica la conexion.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{
		pool:     pool,
		Users:    &UserRepo{pool: pool},
		Sessions: &SessionRepo{pool: pool},
	}, nil
}

// Close libera el pool de conexiones.
func (s *Store) Close() {
	s.pool.Close()
}

// Migrate aplica el esquema inicial (idempotente).
func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, migrations.Init)
	return err
}

// isUniqueViolation detecta el error de unicidad de PostgreSQL (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
