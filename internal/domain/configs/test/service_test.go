package configstest

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	configsvc "payment-gateway/internal/domain/configs/service"
)

func TestGatewayConfigReturnsDefaultsOnFirstRead(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:gateway_config_defaults?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := configsvc.New(client)
	cfg, err := svc.GetGatewayConfig(ctx)
	if err != nil {
		t.Fatalf("GetGatewayConfig() error = %v", err)
	}
	if cfg.GatewayBaseURL != "http://localhost:8080" {
		t.Fatalf("GatewayBaseURL = %q, want default", cfg.GatewayBaseURL)
	}
	if cfg.PaymentNotifyPath != "/v1/channel/notify" {
		t.Fatalf("PaymentNotifyPath = %q, want default", cfg.PaymentNotifyPath)
	}
	if cfg.SiteName != "starpay-支付网关" {
		t.Fatalf("SiteName = %q, want default", cfg.SiteName)
	}
	if !cfg.RequestIDEnabled {
		t.Fatal("RequestIDEnabled = false, want true")
	}
	if cfg.Extra == nil {
		t.Fatal("Extra is nil, want empty map")
	}
}

func TestGatewayConfigUpdateNormalizesValues(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:gateway_config_update?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := configsvc.New(client)
	cfg, err := svc.UpdateGatewayConfig(ctx, configsvc.UpdateGatewayConfigInput{
		GatewayBaseURL:   " https://pay.example.com/ ",
		SiteName:         " 绘星支付中心 ",
		DefaultCurrency:  " usd ",
		DefaultLocale:    " en-US ",
		RequestIDEnabled: false,
		MaintenanceMode:  true,
		Extra: map[string]any{
			"support_email": "ops@example.com",
		},
	})
	if err != nil {
		t.Fatalf("UpdateGatewayConfig() error = %v", err)
	}
	if cfg.GatewayBaseURL != "https://pay.example.com" {
		t.Fatalf("GatewayBaseURL = %q, want normalized", cfg.GatewayBaseURL)
	}
	if cfg.SiteName != "绘星支付中心" {
		t.Fatalf("SiteName = %q, want trimmed", cfg.SiteName)
	}
	if cfg.PaymentNotifyPath != "/v1/channel/notify" {
		t.Fatalf("PaymentNotifyPath = %q, want fixed default", cfg.PaymentNotifyPath)
	}
	if cfg.DefaultCurrency != "USD" {
		t.Fatalf("DefaultCurrency = %q, want upper case", cfg.DefaultCurrency)
	}
	if cfg.DefaultLocale != "en-US" {
		t.Fatalf("DefaultLocale = %q, want trimmed", cfg.DefaultLocale)
	}
	if cfg.RequestIDEnabled {
		t.Fatal("RequestIDEnabled = true, want false")
	}
	if !cfg.MaintenanceMode {
		t.Fatal("MaintenanceMode = false, want true")
	}
	if cfg.Extra["support_email"] != "ops@example.com" {
		t.Fatalf("Extra = %#v, want support_email", cfg.Extra)
	}
}

func TestGatewayConfigRejectsInvalidBaseURL(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:gateway_config_invalid_url?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := configsvc.New(client)
	_, err := svc.UpdateGatewayConfig(ctx, configsvc.UpdateGatewayConfigInput{
		GatewayBaseURL: "pay.example.com",
	})
	if !errors.Is(err, configsvc.ErrInvalidGatewayBaseURL) {
		t.Fatalf("UpdateGatewayConfig() error = %v, want ErrInvalidGatewayBaseURL", err)
	}
}
