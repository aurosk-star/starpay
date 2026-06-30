package config

import "testing"

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
