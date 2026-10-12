package service_test

import (
	"context"
	"testing"

	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/domain"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/engine"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/service"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type mockHandRepo struct {
	hands map[string]*domain.HandDocument
}

func newMockHandRepo() *mockHandRepo {
	return &mockHandRepo{hands: make(map[string]*domain.HandDocument)}
}

func (m *mockHandRepo) CreateHand(ctx context.Context, hand *domain.HandDocument) (*domain.HandDocument, error) {
	if hand.ID.IsZero() {
		hand.ID = bson.NewObjectID()
	}
	m.hands[hand.ID.Hex()] = hand
	return hand, nil
}

func (m *mockHandRepo) GetHandByID(ctx context.Context, id string) (*domain.HandDocument, error) {
	h, ok := m.hands[id]
	if !ok {
		return nil, domain.ErrHandNotFound
	}
	return h, nil
}

func (m *mockHandRepo) GetHandByIdempotencyKey(ctx context.Context, idempKey string) (*domain.HandDocument, error) {
	for _, h := range m.hands {
		if h.IdempotencyKey == idempKey {
			return h, nil
		}
	}
	return nil, domain.ErrHandNotFound
}

func (m *mockHandRepo) UpdateHand(ctx context.Context, hand *domain.HandDocument) error {
	m.hands[hand.ID.Hex()] = hand
	return nil
}

type mockWalletClient struct {
	holds   map[string]int64
	settles map[string]int64
}

func newMockWalletClient() *mockWalletClient {
	return &mockWalletClient{
		holds:   make(map[string]int64),
		settles: make(map[string]int64),
	}
}

func (m *mockWalletClient) Hold(ctx context.Context, userID string, amountCents int64, gameRefID, idempotencyKey string) (string, error) {
	holdID := "hold-" + uuid.New().String()
	m.holds[holdID] = amountCents
	return holdID, nil
}

func (m *mockWalletClient) Settle(ctx context.Context, holdID string, winAmountCents int64) error {
	m.settles[holdID] = winAmountCents
	return nil
}

func (m *mockWalletClient) Release(ctx context.Context, holdID string) error {
	delete(m.holds, holdID)
	return nil
}

func (m *mockWalletClient) Close() error {
	return nil
}

type mockHistoryClient struct {
	events []string
}

func (m *mockHistoryClient) RecordGameEvent(ctx context.Context, eventID, userID, gameRefID, action, detailsJSON string) error {
	m.events = append(m.events, action)
	return nil
}

func (m *mockHistoryClient) Close() error {
	return nil
}

func TestBlackjackServiceFlow(t *testing.T) {
	ctx := context.Background()
	repo := newMockHandRepo()
	wallet := newMockWalletClient()
	history := &mockHistoryClient{}
	eng := engine.NewEngine()

	svc := service.NewBlackjackService(repo, eng, wallet, history)
	userID := uuid.New().String()

	// 1. CreateHand
	hand, err := svc.CreateHand(ctx, userID, 2500)
	if err != nil {
		t.Fatalf("CreateHand failed: %v", err)
	}

	if len(hand.SubHands) != 1 {
		t.Fatalf("expected 1 subhand, got %d", len(hand.SubHands))
	}
	if hand.SubHands[0].BetAmountCents != 2500 {
		t.Fatalf("expected bet 2500, got %d", hand.SubHands[0].BetAmountCents)
	}

	// 2. Si la mano no terminó en Blackjack natural, hacer Stand
	if hand.Status == domain.HandStatusActive {
		actionHand, err := svc.PlayerAction(ctx, hand.ID.Hex(), userID, 0, "stand")
		if err != nil {
			t.Fatalf("PlayerAction stand failed: %v", err)
		}
		if actionHand.Status != domain.HandStatusSettled {
			t.Fatalf("expected hand status settled after stand, got %s", actionHand.Status)
		}
		if actionHand.TotalWinAmountCents == nil {
			t.Fatalf("expected total win amount calculated")
		}
	}

	// 3. GetHandState
	state, err := svc.GetHandState(ctx, hand.ID.Hex(), userID)
	if err != nil {
		t.Fatalf("GetHandState failed: %v", err)
	}
	if state.ID != hand.ID {
		t.Fatalf("mismatched hand ID")
	}

	// 4. Unauthorized User
	otherUser := uuid.New().String()
	_, err = svc.GetHandState(ctx, hand.ID.Hex(), otherUser)
	if err != domain.ErrUnauthorizedHand {
		t.Fatalf("expected ErrUnauthorizedHand, got %v", err)
	}
}

func TestBlackjackServiceValidations(t *testing.T) {
	ctx := context.Background()
	repo := newMockHandRepo()
	wallet := newMockWalletClient()
	history := &mockHistoryClient{}
	eng := engine.NewEngine()

	svc := service.NewBlackjackService(repo, eng, wallet, history)

	// Invalid User ID
	_, err := svc.CreateHand(ctx, "invalid-user", 1000)
	if err != domain.ErrInvalidUserID {
		t.Fatalf("expected ErrInvalidUserID, got %v", err)
	}

	// Invalid Bet
	_, err = svc.CreateHand(ctx, uuid.New().String(), 0)
	if err != domain.ErrInvalidBetAmount {
		t.Fatalf("expected ErrInvalidBetAmount, got %v", err)
	}
}

