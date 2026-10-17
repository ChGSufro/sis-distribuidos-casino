package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/middleware"
	pb_users "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/users"
	pb_wallet "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/wallet"
)

type AuthHandler struct {
	usersClient  pb_users.UsersServiceClient
	walletClient pb_wallet.WalletServiceClient
}

func NewAuthHandler(usersClient pb_users.UsersServiceClient, walletClient pb_wallet.WalletServiceClient) *AuthHandler {
	return &AuthHandler{
		usersClient:  usersClient,
		walletClient: walletClient,
	}
}

type RegisterRequestBody struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body RegisterRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "cuerpo de solicitud JSON inválido")
		return
	}

	res, err := h.usersClient.Register(r.Context(), &pb_users.RegisterRequest{
		Username: body.Username,
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	// Inicializar la billetera para el nuevo usuario de forma complementaria
	if h.walletClient != nil {
		_, err := h.walletClient.InitWallet(r.Context(), &pb_wallet.InitWalletRequest{
			UserId: res.GetUserId(),
		})
		if err != nil {
			log.Printf("[ADVERTENCIA] Fallo al inicializar wallet para usuario %s: %v", res.GetUserId(), err)
		}
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"user_id":    res.GetUserId(),
		"username":   res.GetUsername(),
		"email":      res.GetEmail(),
		"created_at": res.GetCreatedAt(),
	})
}

type LoginRequestBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body LoginRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "cuerpo de solicitud JSON inválido")
		return
	}

	res, err := h.usersClient.Login(r.Context(), &pb_users.LoginRequest{
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  res.GetAccessToken(),
		"refresh_token": res.GetRefreshToken(),
		"expires_at":    res.GetExpiresAt(),
		"user_id":       res.GetUserId(),
		"username":      res.GetUsername(),
		"status":        res.GetStatus(),
	})
}

type RefreshRequestBody struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var body RefreshRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "cuerpo de solicitud JSON inválido")
		return
	}

	res, err := h.usersClient.RefreshToken(r.Context(), &pb_users.RefreshTokenRequest{
		RefreshToken: body.RefreshToken,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  res.GetAccessToken(),
		"refresh_token": res.GetRefreshToken(),
		"expires_at":    res.GetExpiresAt(),
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFromContext(r.Context())
	if token == "" {
		WriteError(w, http.StatusBadRequest, "token no encontrado en contexto")
		return
	}

	res, err := h.usersClient.Logout(r.Context(), &pb_users.LogoutRequest{
		AccessToken: token,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"success": res.GetSuccess(),
	})
}

func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if pathID := r.PathValue("id"); pathID != "" {
		userID = pathID
	}
	if userID == "" {
		WriteError(w, http.StatusBadRequest, "user_id no especificado")
		return
	}

	res, err := h.usersClient.GetProfile(r.Context(), &pb_users.GetProfileRequest{
		UserId: userID,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"user_id":       res.GetUserId(),
		"username":      res.GetUsername(),
		"email":         res.GetEmail(),
		"status":        res.GetStatus(),
		"created_at":    res.GetCreatedAt(),
		"last_login_at": res.GetLastLoginAt(),
	})
}
