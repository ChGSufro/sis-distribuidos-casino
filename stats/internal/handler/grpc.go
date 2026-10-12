package handler

import (
	"context"

	statsPb "github.com/ChGSufro/sis-distribuidos-casino/stats/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/stats/internal/service"
)

// Handler adapta la lógica del Service al gRPC StatsService.
type Handler struct {
	statsPb.UnimplementedStatsServiceServer
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetPlayerStats(ctx context.Context, req *statsPb.GetPlayerStatsRequest) (*statsPb.GetPlayerStatsResponse, error) {
	return h.svc.GetPlayerStats(ctx, req)
}

func (h *Handler) GetPlayerGameStats(ctx context.Context, req *statsPb.GetPlayerGameStatsRequest) (*statsPb.GetPlayerGameStatsResponse, error) {
	return h.svc.GetPlayerGameStats(ctx, req)
}

func (h *Handler) GetLeaderboard(ctx context.Context, req *statsPb.GetLeaderboardRequest) (*statsPb.GetLeaderboardResponse, error) {
	return h.svc.GetLeaderboard(ctx, req)
}