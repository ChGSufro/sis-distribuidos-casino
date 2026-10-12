package handler

import (
	"context"
	"errors"

	pb "github.com/ChGSufro/sis-distribuidos-casino/users/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/users/internal/service"
	"github.com/ChGSufro/sis-distribuidos-casino/users/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UsersHandler implementa la capa de transporte gRPC y delega la logica
// de negocio en el paquete service.
type UsersHandler struct {
	pb.UnimplementedUsersServiceServer
	svc *service.AuthService
}

// New construye un handler gRPC de usuarios.
func New(svc *service.AuthService) *UsersHandler {
	return &UsersHandler{svc: svc}
}

func (h *UsersHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	res, err := h.svc.Register(ctx, req)
	if err != nil {
		return nil, toStatus(err)
	}
	return res, nil
}

func (h *UsersHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	res, err := h.svc.Login(ctx, req)
	if err != nil {
		return nil, toStatus(err)
	}
	return res, nil
}

func (h *UsersHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	res, err := h.svc.ValidateToken(ctx, req)
	if err != nil {
		return nil, toStatus(err)
	}
	return res, nil
}

func (h *UsersHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	res, err := h.svc.RefreshToken(ctx, req)
	if err != nil {
		return nil, toStatus(err)
	}
	return res, nil
}

func (h *UsersHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	res, err := h.svc.Logout(ctx, req)
	if err != nil {
		return nil, toStatus(err)
	}
	return res, nil
}

func (h *UsersHandler) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.GetProfileResponse, error) {
	res, err := h.svc.GetProfile(ctx, req)
	if err != nil {
		return nil, toStatus(err)
	}
	return res, nil
}

// toStatus traduce los errores de negocio a codigos gRPC.
func toStatus(err error) error {
	switch {
	case errors.Is(err, service.ErrValidation):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, store.ErrEmailTaken), errors.Is(err, store.ErrUsernameTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, service.ErrInvalidCredentials), errors.Is(err, service.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, service.ErrAccountInactive):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, service.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, "error interno")
	}
}
