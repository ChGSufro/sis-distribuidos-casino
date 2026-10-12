package config

import (
	"fmt"
	"os"
)

type Config struct {
	GRPCPort string
	MongoURI string
	MongoDB  string
}

func Load() (Config, error) {
	cfg := Config{
		GRPCPort: getenv("GRPC_PORT", "50053"),
		MongoURI: getenv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:  getenv("MONGO_DB", "audit_history_db"),
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
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}