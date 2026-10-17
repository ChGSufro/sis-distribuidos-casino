package handler

import (
	"net/http"
	"strconv"

	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/middleware"
	pb_history "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/history"
)

type HistoryHandler struct {
	historyClient pb_history.HistoryServiceClient
}

func NewHistoryHandler(historyClient pb_history.HistoryServiceClient) *HistoryHandler {
	return &HistoryHandler{historyClient: historyClient}
}

func (h *HistoryHandler) GetPlayerHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	gameType := r.URL.Query().Get("game_type")

	limit := int32(20)
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}

	offset := int32(0)
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	res, err := h.historyClient.GetPlayerHistory(r.Context(), &pb_history.GetPlayerHistoryRequest{
		UserId:   userID,
		GameType: gameType,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}
