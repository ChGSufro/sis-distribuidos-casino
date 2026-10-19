package main

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/client"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/config"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/handler"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/service"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/store"
	pb "github.com/ChGSufro/sis-distribuidos-casino/poker/pb"
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

	// Wallet: obligatorio (fatal si no esta disponible). History: best-effort
	// con fallback a Noop, siguiendo el patron de blackjack.
	walletClient, err := client.NewWalletGRPC(cfg.WalletAddr)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer func() { _ = walletClient.Close() }()
	historyClient := client.NewHistoryGRPC(cfg.HistoryAddr)
	defer func() { _ = historyClient.Close() }()

	svc := service.New(st, walletClient, historyClient, cfg.TurnTTL, cfg.SettleDisplay)
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
