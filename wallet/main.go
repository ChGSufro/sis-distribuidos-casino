package main

import (
	"context"
	"log"
	"net"
	"time"

	pb "github.com/ChGSufro/sis-distribuidos-casino/wallet/pb"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedWalletServiceServer
}

// GetBalance: implementacion minima con datos mockeados
func (s *server) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	log.Printf("GetBalance solicitado para user_id: %s", req.GetUserId())
	return &pb.GetBalanceResponse{
		UserId:       req.GetUserId(),
		BalanceCents: 100000, // 1000 creditos iniciales mockeados
		UpdatedAt:    time.Now().Format(time.RFC3339),
	}, nil
}

// Deposit: respuesta simulada incrementando saldo
func (s *server) Deposit(ctx context.Context, req *pb.DepositRequest) (*pb.DepositResponse, error) {
	log.Printf("Deposit recibido para user_id: %s, monto: %d centavos", req.GetUserId(), req.GetAmountCents())
	return &pb.DepositResponse{
		TransactionId:    "tx-mock-001",
		NewBalanceCents: 100000 + req.GetAmountCents(),
	}, nil
}

// Hold: reserva simulada de fondos para apuestas
func (s *server) Hold(ctx context.Context, req *pb.HoldRequest) (*pb.HoldResponse, error) {
	log.Printf("Hold solicitado para user_id: %s, monto: %d centavos, juego: %s", req.GetUserId(), req.GetAmountCents(), req.GetGameType())
	return &pb.HoldResponse{
		HoldId:      "hold-mock-777",
		AmountCents: req.GetAmountCents(),
		Status:      "active",
	}, nil
}

func main() {
	port := ":50051"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("no se pudo escuchar en %s: %v", port, err)
	}

	srv := grpc.NewServer()
	pb.RegisterWalletServiceServer(srv, &server{})

	log.Printf("Servidor Wallet gRPC escuchando en %s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir gRPC: %v", err)
	}
}
