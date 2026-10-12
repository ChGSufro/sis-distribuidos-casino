package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/wallet/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresWalletRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresWalletRepository(pool *pgxpool.Pool) *PostgresWalletRepository {
	return &PostgresWalletRepository{pool: pool}
}

func (r *PostgresWalletRepository) GetOrCreateWallet(ctx context.Context, userID uuid.UUID) (*domain.Wallet, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("iniciar transaccion: %w", err)
	}
	defer tx.Rollback(ctx)

	var w domain.Wallet
	queryInsert := `
		INSERT INTO wallets (user_id, balance_cents, version, created_at, updated_at)
		VALUES ($1, 0, 1, NOW(), NOW())
		ON CONFLICT (user_id) DO NOTHING
		RETURNING id, user_id, balance_cents, version, created_at, updated_at;
	`
	err = tx.QueryRow(ctx, queryInsert, userID).Scan(
		&w.ID, &w.UserID, &w.BalanceCents, &w.Version, &w.CreatedAt, &w.UpdatedAt,
	)

	if err == nil {
		// Fue creada recién, registrar transacción inicial
		_, errTx := tx.Exec(ctx, `
			INSERT INTO transactions (wallet_id, type, amount_cents, balance_after_cents, created_at)
			VALUES ($1, $2, 0, 0, NOW())
		`, w.ID, domain.TxTypeInit)
		if errTx != nil {
			return nil, false, fmt.Errorf("registrar transaccion inicial: %w", errTx)
		}

		if errCommit := tx.Commit(ctx); errCommit != nil {
			return nil, false, fmt.Errorf("commit transaccion: %w", errCommit)
		}
		return &w, true, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("insertar billetera: %w", err)
	}

	// Ya existía la billetera
	querySelect := `
		SELECT id, user_id, balance_cents, version, created_at, updated_at
		FROM wallets
		WHERE user_id = $1;
	`
	err = tx.QueryRow(ctx, querySelect, userID).Scan(
		&w.ID, &w.UserID, &w.BalanceCents, &w.Version, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("obtener billetera existente: %w", err)
	}

	if errCommit := tx.Commit(ctx); errCommit != nil {
		return nil, false, fmt.Errorf("commit transaccion: %w", errCommit)
	}
	return &w, false, nil
}

func (r *PostgresWalletRepository) GetWalletByUserID(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error) {
	var w domain.Wallet
	query := `
		SELECT id, user_id, balance_cents, version, created_at, updated_at
		FROM wallets
		WHERE user_id = $1;
	`
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&w.ID, &w.UserID, &w.BalanceCents, &w.Version, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrWalletNotFound
		}
		return nil, fmt.Errorf("consultar billetera: %w", err)
	}
	return &w, nil
}

func (r *PostgresWalletRepository) Deposit(ctx context.Context, userID uuid.UUID, amountCents int64) (*domain.Wallet, *domain.Transaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("iniciar transaccion: %w", err)
	}
	defer tx.Rollback(ctx)

	var w domain.Wallet
	// Bloqueo pesimista de fila para asegurar atomicidad
	queryLock := `
		SELECT id, user_id, balance_cents, version, created_at, updated_at
		FROM wallets
		WHERE user_id = $1
		FOR UPDATE;
	`
	err = tx.QueryRow(ctx, queryLock, userID).Scan(
		&w.ID, &w.UserID, &w.BalanceCents, &w.Version, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrWalletNotFound
		}
		return nil, nil, fmt.Errorf("bloquear billetera: %w", err)
	}

	newBalance := w.BalanceCents + amountCents
	queryUpdate := `
		UPDATE wallets
		SET balance_cents = $1, version = version + 1, updated_at = NOW()
		WHERE id = $2
		RETURNING version, updated_at;
	`
	err = tx.QueryRow(ctx, queryUpdate, newBalance, w.ID).Scan(&w.Version, &w.UpdatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("actualizar saldo de billetera: %w", err)
	}
	w.BalanceCents = newBalance

	var t domain.Transaction
	t.WalletID = w.ID
	t.Type = domain.TxTypeDeposit
	t.AmountCents = amountCents
	t.BalanceAfterCents = newBalance

	queryTx := `
		INSERT INTO transactions (wallet_id, type, amount_cents, balance_after_cents, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, created_at;
	`
	err = tx.QueryRow(ctx, queryTx, t.WalletID, t.Type, t.AmountCents, t.BalanceAfterCents).Scan(
		&t.ID, &t.CreatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("insertar transaccion de deposito: %w", err)
	}

	if errCommit := tx.Commit(ctx); errCommit != nil {
		return nil, nil, fmt.Errorf("commit transaccion: %w", errCommit)
	}

	return &w, &t, nil
}

func (r *PostgresWalletRepository) HoldFunds(ctx context.Context, userID uuid.UUID, amountCents int64, gameType, gameRefID, idempotencyKey string) (*domain.Hold, *domain.Transaction, *domain.Wallet, error) {
	// Verificar primero si ya existe un hold con este idempotencyKey
	existingHold, err := r.GetHoldByIdempotencyKey(ctx, idempotencyKey)
	if err == nil && existingHold != nil {
		wallet, _ := r.GetWalletByUserID(ctx, userID)
		return existingHold, nil, wallet, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("iniciar transaccion: %w", err)
	}
	defer tx.Rollback(ctx)

	var w domain.Wallet
	queryLock := `
		SELECT id, user_id, balance_cents, version, created_at, updated_at
		FROM wallets
		WHERE user_id = $1
		FOR UPDATE;
	`
	err = tx.QueryRow(ctx, queryLock, userID).Scan(
		&w.ID, &w.UserID, &w.BalanceCents, &w.Version, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil, domain.ErrWalletNotFound
		}
		return nil, nil, nil, fmt.Errorf("bloquear billetera: %w", err)
	}

	if w.BalanceCents < amountCents {
		return nil, nil, nil, domain.ErrInsufficientFunds
	}

	newBalance := w.BalanceCents - amountCents
	queryUpdate := `
		UPDATE wallets
		SET balance_cents = $1, version = version + 1, updated_at = NOW()
		WHERE id = $2
		RETURNING version, updated_at;
	`
	err = tx.QueryRow(ctx, queryUpdate, newBalance, w.ID).Scan(&w.Version, &w.UpdatedAt)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("descontar saldo para hold: %w", err)
	}
	w.BalanceCents = newBalance

	var h domain.Hold
	h.WalletID = w.ID
	h.AmountCents = amountCents
	h.GameType = gameType
	h.GameRefID = gameRefID
	h.Status = domain.HoldStatusActive
	h.IdempotencyKey = idempotencyKey

	queryHold := `
		INSERT INTO holds (wallet_id, amount_cents, game_type, game_ref_id, status, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id, created_at;
	`
	err = tx.QueryRow(ctx, queryHold, h.WalletID, h.AmountCents, h.GameType, h.GameRefID, h.Status, h.IdempotencyKey).Scan(
		&h.ID, &h.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			_ = tx.Rollback(ctx)
			// Reintentar lectura idempotente
			existing, getErr := r.GetHoldByIdempotencyKey(ctx, idempotencyKey)
			if getErr == nil {
				wallet, _ := r.GetWalletByUserID(ctx, userID)
				return existing, nil, wallet, nil
			}
		}
		return nil, nil, nil, fmt.Errorf("crear hold: %w", err)
	}

	var t domain.Transaction
	t.WalletID = w.ID
	t.HoldID = &h.ID
	t.Type = domain.TxTypeHold
	t.AmountCents = -amountCents
	t.BalanceAfterCents = newBalance

	queryTx := `
		INSERT INTO transactions (wallet_id, hold_id, type, amount_cents, balance_after_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, created_at;
	`
	err = tx.QueryRow(ctx, queryTx, t.WalletID, t.HoldID, t.Type, t.AmountCents, t.BalanceAfterCents).Scan(
		&t.ID, &t.CreatedAt,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insertar transaccion de hold: %w", err)
	}

	if errCommit := tx.Commit(ctx); errCommit != nil {
		return nil, nil, nil, fmt.Errorf("commit transaccion hold: %w", errCommit)
	}

	return &h, &t, &w, nil
}

func (r *PostgresWalletRepository) GetHoldByIdempotencyKey(ctx context.Context, idempotencyKey string) (*domain.Hold, error) {
	var h domain.Hold
	query := `
		SELECT id, wallet_id, amount_cents, game_type, game_ref_id, status, idempotency_key, created_at, resolved_at
		FROM holds
		WHERE idempotency_key = $1;
	`
	err := r.pool.QueryRow(ctx, query, idempotencyKey).Scan(
		&h.ID, &h.WalletID, &h.AmountCents, &h.GameType, &h.GameRefID, &h.Status, &h.IdempotencyKey, &h.CreatedAt, &h.ResolvedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrHoldNotFound
		}
		return nil, fmt.Errorf("consultar hold por idempotencyKey: %w", err)
	}
	return &h, nil
}

func (r *PostgresWalletRepository) GetHoldByID(ctx context.Context, holdID uuid.UUID) (*domain.Hold, error) {
	var h domain.Hold
	query := `
		SELECT id, wallet_id, amount_cents, game_type, game_ref_id, status, idempotency_key, created_at, resolved_at
		FROM holds
		WHERE id = $1;
	`
	err := r.pool.QueryRow(ctx, query, holdID).Scan(
		&h.ID, &h.WalletID, &h.AmountCents, &h.GameType, &h.GameRefID, &h.Status, &h.IdempotencyKey, &h.CreatedAt, &h.ResolvedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrHoldNotFound
		}
		return nil, fmt.Errorf("consultar hold por id: %w", err)
	}
	return &h, nil
}

func (r *PostgresWalletRepository) SettleHold(ctx context.Context, holdID uuid.UUID, winAmountCents int64) (*domain.Hold, *domain.Transaction, *domain.Wallet, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("iniciar transaccion settle: %w", err)
	}
	defer tx.Rollback(ctx)

	var h domain.Hold
	queryHoldLock := `
		SELECT id, wallet_id, amount_cents, game_type, game_ref_id, status, idempotency_key, created_at, resolved_at
		FROM holds
		WHERE id = $1
		FOR UPDATE;
	`
	err = tx.QueryRow(ctx, queryHoldLock, holdID).Scan(
		&h.ID, &h.WalletID, &h.AmountCents, &h.GameType, &h.GameRefID, &h.Status, &h.IdempotencyKey, &h.CreatedAt, &h.ResolvedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil, domain.ErrHoldNotFound
		}
		return nil, nil, nil, fmt.Errorf("bloquear hold: %w", err)
	}

	if h.Status != domain.HoldStatusActive {
		return nil, nil, nil, domain.ErrHoldAlreadyResolved
	}

	var resolvedAt time.Time
	queryUpdateHold := `
		UPDATE holds
		SET status = $1, resolved_at = NOW()
		WHERE id = $2
		RETURNING resolved_at;
	`
	err = tx.QueryRow(ctx, queryUpdateHold, domain.HoldStatusSettled, h.ID).Scan(&resolvedAt)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("actualizar estado del hold a settled: %w", err)
	}
	h.Status = domain.HoldStatusSettled
	h.ResolvedAt = &resolvedAt

	var w domain.Wallet
	queryWalletLock := `
		SELECT id, user_id, balance_cents, version, created_at, updated_at
		FROM wallets
		WHERE id = $1
		FOR UPDATE;
	`
	err = tx.QueryRow(ctx, queryWalletLock, h.WalletID).Scan(
		&w.ID, &w.UserID, &w.BalanceCents, &w.Version, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("bloquear billetera para settle: %w", err)
	}

	newBalance := w.BalanceCents + winAmountCents
	if winAmountCents > 0 {
		queryUpdateWallet := `
			UPDATE wallets
			SET balance_cents = $1, version = version + 1, updated_at = NOW()
			WHERE id = $2
			RETURNING version, updated_at;
		`
		err = tx.QueryRow(ctx, queryUpdateWallet, newBalance, w.ID).Scan(&w.Version, &w.UpdatedAt)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("acreditar ganancia en billetera: %w", err)
		}
	}
	w.BalanceCents = newBalance

	var t domain.Transaction
	t.WalletID = w.ID
	t.HoldID = &h.ID
	t.Type = domain.TxTypeSettle
	t.AmountCents = winAmountCents
	t.BalanceAfterCents = newBalance

	queryTx := `
		INSERT INTO transactions (wallet_id, hold_id, type, amount_cents, balance_after_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, created_at;
	`
	err = tx.QueryRow(ctx, queryTx, t.WalletID, t.HoldID, t.Type, t.AmountCents, t.BalanceAfterCents).Scan(
		&t.ID, &t.CreatedAt,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insertar transaccion de settle: %w", err)
	}

	if errCommit := tx.Commit(ctx); errCommit != nil {
		return nil, nil, nil, fmt.Errorf("commit transaccion settle: %w", errCommit)
	}

	return &h, &t, &w, nil
}

func (r *PostgresWalletRepository) ReleaseHold(ctx context.Context, holdID uuid.UUID) (*domain.Hold, *domain.Transaction, *domain.Wallet, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("iniciar transaccion release: %w", err)
	}
	defer tx.Rollback(ctx)

	var h domain.Hold
	queryHoldLock := `
		SELECT id, wallet_id, amount_cents, game_type, game_ref_id, status, idempotency_key, created_at, resolved_at
		FROM holds
		WHERE id = $1
		FOR UPDATE;
	`
	err = tx.QueryRow(ctx, queryHoldLock, holdID).Scan(
		&h.ID, &h.WalletID, &h.AmountCents, &h.GameType, &h.GameRefID, &h.Status, &h.IdempotencyKey, &h.CreatedAt, &h.ResolvedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil, domain.ErrHoldNotFound
		}
		return nil, nil, nil, fmt.Errorf("bloquear hold: %w", err)
	}

	if h.Status != domain.HoldStatusActive {
		return nil, nil, nil, domain.ErrHoldAlreadyResolved
	}

	var resolvedAt time.Time
	queryUpdateHold := `
		UPDATE holds
		SET status = $1, resolved_at = NOW()
		WHERE id = $2
		RETURNING resolved_at;
	`
	err = tx.QueryRow(ctx, queryUpdateHold, domain.HoldStatusReleased, h.ID).Scan(&resolvedAt)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("actualizar estado del hold a released: %w", err)
	}
	h.Status = domain.HoldStatusReleased
	h.ResolvedAt = &resolvedAt

	var w domain.Wallet
	queryWalletLock := `
		SELECT id, user_id, balance_cents, version, created_at, updated_at
		FROM wallets
		WHERE id = $1
		FOR UPDATE;
	`
	err = tx.QueryRow(ctx, queryWalletLock, h.WalletID).Scan(
		&w.ID, &w.UserID, &w.BalanceCents, &w.Version, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("bloquear billetera para release: %w", err)
	}

	newBalance := w.BalanceCents + h.AmountCents
	queryUpdateWallet := `
		UPDATE wallets
		SET balance_cents = $1, version = version + 1, updated_at = NOW()
		WHERE id = $2
		RETURNING version, updated_at;
	`
	err = tx.QueryRow(ctx, queryUpdateWallet, newBalance, w.ID).Scan(&w.Version, &w.UpdatedAt)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reintegrar saldo liberado en billetera: %w", err)
	}
	w.BalanceCents = newBalance

	var t domain.Transaction
	t.WalletID = w.ID
	t.HoldID = &h.ID
	t.Type = domain.TxTypeRelease
	t.AmountCents = h.AmountCents
	t.BalanceAfterCents = newBalance

	queryTx := `
		INSERT INTO transactions (wallet_id, hold_id, type, amount_cents, balance_after_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, created_at;
	`
	err = tx.QueryRow(ctx, queryTx, t.WalletID, t.HoldID, t.Type, t.AmountCents, t.BalanceAfterCents).Scan(
		&t.ID, &t.CreatedAt,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insertar transaccion de release: %w", err)
	}

	if errCommit := tx.Commit(ctx); errCommit != nil {
		return nil, nil, nil, fmt.Errorf("commit transaccion release: %w", errCommit)
	}

	return &h, &t, &w, nil
}

