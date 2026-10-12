package config

import (
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	Port       string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
}

func Load() *Config {
	return &Config{
		Port:       getEnv("PORT", "50051"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "casino_user"),
		DBPassword: getEnv("DB_PASSWORD", "casino_pass"),
		DBName:     getEnv("DB_NAME", "wallet_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

func (c *Config) DatabaseURL() string {
	// Si se proporciona directamente una URL completa de base de datos
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(c.DBUser),
		url.QueryEscape(c.DBPassword),
		c.DBHost,
		c.DBPort,
		c.DBName,
		c.DBSSLMode,
	)
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

