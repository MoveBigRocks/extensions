package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseDSN     string
	APIBaseURL      string
	DatabasePool    DatabasePoolConfig
	ErrorProcessing ErrorProcessingConfig
}

type DatabasePoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type ErrorProcessingConfig struct {
	WorkerCount int
	QueueSize   int
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	dsn := strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_DSN must be set")
	}
	return &Config{
		DatabaseDSN: dsn,
		APIBaseURL:  getEnv("API_BASE_URL", defaultAPIBaseURL()),
		DatabasePool: DatabasePoolConfig{
			MaxOpenConns: envInt("DATABASE_MAX_OPEN_CONNS", 25), MaxIdleConns: envInt("DATABASE_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envDuration("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: envDuration("DATABASE_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},
		ErrorProcessing: ErrorProcessingConfig{
			WorkerCount: envInt("ERROR_PROCESSING_WORKER_COUNT", 4),
			QueueSize:   envInt("ERROR_PROCESSING_QUEUE_SIZE", 1000),
		},
	}, nil
}

func defaultAPIBaseURL() string {
	environment := getEnv("ENVIRONMENT", "development")
	domain := getEnv("DOMAIN", "movebigrocks.com")
	port := getEnv("PORT", "8080")
	protocol := "https"
	if environment == "development" {
		protocol = "http"
		if domain == "movebigrocks.com" {
			domain = "lvh.me"
		}
	}
	result := fmt.Sprintf("%s://api.%s", protocol, domain)
	if environment == "development" {
		result += ":" + port
	}
	return result
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}
