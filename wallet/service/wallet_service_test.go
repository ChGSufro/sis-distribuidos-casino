package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/wallet/domain"
	"github.com/ChGSufro/sis-distribuidos-casino/wallet/service"
	"github.com/google/uuid"
)

type mockWalletRepository struct {
	wallets map[uuid.UUID]*domain.Wallet
	holds   map[uuid.UUID]*domain.Hold
	byKey   map[string]*domain.Hold
}

func newMockRepo() *mockWalletRepository {
	return &mockWalletRepository{
		wallets: make(map[uuid.UUID]*domain.Wallet),
		holds:   make(map[uuid.UUID]*domain.Hold),
		byKey:   make(map[string]*domain.Hold),
	}
}

func (m *mockRepo) GetOrCreateWallet(ctx context.Context, userID uuid.UUID) (*domain.Wallet, bool, error) {
	for _, w := range m.wallets {
		if w.UserID == userID {
			return w, false, nil
		}
	}
	w := &domain.Wallet{
		ID:           uuid.New(),
		UserID:       userID,
		BalanceCents: 0,
		Version:      1,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.wallets[w.ID] = w
	return w, true, nil
}

func (m *mockRepo) GetWalletByUserID(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error) {
	for _, w := range m.wallets {
		if w.UserID == userID {
			return w, nil
		}
	}
	return nil, domain.ErrWalletNotFound
}

func (m *mockRepo) Deposit(ctx context.Context, userID uuid.UUID, amountCents int64) (*domain.Wallet, *domain.Transaction, error) {
	w, err := m.GetWalletByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	w.BalanceCents += amountCents
	w.Version++
	tx := &domain.Transaction{
		ID:                uuid.New(),
		WalletID:          w.ID,
		Type:              domain.TxTypeDeposit,
		AmountCents:       amountCents,
		BalanceAfterCents: w.BalanceCents,
		CreatedAt:         time.Now(),
	}
	return w, tx, nil
}

func (m *mockRepo) HoldFunds(ctx context.Context, userID uuid.UUID, amountCents int64, gameType, gameRefID, idempotencyKey string) (*domain.Hold, *domain.Transaction, *domain.Wallet, error) {
	if h, ok := m.byKey[idempotencyKey]; ok {
		w := m.wallets[h.WalletID]
		return h, nil, w, nil
	}
	w, err := m.GetWalletByUserID(ctx, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	if w.BalanceCents < amountCents {
		return nil, nil, nil, domain.ErrInsufficientFunds
	}
	w.BalanceCents -= amountCents
	w.Version++
	h := &domain.Hold{
		ID:             uuid.New(),
		WalletID:       w.ID,
		AmountCents:    amountCents,
		GameType:       gameType,
		GameRefID:      gameRefID,
		Status:         domain.HoldStatusActive,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}
	m.holds[h.ID] = h
	m.byKey[idempotencyKey] = h
	tx := &domain.Transaction{
		ID:                uuid.New(),
		WalletID:          w.ID,
		HoldID:            &h.ID,
		Type:              domain.TxTypeHold,
		AmountCents:       -amountCents,
		BalanceAfterCents: w.BalanceCents,
		CreatedAt:         time.Now(),
	}
	return h, tx, w, nil
}

func (m *mockRepo) GetHoldByIdempotencyKey(ctx context.Context, idempotencyKey string) (*domain.Hold, error) {
	if h, ok := m.byKey[idempotencyKey]; ok {
		return h, nil
	}
	return nil, domain.ErrHoldNotFound
}

func (m *mockRepo) GetHoldByID(ctx context.Context, holdID uuid.UUID) (*domain.Hold, error) {
	if h, ok := m.holds[holdID]; ok {
		return h, nil
	}
	return nil, domain.ErrHoldNotFound
}

func (m *mockRepo) SettleHold(ctx context.Context, holdID uuid.UUID, winAmountCents int64) (*domain.Hold, *domain.Transaction, *domain.Wallet, error) {
	h, err := m.GetHoldByID(ctx, holdID)
	if err != nil {
		return nil, nil, nil, err
	}
	if h.Status != domain.HoldStatusActive {
		return nil, nil, nil, domain.ErrHoldAlreadyResolved
	}
	h.Status = domain.HoldStatusSettled
	now := time.Now()
	h.ResolvedAt = &now

	w := m.wallets[h.WalletID]
	w.BalanceCents += winAmountCents
	w.Version++
	tx := &domain.Transaction{
		ID:                uuid.New(),
		WalletID:          w.ID,
		HoldID:            &h.ID,
		Type:              domain.TxTypeSettle,
		AmountCents:       winAmountCents,
		BalanceAfterCents: w.BalanceCents,
		CreatedAt:         time.Now(),
	}
	return h, tx, w, nil
}

func (m *mockRepo) ReleaseHold(ctx context.Context, holdID uuid.UUID) (*domain.Hold, *domain.Transaction, *domain.Wallet, error) {
	h, err := m.GetHoldByID(ctx, holdID)
	if err != nil {
		return nil, nil, nil, err
	}
	if h.Status != domain.HoldStatusActive {
		return nil, nil, nil, domain.ErrHoldAlreadyResolved
	}
	h.Status = domain.HoldStatusReleased
	now := time.Now()
	h.ResolvedAt = &now

	w := m.wallets[h.WalletID]
	w.BalanceCents += h.AmountCents
	w.Version++
	tx := &domain.Transaction{
		ID:                uuid.New(),
		WalletID:          w.ID,
		HoldID:            &h.ID,
		Type:              domain.TxTypeRelease,
		AmountCents:       h.AmountCents,
		BalanceAfterCents: w.BalanceCents,
		CreatedAt:         time.Now(),
	}
	return h, tx, w, nil
}

type mockRepo = mockWalletRepository

func TestWalletServiceFlow(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewWalletService(repo)

	userID := uuid.New().String()

	// 1. InitWallet
	w, err := svc.InitWallet(ctx, userID)
	if err != nil {
		t.Fatalf("InitWallet failed: %v", err)
	}
	if w.BalanceCents != 0 {
		t.Fatalf("expected balance 0, got %d", w.BalanceCents)
	}

	// 2. Deposit
	_, newBal, err := svc.Deposit(ctx, userID, 10000) // $100.00
	if err != nil {
		t.Fatalf("Deposit failed: %v", err)
	}
	if newBal != 10000 {
		t.Fatalf("expected new balance 10000, got %d", newBal)
	}

	// 3. GetBalance
	bal, err := svc.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}
	if bal.BalanceCents != 10000 {
		t.Fatalf("expected balance 10000, got %d", bal.BalanceCents)
	}

	// 4. Hold
	hold, err := svc.Hold(ctx, userID, 3000, "blackjack", "hand-1", "idemp-1")
	if err != nil {
		t.Fatalf("Hold failed: %v", err)
	}
	if hold.AmountCents != 3000 {
		t.Fatalf("expected hold amount 3000, got %d", hold.AmountCents)
	}

	// Balance after hold must be 7000
	bal, _ = svc.GetBalance(ctx, userID)
	if bal.BalanceCents != 7000 {
		t.Fatalf("expected balance 7000 after hold, got %d", bal.BalanceCents)
	}

	// Idempotent Hold check
	holdDup, err := svc.Hold(ctx, userID, 3000, "blackjack", "hand-1", "idemp-1")
	if err != nil {
		t.Fatalf("idempotent hold failed: %v", err)
	}
	if holdDup.ID != hold.ID {
		t.Fatalf("expected identical hold ID for idempotent key")
	}

	// 5. Insufficient funds Hold
	_, err = svc.Hold(ctx, userID, 90000, "blackjack", "hand-2", "idemp-2")
	if err != domain.ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}

	// 6. Settle with win (win $60.00 -> 6000 cents)
	settledHold, _, finalBal, err := svc.Settle(ctx, hold.ID.String(), 6000)
	if err != nil {
		t.Fatalf("Settle failed: %v", err)
	}
	if settledHold.Status != domain.HoldStatusSettled {
		t.Fatalf("expected settled status, got %s", settledHold.Status)
	}
	if finalBal != 13000 { // 7000 + 6000
		t.Fatalf("expected final balance 13000, got %d", finalBal)
	}

	// 7. Settle already settled hold fails
	_, _, _, err = svc.Settle(ctx, hold.ID.String(), 1000)
	if err != domain.ErrHoldAlreadyResolved {
		t.Fatalf("expected ErrHoldAlreadyResolved, got %v", err)
	}

	// 8. Release flow
	hold2, err := svc.Hold(ctx, userID, 2000, "poker", "table-1", "idemp-3")
	if err != nil {
		t.Fatalf("Hold 2 failed: %v", err)
	}
	bal, _ = svc.GetBalance(ctx, userID)
	if bal.BalanceCents != 11000 {
		t.Fatalf("expected balance 11000, got %d", bal.BalanceCents)
	}

	releasedHold, _, relBal, err := svc.Release(ctx, hold2.ID.String())
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	if releasedHold.Status != domain.HoldStatusReleased {
		t.Fatalf("expected released status, got %s", releasedHold.Status)
	}
	if relBal != 13000 { // 11000 + 2000
		t.Fatalf("expected balance 13000 after release, got %d", relBal)
	}
}

func TestWalletServiceValidations(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewWalletService(repo)

	// Invalid UUID
	_, err := svc.InitWallet(ctx, "not-a-uuid")
	if err != domain.ErrInvalidUUID {
		t.Fatalf("expected ErrInvalidUUID, got %v", err)
	}

	// Invalid Deposit amount
	_, _, err = svc.Deposit(ctx, uuid.New().String(), 0)
	if err != domain.ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount for 0, got %v", err)
	}

	_, _, err = svc.Deposit(ctx, uuid.New().String(), -100)
	if err != domain.ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount for negative, got %v", err)
	}

	// Invalid Settle win amount
	_, _, _, err = svc.Settle(ctx, uuid.New().String(), -10)
	if err != domain.ErrInvalidWinAmount {
		t.Fatalf("expected ErrInvalidWinAmount, got %v", err)
	}
}

