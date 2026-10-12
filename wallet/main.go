package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ChGSufro/sis-distribuidos-casino/wallet/config"
	"github.com/ChGSufro/sis-distribuidos-casino/wallet/db"
	"github.com/ChGSufro/sis-distribuidos-casino/wallet/handler"
	pb "github.com/ChGSufro/sis-distribuidos-casino/wallet/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/wallet/repository"
	"github.com/ChGSufro/sis-distribuidos-casino/wallet/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	log.Println("Iniciando Wallet Service...")

	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Conectar a la base de datos PostgreSQL y aplicar migraciones
	pool, err := db.Connect(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("Fallo crítico al conectar con PostgreSQL: %v", err)
	}
	defer pool.Close()

	// Inyección de dependencias
	repo := repository.NewPostgresWalletRepository(pool)
	svc := service.NewWalletService(repo)
	grpcHandler := handler.NewWalletGRPCHandler(svc)

	addr := fmt.Sprintf(":%s", cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("No se pudo escuchar en el puerto %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterWalletServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer) // Permite introspección con grpcurl

	// Canal para escuchar señales de terminación
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Wallet Service gRPC escuchando en %s", addr)
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Fatalf("Error en el servidor gRPC: %v", err)
		}
	}()

	// Esperar señal de apagado
	sig := <-stopChan
	log.Printf("Señal de apagado recibida (%s). Deteniendo servidor gRPC...", sig)

	grpcServer.GracefulStop()
	log.Println("Wallet Service detenido de forma ordenada.")
}
