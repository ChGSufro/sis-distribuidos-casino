package engine

import (
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/domain"
)

type Engine interface {
	NewGame(deck *domain.Deck, userID string, betAmountCents int64, holdID string, idempKey string) *domain.HandDocument
	Hit(hand *domain.HandDocument, subHandIdx int32, deck *domain.Deck) error
	Stand(hand *domain.HandDocument, subHandIdx int32, deck *domain.Deck) error
	Double(hand *domain.HandDocument, subHandIdx int32, deck *domain.Deck) error
	Split(hand *domain.HandDocument, subHandIdx int32, deck *domain.Deck, newHoldID string) error
	Surrender(hand *domain.HandDocument, subHandIdx int32) error
	PlayDealerTurn(hand *domain.HandDocument, deck *domain.Deck)
	EvaluateResults(hand *domain.HandDocument)
}

type blackjackEngine struct{}

func NewEngine() Engine {
	return &blackjackEngine{}
}

func (e *blackjackEngine) NewGame(deck *domain.Deck, userID string, betAmountCents int64, holdID string, idempKey string) *domain.HandDocument {
	if deck == nil {
		deck = domain.NewStandardDeck(6)
	}

	p1 := deck.Draw(false)
	d1 := deck.Draw(false) // Dealer carta visible
	p2 := deck.Draw(false)
	d2 := deck.Draw(true)  // Dealer carta oculta

	hand := &domain.HandDocument{
		UserID: userID,
		SubHands: []domain.SubHand{
			{
				Index:          0,
				Cards:          []domain.Card{p1, p2},
				BetAmountCents: betAmountCents,
				HoldID:         holdID,
				Status:         domain.SubHandStatusActive,
				Result:         domain.ResultNone,
			},
		},
		DealerCards:    []domain.Card{d1, d2},
		PlayerActions:  []domain.PlayerAction{},
		Status:         domain.HandStatusActive,
		IdempotencyKey: idempKey,
		CreatedAt:      time.Now(),
	}

	// Comprobar Blackjack natural inicial del jugador
	if domain.IsBlackjack(hand.SubHands[0].Cards) {
		hand.SubHands[0].Status = domain.SubHandStatusBlackjack
		hand.DealerCards[1].Hidden = false // Revelar carta del dealer

		if domain.IsBlackjack(hand.DealerCards) {
			hand.SubHands[0].Result = domain.ResultPush
			win := betAmountCents
			hand.TotalWinAmountCents = &win
		} else {
			hand.SubHands[0].Result = domain.ResultBlackjackWin
			win := betAmountCents * 5 / 2 // Payout 3:2 (2.5x apuesta)
			hand.TotalWinAmountCents = &win
		}

		hand.Status = domain.HandStatusSettled
		now := time.Now()
		hand.SettledAt = &now
	}

	return hand
}

func (e *blackjackEngine) Hit(hand *domain.HandDocument, subHandIdx int32, deck *domain.Deck) error {
	if hand.Status != domain.HandStatusActive {
		return domain.ErrHandNotActive
	}

	subHand, err := getSubHand(hand, subHandIdx)
	if err != nil {
		return err
	}

	if subHand.Status != domain.SubHandStatusActive {
		return domain.ErrSubHandNotActive
	}

	card := deck.Draw(false)
	subHand.Cards = append(subHand.Cards, card)

	val := domain.CalculateHandValue(subHand.Cards)
	if val > 21 {
		subHand.Status = domain.SubHandStatusBust
		subHand.Result = domain.ResultLose
	} else if val == 21 {
		subHand.Status = domain.SubHandStatusStand
	}

	recordAction(hand, subHandIdx, domain.ActionHit)

	if allSubHandsResolved(hand) {
		e.PlayDealerTurn(hand, deck)
	}

	return nil
}

func (e *blackjackEngine) Stand(hand *domain.HandDocument, subHandIdx int32, deck *domain.Deck) error {
	if hand.Status != domain.HandStatusActive {
		return domain.ErrHandNotActive
	}

	subHand, err := getSubHand(hand, subHandIdx)
	if err != nil {
		return err
	}

	if subHand.Status != domain.SubHandStatusActive {
		return domain.ErrSubHandNotActive
	}

	subHand.Status = domain.SubHandStatusStand
	recordAction(hand, subHandIdx, domain.ActionStand)

	if allSubHandsResolved(hand) {
		e.PlayDealerTurn(hand, deck)
	}

	return nil
}

func (e *blackjackEngine) Double(hand *domain.HandDocument, subHandIdx int32, deck *domain.Deck) error {
	if hand.Status != domain.HandStatusActive {
		return domain.ErrHandNotActive
	}

	subHand, err := getSubHand(hand, subHandIdx)
	if err != nil {
		return err
	}

	if subHand.Status != domain.SubHandStatusActive {
		return domain.ErrSubHandNotActive
	}

	if len(subHand.Cards) != 2 {
		return domain.ErrCannotDouble
	}

	subHand.BetAmountCents *= 2
	card := deck.Draw(false)
	subHand.Cards = append(subHand.Cards, card)

	val := domain.CalculateHandValue(subHand.Cards)
	if val > 21 {
		subHand.Status = domain.SubHandStatusBust
		subHand.Result = domain.ResultLose
	} else {
		subHand.Status = domain.SubHandStatusDoubled
	}

	recordAction(hand, subHandIdx, domain.ActionDouble)

	if allSubHandsResolved(hand) {
		e.PlayDealerTurn(hand, deck)
	}

	return nil
}

func (e *blackjackEngine) Split(hand *domain.HandDocument, subHandIdx int32, deck *domain.Deck, newHoldID string) error {
	if hand.Status != domain.HandStatusActive {
		return domain.ErrHandNotActive
	}

	subHand, err := getSubHand(hand, subHandIdx)
	if err != nil {
		return err
	}

	if subHand.Status != domain.SubHandStatusActive {
		return domain.ErrSubHandNotActive
	}

	if len(subHand.Cards) != 2 {
		return domain.ErrCannotSplit
	}

	c1, c2 := subHand.Cards[0], subHand.Cards[1]
	if c1.Value != c2.Value && c1.Rank != c2.Rank {
		return domain.ErrCannotSplit
	}

	// Separar par
	subHand.Cards = []domain.Card{c1, deck.Draw(false)}

	newSubHand := domain.SubHand{
		Index:          int32(len(hand.SubHands)),
		Cards:          []domain.Card{c2, deck.Draw(false)},
		BetAmountCents: subHand.BetAmountCents,
		HoldID:         newHoldID,
		Status:         domain.SubHandStatusActive,
		Result:         domain.ResultNone,
	}
	hand.SubHands = append(hand.SubHands, newSubHand)

	recordAction(hand, subHandIdx, domain.ActionSplit)
	return nil
}

func (e *blackjackEngine) Surrender(hand *domain.HandDocument, subHandIdx int32) error {
	if hand.Status != domain.HandStatusActive {
		return domain.ErrHandNotActive
	}

	if len(hand.PlayerActions) > 0 || len(hand.SubHands) != 1 {
		return domain.ErrCannotSurrender
	}

	subHand := &hand.SubHands[0]
	if len(subHand.Cards) != 2 {
		return domain.ErrCannotSurrender
	}

	subHand.Status = domain.SubHandStatusSettled
	subHand.Result = domain.ResultSurrender

	// Revelar carta del dealer
	hand.DealerCards[1].Hidden = false

	win := subHand.BetAmountCents / 2
	hand.TotalWinAmountCents = &win
	hand.Status = domain.HandStatusSettled
	now := time.Now()
	hand.SettledAt = &now

	recordAction(hand, subHandIdx, domain.ActionSurrender)
	return nil
}

func (e *blackjackEngine) PlayDealerTurn(hand *domain.HandDocument, deck *domain.Deck) {
	// Revelar carta oculta del dealer
	for i := range hand.DealerCards {
		hand.DealerCards[i].Hidden = false
	}

	// Comprobar si todas las submanos se pasaron (bust) o se rindieron
	allBustOrSurrender := true
	for _, sh := range hand.SubHands {
		if sh.Status != domain.SubHandStatusBust && sh.Result != domain.ResultSurrender {
			allBustOrSurrender = false
			break
		}
	}

	if allBustOrSurrender {
		e.EvaluateResults(hand)
		return
	}

	hand.Status = domain.HandStatusDealerTurn

	// El dealer pide cartas hasta alcanzar al menos 17 puntos
	for domain.CalculateHandValue(hand.DealerCards) < 17 {
		hand.DealerCards = append(hand.DealerCards, deck.Draw(false))
	}

	e.EvaluateResults(hand)
}

func (e *blackjackEngine) EvaluateResults(hand *domain.HandDocument) {
	dealerVal := domain.CalculateHandValue(hand.DealerCards)
	dealerBust := dealerVal > 21

	var totalWin int64 = 0

	for i := range hand.SubHands {
		sh := &hand.SubHands[i]

		switch {
		case sh.Status == domain.SubHandStatusBust:
			sh.Result = domain.ResultLose

		case sh.Result == domain.ResultSurrender:
			totalWin += sh.BetAmountCents / 2

		case sh.Status == domain.SubHandStatusBlackjack:
			if domain.IsBlackjack(hand.DealerCards) {
				sh.Result = domain.ResultPush
				totalWin += sh.BetAmountCents
			} else {
				sh.Result = domain.ResultBlackjackWin
				totalWin += sh.BetAmountCents * 5 / 2
			}

		case sh.Status == domain.SubHandStatusStand || sh.Status == domain.SubHandStatusDoubled:
			pVal := domain.CalculateHandValue(sh.Cards)

			if dealerBust {
				sh.Result = domain.ResultWin
				totalWin += sh.BetAmountCents * 2
			} else if pVal > dealerVal {
				sh.Result = domain.ResultWin
				totalWin += sh.BetAmountCents * 2
			} else if pVal == dealerVal {
				sh.Result = domain.ResultPush
				totalWin += sh.BetAmountCents
			} else {
				sh.Result = domain.ResultLose
			}
		}
	}

	hand.TotalWinAmountCents = &totalWin
	hand.Status = domain.HandStatusSettled
	now := time.Now()
	hand.SettledAt = &now
}

func getSubHand(hand *domain.HandDocument, idx int32) (*domain.SubHand, error) {
	for i := range hand.SubHands {
		if hand.SubHands[i].Index == idx {
			return &hand.SubHands[i], nil
		}
	}
	return nil, domain.ErrSubHandNotFound
}

func recordAction(hand *domain.HandDocument, idx int32, act domain.ActionType) {
	hand.PlayerActions = append(hand.PlayerActions, domain.PlayerAction{
		SubHandIndex: idx,
		Action:       act,
		Timestamp:    time.Now(),
	})
}

func allSubHandsResolved(hand *domain.HandDocument) bool {
	for _, sh := range hand.SubHands {
		if sh.Status == domain.SubHandStatusActive {
			return false
		}
	}
	return true
}

