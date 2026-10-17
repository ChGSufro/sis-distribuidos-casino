package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/middleware"
	pb_bj "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/blackjack"
)

type BlackjackHandler struct {
	bjClient pb_bj.BlackjackServiceClient
}

func NewBlackjackHandler(bjClient pb_bj.BlackjackServiceClient) *BlackjackHandler {
	return &BlackjackHandler{bjClient: bjClient}
}

type CreateHandRequestBody struct {
	BetAmountCents int64 `json:"bet_amount_cents"`
}

func (h *BlackjackHandler) CreateHand(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	var body CreateHandRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "cuerpo de solicitud JSON inválido")
		return
	}
	if body.BetAmountCents <= 0 {
		WriteError(w, http.StatusBadRequest, "bet_amount_cents debe ser mayor a 0")
		return
	}

	res, err := h.bjClient.CreateHand(r.Context(), &pb_bj.CreateHandRequest{
		UserId:         userID,
		BetAmountCents: body.BetAmountCents,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, res)
}

type PlayerActionRequestBody struct {
	SubHandIndex int32  `json:"sub_hand_index"`
	Action       string `json:"action"` // "hit", "stand", "double", "split", "surrender"
}

func (h *BlackjackHandler) PlayerAction(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	handID := r.PathValue("id")
	if handID == "" {
		WriteError(w, http.StatusBadRequest, "id de mano requerido en la ruta")
		return
	}

	var body PlayerActionRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "cuerpo de solicitud JSON inválido")
		return
	}
	if body.Action == "" {
		WriteError(w, http.StatusBadRequest, "action es requerido")
		return
	}

	res, err := h.bjClient.PlayerAction(r.Context(), &pb_bj.PlayerActionRequest{
		HandId:       handID,
		UserId:       userID,
		SubHandIndex: body.SubHandIndex,
		Action:       body.Action,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}

func (h *BlackjackHandler) GetHandState(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	handID := r.PathValue("id")
	if handID == "" {
		WriteError(w, http.StatusBadRequest, "id de mano requerido en la ruta")
		return
	}

	res, err := h.bjClient.GetHandState(r.Context(), &pb_bj.GetHandStateRequest{
		HandId: handID,
		UserId: userID,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}
