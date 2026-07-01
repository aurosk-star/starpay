package service

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"payment-gateway/ent"
	configrepo "payment-gateway/internal/domain/configs/repository"
)

const (
	DefaultGatewayBaseURL    = "http://localhost:8080"
	DefaultPaymentNotifyPath = "/v1/channel/notify"
	DefaultCurrency          = "CNY"
	DefaultLocale            = "zh-CN"
)

var ErrInvalidGatewayBaseURL = errors.New("invalid gateway_base_url")

type Service struct {
	configs configrepo.Repository
}

func New(client *ent.Client) Service {
	return Service{configs: configrepo.New(client)}
}

type UpdateGatewayConfigInput struct {
	GatewayBaseURL    string
	PaymentNotifyPath string
	DefaultCurrency   string
	DefaultLocale     string
	RequestIDEnabled  bool
	MaintenanceMode   bool
	Extra             map[string]any
}

func (s Service) GetGatewayConfig(ctx context.Context) (*ent.GatewayConfig, error) {
	cfg, err := s.configs.First(ctx)
	if err == nil {
		return ensureExtra(cfg), nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	cfg, err = s.configs.CreateDefault(ctx)
	if err != nil {
		return nil, err
	}
	return ensureExtra(cfg), nil
}

func (s Service) UpdateGatewayConfig(ctx context.Context, input UpdateGatewayConfigInput) (*ent.GatewayConfig, error) {
	current, err := s.GetGatewayConfig(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, err
	}
	return s.configs.Update(ctx, current.ID, configrepo.UpdateGatewayConfigInput{
		GatewayBaseURL:    normalized.GatewayBaseURL,
		PaymentNotifyPath: normalized.PaymentNotifyPath,
		DefaultCurrency:   normalized.DefaultCurrency,
		DefaultLocale:     normalized.DefaultLocale,
		RequestIDEnabled:  normalized.RequestIDEnabled,
		MaintenanceMode:   normalized.MaintenanceMode,
		Extra:             normalized.Extra,
	})
}

func normalizeInput(input UpdateGatewayConfigInput) (UpdateGatewayConfigInput, error) {
	baseURL := strings.TrimSpace(input.GatewayBaseURL)
	if baseURL == "" {
		baseURL = DefaultGatewayBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return UpdateGatewayConfigInput{}, ErrInvalidGatewayBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	currency := strings.ToUpper(strings.TrimSpace(input.DefaultCurrency))
	if currency == "" {
		currency = DefaultCurrency
	}
	locale := strings.TrimSpace(input.DefaultLocale)
	if locale == "" {
		locale = DefaultLocale
	}
	extra := input.Extra
	if extra == nil {
		extra = map[string]any{}
	}
	return UpdateGatewayConfigInput{
		GatewayBaseURL:    baseURL,
		PaymentNotifyPath: normalizePath(input.PaymentNotifyPath, DefaultPaymentNotifyPath),
		DefaultCurrency:   currency,
		DefaultLocale:     locale,
		RequestIDEnabled:  input.RequestIDEnabled,
		MaintenanceMode:   input.MaintenanceMode,
		Extra:             extra,
	}, nil
}

func normalizePath(value string, fallback string) string {
	path := strings.TrimSpace(value)
	if path == "" {
		return fallback
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func ensureExtra(cfg *ent.GatewayConfig) *ent.GatewayConfig {
	if cfg != nil && cfg.Extra == nil {
		cfg.Extra = map[string]any{}
	}
	return cfg
}
