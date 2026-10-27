package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Runtime application config
type Config struct {
	ServerPort      string
	DatabaseURL     string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	InviteTokenTTL  time.Duration
}

// Load config
func Load() (*Config, error) {
	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}
	jwtSecret := getEnv("JWT_SECRET", "dev-secret-do-not-use-in-production")
	return &Config{
		ServerPort:      getEnv("PORT", "8080"),
		DatabaseURL:     dbURL,
		JWTSecret:       jwtSecret,
		AccessTokenTTL:  getDurationEnv("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: getDurationEnv("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		InviteTokenTTL:  getDurationEnv("INVITE_TOKEN_TTL", 48*time.Hour),
	}, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
func getDurationEnv(key string, fallback time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}
	return d
}
