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
	Env       string
	Port      string
	DB        DBConfig
	Redis     RedisConfig
	JWT       JWTConfig
	CORS      CORSConfig
	ExportDir string
}

type DBConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type JWTConfig struct {
	Secret        string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

type CORSConfig struct {
	AllowedOrigins []string
}

func Load() (*Config, error) {
	// Load .env if present (dev convenience; Docker sets vars directly)
	_ = godotenv.Load()

	cfg := &Config{
		Env:       getEnv("ENV", "development"),
		Port:      getEnv("PORT", "8080"),
		ExportDir: getEnv("EXPORT_DIR", "/app/exports"),
		DB: DBConfig{
			URL: mustEnv("DATABASE_URL"),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379"),
		},
		JWT: JWTConfig{
			Secret:        mustEnv("JWT_SECRET"),
			RefreshSecret: mustEnv("JWT_REFRESH_SECRET"),
			AccessTTL:     time.Duration(envInt("JWT_ACCESS_TTL_SECONDS", 900)) * time.Second,
			RefreshTTL:    time.Duration(envInt("JWT_REFRESH_TTL_SECONDS", 604800)) * time.Second,
		},
		CORS: CORSConfig{
			AllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173"), ","),
		},
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
