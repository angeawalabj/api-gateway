package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr   string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	PostgresDSN   string
	JWTPublicKeyPath string
	JWTIssuer        string
	MetricsAddr string
	LogLevel    string
	DevMode     bool
}

func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:       getEnv("GATEWAY_ADDR", ":8080"),
		ReadTimeout:      getDuration("GATEWAY_READ_TIMEOUT", 30*time.Second),
		WriteTimeout:     getDuration("GATEWAY_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:      getDuration("GATEWAY_IDLE_TIMEOUT", 120*time.Second),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          getInt("REDIS_DB", 0),
		PostgresDSN:      getEnv("POSTGRES_DSN", "postgres://gwuser:gwpass@localhost:5432/gateway?sslmode=disable"),
		JWTPublicKeyPath: getEnv("JWT_PUBLIC_KEY_PATH", "./keys/public.pem"),
		JWTIssuer:        getEnv("JWT_ISSUER", "api-gateway"),
		MetricsAddr:      getEnv("METRICS_ADDR", ":9090"),
		LogLevel:         strings.ToLower(getEnv("LOG_LEVEL", "info")),
		DevMode:          getBool("DEV_MODE", false),
	}
	if cfg.PostgresDSN == "" {
		return nil, fmt.Errorf("POSTGRES_DSN est requis")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
