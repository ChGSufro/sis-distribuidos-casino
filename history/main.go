package main

import (
	"context"
	"log"
	"net"

	pb "github.com/ChGSufro/sis-distribuidos-casino/history/pb"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedHistoryServiceServer
}

// RecordGameEvent: registro simulado de un evento de juego
func (s *server) RecordGameEvent(ctx context.Context, req *pb.RecordGameEventRequest) (*pb.RecordGameEventResponse, error) {
	log.Printf("RecordGameEvent recibido para user_id: %s, juego: %s, accion: %s", req.GetUserId(), req.GetGameType(), req.GetAction())
	return &pb.RecordGameEventResponse{
		Recorded: true,
		EventId:  req.GetEventId(),
	}, nil
}

// GetPlayerHistory: historial simulado de un jugador
func (s *server) GetPlayerHistory(ctx context.Context, req *pb.GetPlayerHistoryRequest) (*pb.GetPlayerHistoryResponse, error) {
	log.Printf("GetPlayerHistory solicitado para user_id: %s, juego: %s, limit: %d, offset: %d", req.GetUserId(), req.GetGameType(), req.GetLimit(), req.GetOffset())
	return &pb.GetPlayerHistoryResponse{
		Events: []*pb.GameEvent{
			{
				EventId:    "evt-mock-001",
				UserId:     req.GetUserId(),
				GameType:   "blackjack",
				GameRefId:  "64b8f1c2e4b0a1a2b3c4d5e6",
				Action:     "hand_result",
				Details:    `{"result":"win","win_amount_cents":2000}`,
				Timestamp:  "2026-10-04T12:30:00Z",
			},
			{
				EventId:    "evt-mock-002",
				UserId:     req.GetUserId(),
				GameType:   "poker",
				GameRefId:  "64b8f1c2e4b0a1a2b3c4d5e7",
				Action:     "player_buyin",
				Details:    `{"amount_cents":50000}`,
				Timestamp:  "2026-10-04T11:15:00Z",
			},
		},
		TotalCount: 2,
	}, nil
}

func main() {
	port := ":50053"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("no se pudo escuchar en %s: %v", port, err)
	}

	srv := grpc.NewServer()
	pb.RegisterHistoryServiceServer(srv, &server{})

	log.Printf("Servidor History gRPC escuchando en %s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir gRPC: %v", err)
	}
}