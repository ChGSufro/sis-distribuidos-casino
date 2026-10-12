package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	GRPCPort     string
	MongoURI     string
	MongoDB      string
	HistoryAddr  string
	ReconnectMin time.Duration
	ReconnectMax time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		GRPCPort:     getenv("GRPC_PORT", "50054"),
		MongoURI:     getenv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:      getenv("MONGO_DB", "stats_db"),
		HistoryAddr:  getenv("HISTORY_ADDR", "localhost:50053"),
		ReconnectMin: time.Duration(getenvInt("RECONNECT_INITIAL_MS", 1000)) * time.Millisecond,
		ReconnectMax: time.Duration(getenvInt("RECONNECT_MAX_MS", 30000)) * time.Millisecond,
	}

	if cfg.GRPCPort == "" {
		return Config{}, fmt.Errorf("GRPC_PORT no puede estar vacio")
	}
	if cfg.MongoURI == "" {
		return Config{}, fmt.Errorf("MONGO_URI no puede estar vacio")
	}
	if cfg.MongoDB == "" {
		return Config{}, fmt.Errorf("MONGO_DB no puede estar vacio")
	}
	if cfg.HistoryAddr == "" {
		return Config{}, fmt.Errorf("HISTORY_ADDR no puede estar vacio")
	}
	if cfg.ReconnectMin <= 0 {
		return Config{}, fmt.Errorf("RECONNECT_INITIAL_MS debe ser mayor a 0")
	}
	if cfg.ReconnectMax < cfg.ReconnectMin {
		return Config{}, fmt.Errorf("RECONNECT_MAX_MS no puede ser menor a RECONNECT_INITIAL_MS")
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}