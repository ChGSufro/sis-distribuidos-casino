package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/client/historypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const historyCallTimeout = 2 * time.Second

// grpcHistory implementa HistoryClient contra el History Service via gRPC.
// Los fallos de registro se registran en log pero nunca interrumpen la
// partida (el historial es best-effort).
type grpcHistory struct {
	conn *grpc.ClientConn
	cli  historypb.HistoryServiceClient
}

// NewHistoryGRPC conecta con el History Service en addr ("host:puerto").
// Si addr esta vacio o el dial falla, devuelve un cliente Noop con aviso
// (operacion degradada), siguiendo el patron de blackjack.
func NewHistoryGRPC(addr string) HistoryClient {
	if addr == "" {
		log.Printf("Advertencia: HISTORY_ADDR vacio. Operando sin historial.")
		return NoopHistory{}
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Advertencia: no se pudo conectar a HistoryService en %s: %v. Operando sin historial.", addr, err)
		return NoopHistory{}
	}
	return &grpcHistory{conn: conn, cli: historypb.NewHistoryServiceClient(conn)}
}

func (h *grpcHistory) RecordGameEvent(ctx context.Context, event GameEvent) error {
	ctx, cancel := context.WithTimeout(ctx, historyCallTimeout)
	defer cancel()
	_, err := h.cli.RecordGameEvent(ctx, &historypb.RecordGameEventRequest{
		EventId:   event.EventID,
		UserId:    event.UserID,
		GameType:  event.GameType,
		GameRefId: event.GameRefID,
		Action:    event.Action,
		Details:   event.Details,
		Timestamp: event.Timestamp.UTC().Format(time.RFC3339),
	})
	if err != nil {
		log.Printf("[RESILIENCIA] Fallo al registrar evento en HistoryService (%s/%s): %v", event.Action, event.EventID, err)
		return fmt.Errorf("history service record: %w", err)
	}
	return nil
}

func (h *grpcHistory) Close() error { return h.conn.Close() }
