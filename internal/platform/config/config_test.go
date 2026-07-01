package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_NAME", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_SSLMODE", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("APP_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("ORDER_DEFAULT_TTL", "")
	t.Setenv("ORDER_EXPIRE_SCAN_INTERVAL", "")
	t.Setenv("ORDER_EXPIRE_SCAN_LIMIT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Name != "payment-gateway" {
		t.Fatalf("App.Name = %q, want payment-gateway", cfg.App.Name)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Fatalf("HTTP.Addr = %q, want :8080", cfg.HTTP.Addr)
	}
	if cfg.Database.Driver != "postgres" {
		t.Fatalf("Database.Driver = %q, want postgres", cfg.Database.Driver)
	}
	if cfg.Database.DSN == "" {
		t.Fatal("Database.DSN is empty")
	}
	if cfg.Redis.Addr != "localhost:6379" {
		t.Fatalf("Redis.Addr = %q, want localhost:6379", cfg.Redis.Addr)
	}
	if cfg.Auth.AppSecretEncryptionKey != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("Auth.AppSecretEncryptionKey = %q, want default key", cfg.Auth.AppSecretEncryptionKey)
	}
	if cfg.Orders.DefaultTTL != 15*time.Minute {
		t.Fatalf("Orders.DefaultTTL = %v, want 15m", cfg.Orders.DefaultTTL)
	}
	if cfg.Orders.ExpireScanInterval != 30*time.Second {
		t.Fatalf("Orders.ExpireScanInterval = %v, want 30s", cfg.Orders.ExpireScanInterval)
	}
	if cfg.Orders.ExpireScanLimit != 100 {
		t.Fatalf("Orders.ExpireScanLimit = %d, want 100", cfg.Orders.ExpireScanLimit)
	}
	if cfg.Orders.ExpireWorkerConcurrency != 2 {
		t.Fatalf("Orders.ExpireWorkerConcurrency = %d, want 2", cfg.Orders.ExpireWorkerConcurrency)
	}
}

func TestLoadOrderExpirationConfig(t *testing.T) {
	t.Setenv("ORDER_DEFAULT_TTL", "20m")
	t.Setenv("ORDER_EXPIRE_SCAN_INTERVAL", "45s")
	t.Setenv("ORDER_EXPIRE_SCAN_LIMIT", "50")
	t.Setenv("ORDER_EXPIRE_WORKER_CONCURRENCY", "4")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Orders.DefaultTTL != 20*time.Minute {
		t.Fatalf("Orders.DefaultTTL = %v, want 20m", cfg.Orders.DefaultTTL)
	}
	if cfg.Orders.ExpireScanInterval != 45*time.Second {
		t.Fatalf("Orders.ExpireScanInterval = %v, want 45s", cfg.Orders.ExpireScanInterval)
	}
	if cfg.Orders.ExpireScanLimit != 50 {
		t.Fatalf("Orders.ExpireScanLimit = %d, want 50", cfg.Orders.ExpireScanLimit)
	}
	if cfg.Orders.ExpireWorkerConcurrency != 4 {
		t.Fatalf("Orders.ExpireWorkerConcurrency = %d, want 4", cfg.Orders.ExpireWorkerConcurrency)
	}
}

func TestLoadDatabaseDSNFromParts(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "15434")
	t.Setenv("DB_NAME", "payment_gateway")
	t.Setenv("DB_USER", "payment")
	t.Setenv("DB_PASSWORD", "payment")
	t.Setenv("DB_SSLMODE", "disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := "postgres://payment:payment@localhost:15434/payment_gateway?sslmode=disable"
	if cfg.Database.DSN != want {
		t.Fatalf("Database.DSN = %q, want %q", cfg.Database.DSN, want)
	}
}

func TestLoadMySQLDatabaseDSNFromParts(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_DRIVER", "mysql")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "3306")
	t.Setenv("DB_NAME", "payment_gateway")
	t.Setenv("DB_USER", "payment")
	t.Setenv("DB_PASSWORD", "payment")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := "payment:payment@tcp(127.0.0.1:3306)/payment_gateway?parseTime=true&charset=utf8mb4&loc=Local"
	if cfg.Database.Driver != "mysql" {
		t.Fatalf("Database.Driver = %q, want mysql", cfg.Database.Driver)
	}
	if cfg.Database.DSN != want {
		t.Fatalf("Database.DSN = %q, want %q", cfg.Database.DSN, want)
	}
}

func TestLoadReadsDotEnvFile(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(`
HTTP_ADDR=:18080
DB_DRIVER=postgres
DB_HOST=localhost
DB_PORT=15434
DB_NAME=payment_gateway
DB_USER=payment
DB_PASSWORD=payment
DB_SSLMODE=disable
REDIS_ADDR=localhost:16380
`), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.Addr != ":18080" {
		t.Fatalf("HTTP.Addr = %q, want :18080", cfg.HTTP.Addr)
	}
	if cfg.Redis.Addr != "localhost:16380" {
		t.Fatalf("Redis.Addr = %q, want localhost:16380", cfg.Redis.Addr)
	}
	wantDSN := "postgres://payment:payment@localhost:15434/payment_gateway?sslmode=disable"
	if cfg.Database.DSN != wantDSN {
		t.Fatalf("Database.DSN = %q, want %q", cfg.Database.DSN, wantDSN)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"APP_NAME",
		"APP_ENV",
		"HTTP_ADDR",
		"DATABASE_URL",
		"DB_DRIVER",
		"DB_HOST",
		"DB_PORT",
		"DB_NAME",
		"DB_USER",
		"DB_PASSWORD",
		"DB_SSLMODE",
		"REDIS_ADDR",
		"APP_SECRET_ENCRYPTION_KEY",
		"ORDER_DEFAULT_TTL",
		"ORDER_EXPIRE_SCAN_INTERVAL",
		"ORDER_EXPIRE_SCAN_LIMIT",
		"ORDER_EXPIRE_WORKER_CONCURRENCY",
	} {
		t.Setenv(key, "")
	}
}
