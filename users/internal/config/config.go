package config

import (
	"os"
	"time"
)

// Config agrupa la configuracion del servicio leida desde variables de entorno.
type Config struct {
	GRPCPort    string
	DatabaseURL string
	JWTSecret   string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
}

// Load construye la configuracion a partir del entorno, con valores por
// defecto sensatos para desarrollo.
func Load() Config {
	return Config{
		GRPCPort:    getEnv("GRPC_PORT", "50056"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://casino:casino@postgres-users:5432/users_db?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-me"),
		AccessTTL:   getEnvDuration("ACCESS_TTL", 15*time.Minute),
		RefreshTTL:  getEnvDuration("REFRESH_TTL", 168*time.Hour),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
