package main

import (
	"context"
	"log"
	"net"

	pb "github.com/ChGSufro/sis-distribuidos-casino/users/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/users/internal/auth"
	"github.com/ChGSufro/sis-distribuidos-casino/users/internal/config"
	"github.com/ChGSufro/sis-distribuidos-casino/users/internal/handler"
	"github.com/ChGSufro/sis-distribuidos-casino/users/internal/service"
	"github.com/ChGSufro/sis-distribuidos-casino/users/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("no se pudo conectar a users_db: %v", err)
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("no se pudieron aplicar las migraciones: %v", err)
	}

	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	svc := service.New(st, tokens)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("no se pudo escuchar en :%s: %v", cfg.GRPCPort, err)
	}

	srv := grpc.NewServer()
	pb.RegisterUsersServiceServer(srv, handler.New(svc))
	reflection.Register(srv)

	log.Printf("Servidor Users gRPC escuchando en :%s", cfg.GRPCPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir gRPC: %v", err)
	}
}
