package main

import (
	"context"
	"log"
	"net"
	"time"

	pb "github.com/ChGSufro/sis-distribuidos-casino/stats/pb"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedStatsServiceServer
}

// GetPlayerStats: estadisticas generales simuladas de un jugador
func (s *server) GetPlayerStats(ctx context.Context, req *pb.GetPlayerStatsRequest) (*pb.GetPlayerStatsResponse, error) {
	log.Printf("GetPlayerStats solicitado para user_id: %s", req.GetUserId())
	return &pb.GetPlayerStatsResponse{
		UserId:            req.GetUserId(),
		TotalWageredCents: 250000, // 2500 creditos
		TotalWonCents:     180000, // 1800 creditos
		NetProfitCents:    -70000,
		GamesPlayed:       34,
		WinRate:           0.47,
		LargestWinCents:   25000,
		CurrentStreak:     2,
		BestStreak:        5,
		LastUpdated:       time.Now().Format(time.RFC3339),
	}, nil
}

// GetPlayerGameStats: estadisticas simuladas por juego
func (s *server) GetPlayerGameStats(ctx context.Context, req *pb.GetPlayerGameStatsRequest) (*pb.GetPlayerGameStatsResponse, error) {
	log.Printf("GetPlayerGameStats solicitado para user_id: %s, juego: %s", req.GetUserId(), req.GetGameType())
	return &pb.GetPlayerGameStatsResponse{
		UserId:            req.GetUserId(),
		GameType:          req.GetGameType(),
		GamesPlayed:       18,
		TotalWageredCents: 150000,
		TotalWonCents:     110000,
		LargestWinCents:   25000,
		LastUpdated:       time.Now().Format(time.RFC3339),
	}, nil
}

// GetLeaderboard: ranking simulado (StreamGameEvents queda sin implementar)
func (s *server) GetLeaderboard(ctx context.Context, req *pb.GetLeaderboardRequest) (*pb.GetLeaderboardResponse, error) {
	log.Printf("GetLeaderboard solicitado para periodo: %s, limit: %d", req.GetPeriod(), req.GetLimit())
	return &pb.GetLeaderboardResponse{
		Period:      req.GetPeriod(),
		PeriodStart: time.Now().Format(time.RFC3339),
		Entries: []*pb.LeaderboardEntry{
			{Rank: 1, UserId: "u-100", Username: "crusher88",  ScoreCents: 125000, GamesPlayed: 12},
			{Rank: 2, UserId: "u-101", Username: "riverking",  ScoreCents: 98000,  GamesPlayed: 9},
			{Rank: 3, UserId: "u-102", Username: "acehunter",  ScoreCents: 75000,  GamesPlayed: 15},
		},
		ComputedAt: time.Now().Format(time.RFC3339),
	}, nil
}

func main() {
	port := ":50054"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("no se pudo escuchar en %s: %v", port, err)
	}

	srv := grpc.NewServer()
	pb.RegisterStatsServiceServer(srv, &server{})

	log.Printf("Servidor Stats gRPC escuchando en %s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir gRPC: %v", err)
	}
}