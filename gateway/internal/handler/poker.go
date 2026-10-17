package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/middleware"
	pb_poker "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/poker"
)

type PokerHandler struct {
	pokerClient pb_poker.PokerServiceClient
}

func NewPokerHandler(pokerClient pb_poker.PokerServiceClient) *PokerHandler {
	return &PokerHandler{pokerClient: pokerClient}
}

func (h *PokerHandler) ListTables(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")

	res, err := h.pokerClient.ListTables(r.Context(), &pb_poker.ListTablesRequest{
		StatusFilter: statusFilter,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}

type CreateTableRequestBody struct {
	Name            string `json:"name"`
	SmallBlindCents int64  `json:"small_blind_cents"`
	BigBlindCents   int64  `json:"big_blind_cents"`
	MaxPlayers      int32  `json:"max_players"`
}

func (h *PokerHandler) CreateTable(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	var body CreateTableRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "cuerpo de solicitud JSON inválido")
		return
	}
	if body.Name == "" {
		WriteError(w, http.StatusBadRequest, "name es requerido")
		return
	}
	if body.MaxPlayers < 2 || body.MaxPlayers > 10 {
		body.MaxPlayers = 6
	}

	res, err := h.pokerClient.CreateTable(r.Context(), &pb_poker.CreateTableRequest{
		UserId:          userID,
		Name:            body.Name,
		SmallBlindCents: body.SmallBlindCents,
		BigBlindCents:   body.BigBlindCents,
		MaxPlayers:      body.MaxPlayers,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, res)
}

type JoinTableRequestBody struct {
	BuyInAmountCents int64 `json:"buy_in_amount_cents"`
	PreferredSeat    int32 `json:"preferred_seat"`
}

func (h *PokerHandler) JoinTable(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	tableID := r.PathValue("id")
	if tableID == "" {
		WriteError(w, http.StatusBadRequest, "id de mesa requerido en la ruta")
		return
	}

	var body JoinTableRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "cuerpo de solicitud JSON inválido")
		return
	}
	if body.BuyInAmountCents <= 0 {
		WriteError(w, http.StatusBadRequest, "buy_in_amount_cents debe ser mayor a 0")
		return
	}

	res, err := h.pokerClient.JoinTable(r.Context(), &pb_poker.JoinTableRequest{
		TableId:          tableID,
		UserId:           userID,
		BuyInAmountCents: body.BuyInAmountCents,
		PreferredSeat:    body.PreferredSeat,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}

func (h *PokerHandler) LeaveTable(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	tableID := r.PathValue("id")
	if tableID == "" {
		WriteError(w, http.StatusBadRequest, "id de mesa requerido en la ruta")
		return
	}

	res, err := h.pokerClient.LeaveTable(r.Context(), &pb_poker.LeaveTableRequest{
		TableId: tableID,
		UserId:  userID,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}

type PokerActionRequestBody struct {
	RoundID     string `json:"round_id"`
	Action      string `json:"action"` // "fold", "check", "call", "raise", "all-in"
	AmountCents int64  `json:"amount_cents"`
}

func (h *PokerHandler) PlayerAction(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	tableID := r.PathValue("id")
	if tableID == "" {
		WriteError(w, http.StatusBadRequest, "id de mesa requerido en la ruta")
		return
	}

	var body PokerActionRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "cuerpo de solicitud JSON inválido")
		return
	}

	res, err := h.pokerClient.PlayerAction(r.Context(), &pb_poker.PlayerActionRequest{
		TableId:     tableID,
		RoundId:     body.RoundID,
		UserId:      userID,
		Action:      body.Action,
		AmountCents: body.AmountCents,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}

func (h *PokerHandler) GetTableState(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	tableID := r.PathValue("id")
	if tableID == "" {
		WriteError(w, http.StatusBadRequest, "id de mesa requerido en la ruta")
		return
	}

	res, err := h.pokerClient.GetTableState(r.Context(), &pb_poker.GetTableStateRequest{
		TableId: tableID,
		UserId:  userID,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}
