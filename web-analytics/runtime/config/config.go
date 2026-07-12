package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the web analytics runtime configuration read from the
// environment. It replaces the dependency on the platform core config so the
// extension depends only on the public SDK surface.
type Config struct {
	DSN         string
	GeoIPDBPath string
	APIBaseURL  string
}

// Load reads the runtime configuration from the environment. The API base URL
// derivation matches the platform, so the tracking snippet keeps pointing at
// the same endpoint when API_BASE_URL is not set explicitly.
func Load() (*Config, error) {
	_ = godotenv.Load() //nolint:errcheck // .env file is optional

	return &Config{
		DSN:         strings.TrimSpace(os.Getenv("DATABASE_DSN")),
		GeoIPDBPath: getEnv("GEOIP_DB_PATH", ""),
		APIBaseURL:  getEnv("API_BASE_URL", defaultAPIBaseURL()),
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

	url := fmt.Sprintf("%s://api.%s", protocol, domain)
	if environment == "development" {
		url = fmt.Sprintf("%s:%s", url, port)
	}
	return url
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
