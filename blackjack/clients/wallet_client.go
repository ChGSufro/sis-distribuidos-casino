package clients

import (
	"context"
	"fmt"
	"time"

	walletpb "github.com/ChGSufro/sis-distribuidos-casino/wallet/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type WalletClient interface {
	Hold(ctx context.Context, userID string, amountCents int64, gameRefID, idempotencyKey string) (string, error)
	Settle(ctx context.Context, holdID string, winAmountCents int64) error
	Release(ctx context.Context, holdID string) error
	Close() error
}

type grpcWalletClient struct {
	conn   *grpc.ClientConn
	client walletpb.WalletServiceClient
}

func NewWalletClient(serverAddr string) (WalletClient, error) {
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("conectar con WalletService en %s: %w", serverAddr, err)
	}

	return &grpcWalletClient{
		conn:   conn,
		client: walletpb.NewWalletServiceClient(conn),
	}, nil
}

func (c *grpcWalletClient) Hold(ctx context.Context, userID string, amountCents int64, gameRefID, idempotencyKey string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := c.client.Hold(callCtx, &walletpb.HoldRequest{
		UserId:         userID,
		AmountCents:    amountCents,
		GameType:       "blackjack",
		GameRefId:      gameRefID,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return "", err
	}

	return res.GetHoldId(), nil
}

func (c *grpcWalletClient) Settle(ctx context.Context, holdID string, winAmountCents int64) error {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.client.Settle(callCtx, &walletpb.SettleRequest{
		HoldId:         holdID,
		WinAmountCents: winAmountCents,
	})
	return err
}

func (c *grpcWalletClient) Release(ctx context.Context, holdID string) error {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.client.Release(callCtx, &walletpb.ReleaseRequest{
		HoldId: holdID,
	})
	return err
}

func (c *grpcWalletClient) Close() error {
	return c.conn.Close()
}

