package handler

import (
	"net/http"
	"strconv"

	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/middleware"
	pb_stats "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/stats"
)

type StatsHandler struct {
	statsClient pb_stats.StatsServiceClient
}

func NewStatsHandler(statsClient pb_stats.StatsServiceClient) *StatsHandler {
	return &StatsHandler{statsClient: statsClient}
}

func (h *StatsHandler) GetPlayerStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	res, err := h.statsClient.GetPlayerStats(r.Context(), &pb_stats.GetPlayerStatsRequest{
		UserId: userID,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}

func (h *StatsHandler) GetPlayerGameStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "usuario no identificado")
		return
	}

	gameType := r.PathValue("game")
	if gameType == "" {
		WriteError(w, http.StatusBadRequest, "tipo de juego requerido en la ruta")
		return
	}

	res, err := h.statsClient.GetPlayerGameStats(r.Context(), &pb_stats.GetPlayerGameStatsRequest{
		UserId:   userID,
		GameType: gameType,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}

func (h *StatsHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "all_time"
	}

	limit := int32(10)
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}

	res, err := h.statsClient.GetLeaderboard(r.Context(), &pb_stats.GetLeaderboardRequest{
		Period: period,
		Limit:  limit,
	})
	if err != nil {
		WriteGRPCError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, res)
}
