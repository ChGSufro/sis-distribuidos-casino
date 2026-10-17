package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/middleware"
	pb_wallet "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/wallet"
)

type WalletHandler struct {
	walletClient pb_wallet.WalletServiceClient
}

func NewWalletHandler(walletClient pb_wallet.WalletServiceClient) *WalletHandler {
	return &WalletHandler{walletClient: walletClient}
}

func (h *WalletHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	res, err := h.walletClient.GetBalance(r.Context(), &pb_wallet.GetBalanceRequest{
		UserId: userID,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"user_id":       res.GetUserId(),
		"balance_cents": res.GetBalanceCents(),
		"updated_at":    res.GetUpdatedAt(),
	})
}

type DepositRequestBody struct {
	AmountCents int64 `json:"amount_cents"`
}

func (h *WalletHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	var body DepositRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "cuerpo de solicitud JSON inválido")
		return
	}
	if body.AmountCents <= 0 {
		WriteError(w, http.StatusBadRequest, "amount_cents debe ser mayor a 0")
		return
	}

	res, err := h.walletClient.Deposit(r.Context(), &pb_wallet.DepositRequest{
		UserId:      userID,
		AmountCents: body.AmountCents,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"transaction_id":    res.GetTransactionId(),
		"new_balance_cents": res.GetNewBalanceCents(),
	})
}
