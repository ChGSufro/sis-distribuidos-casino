package config

import (
	"os"
	"strconv"
	"time"
)

// Config agrupa la configuracion del servicio leida desde variables de entorno.
type Config struct {
	GRPCPort    string
	MongoURI    string
	MongoDB     string
	HistoryAddr string
	TurnTTL     time.Duration
}

// Load construye la configuracion a partir del entorno, con valores por
// defecto sensatos para desarrollo.
func Load() Config {
	return Config{
		GRPCPort:    getEnv("GRPC_PORT", "50055"),
		MongoURI:    getEnv("MONGO_URI", "mongodb://mongo-poker:27017"),
		MongoDB:     getEnv("MONGO_DB", "poker_state_db"),
		HistoryAddr: getEnv("HISTORY_ADDR", "history:50053"),
		TurnTTL:     time.Duration(getDuration("TURN_TTL_SECONDS", 30)) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
