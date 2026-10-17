package config

import (
	"os"
)

type Config struct {
	HTTPPort    string
	JWTSecret   string
	UsersAddr   string
	WalletAddr  string
	BJAddr      string
	PokerAddr   string
	HistoryAddr string
	StatsAddr   string
}

func Load() *Config {
	return &Config{
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-me"),
		UsersAddr:   getEnv("USERS_ADDR", "users:50056"),
		WalletAddr:  getEnv("WALLET_ADDR", "wallet:50051"),
		BJAddr:      getEnv("BJ_ADDR", "blackjack:50052"),
		PokerAddr:   getEnv("POKER_ADDR", "poker:50055"),
		HistoryAddr: getEnv("HISTORY_ADDR", "history:50053"),
		StatsAddr:   getEnv("STATS_ADDR", "stats:50054"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
