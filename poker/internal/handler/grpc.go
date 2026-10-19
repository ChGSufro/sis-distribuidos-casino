package handler

import (
	"context"
	"errors"

	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/service"
	pb "github.com/ChGSufro/sis-distribuidos-casino/poker/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PokerHandler implementa la capa de transporte gRPC y delega la logica de
// negocio en el paquete service.
type PokerHandler struct {
	pb.UnimplementedPokerServiceServer
	svc *service.PokerService
}

// New construye un handler gRPC de poker.
func New(svc *service.PokerService) *PokerHandler {
	return &PokerHandler{svc: svc}
}

func (h *PokerHandler) ListTables(ctx context.Context, req *pb.ListTablesRequest) (*pb.ListTablesResponse, error) {
	res, err := h.svc.ListTables(ctx, req)
	return res, toStatus(err)
}

func (h *PokerHandler) CreateTable(ctx context.Context, req *pb.CreateTableRequest) (*pb.CreateTableResponse, error) {
	res, err := h.svc.CreateTable(ctx, req)
	return res, toStatus(err)
}

func (h *PokerHandler) JoinTable(ctx context.Context, req *pb.JoinTableRequest) (*pb.JoinTableResponse, error) {
	res, err := h.svc.JoinTable(ctx, req)
	return res, toStatus(err)
}

func (h *PokerHandler) LeaveTable(ctx context.Context, req *pb.LeaveTableRequest) (*pb.LeaveTableResponse, error) {
	res, err := h.svc.LeaveTable(ctx, req)
	return res, toStatus(err)
}

func (h *PokerHandler) PlayerAction(ctx context.Context, req *pb.PlayerActionRequest) (*pb.PlayerActionResponse, error) {
	res, err := h.svc.PlayerAction(ctx, req)
	return res, toStatus(err)
}

func (h *PokerHandler) GetTableState(ctx context.Context, req *pb.GetTableStateRequest) (*pb.GetTableStateResponse, error) {
	res, err := h.svc.GetTableState(ctx, req)
	return res, toStatus(err)
}

func (h *PokerHandler) CloseTable(ctx context.Context, req *pb.CloseTableRequest) (*pb.CloseTableResponse, error) {
	res, err := h.svc.CloseTable(ctx, req)
	return res, toStatus(err)
}

// toStatus traduce los errores de negocio a codigos gRPC.
func toStatus(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, service.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrTableNotFound), errors.Is(err, service.ErrRoundNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, service.ErrTableFull),
		errors.Is(err, service.ErrAlreadySeated),
		errors.Is(err, service.ErrSeatTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, service.ErrNotSeated):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, service.ErrNotYourTurn),
		errors.Is(err, service.ErrInvalidAction),
		errors.Is(err, service.ErrNoActiveRound),
		errors.Is(err, service.ErrTableHasPlayers):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, service.ErrNotCreator):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, "error interno")
	}
}
