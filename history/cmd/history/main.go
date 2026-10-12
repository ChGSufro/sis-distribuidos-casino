package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ChGSufro/sis-distribuidos-casino/history/internal/broadcast"
	"github.com/ChGSufro/sis-distribuidos-casino/history/internal/config"
	"github.com/ChGSufro/sis-distribuidos-casino/history/internal/handler"
	"github.com/ChGSufro/sis-distribuidos-casino/history/internal/service"
	"github.com/ChGSufro/sis-distribuidos-casino/history/internal/store"
	historypb "github.com/ChGSufro/sis-distribuidos-casino/history/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config invalida", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	events, err := store.New(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		slog.Error("conectar storage", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = events.Close(context.Background())
	}()

	svc := service.New(events, broadcast.NewBroker())
	grpcServer := grpc.NewServer()
	historypb.RegisterHistoryServiceServer(grpcServer, handler.New(svc))
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		slog.Error("escuchar puerto", "port", cfg.GRPCPort, "error", err)
		os.Exit(1)
	}

	go func() {
		<-ctx.Done()
		slog.Info("deteniendo History Service")
		grpcServer.GracefulStop()
	}()

	slog.Info("History Service escuchando", "addr", lis.Addr().String(), "mongo_db", cfg.MongoDB)
	if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		slog.Error("servir gRPC", "error", err)
		os.Exit(1)
	}
}