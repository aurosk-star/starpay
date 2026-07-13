package paypal

import (
	"errors"
	"strings"
)

const (
	DefaultIntent = "CAPTURE"
	DefaultLocale = "zh-CN"
)

var (
	ErrClientIDRequired     = errors.New("paypal client_id is required")
	ErrClientSecretRequired = errors.New("paypal client_secret is required")
	ErrIntentUnsupported    = errors.New("paypal intent must be CAPTURE")
)

type Config struct {
	ClientID     string
	ClientSecret string
	WebhookID    string
	BrandName    string
	Intent       string
	Locale       string
	IsProd       bool
}

func ParseConfig(values map[string]any, env string) (Config, error) {
	cfg := Config{
		ClientID:     strings.TrimSpace(stringValue(values["client_id"])),
		ClientSecret: strings.TrimSpace(stringValue(values["client_secret"])),
		WebhookID:    strings.TrimSpace(stringValue(values["webhook_id"])),
		BrandName:    strings.TrimSpace(stringValue(values["brand_name"])),
		Intent:       strings.ToUpper(strings.TrimSpace(stringValue(values["intent"]))),
		Locale:       strings.TrimSpace(stringValue(values["locale"])),
		IsProd:       strings.EqualFold(strings.TrimSpace(env), "prod"),
	}
	if cfg.ClientID == "" {
		return Config{}, ErrClientIDRequired
	}
	if cfg.ClientSecret == "" {
		return Config{}, ErrClientSecretRequired
	}
	if cfg.Intent == "" {
		cfg.Intent = DefaultIntent
	}
	if cfg.Intent != DefaultIntent {
		return Config{}, ErrIntentUnsupported
	}
	if cfg.Locale == "" {
		cfg.Locale = DefaultLocale
	}
	return cfg, nil
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return ""
	}
}
