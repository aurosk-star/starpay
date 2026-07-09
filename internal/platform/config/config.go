package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App       AppConfig
	HTTP      HTTPConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Auth      AuthConfig
	Orders    OrdersConfig
	RateLimit RateLimitConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type HTTPConfig struct {
	Addr string
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AuthConfig struct {
	JWTSecret              string
	AccessTokenTTL         time.Duration
	RefreshTokenTTL        time.Duration
	RefreshCookieName      string
	RefreshCookieSecure    bool
	RefreshCookieSameSite  string
	AppSecretEncryptionKey string
}

type OrdersConfig struct {
	DefaultTTL              time.Duration
	ExpireScanInterval      time.Duration
	ExpireScanLimit         int
	ExpireWorkerConcurrency int
}

type RateLimitConfig struct {
	OpenAPIEnabled bool
	OpenAPILimit   int
	OpenAPIWindow  time.Duration
}

func Load() (Config, error) {
	loadDotEnv(".env")
	return Config{
		App: AppConfig{
			Name: env("APP_NAME", "payment-gateway"),
			Env:  env("APP_ENV", "local"),
		},
		HTTP: HTTPConfig{
			Addr: env("HTTP_ADDR", ":8080"),
		},
		Database: loadDatabaseConfig(),
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       0,
		},
		Auth: AuthConfig{
			JWTSecret:              env("JWT_SECRET", "local-development-secret-change-me"),
			AccessTokenTTL:         durationEnv("ACCESS_TOKEN_TTL", 12*time.Hour),
			RefreshTokenTTL:        durationEnv("REFRESH_TOKEN_TTL", 7*24*time.Hour),
			RefreshCookieName:      env("REFRESH_COOKIE_NAME", "pg_refresh_token"),
			RefreshCookieSecure:    boolEnv("REFRESH_COOKIE_SECURE", false),
			RefreshCookieSameSite:  env("REFRESH_COOKIE_SAME_SITE", "lax"),
			AppSecretEncryptionKey: env("APP_SECRET_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef"),
		},
		Orders: OrdersConfig{
			DefaultTTL:              durationEnv("ORDER_DEFAULT_TTL", 15*time.Minute),
			ExpireScanInterval:      durationEnv("ORDER_EXPIRE_SCAN_INTERVAL", 30*time.Second),
			ExpireScanLimit:         intEnv("ORDER_EXPIRE_SCAN_LIMIT", 100),
			ExpireWorkerConcurrency: intEnv("ORDER_EXPIRE_WORKER_CONCURRENCY", 2),
		},
		RateLimit: RateLimitConfig{
			OpenAPIEnabled: boolEnv("OPEN_API_RATE_LIMIT_ENABLED", true),
			OpenAPILimit:   intEnv("OPEN_API_RATE_LIMIT", 120),
			OpenAPIWindow:  durationEnv("OPEN_API_RATE_LIMIT_WINDOW", time.Minute),
		},
	}, nil
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseDotEnvLine(scanner.Text())
		if !ok {
			continue
		}
		if os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func parseDotEnvLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	key, value, ok := strings.Cut(trimmed, "=")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return key, value, true
}

func loadDatabaseConfig() DatabaseConfig {
	driver := strings.ToLower(env("DB_DRIVER", "postgres"))
	dsn := env("DATABASE_URL", "")
	if dsn == "" {
		dsn = buildDatabaseDSN(driver)
	}
	return DatabaseConfig{Driver: driver, DSN: dsn}
}

func buildDatabaseDSN(driver string) string {
	switch driver {
	case "mysql":
		return fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local",
			env("DB_USER", "payment"),
			env("DB_PASSWORD", "payment"),
			env("DB_HOST", "localhost"),
			env("DB_PORT", "3306"),
			env("DB_NAME", "payment_gateway"),
		)
	default:
		user := url.QueryEscape(env("DB_USER", "payment"))
		password := url.QueryEscape(env("DB_PASSWORD", "payment"))
		host := env("DB_HOST", "localhost")
		port := env("DB_PORT", "5432")
		name := env("DB_NAME", "payment_gateway")
		sslMode := env("DB_SSLMODE", "disable")
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, name, sslMode)
	}
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

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
