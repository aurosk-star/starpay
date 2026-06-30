package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Auth     AuthConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type HTTPConfig struct {
	Addr string
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AuthConfig struct {
	JWTSecret             string
	AccessTokenTTL        time.Duration
	RefreshTokenTTL       time.Duration
	RefreshCookieName     string
	RefreshCookieSecure   bool
	RefreshCookieSameSite string
}

func Load() (Config, error) {
	return Config{
		App: AppConfig{
			Name: env("APP_NAME", "payment-gateway"),
			Env:  env("APP_ENV", "local"),
		},
		HTTP: HTTPConfig{
			Addr: env("HTTP_ADDR", ":8080"),
		},
		Database: DatabaseConfig{
			URL: env("DATABASE_URL", "postgres://payment:payment@localhost:5432/payment_gateway?sslmode=disable"),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       0,
		},
		Auth: AuthConfig{
			JWTSecret:             env("JWT_SECRET", "local-development-secret-change-me"),
			AccessTokenTTL:        durationEnv("ACCESS_TOKEN_TTL", 12*time.Hour),
			RefreshTokenTTL:       durationEnv("REFRESH_TOKEN_TTL", 7*24*time.Hour),
			RefreshCookieName:     env("REFRESH_COOKIE_NAME", "pg_refresh_token"),
			RefreshCookieSecure:   boolEnv("REFRESH_COOKIE_SECURE", false),
			RefreshCookieSameSite: env("REFRESH_COOKIE_SAME_SITE", "lax"),
		},
	}, nil
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
