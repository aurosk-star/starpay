package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"payment-gateway/ent"
	configrepo "payment-gateway/internal/domain/configs/repository"
)

const (
	DefaultSiteName                       = "starpay-支付网关"
	DefaultGatewayBaseURL                 = "http://localhost:8080"
	DefaultPaymentNotifyPath              = "/v1/channel/notify"
	DefaultCurrency                       = "CNY"
	DefaultLocale                         = "zh-CN"
	DefaultOrderDefaultTTLSeconds         = 900
	DefaultOrderExpireScanIntervalSeconds = 30
	DefaultOrderExpireScanLimit           = 100
	DefaultOrderExpireWorkerConcurrency   = 2
	DefaultOpenAPIRateLimit               = 120
	DefaultOpenAPIRateLimitWindowSeconds  = 60
)

var ErrInvalidGatewayBaseURL = errors.New("invalid gateway_base_url")

type Service struct {
	configs  configrepo.Repository
	defaults RuntimeDefaults
}

type Option func(*Service)

type RuntimeDefaults struct {
	OrderDefaultTTL              time.Duration
	OrderExpireScanInterval      time.Duration
	OrderExpireScanLimit         int
	OrderExpireWorkerConcurrency int
	OpenAPIRateLimitEnabled      bool
	OpenAPIRateLimit             int
	OpenAPIRateLimitWindow       time.Duration
}

func WithRuntimeDefaults(defaults RuntimeDefaults) Option {
	return func(s *Service) {
		s.defaults = defaults
	}
}

func New(client *ent.Client, opts ...Option) Service {
	svc := Service{
		configs: configrepo.New(client),
		defaults: RuntimeDefaults{
			OpenAPIRateLimitEnabled: true,
		},
	}
	for _, opt := range opts {
		opt(&svc)
	}
	return svc
}

type UpdateGatewayConfigInput struct {
	SiteName                       string
	GatewayBaseURL                 string
	DefaultCurrency                string
	DefaultLocale                  string
	RequestIDEnabled               bool
	MaintenanceMode                bool
	OrderDefaultTTLSeconds         int
	OrderExpireScanIntervalSeconds int
	OrderExpireScanLimit           int
	OrderExpireWorkerConcurrency   int
	OpenAPIRateLimitEnabled        bool
	OpenAPIRateLimit               int
	OpenAPIRateLimitWindowSeconds  int
	Extra                          map[string]any
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
	cfg, err = s.configs.Update(ctx, cfg.ID, configrepo.UpdateGatewayConfigInput{
		SiteName:                       cfg.SiteName,
		GatewayBaseURL:                 cfg.GatewayBaseURL,
		PaymentNotifyPath:              cfg.PaymentNotifyPath,
		DefaultCurrency:                cfg.DefaultCurrency,
		DefaultLocale:                  cfg.DefaultLocale,
		RequestIDEnabled:               cfg.RequestIDEnabled,
		MaintenanceMode:                cfg.MaintenanceMode,
		OrderDefaultTTLSeconds:         s.defaultOrderDefaultTTLSeconds(),
		OrderExpireScanIntervalSeconds: s.defaultOrderExpireScanIntervalSeconds(),
		OrderExpireScanLimit:           s.defaultOrderExpireScanLimit(),
		OrderExpireWorkerConcurrency:   s.defaultOrderExpireWorkerConcurrency(),
		OpenAPIRateLimitEnabled:        s.defaults.OpenAPIRateLimitEnabled,
		OpenAPIRateLimit:               s.defaultOpenAPIRateLimit(),
		OpenAPIRateLimitWindowSeconds:  s.defaultOpenAPIRateLimitWindowSeconds(),
		Extra:                          cfg.Extra,
	})
	if err != nil {
		return nil, err
	}
	return ensureExtra(cfg), nil
}

func (s Service) OrderDefaultTTL(ctx context.Context) (time.Duration, error) {
	cfg, err := s.GetGatewayConfig(ctx)
	if err != nil {
		return 0, err
	}
	return time.Duration(cfg.OrderDefaultTTLSeconds) * time.Second, nil
}

func (s Service) OrderExpireScanConfig(ctx context.Context) (time.Duration, int, error) {
	cfg, err := s.GetGatewayConfig(ctx)
	if err != nil {
		return 0, 0, err
	}
	return time.Duration(cfg.OrderExpireScanIntervalSeconds) * time.Second, cfg.OrderExpireScanLimit, nil
}

func (s Service) OrderExpireWorkerConcurrency(ctx context.Context) (int, error) {
	cfg, err := s.GetGatewayConfig(ctx)
	if err != nil {
		return 0, err
	}
	return cfg.OrderExpireWorkerConcurrency, nil
}

func (s Service) OpenAPIRateLimitConfig(ctx context.Context) (bool, int, time.Duration, error) {
	cfg, err := s.GetGatewayConfig(ctx)
	if err != nil {
		return false, 0, 0, err
	}
	return cfg.OpenAPIRateLimitEnabled, cfg.OpenAPIRateLimit, time.Duration(cfg.OpenAPIRateLimitWindowSeconds) * time.Second, nil
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
		SiteName:                       normalized.SiteName,
		GatewayBaseURL:                 normalized.GatewayBaseURL,
		PaymentNotifyPath:              DefaultPaymentNotifyPath,
		DefaultCurrency:                normalized.DefaultCurrency,
		DefaultLocale:                  normalized.DefaultLocale,
		RequestIDEnabled:               normalized.RequestIDEnabled,
		MaintenanceMode:                normalized.MaintenanceMode,
		OrderDefaultTTLSeconds:         normalized.OrderDefaultTTLSeconds,
		OrderExpireScanIntervalSeconds: normalized.OrderExpireScanIntervalSeconds,
		OrderExpireScanLimit:           normalized.OrderExpireScanLimit,
		OrderExpireWorkerConcurrency:   normalized.OrderExpireWorkerConcurrency,
		OpenAPIRateLimitEnabled:        normalized.OpenAPIRateLimitEnabled,
		OpenAPIRateLimit:               normalized.OpenAPIRateLimit,
		OpenAPIRateLimitWindowSeconds:  normalized.OpenAPIRateLimitWindowSeconds,
		Extra:                          normalized.Extra,
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

	siteName := strings.TrimSpace(input.SiteName)
	if siteName == "" {
		siteName = DefaultSiteName
	}
	orderDefaultTTLSeconds := input.OrderDefaultTTLSeconds
	if orderDefaultTTLSeconds <= 0 {
		orderDefaultTTLSeconds = DefaultOrderDefaultTTLSeconds
	}
	orderExpireScanIntervalSeconds := input.OrderExpireScanIntervalSeconds
	if orderExpireScanIntervalSeconds <= 0 {
		orderExpireScanIntervalSeconds = DefaultOrderExpireScanIntervalSeconds
	}
	orderExpireScanLimit := input.OrderExpireScanLimit
	if orderExpireScanLimit <= 0 {
		orderExpireScanLimit = DefaultOrderExpireScanLimit
	}
	orderExpireWorkerConcurrency := input.OrderExpireWorkerConcurrency
	if orderExpireWorkerConcurrency <= 0 {
		orderExpireWorkerConcurrency = DefaultOrderExpireWorkerConcurrency
	}
	openAPIRateLimit := input.OpenAPIRateLimit
	if openAPIRateLimit <= 0 {
		openAPIRateLimit = DefaultOpenAPIRateLimit
	}
	openAPIRateLimitWindowSeconds := input.OpenAPIRateLimitWindowSeconds
	if openAPIRateLimitWindowSeconds <= 0 {
		openAPIRateLimitWindowSeconds = DefaultOpenAPIRateLimitWindowSeconds
	}
	return UpdateGatewayConfigInput{
		SiteName:                       siteName,
		GatewayBaseURL:                 baseURL,
		DefaultCurrency:                currency,
		DefaultLocale:                  locale,
		RequestIDEnabled:               input.RequestIDEnabled,
		MaintenanceMode:                input.MaintenanceMode,
		OrderDefaultTTLSeconds:         orderDefaultTTLSeconds,
		OrderExpireScanIntervalSeconds: orderExpireScanIntervalSeconds,
		OrderExpireScanLimit:           orderExpireScanLimit,
		OrderExpireWorkerConcurrency:   orderExpireWorkerConcurrency,
		OpenAPIRateLimitEnabled:        input.OpenAPIRateLimitEnabled,
		OpenAPIRateLimit:               openAPIRateLimit,
		OpenAPIRateLimitWindowSeconds:  openAPIRateLimitWindowSeconds,
		Extra:                          extra,
	}, nil
}

func ensureExtra(cfg *ent.GatewayConfig) *ent.GatewayConfig {
	if cfg != nil && cfg.Extra == nil {
		cfg.Extra = map[string]any{}
	}
	return cfg
}

func (s Service) defaultOrderDefaultTTLSeconds() int {
	if s.defaults.OrderDefaultTTL > 0 {
		return int(s.defaults.OrderDefaultTTL / time.Second)
	}
	return DefaultOrderDefaultTTLSeconds
}

func (s Service) defaultOrderExpireScanIntervalSeconds() int {
	if s.defaults.OrderExpireScanInterval > 0 {
		return int(s.defaults.OrderExpireScanInterval / time.Second)
	}
	return DefaultOrderExpireScanIntervalSeconds
}

func (s Service) defaultOrderExpireScanLimit() int {
	if s.defaults.OrderExpireScanLimit > 0 {
		return s.defaults.OrderExpireScanLimit
	}
	return DefaultOrderExpireScanLimit
}

func (s Service) defaultOrderExpireWorkerConcurrency() int {
	if s.defaults.OrderExpireWorkerConcurrency > 0 {
		return s.defaults.OrderExpireWorkerConcurrency
	}
	return DefaultOrderExpireWorkerConcurrency
}

func (s Service) defaultOpenAPIRateLimit() int {
	if s.defaults.OpenAPIRateLimit > 0 {
		return s.defaults.OpenAPIRateLimit
	}
	return DefaultOpenAPIRateLimit
}

func (s Service) defaultOpenAPIRateLimitWindowSeconds() int {
	if s.defaults.OpenAPIRateLimitWindow > 0 {
		return int(s.defaults.OpenAPIRateLimitWindow / time.Second)
	}
	return DefaultOpenAPIRateLimitWindowSeconds
}
