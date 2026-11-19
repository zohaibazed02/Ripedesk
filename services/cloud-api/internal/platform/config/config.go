package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                string
	DatabaseURL         string
	AppDatabaseURL      string
	JWTSecret           string
	CORSOrigins         []string
	HeartbeatTTLSeconds int
}

func Load() Config {
	port := envOrDefault("PORT", "8080")
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	dbURL := envOrDefault(
		"DATABASE_URL",
		"postgres://hospitality:hospitality_dev_secret@localhost:5432/hospitality_cloud?sslmode=disable",
	)
	appDBURL := envOrDefault(
		"APP_DATABASE_URL",
		"postgres://pos_app_user:pos_app_user_dev_secret@localhost:5432/hospitality_cloud?sslmode=disable",
	)
	jwtSecret := envOrDefault("JWT_SECRET", "hospitality-os-dev-jwt-secret-change-in-prod")
	corsRaw := envOrDefault("CORS_ORIGINS", "http://localhost:3000,http://localhost:3001,http://localhost:3002,http://localhost:3003")

	ttl := 45
	if rawTTL := os.Getenv("HEARTBEAT_TTL_SECONDS"); rawTTL != "" {
		if parsed, err := strconv.Atoi(rawTTL); err == nil && parsed > 0 {
			ttl = parsed
		}
	}

	var origins []string
	for _, o := range strings.Split(corsRaw, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return Config{
		Port:                port,
		DatabaseURL:         dbURL,
		AppDatabaseURL:      appDBURL,
		JWTSecret:           jwtSecret,
		CORSOrigins:         origins,
		HeartbeatTTLSeconds: ttl,
	}
}

func envOrDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}
