package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os/signal"
	"syscall"

	statsPb "github.com/ChGSufro/sis-distribuidos-casino/stats/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/stats/internal/client"
	"github.com/ChGSufro/sis-distribuidos-casino/stats/internal/config"
	"github.com/ChGSufro/sis-distribuidos-casino/stats/internal/handler"
	"github.com/ChGSufro/sis-distribuidos-casino/stats/internal/service"
	"github.com/ChGSufro/sis-distribuidos-casino/stats/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	if err := run(); err != nil {
		slog.Error("servicio de stats termino con error", "error", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		return err
	}
	defer st.Close(context.Background())

	svc := service.New(st)

	stream := client.New(cfg.HistoryAddr, cfg.ReconnectMin, cfg.ReconnectMax, st, svc)
	go stream.Run(ctx)

	gs := grpc.NewServer()
	statsPb.RegisterStatsServiceServer(gs, handler.New(svc))
	reflection.Register(gs)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		stop()
		slog.Info("apagando stats", "puerto", cfg.GRPCPort)
		gs.GracefulStop()
	}()

	slog.Info("servicio de stats escuchando", "addr", lis.Addr().String())
	if err := gs.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}