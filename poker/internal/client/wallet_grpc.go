package client

import (
	"context"
	"fmt"
	"time"

	walletpb "github.com/ChGSufro/sis-distribuidos-casino/wallet/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const grpcCallTimeout = 5 * time.Second

// grpcWallet implementa WalletClient contra el Wallet Service via gRPC.
type grpcWallet struct {
	conn *grpc.ClientConn
	cli  walletpb.WalletServiceClient
}

// NewWalletGRPC conecta con el Wallet Service en addr ("host:puerto").
func NewWalletGRPC(addr string) (WalletClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("wallet: no se pudo conectar a %s: %w", addr, err)
	}
	return &grpcWallet{conn: conn, cli: walletpb.NewWalletServiceClient(conn)}, nil
}

func (w *grpcWallet) Hold(ctx context.Context, userID string, amountCents int64, gameType, gameRefID, idempotencyKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcCallTimeout)
	defer cancel()
	resp, err := w.cli.Hold(ctx, &walletpb.HoldRequest{
		UserId:         userID,
		AmountCents:    amountCents,
		GameType:       gameType,
		GameRefId:      gameRefID,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return "", fmt.Errorf("wallet.Hold: %w", err)
	}
	return resp.GetHoldId(), nil
}

func (w *grpcWallet) Settle(ctx context.Context, holdID string, winAmountCents int64) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcCallTimeout)
	defer cancel()
	if _, err := w.cli.Settle(ctx, &walletpb.SettleRequest{
		HoldId:         holdID,
		WinAmountCents: winAmountCents,
	}); err != nil {
		return 0, fmt.Errorf("wallet.Settle: %w", err)
	}
	return winAmountCents, nil
}

func (w *grpcWallet) Release(ctx context.Context, holdID string) error {
	ctx, cancel := context.WithTimeout(ctx, grpcCallTimeout)
	defer cancel()
	if _, err := w.cli.Release(ctx, &walletpb.ReleaseRequest{HoldId: holdID}); err != nil {
		return fmt.Errorf("wallet.Release: %w", err)
	}
	return nil
}

func (w *grpcWallet) Close() error { return w.conn.Close() }
