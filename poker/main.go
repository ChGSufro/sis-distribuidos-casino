package main

import (
	"context"
	"log"
	"net"
	"time"

	pb "github.com/ChGSufro/sis-distribuidos-casino/poker/pb"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedPokerServiceServer
}

// ListTables: devuelve un catalogo simulado de mesas
func (s *server) ListTables(ctx context.Context, req *pb.ListTablesRequest) (*pb.ListTablesResponse, error) {
	log.Printf("ListTables solicitado, status_filter: %q", req.GetStatusFilter())
	return &pb.ListTablesResponse{
		Tables: []*pb.Table{
			{
				TableId:         "tbl-mock-001",
				Name:            "Mesa Principiantes",
				SmallBlindCents: 500,
				BigBlindCents:   1000,
				MaxPlayers:      6,
				CurrentPlayers:  4,
				Status:          "active",
			},
			{
				TableId:         "tbl-mock-002",
				Name:            "Mesa Alta Gasta",
				SmallBlindCents: 2500,
				BigBlindCents:   5000,
				MaxPlayers:      9,
				CurrentPlayers:  2,
				Status:          "waiting",
			},
		},
	}, nil
}

// CreateTable: crea una mesa simulada recien creada, sin jugadores
func (s *server) CreateTable(ctx context.Context, req *pb.CreateTableRequest) (*pb.CreateTableResponse, error) {
	log.Printf("CreateTable recibido de user_id: %s, mesa: %q, ciegas: %d/%d, max_players: %d",
		req.GetUserId(), req.GetName(), req.GetSmallBlindCents(), req.GetBigBlindCents(), req.GetMaxPlayers())
	return &pb.CreateTableResponse{
		Table: &pb.Table{
			TableId:         "tbl-mock-003",
			Name:            req.GetName(),
			SmallBlindCents: req.GetSmallBlindCents(),
			BigBlindCents:   req.GetBigBlindCents(),
			MaxPlayers:      req.GetMaxPlayers(),
			CurrentPlayers:  0,
			Status:          "waiting",
		},
	}, nil
}

// JoinTable: asigna un buy-in a la mesa. El hold se resolveria
// internamente contra Wallet.Hold antes de responder.
func (s *server) JoinTable(ctx context.Context, req *pb.JoinTableRequest) (*pb.JoinTableResponse, error) {
	log.Printf("JoinTable: user_id %s en mesa %s, buy-in: %d centavos, asiento preferido: %d",
		req.GetUserId(), req.GetTableId(), req.GetBuyInAmountCents(), req.GetPreferredSeat())
	seat := req.GetPreferredSeat()
	if seat == 0 {
		seat = 5 // primer asiento libre simulado
	}
	return &pb.JoinTableResponse{
		TableId:      req.GetTableId(),
		SeatPosition: seat,
		StackCents:   req.GetBuyInAmountCents(),
		HoldId:       "hold-mock-777",
	}, nil
}

// LeaveTable: liquida el stack del jugador que abandona la mesa
func (s *server) LeaveTable(ctx context.Context, req *pb.LeaveTableRequest) (*pb.LeaveTableResponse, error) {
	log.Printf("LeaveTable: user_id %s abandona la mesa %s", req.GetUserId(), req.GetTableId())
	const finalStack = 12500
	return &pb.LeaveTableResponse{
		TableId:            req.GetTableId(),
		FinalStackCents:    finalStack,
		SettledAmountCents: finalStack,
	}, nil
}

// PlayerAction: devuelve la difusion del estado de la mesa que
// el Gateway reenviaria via WebSocket a todos los sentados
func (s *server) PlayerAction(ctx context.Context, req *pb.PlayerActionRequest) (*pb.PlayerActionResponse, error) {
	log.Printf("PlayerAction: user_id %s, mesa %s, ronda %s, accion: %s, monto: %d",
		req.GetUserId(), req.GetTableId(), req.GetRoundId(), req.GetAction(), req.GetAmountCents())
	return &pb.PlayerActionResponse{
		Update: &pb.TableUpdate{
			RoundId:         req.GetRoundId(),
			Stage:           "flop",
			CurrentTurn:     6,
			CurrentBetCents: 2000,
			PotSizeCents:    18000,
			CommunityCards: []*pb.Card{
				{Suit: "H", Rank: "A"},
				{Suit: "D", Rank: "7"},
				{Suit: "C", Rank: "K"},
			},
			Players: []*pb.PlayerState{
				{
					UserId:               "user-42",
					SeatPosition:         5,
					StackCents:           12500,
					TotalBetInRoundCents: 2000,
					Status:               "active",
				},
				{
					UserId:               "user-77",
					SeatPosition:         6,
					StackCents:           23000,
					TotalBetInRoundCents: 6000,
					Status:               "active",
				},
				{
					UserId:               "user-13",
					SeatPosition:         2,
					StackCents:           31000,
					TotalBetInRoundCents: 10000,
					Status:               "folded",
				},
			},
			TurnExpiresAt: time.Now().Add(30 * time.Second).Format(time.RFC3339),
			Winners:       nil,
		},
	}, nil
}

// GetTableState: estado completo de la mesa. your_hole_cards
// solo trae las cartas privadas del usuario que consulta.
func (s *server) GetTableState(ctx context.Context, req *pb.GetTableStateRequest) (*pb.GetTableStateResponse, error) {
	log.Printf("GetTableState: mesa %s, user_id %s", req.GetTableId(), req.GetUserId())
	return &pb.GetTableStateResponse{
		Table: &pb.Table{
			TableId:         req.GetTableId(),
			Name:            "Mesa Principiantes",
			SmallBlindCents: 500,
			BigBlindCents:   1000,
			Status:          "active",
		},
		Seats: []*pb.Seat{
			{
				SeatPosition: 2,
				UserId:       "user-13",
				StackCents:   31000,
				Status:       "folded",
			},
			{
				SeatPosition: 5,
				UserId:       "user-42",
				StackCents:   12500,
				Status:       "active",
			},
			{
				SeatPosition: 6,
				UserId:       "user-77",
				StackCents:   23000,
				Status:       "active",
			},
		},
		CurrentRound: &pb.RoundState{
			RoundId: "rnd-mock-88",
			Stage:   "flop",
			CommunityCards: []*pb.Card{
				{Suit: "H", Rank: "A"},
				{Suit: "D", Rank: "7"},
				{Suit: "C", Rank: "K"},
			},
			YourHoleCards: []*pb.Card{
				{Suit: "S", Rank: "Q"},
				{Suit: "H", Rank: "Q"},
			},
			PotSizeCents:    18000,
			CurrentBetCents: 2000,
			CurrentTurn:     6,
			TurnExpiresAt:   time.Now().Add(30 * time.Second).Format(time.RFC3339),
		},
	}, nil
}

func main() {
	port := ":50055"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("no se pudo escuchar en %s: %v", port, err)
	}

	srv := grpc.NewServer()
	pb.RegisterPokerServiceServer(srv, &server{})

	log.Printf("Servidor Poker gRPC escuchando en %s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir gRPC: %v", err)
	}
}
