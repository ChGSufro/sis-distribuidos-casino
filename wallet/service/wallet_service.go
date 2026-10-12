package service

import (
	"context"
	"strings"

	"github.com/ChGSufro/sis-distribuidos-casino/wallet/domain"
	"github.com/ChGSufro/sis-distribuidos-casino/wallet/repository"
	"github.com/google/uuid"
)

type WalletService interface {
	InitWallet(ctx context.Context, userIDStr string) (*domain.Wallet, error)
	GetBalance(ctx context.Context, userIDStr string) (*domain.Wallet, error)
	Deposit(ctx context.Context, userIDStr string, amountCents int64) (*domain.Transaction, int64, error)
	Hold(ctx context.Context, userIDStr string, amountCents int64, gameType, gameRefID, idempotencyKey string) (*domain.Hold, error)
	Settle(ctx context.Context, holdIDStr string, winAmountCents int64) (*domain.Hold, *domain.Transaction, int64, error)
	Release(ctx context.Context, holdIDStr string) (*domain.Hold, *domain.Transaction, int64, error)
}

type walletService struct {
	repo repository.WalletRepository
}

func NewWalletService(repo repository.WalletRepository) WalletService {
	return &walletService{repo: repo}
}

func (s *walletService) InitWallet(ctx context.Context, userIDStr string) (*domain.Wallet, error) {
	uid, err := uuid.Parse(strings.TrimSpace(userIDStr))
	if err != nil {
		return nil, domain.ErrInvalidUUID
	}
	w, _, err := s.repo.GetOrCreateWallet(ctx, uid)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (s *walletService) GetBalance(ctx context.Context, userIDStr string) (*domain.Wallet, error) {
	uid, err := uuid.Parse(strings.TrimSpace(userIDStr))
	if err != nil {
		return nil, domain.ErrInvalidUUID
	}
	return s.repo.GetWalletByUserID(ctx, uid)
}

func (s *walletService) Deposit(ctx context.Context, userIDStr string, amountCents int64) (*domain.Transaction, int64, error) {
	if amountCents <= 0 {
		return nil, 0, domain.ErrInvalidAmount
	}
	uid, err := uuid.Parse(strings.TrimSpace(userIDStr))
	if err != nil {
		return nil, 0, domain.ErrInvalidUUID
	}

	wallet, tx, err := s.repo.Deposit(ctx, uid, amountCents)
	if err != nil {
		return nil, 0, err
	}
	return tx, wallet.BalanceCents, nil
}

func (s *walletService) Hold(ctx context.Context, userIDStr string, amountCents int64, gameType, gameRefID, idempotencyKey string) (*domain.Hold, error) {
	if amountCents <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	if strings.TrimSpace(gameRefID) == "" {
		return nil, domain.ErrEmptyGameRef
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, domain.ErrEmptyIdempotencyKey
	}
	uid, err := uuid.Parse(strings.TrimSpace(userIDStr))
	if err != nil {
		return nil, domain.ErrInvalidUUID
	}

	hold, _, _, err := s.repo.HoldFunds(ctx, uid, amountCents, gameType, gameRefID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	return hold, nil
}

func (s *walletService) Settle(ctx context.Context, holdIDStr string, winAmountCents int64) (*domain.Hold, *domain.Transaction, int64, error) {
	if winAmountCents < 0 {
		return nil, nil, 0, domain.ErrInvalidWinAmount
	}
	hid, err := uuid.Parse(strings.TrimSpace(holdIDStr))
	if err != nil {
		return nil, nil, 0, domain.ErrInvalidUUID
	}

	hold, tx, wallet, err := s.repo.SettleHold(ctx, hid, winAmountCents)
	if err != nil {
		return nil, nil, 0, err
	}
	return hold, tx, wallet.BalanceCents, nil
}

func (s *walletService) Release(ctx context.Context, holdIDStr string) (*domain.Hold, *domain.Transaction, int64, error) {
	hid, err := uuid.Parse(strings.TrimSpace(holdIDStr))
	if err != nil {
		return nil, nil, 0, domain.ErrInvalidUUID
	}

	hold, tx, wallet, err := s.repo.ReleaseHold(ctx, hid)
	if err != nil {
		return nil, nil, 0, err
	}
	return hold, tx, wallet.BalanceCents, nil
}

