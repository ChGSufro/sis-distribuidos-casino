package engine_test

import (
	"testing"

	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/domain"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/engine"
)

func TestCalculateHandValue(t *testing.T) {
	// A + K = 21
	bjCards := []domain.Card{
		domain.NewCard(domain.SuitSpades, domain.RankAce, false),
		domain.NewCard(domain.SuitHearts, domain.RankKing, false),
	}
	if val := domain.CalculateHandValue(bjCards); val != 21 {
		t.Fatalf("expected 21, got %d", val)
	}

	// A + A = 12
	twoAces := []domain.Card{
		domain.NewCard(domain.SuitSpades, domain.RankAce, false),
		domain.NewCard(domain.SuitHearts, domain.RankAce, false),
	}
	if val := domain.CalculateHandValue(twoAces); val != 12 {
		t.Fatalf("expected 12 for two aces, got %d", val)
	}

	// A + 9 + A = 21
	soft21 := []domain.Card{
		domain.NewCard(domain.SuitSpades, domain.RankAce, false),
		domain.NewCard(domain.SuitHearts, domain.Rank9, false),
		domain.NewCard(domain.SuitClubs, domain.RankAce, false),
	}
	if val := domain.CalculateHandValue(soft21); val != 21 {
		t.Fatalf("expected 21, got %d", val)
	}

	// 10 + 7 + 8 = 25 (Bust)
	bustCards := []domain.Card{
		domain.NewCard(domain.SuitSpades, domain.Rank10, false),
		domain.NewCard(domain.SuitHearts, domain.Rank7, false),
		domain.NewCard(domain.SuitClubs, domain.Rank8, false),
	}
	if val := domain.CalculateHandValue(bustCards); val != 25 {
		t.Fatalf("expected 25, got %d", val)
	}
}

func TestEngineSurrender(t *testing.T) {
	eng := engine.NewEngine()
	hand := &domain.HandDocument{
		UserID: "user-1",
		SubHands: []domain.SubHand{
			{
				Index:          0,
				Cards:          []domain.Card{domain.NewCard(domain.SuitHearts, domain.Rank10, false), domain.NewCard(domain.SuitClubs, domain.Rank6, false)},
				BetAmountCents: 1000,
				Status:         domain.SubHandStatusActive,
			},
		},
		DealerCards: []domain.Card{
			domain.NewCard(domain.SuitDiamonds, domain.RankAce, false),
			domain.NewCard(domain.SuitDiamonds, domain.Rank9, true),
		},
		Status: domain.HandStatusActive,
	}

	err := eng.Surrender(hand, 0)
	if err != nil {
		t.Fatalf("Surrender failed: %v", err)
	}

	if hand.Status != domain.HandStatusSettled {
		t.Fatalf("expected hand status settled, got %s", hand.Status)
	}
	if hand.TotalWinAmountCents == nil || *hand.TotalWinAmountCents != 500 {
		t.Fatalf("expected surrender return of 500 cents, got %v", hand.TotalWinAmountCents)
	}
}

func TestEngineDouble(t *testing.T) {
	eng := engine.NewEngine()
	deck := domain.NewStandardDeck(1)

	hand := &domain.HandDocument{
		UserID: "user-1",
		SubHands: []domain.SubHand{
			{
				Index:          0,
				Cards:          []domain.Card{domain.NewCard(domain.SuitHearts, domain.Rank5, false), domain.NewCard(domain.SuitClubs, domain.Rank6, false)},
				BetAmountCents: 1000,
				Status:         domain.SubHandStatusActive,
			},
		},
		DealerCards: []domain.Card{
			domain.NewCard(domain.SuitDiamonds, domain.Rank7, false),
			domain.NewCard(domain.SuitDiamonds, domain.Rank10, true),
		},
		Status: domain.HandStatusActive,
	}

	err := eng.Double(hand, 0, deck)
	if err != nil {
		t.Fatalf("Double failed: %v", err)
	}

	if hand.SubHands[0].BetAmountCents != 2000 {
		t.Fatalf("expected bet to double to 2000, got %d", hand.SubHands[0].BetAmountCents)
	}
	if len(hand.SubHands[0].Cards) != 3 {
		t.Fatalf("expected exactly 3 cards after double, got %d", len(hand.SubHands[0].Cards))
	}
	if hand.Status != domain.HandStatusSettled {
		t.Fatalf("expected hand settled after double finished, got %s", hand.Status)
	}
}

func TestEngineSplit(t *testing.T) {
	eng := engine.NewEngine()
	deck := domain.NewStandardDeck(1)

	hand := &domain.HandDocument{
		UserID: "user-1",
		SubHands: []domain.SubHand{
			{
				Index:          0,
				Cards:          []domain.Card{domain.NewCard(domain.SuitHearts, domain.Rank8, false), domain.NewCard(domain.SuitClubs, domain.Rank8, false)},
				BetAmountCents: 1000,
				Status:         domain.SubHandStatusActive,
			},
		},
		DealerCards: []domain.Card{
			domain.NewCard(domain.SuitDiamonds, domain.Rank6, false),
			domain.NewCard(domain.SuitDiamonds, domain.Rank10, true),
		},
		Status: domain.HandStatusActive,
	}

	err := eng.Split(hand, 0, deck, "new-hold-id")
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(hand.SubHands) != 2 {
		t.Fatalf("expected 2 subhands after split, got %d", len(hand.SubHands))
	}
	if hand.SubHands[0].HoldID == hand.SubHands[1].HoldID {
		t.Fatalf("expected different hold IDs for split subhands")
	}
	if len(hand.SubHands[0].Cards) != 2 || len(hand.SubHands[1].Cards) != 2 {
		t.Fatalf("expected 2 cards per split subhand")
	}
}

