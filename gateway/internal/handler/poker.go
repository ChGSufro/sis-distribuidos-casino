package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/middleware"
	pb_poker "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/poker"
	pb_users "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/users"
)

// usernameTTL es cuanto se cachea el username resuelto via UsersService.
const usernameTTL = 5 * time.Minute

type usernameEntry struct {
	name      string
	expiresAt time.Time
}

type PokerHandler struct {
	pokerClient pb_poker.PokerServiceClient
	usersClient pb_users.UsersServiceClient

	mu    sync.Mutex
	cache map[string]usernameEntry
}

func NewPokerHandler(pokerClient pb_poker.PokerServiceClient, usersClient pb_users.UsersServiceClient) *PokerHandler {
	return &PokerHandler{
		pokerClient: pokerClient,
		usersClient: usersClient,
		cache:       make(map[string]usernameEntry),
	}
}

// username resuelve el nombre de un usuario con cache TTL. Ante cualquier
// error del servicio de usuarios devuelve cadena vacia (el frontend cae
// al user_id como fallback).
func (h *PokerHandler) username(ctx context.Context, userID string) string {
	if userID == "" {
		return ""
	}

	h.mu.Lock()
	if e, ok := h.cache[userID]; ok && time.Now().Before(e.expiresAt) {
		h.mu.Unlock()
		return e.name
	}
	h.mu.Unlock()

	if h.usersClient == nil {
		return ""
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	res, err := h.usersClient.GetProfile(reqCtx, &pb_users.GetProfileRequest{UserId: userID})
	if err != nil {
		return ""
	}

	name := res.GetUsername()
	h.mu.Lock()
	h.cache[userID] = usernameEntry{name: name, expiresAt: time.Now().Add(usernameTTL)}
	h.mu.Unlock()
	return name
}

func (h *PokerHandler) enrichSeats(ctx context.Context, seats []*pb_poker.Seat) {
	for _, s := range seats {
		if s.GetUserId() != "" && s.GetUsername() == "" {
			s.Username = h.username(ctx, s.GetUserId())
		}
	}
}

func (h *PokerHandler) enrichPlayers(ctx context.Context, players []*pb_poker.PlayerState) {
	for _, p := range players {
		if p.GetUserId() != "" && p.GetUsername() == "" {
			p.Username = h.username(ctx, p.GetUserId())
		}
	}
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
	Name             string `json:"name"`
	SmallBlindCents  int64  `json:"small_blind_cents"`
	BigBlindCents    int64  `json:"big_blind_cents"`
	MaxPlayers       int32  `json:"max_players"`
	BuyInAmountCents int64  `json:"buy_in_amount_cents"`
	MinBuyInCents    int64  `json:"min_buy_in_cents"`
	MaxBuyInCents    int64  `json:"max_buy_in_cents"`
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
		UserId:           userID,
		Name:             body.Name,
		SmallBlindCents:  body.SmallBlindCents,
		BigBlindCents:    body.BigBlindCents,
		MaxPlayers:       body.MaxPlayers,
		BuyInAmountCents: body.BuyInAmountCents,
		MinBuyInCents:    body.MinBuyInCents,
		MaxBuyInCents:    body.MaxBuyInCents,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, res)
}

type JoinTableRequestBody struct {
	BuyInAmountCents int64  `json:"buy_in_amount_cents"`
	PreferredSeat    *int32 `json:"preferred_seat"`
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

	// preferred_seat ausente => -1 (primer asiento libre), no 0.
	preferredSeat := int32(-1)
	if body.PreferredSeat != nil {
		preferredSeat = *body.PreferredSeat
	}

	res, err := h.pokerClient.JoinTable(r.Context(), &pb_poker.JoinTableRequest{
		TableId:          tableID,
		UserId:           userID,
		BuyInAmountCents: body.BuyInAmountCents,
		PreferredSeat:    preferredSeat,
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

	h.enrichPlayers(r.Context(), res.GetUpdate().GetPlayers())

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

	h.enrichSeats(r.Context(), res.GetSeats())
	h.enrichPlayers(r.Context(), res.GetCurrentRound().GetPlayers())

	WriteJSON(w, http.StatusOK, res)
}

func (h *PokerHandler) CloseTable(w http.ResponseWriter, r *http.Request) {
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

	res, err := h.pokerClient.CloseTable(r.Context(), &pb_poker.CloseTableRequest{
		TableId: tableID,
		UserId:  userID,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}
