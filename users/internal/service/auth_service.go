package service

import (
	"context"
	"errors"
	"strings"
	"time"

	pb "github.com/ChGSufro/sis-distribuidos-casino/users/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/users/internal/auth"
	"github.com/ChGSufro/sis-distribuidos-casino/users/internal/store"
	"github.com/google/uuid"
)

// Errores de negocio del servicio de autenticacion.
var (
	ErrValidation         = errors.New("datos invalidos")
	ErrInvalidCredentials = errors.New("credenciales invalidas")
	ErrAccountInactive    = errors.New("cuenta no activa")
	ErrInvalidToken       = errors.New("token invalido o expirado")
	ErrUserNotFound       = errors.New("usuario no encontrado")
)

// AuthService concentra la logica de negocio de registro y sesiones.
type AuthService struct {
	store  *store.Store
	tokens *auth.TokenManager
}

// New construye el servicio de autenticacion.
func New(st *store.Store, tokens *auth.TokenManager) *AuthService {
	return &AuthService{store: st, tokens: tokens}
}

// Register crea un usuario con la contrasena hasheada.
func (s *AuthService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	email := strings.ToLower(strings.TrimSpace(req.GetEmail()))
	password := req.GetPassword()

	if username == "" || email == "" || password == "" {
		return nil, ErrValidation
	}
	if !strings.Contains(email, "@") || len(password) < 8 {
		return nil, ErrValidation
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}

	u := &store.User{
		ID:           uuid.NewString(),
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Status:       "active",
	}
	if err := s.store.Users.Create(ctx, u); err != nil {
		return nil, err
	}

	return &pb.RegisterResponse{
		UserId:    u.ID,
		Username:  u.Username,
		Email:     u.Email,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}, nil
}

// Login valida credenciales, abre una sesion y emite los tokens.
func (s *AuthService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.GetEmail()))

	u, err := s.store.Users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !auth.CheckPassword(u.PasswordHash, req.GetPassword()) {
		return nil, ErrInvalidCredentials
	}
	if u.Status != "active" {
		return nil, ErrAccountInactive
	}

	access, accessExp, err := s.tokens.GenerateAccess(u.ID, u.Username)
	if err != nil {
		return nil, err
	}
	refresh, refreshExp, err := s.tokens.GenerateRefresh(u.ID)
	if err != nil {
		return nil, err
	}

	sess := &store.Session{
		ID:               uuid.NewString(),
		UserID:           u.ID,
		TokenHash:        auth.HashToken(access),
		RefreshTokenHash: ptr(auth.HashToken(refresh)),
		ExpiresAt:        refreshExp,
	}
	if err := s.store.Sessions.Create(ctx, sess); err != nil {
		return nil, err
	}
	if err := s.store.Users.UpdateLastLogin(ctx, u.ID); err != nil {
		return nil, err
	}

	return &pb.LoginResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    accessExp.Format(time.RFC3339),
		UserId:       u.ID,
		Username:     u.Username,
		Status:       u.Status,
	}, nil
}

// ValidateToken verifica firma, expiracion y revocacion de un access token.
// No devuelve error para un token invalido: retorna Valid=false.
func (s *AuthService) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	raw := req.GetAccessToken()
	claims, err := s.tokens.ParseAccess(raw)
	if err != nil {
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}

	sess, err := s.store.Sessions.GetByTokenHash(ctx, auth.HashToken(raw))
	if err != nil || sess.RevokedAt != nil || time.Now().After(sess.ExpiresAt) {
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}

	return &pb.ValidateTokenResponse{
		Valid:    true,
		UserId:   claims.Subject,
		Username: claims.Username,
	}, nil
}

// RefreshToken rota el par access/refresh de una sesion valida.
func (s *AuthService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	raw := req.GetRefreshToken()
	userID, err := s.tokens.ParseRefresh(raw)
	if err != nil {
		return nil, ErrInvalidToken
	}

	sess, err := s.store.Sessions.GetByRefreshHash(ctx, auth.HashToken(raw))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	if sess.RevokedAt != nil || time.Now().After(sess.ExpiresAt) || sess.UserID != userID {
		return nil, ErrInvalidToken
	}

	u, err := s.store.Users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	access, accessExp, err := s.tokens.GenerateAccess(u.ID, u.Username)
	if err != nil {
		return nil, err
	}
	refresh, refreshExp, err := s.tokens.GenerateRefresh(u.ID)
	if err != nil {
		return nil, err
	}
	if err := s.store.Sessions.Rotate(ctx, sess.ID, auth.HashToken(access), auth.HashToken(refresh), refreshExp); err != nil {
		return nil, err
	}

	return &pb.RefreshTokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    accessExp.Format(time.RFC3339),
	}, nil
}

// Logout revoca la sesion asociada al access token (idempotente).
func (s *AuthService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	raw := req.GetAccessToken()
	if raw == "" {
		return nil, ErrValidation
	}

	sess, err := s.store.Sessions.GetByTokenHash(ctx, auth.HashToken(raw))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return &pb.LogoutResponse{Success: false}, nil
		}
		return nil, err
	}
	if err := s.store.Sessions.Revoke(ctx, sess.ID); err != nil {
		return nil, err
	}
	return &pb.LogoutResponse{Success: true}, nil
}

// GetProfile devuelve el perfil publico de un usuario.
func (s *AuthService) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.GetProfileResponse, error) {
	id := strings.TrimSpace(req.GetUserId())
	if id == "" {
		return nil, ErrValidation
	}

	u, err := s.store.Users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	lastLogin := ""
	if u.LastLoginAt != nil {
		lastLogin = u.LastLoginAt.Format(time.RFC3339)
	}
	return &pb.GetProfileResponse{
		UserId:      u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Status:      u.Status,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
		LastLoginAt: lastLogin,
	}, nil
}

func ptr(s string) *string { return &s }
