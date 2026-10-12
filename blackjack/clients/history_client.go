package clients

import (
	"context"
	"fmt"
	"log"
	"time"

	historypb "github.com/ChGSufro/sis-distribuidos-casino/blackjack/clients/historypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type HistoryClient interface {
	RecordGameEvent(ctx context.Context, eventID, userID, gameRefID, action, detailsJSON string) error
	Close() error
}

type grpcHistoryClient struct {
	conn   *grpc.ClientConn
	client historypb.HistoryServiceClient
}

func NewHistoryClient(serverAddr string) (HistoryClient, error) {
	if serverAddr == "" {
		return &noopHistoryClient{}, nil
	}

	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Advertencia: No se pudo conectar a HistoryService en %s: %v. Operando en modo degradado.", serverAddr, err)
		return &noopHistoryClient{}, nil
	}

	return &grpcHistoryClient{
		conn:   conn,
		client: historypb.NewHistoryServiceClient(conn),
	}, nil
}

func (c *grpcHistoryClient) RecordGameEvent(ctx context.Context, eventID, userID, gameRefID, action, detailsJSON string) error {
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	now := time.Now().UTC().Format(time.RFC3339)
	req := &historypb.RecordGameEventRequest{
		EventId:   eventID,
		UserId:    userID,
		GameType:  "blackjack",
		GameRefId: gameRefID,
		Action:    action,
		Details:   detailsJSON,
		Timestamp: now,
	}

	_, err := c.client.RecordGameEvent(callCtx, req)
	if err != nil {
		// Principio de resiliencia: la falla de auditoría no tumba la jugada del usuario
		log.Printf("[RESILIENCIA] Fallo al registrar evento en HistoryService (%s): %v", eventID, err)
		return fmt.Errorf("history service record: %w", err)
	}

	return nil
}

func (c *grpcHistoryClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

type noopHistoryClient struct{}

func (n *noopHistoryClient) RecordGameEvent(ctx context.Context, eventID, userID, gameRefID, action, detailsJSON string) error {
	log.Printf("[NOOP History] Evento omitido: eventID=%s action=%s", eventID, action)
	return nil
}

func (n *noopHistoryClient) Close() error {
	return nil
}

