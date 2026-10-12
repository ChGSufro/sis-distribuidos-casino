package config

import "os"

type Config struct {
	Port              string
	MongoURI          string
	MongoDBName       string
	WalletServiceURL  string
	HistoryServiceURL string
}

func Load() *Config {
	return &Config{
		Port:              getEnv("PORT", "50052"),
		MongoURI:          getEnv("MONGO_URI", "mongodb://casino_user:casino_pass@localhost:27017/bj_state_db?authSource=admin"),
		MongoDBName:       getEnv("MONGO_DB_NAME", "bj_state_db"),
		WalletServiceURL:  getEnv("WALLET_SERVICE_URL", "localhost:50051"),
		HistoryServiceURL: getEnv("HISTORY_SERVICE_URL", "localhost:50053"),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

