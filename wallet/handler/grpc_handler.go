package handler

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/wallet/domain"
	pb "github.com/ChGSufro/sis-distribuidos-casino/wallet/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/wallet/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WalletGRPCHandler struct {
	pb.UnimplementedWalletServiceServer
	service service.WalletService
}

func NewWalletGRPCHandler(svc service.WalletService) *WalletGRPCHandler {
	return &WalletGRPCHandler{service: svc}
}

func (h *WalletGRPCHandler) InitWallet(ctx context.Context, req *pb.InitWalletRequest) (*pb.InitWalletResponse, error) {
	log.Printf("[InitWallet] Solicitado para user_id: %s", req.GetUserId())
	wallet, err := h.service.InitWallet(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.InitWalletResponse{
		WalletId:     wallet.ID.String(),
		BalanceCents: wallet.BalanceCents,
	}, nil
}

func (h *WalletGRPCHandler) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	log.Printf("[GetBalance] Solicitado para user_id: %s", req.GetUserId())
	wallet, err := h.service.GetBalance(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.GetBalanceResponse{
		UserId:       wallet.UserID.String(),
		BalanceCents: wallet.BalanceCents,
		UpdatedAt:    wallet.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (h *WalletGRPCHandler) Deposit(ctx context.Context, req *pb.DepositRequest) (*pb.DepositResponse, error) {
	log.Printf("[Deposit] Solicitado para user_id: %s, monto: %d centavos", req.GetUserId(), req.GetAmountCents())
	tx, newBal, err := h.service.Deposit(ctx, req.GetUserId(), req.GetAmountCents())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.DepositResponse{
		TransactionId:    tx.ID.String(),
		NewBalanceCents: newBal,
	}, nil
}

func (h *WalletGRPCHandler) Hold(ctx context.Context, req *pb.HoldRequest) (*pb.HoldResponse, error) {
	log.Printf("[Hold] Solicitado para user_id: %s, monto: %d centavos, juego: %s, gameRef: %s, idempKey: %s",
		req.GetUserId(), req.GetAmountCents(), req.GetGameType(), req.GetGameRefId(), req.GetIdempotencyKey())

	hold, err := h.service.Hold(ctx, req.GetUserId(), req.GetAmountCents(), req.GetGameType(), req.GetGameRefId(), req.GetIdempotencyKey())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.HoldResponse{
		HoldId:      hold.ID.String(),
		AmountCents: hold.AmountCents,
		Status:      string(hold.Status),
	}, nil
}

func (h *WalletGRPCHandler) Settle(ctx context.Context, req *pb.SettleRequest) (*pb.SettleResponse, error) {
	log.Printf("[Settle] Solicitado para hold_id: %s, ganancia: %d centavos", req.GetHoldId(), req.GetWinAmountCents())
	hold, tx, newBal, err := h.service.Settle(ctx, req.GetHoldId(), req.GetWinAmountCents())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.SettleResponse{
		HoldId:           hold.ID.String(),
		Status:           string(hold.Status),
		TransactionId:    tx.ID.String(),
		NewBalanceCents: newBal,
	}, nil
}

func (h *WalletGRPCHandler) Release(ctx context.Context, req *pb.ReleaseRequest) (*pb.ReleaseResponse, error) {
	log.Printf("[Release] Solicitado para hold_id: %s", req.GetHoldId())
	hold, tx, newBal, err := h.service.Release(ctx, req.GetHoldId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.ReleaseResponse{
		HoldId:           hold.ID.String(),
		Status:           string(hold.Status),
		TransactionId:    tx.ID.String(),
		NewBalanceCents: newBal,
	}, nil
}

func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrInvalidUUID),
		errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrInvalidWinAmount),
		errors.Is(err, domain.ErrEmptyGameRef),
		errors.Is(err, domain.ErrEmptyIdempotencyKey):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, domain.ErrWalletNotFound),
		errors.Is(err, domain.ErrHoldNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, domain.ErrInsufficientFunds),
		errors.Is(err, domain.ErrHoldAlreadyResolved):
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, domain.ErrWalletAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())

	default:
		log.Printf("[ERROR] Interno no mapeado: %v", err)
		return status.Errorf(codes.Internal, "error interno del servicio: %v", err)
	}
}

