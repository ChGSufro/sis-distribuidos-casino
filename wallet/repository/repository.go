package repository

import (
	"context"

	"github.com/ChGSufro/sis-distribuidos-casino/wallet/domain"
	"github.com/google/uuid"
)

type WalletRepository interface {
	GetOrCreateWallet(ctx context.Context, userID uuid.UUID) (*domain.Wallet, bool, error)
	GetWalletByUserID(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error)
	Deposit(ctx context.Context, userID uuid.UUID, amountCents int64) (*domain.Wallet, *domain.Transaction, error)
	HoldFunds(ctx context.Context, userID uuid.UUID, amountCents int64, gameType, gameRefID, idempotencyKey string) (*domain.Hold, *domain.Transaction, *domain.Wallet, error)
	GetHoldByIdempotencyKey(ctx context.Context, idempotencyKey string) (*domain.Hold, error)
	GetHoldByID(ctx context.Context, holdID uuid.UUID) (*domain.Hold, error)
	SettleHold(ctx context.Context, holdID uuid.UUID, winAmountCents int64) (*domain.Hold, *domain.Transaction, *domain.Wallet, error)
	ReleaseHold(ctx context.Context, holdID uuid.UUID) (*domain.Hold, *domain.Transaction, *domain.Wallet, error)
}

