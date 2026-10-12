package db

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_init.sql
var initSQL string

// Connect establece conexión con el pool de PostgreSQL reintentando en caso de no disponibilidad temporal.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("error al parsear DSN de base de datos: %w", err)
	}

	poolConfig.MaxConns = 25
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	var pool *pgxpool.Pool
	maxRetries := 15
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = pool.Ping(pingCtx)
			cancel()
			if err == nil {
				log.Printf("Conexión exitosa a PostgreSQL (intento %d)", attempt)
				break
			}
			pool.Close()
		}

		log.Printf("Aguardando por PostgreSQL (%d/%d): %v", attempt, maxRetries, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a PostgreSQL tras varios intentos: %w", err)
	}

	// Ejecutar migraciones iniciales
	if err := RunMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("error al ejecutar migraciones: %w", err)
	}

	return pool, nil
}

// RunMigrations aplica el esquema SQL embebido si aún no existe.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	log.Println("Ejecutando migraciones iniciales de Wallet DB...")
	_, err := pool.Exec(ctx, initSQL)
	if err != nil {
		return fmt.Errorf("fallo al aplicar esquema inicial: %w", err)
	}
	log.Println("Migraciones iniciales aplicadas correctamente.")
	return nil
}

