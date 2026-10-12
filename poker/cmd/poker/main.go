package main

import (
	"context"
	"log"
	"net"
	"time"

	pb "github.com/ChGSufro/sis-distribuidos-casino/poker/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/client"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/config"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/handler"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/service"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	st, err := store.New(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("no se pudo conectar a MongoDB: %v", err)
	}
	defer func() { _ = st.Close(ctx) }()

	// Seams Wallet/History: por ahora implementaciones no-op locales.
	// Ver internal/client para conectar los servicios reales.
	svc := service.New(st, client.NoopWallet{}, client.NoopHistory{}, cfg.TurnTTL)
	go svc.RunAutoFold(ctx, time.Second)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("no se pudo escuchar en :%s: %v", cfg.GRPCPort, err)
	}

	srv := grpc.NewServer()
	pb.RegisterPokerServiceServer(srv, handler.New(svc))
	reflection.Register(srv)

	log.Printf("Servidor Poker gRPC escuchando en :%s", cfg.GRPCPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir gRPC: %v", err)
	}
}
