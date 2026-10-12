package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/clients"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/config"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/db"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/engine"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/handler"
	pb "github.com/ChGSufro/sis-distribuidos-casino/blackjack/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/repository"
	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	log.Println("Iniciando Blackjack Game Service...")

	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Conexión a MongoDB (bj_state_db)
	mongoClient, database, err := db.Connect(ctx, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		log.Fatalf("Fallo crítico al conectar con MongoDB: %v", err)
	}
	defer func() {
		_ = mongoClient.Disconnect(context.Background())
	}()

	// 2. Conexión gRPC cliente hacia WalletService
	walletClient, err := clients.NewWalletClient(cfg.WalletServiceURL)
	if err != nil {
		log.Fatalf("Fallo crítico al inicializar cliente Wallet: %v", err)
	}
	defer walletClient.Close()

	// 3. Conexión gRPC cliente hacia HistoryService (resiliente)
	historyClient, err := clients.NewHistoryClient(cfg.HistoryServiceURL)
	if err != nil {
		log.Printf("Advertencia al inicializar cliente History: %v", err)
	}
	defer historyClient.Close()

	// 4. Inyección de dependencias
	repo := repository.NewMongoHandRepository(database)
	gameEngine := engine.NewEngine()
	gameService := service.NewBlackjackService(repo, gameEngine, walletClient, historyClient)
	grpcHandler := handler.NewBlackjackGRPCHandler(gameService)

	addr := fmt.Sprintf(":%s", cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("No se pudo escuchar en el puerto %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterBlackjackServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer)

	// Manejo de señales para graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Blackjack Game Service gRPC escuchando en %s", addr)
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Fatalf("Error en el servidor gRPC: %v", err)
		}
	}()

	sig := <-stopChan
	log.Printf("Señal de apagado recibida (%s). Deteniendo Blackjack Service...", sig)

	grpcServer.GracefulStop()
	log.Println("Blackjack Game Service detenido ordenadamente.")
}
