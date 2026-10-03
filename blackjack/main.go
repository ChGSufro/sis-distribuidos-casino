package main

import (
	"context"
	"log"
	"net"

	pb "github.com/ChGSufro/sis-distribuidos-casino/blackjack/pb"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedBlackjackServiceServer
}

// CreateHand: crea una mano con datos simulados
func (s *server) CreateHand(ctx context.Context, req *pb.CreateHandRequest) (*pb.CreateHandResponse, error) {
	log.Printf("CreateHand recibido para user_id: %s, apuesta: %d centavos", req.GetUserId(), req.GetBetAmountCents())
	return &pb.CreateHandResponse{
		HandId: "hand-mock-101",
		SubHands: []*pb.SubHand{
			{
				Index: 0,
				Cards: []*pb.Card{
					{Suit: "C", Rank: "A", Value: 11, Hidden: false},
					{Suit: "S", Rank: "K", Value: 10, Hidden: false},
				},
				BetAmountCents: req.GetBetAmountCents(),
				Status:         "active",
				Result:         "",
			},
		},
		DealerVisibleCard: &pb.Card{
			Suit:   "D",
			Rank:   "10",
			Value:  10,
			Hidden: false,
		},
		Status: "active",
	}, nil
}

// GetHandState: retorna el estado actual de una mano simulada
func (s *server) GetHandState(ctx context.Context, req *pb.GetHandStateRequest) (*pb.GetHandStateResponse, error) {
	log.Printf("GetHandState solicitado para hand_id: %s, user_id: %s", req.GetHandId(), req.GetUserId())
	return &pb.GetHandStateResponse{
		HandId: req.GetHandId(),
		SubHands: []*pb.SubHand{
			{
				Index: 0,
				Cards: []*pb.Card{
					{Suit: "C", Rank: "A", Value: 11, Hidden: false},
					{Suit: "S", Rank: "K", Value: 10, Hidden: false},
				},
				BetAmountCents: 1000,
				Status:         "blackjack",
				Result:         "blackjack_win",
			},
		},
		DealerCards: []*pb.Card{
			{Suit: "D", Rank: "10", Value: 10, Hidden: false},
			{Suit: "H", Rank: "9", Value: 9, Hidden: false},
		},
		Status:                 "settled",
		TotalWinAmountCents:    2500,
	}, nil
}

func main() {
	port := ":50052"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("no se pudo escuchar en %s: %v", port, err)
	}

	srv := grpc.NewServer()
	pb.RegisterBlackjackServiceServer(srv, &server{})

	log.Printf("Servidor Blackjack gRPC escuchando en %s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir gRPC: %v", err)
	}
}
