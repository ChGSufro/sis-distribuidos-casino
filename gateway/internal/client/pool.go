package client

import (
	"fmt"
	"log"

	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/config"
	pb_bj "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/blackjack"
	pb_history "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/history"
	pb_poker "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/poker"
	pb_stats "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/stats"
	pb_users "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/users"
	pb_wallet "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/wallet"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Clients struct {
	conns []*grpc.ClientConn

	Users     pb_users.UsersServiceClient
	Wallet    pb_wallet.WalletServiceClient
	Blackjack pb_bj.BlackjackServiceClient
	Poker     pb_poker.PokerServiceClient
	History   pb_history.HistoryServiceClient
	Stats     pb_stats.StatsServiceClient
}

func Init(cfg *config.Config) (*Clients, error) {
	c := &Clients{
		conns: make([]*grpc.ClientConn, 0, 6),
	}

	dial := func(addr, name string) (*grpc.ClientConn, error) {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("conectar a %s en %s: %w", name, addr, err)
		}
		c.conns = append(c.conns, conn)
		log.Printf("Conexión gRPC configurada para %s (%s)", name, addr)
		return conn, nil
	}

	connUsers, err := dial(cfg.UsersAddr, "UsersService")
	if err != nil {
		c.Close()
		return nil, err
	}
	c.Users = pb_users.NewUsersServiceClient(connUsers)

	connWallet, err := dial(cfg.WalletAddr, "WalletService")
	if err != nil {
		c.Close()
		return nil, err
	}
	c.Wallet = pb_wallet.NewWalletServiceClient(connWallet)

	connBJ, err := dial(cfg.BJAddr, "BlackjackService")
	if err != nil {
		c.Close()
		return nil, err
	}
	c.Blackjack = pb_bj.NewBlackjackServiceClient(connBJ)

	connPoker, err := dial(cfg.PokerAddr, "PokerService")
	if err != nil {
		c.Close()
		return nil, err
	}
	c.Poker = pb_poker.NewPokerServiceClient(connPoker)

	connHistory, err := dial(cfg.HistoryAddr, "HistoryService")
	if err != nil {
		c.Close()
		return nil, err
	}
	c.History = pb_history.NewHistoryServiceClient(connHistory)

	connStats, err := dial(cfg.StatsAddr, "StatsService")
	if err != nil {
		c.Close()
		return nil, err
	}
	c.Stats = pb_stats.NewStatsServiceClient(connStats)

	return c, nil
}

func (c *Clients) Close() {
	for _, conn := range c.conns {
		if conn != nil {
			_ = conn.Close()
		}
	}
}
