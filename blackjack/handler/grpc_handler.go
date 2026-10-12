package handler

import (
	"context"
	"errors"
	"log"

	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/domain"
	pb "github.com/ChGSufro/sis-distribuidos-casino/blackjack/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BlackjackGRPCHandler struct {
	pb.UnimplementedBlackjackServiceServer
	service service.BlackjackService
}

func NewBlackjackGRPCHandler(svc service.BlackjackService) *BlackjackGRPCHandler {
	return &BlackjackGRPCHandler{service: svc}
}

func (h *BlackjackGRPCHandler) CreateHand(ctx context.Context, req *pb.CreateHandRequest) (*pb.CreateHandResponse, error) {
	log.Printf("[CreateHand] Solicitado para user_id: %s, apuesta: %d centavos", req.GetUserId(), req.GetBetAmountCents())
	hand, err := h.service.CreateHand(ctx, req.GetUserId(), req.GetBetAmountCents())
	if err != nil {
		return nil, toGRPCError(err)
	}

	var dealerVisible *pb.Card
	for _, c := range hand.DealerCards {
		if !c.Hidden {
			dealerVisible = toPBCard(c)
			break
		}
	}
	if dealerVisible == nil && len(hand.DealerCards) > 0 {
		dealerVisible = toPBCard(hand.DealerCards[0])
	}

	return &pb.CreateHandResponse{
		HandId:            hand.ID.Hex(),
		SubHands:          toPBSubHands(hand.SubHands),
		DealerVisibleCard: dealerVisible,
		Status:            string(hand.Status),
	}, nil
}

func (h *BlackjackGRPCHandler) PlayerAction(ctx context.Context, req *pb.PlayerActionRequest) (*pb.PlayerActionResponse, error) {
	log.Printf("[PlayerAction] Solicitado hand_id: %s, user_id: %s, subHand: %d, accion: %s",
		req.GetHandId(), req.GetUserId(), req.GetSubHandIndex(), req.GetAction())

	hand, err := h.service.PlayerAction(ctx, req.GetHandId(), req.GetUserId(), req.GetSubHandIndex(), req.GetAction())
	if err != nil {
		return nil, toGRPCError(err)
	}

	var winAmount int64 = 0
	if hand.TotalWinAmountCents != nil {
		winAmount = *hand.TotalWinAmountCents
	}

	return &pb.PlayerActionResponse{
		HandId:              hand.ID.Hex(),
		SubHands:            toPBSubHands(hand.SubHands),
		DealerCards:         toPBDealerCards(hand),
		Status:              string(hand.Status),
		TotalWinAmountCents: winAmount,
	}, nil
}

func (h *BlackjackGRPCHandler) GetHandState(ctx context.Context, req *pb.GetHandStateRequest) (*pb.GetHandStateResponse, error) {
	log.Printf("[GetHandState] Solicitado hand_id: %s, user_id: %s", req.GetHandId(), req.GetUserId())

	hand, err := h.service.GetHandState(ctx, req.GetHandId(), req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	var winAmount int64 = 0
	if hand.TotalWinAmountCents != nil {
		winAmount = *hand.TotalWinAmountCents
	}

	return &pb.GetHandStateResponse{
		HandId:              hand.ID.Hex(),
		SubHands:            toPBSubHands(hand.SubHands),
		DealerCards:         toPBDealerCards(hand),
		Status:              string(hand.Status),
		TotalWinAmountCents: winAmount,
	}, nil
}

func toPBCard(c domain.Card) *pb.Card {
	return &pb.Card{
		Suit:   string(c.Suit),
		Rank:   string(c.Rank),
		Value:  int32(c.Value),
		Hidden: c.Hidden,
	}
}

func toPBSubHands(subHands []domain.SubHand) []*pb.SubHand {
	res := make([]*pb.SubHand, len(subHands))
	for i, sh := range subHands {
		cards := make([]*pb.Card, len(sh.Cards))
		for j, c := range sh.Cards {
			cards[j] = toPBCard(c)
		}
		res[i] = &pb.SubHand{
			Index:          sh.Index,
			Cards:          cards,
			BetAmountCents: sh.BetAmountCents,
			Status:         string(sh.Status),
			Result:         string(sh.Result),
		}
	}
	return res
}

func toPBDealerCards(hand *domain.HandDocument) []*pb.Card {
	res := make([]*pb.Card, len(hand.DealerCards))
	for i, c := range hand.DealerCards {
		// Si la mano sigue activa, la carta oculta permanece protegida
		if hand.Status == domain.HandStatusActive && c.Hidden {
			res[i] = &pb.Card{
				Suit:   "",
				Rank:   "",
				Value:  0,
				Hidden: true,
			}
		} else {
			res[i] = toPBCard(c)
		}
	}
	return res
}

func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	// Propagación de errores gRPC directos desde Wallet
	if s, ok := status.FromError(err); ok {
		return s.Err()
	}

	switch {
	case errors.Is(err, domain.ErrInvalidUserID),
		errors.Is(err, domain.ErrInvalidBetAmount),
		errors.Is(err, domain.ErrInvalidAction),
		errors.Is(err, domain.ErrCannotDouble),
		errors.Is(err, domain.ErrCannotSplit),
		errors.Is(err, domain.ErrCannotSurrender):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, domain.ErrHandNotFound),
		errors.Is(err, domain.ErrSubHandNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, domain.ErrHandNotActive),
		errors.Is(err, domain.ErrSubHandNotActive):
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, domain.ErrUnauthorizedHand):
		return status.Error(codes.PermissionDenied, err.Error())

	default:
		log.Printf("[ERROR] Interno no mapeado en Blackjack: %v", err)
		return status.Errorf(codes.Internal, "error interno en Blackjack: %v", err)
	}
}

