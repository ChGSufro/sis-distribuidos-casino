package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/clients"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/domain"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/engine"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/repository"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type BlackjackService interface {
	CreateHand(ctx context.Context, userID string, betAmountCents int64) (*domain.HandDocument, error)
	PlayerAction(ctx context.Context, handID, userID string, subHandIndex int32, actionStr string) (*domain.HandDocument, error)
	GetHandState(ctx context.Context, handID, userID string) (*domain.HandDocument, error)
}

type blackjackService struct {
	repo          repository.HandRepository
	engine        engine.Engine
	walletClient  clients.WalletClient
	historyClient clients.HistoryClient
	deck          *domain.Deck
}

func NewBlackjackService(
	repo repository.HandRepository,
	eng engine.Engine,
	wallet clients.WalletClient,
	history clients.HistoryClient,
) BlackjackService {
	return &blackjackService{
		repo:          repo,
		engine:        eng,
		walletClient:  wallet,
		historyClient: history,
		deck:          domain.NewStandardDeck(6),
	}
}

func (s *blackjackService) CreateHand(ctx context.Context, userID string, betAmountCents int64) (*domain.HandDocument, error) {
	if _, err := uuid.Parse(strings.TrimSpace(userID)); err != nil {
		return nil, domain.ErrInvalidUserID
	}
	if betAmountCents <= 0 {
		return nil, domain.ErrInvalidBetAmount
	}

	idempKey := fmt.Sprintf("bj-%s", uuid.New().String())
	tempRefID := bson.NewObjectID().Hex()

	// 1. Reservar fondos en WalletService
	holdID, err := s.walletClient.Hold(ctx, userID, betAmountCents, tempRefID, idempKey)
	if err != nil {
		return nil, fmt.Errorf("wallet hold: %w", err)
	}

	// 2. Crear mano inicial con el motor
	hand := s.engine.NewGame(s.deck, userID, betAmountCents, holdID, idempKey)

	// 3. Persistir en MongoDB
	savedHand, err := s.repo.CreateHand(ctx, hand)
	if err != nil {
		// Compensación de reserva si falla la base de datos
		_ = s.walletClient.Release(ctx, holdID)
		return nil, fmt.Errorf("guardar mano: %w", err)
	}

	// 4. Si ocurrió Blackjack natural de entrada, liquidar y auditar
	if savedHand.Status == domain.HandStatusSettled {
		s.settleHandFinancesAndAudit(ctx, savedHand)
		_ = s.repo.UpdateHand(ctx, savedHand)
	}

	return savedHand, nil
}

func (s *blackjackService) PlayerAction(ctx context.Context, handID, userID string, subHandIndex int32, actionStr string) (*domain.HandDocument, error) {
	if strings.TrimSpace(handID) == "" {
		return nil, domain.ErrHandNotFound
	}
	if _, err := uuid.Parse(strings.TrimSpace(userID)); err != nil {
		return nil, domain.ErrInvalidUserID
	}

	hand, err := s.repo.GetHandByID(ctx, handID)
	if err != nil {
		return nil, err
	}

	if hand.UserID != userID {
		return nil, domain.ErrUnauthorizedHand
	}
	if hand.Status != domain.HandStatusActive {
		return nil, domain.ErrHandNotActive
	}

	act := domain.ActionType(strings.ToLower(strings.TrimSpace(actionStr)))

	switch act {
	case domain.ActionHit:
		if err := s.engine.Hit(hand, subHandIndex, s.deck); err != nil {
			return nil, err
		}

	case domain.ActionStand:
		if err := s.engine.Stand(hand, subHandIndex, s.deck); err != nil {
			return nil, err
		}

	case domain.ActionDouble:
		// Se necesita reservar el monto adicional
		var subHandBet int64
		for _, sh := range hand.SubHands {
			if sh.Index == subHandIndex {
				subHandBet = sh.BetAmountCents
				break
			}
		}
		if subHandBet <= 0 {
			return nil, domain.ErrSubHandNotFound
		}

		doubleIdempKey := fmt.Sprintf("bj-dbl-%s", uuid.New().String())
		doubleHoldID, errHold := s.walletClient.Hold(ctx, userID, subHandBet, handID, doubleIdempKey)
		if errHold != nil {
			return nil, fmt.Errorf("wallet hold adicional para double: %w", errHold)
		}

		if err := s.engine.Double(hand, subHandIndex, s.deck); err != nil {
			return nil, err
		}

		for i := range hand.SubHands {
			if hand.SubHands[i].Index == subHandIndex {
				hand.SubHands[i].DoubleHoldID = doubleHoldID
				break
			}
		}

	case domain.ActionSplit:
		var subHandBet int64
		for _, sh := range hand.SubHands {
			if sh.Index == subHandIndex {
				subHandBet = sh.BetAmountCents
				break
			}
		}
		if subHandBet <= 0 {
			return nil, domain.ErrSubHandNotFound
		}

		splitIdempKey := fmt.Sprintf("bj-splt-%s", uuid.New().String())
		splitHoldID, errHold := s.walletClient.Hold(ctx, userID, subHandBet, handID, splitIdempKey)
		if errHold != nil {
			return nil, fmt.Errorf("wallet hold para split: %w", errHold)
		}

		if err := s.engine.Split(hand, subHandIndex, s.deck, splitHoldID); err != nil {
			return nil, err
		}

	case domain.ActionSurrender:
		if err := s.engine.Surrender(hand, subHandIndex); err != nil {
			return nil, err
		}

	default:
		return nil, domain.ErrInvalidAction
	}

	// Si la partida concluyó, liquidar holds en Wallet y registrar en History
	if hand.Status == domain.HandStatusSettled {
		s.settleHandFinancesAndAudit(ctx, hand)
	}

	if err := s.repo.UpdateHand(ctx, hand); err != nil {
		return nil, fmt.Errorf("actualizar mano: %w", err)
	}

	return hand, nil
}

func (s *blackjackService) GetHandState(ctx context.Context, handID, userID string) (*domain.HandDocument, error) {
	hand, err := s.repo.GetHandByID(ctx, handID)
	if err != nil {
		return nil, err
	}
	if hand.UserID != userID {
		return nil, domain.ErrUnauthorizedHand
	}
	return hand, nil
}

func (s *blackjackService) settleHandFinancesAndAudit(ctx context.Context, hand *domain.HandDocument) {
	var totalBetCents int64
	var totalWinCents int64

	for i := range hand.SubHands {
		sh := &hand.SubHands[i]
		var winAmount int64 = 0

		switch sh.Result {
		case domain.ResultWin:
			winAmount = sh.BetAmountCents * 2
		case domain.ResultBlackjackWin:
			winAmount = sh.BetAmountCents * 5 / 2
		case domain.ResultPush:
			winAmount = sh.BetAmountCents
		case domain.ResultSurrender:
			winAmount = sh.BetAmountCents / 2
		case domain.ResultLose:
			winAmount = 0
		}

		totalBetCents += sh.BetAmountCents
		totalWinCents += winAmount

		if sh.DoubleHoldID != "" {
			halfWin := winAmount / 2
			otherHalf := winAmount - halfWin

			if err := s.walletClient.Settle(ctx, sh.HoldID, halfWin); err != nil {
				log.Printf("[ALERTA] Fallo al liquidar HoldID %s en Wallet: %v", sh.HoldID, err)
			}
			if err := s.walletClient.Settle(ctx, sh.DoubleHoldID, otherHalf); err != nil {
				log.Printf("[ALERTA] Fallo al liquidar DoubleHoldID %s en Wallet: %v", sh.DoubleHoldID, err)
			}
		} else {
			if err := s.walletClient.Settle(ctx, sh.HoldID, winAmount); err != nil {
				log.Printf("[ALERTA] Fallo al liquidar HoldID %s en Wallet: %v", sh.HoldID, err)
			}
		}
	}

	var consolidatedResult string
	if len(hand.SubHands) == 1 {
		consolidatedResult = string(hand.SubHands[0].Result)
	} else {
		if totalWinCents > totalBetCents {
			consolidatedResult = string(domain.ResultWin)
		} else if totalWinCents < totalBetCents {
			consolidatedResult = string(domain.ResultLose)
		} else {
			consolidatedResult = string(domain.ResultPush)
		}
	}

	detailsMap := map[string]interface{}{
		"betAmountCents": totalBetCents,
		"winAmountCents": totalWinCents,
		"result":         consolidatedResult,
		"subHands":       hand.SubHands,
		"dealerCards":    hand.DealerCards,
		"totalWin":       totalWinCents,
		"status":         hand.Status,
	}
	detailsBytes, _ := json.Marshal(detailsMap)
	eventID := uuid.New().String()

	_ = s.historyClient.RecordGameEvent(
		ctx,
		eventID,
		hand.UserID,
		hand.ID.Hex(),
		"hand_result",
		string(detailsBytes),
	)
}

