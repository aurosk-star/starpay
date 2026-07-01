package alipay

import (
	"errors"
	"strings"
)

const (
	DefaultProductCode = "FAST_INSTANT_TRADE_PAY"
	DefaultMode        = "page"
)

var (
	ErrAppIDRequired      = errors.New("alipay app_id is required")
	ErrPrivateKeyRequired = errors.New("alipay private_key is required")
)

type Config struct {
	AppID           string
	PrivateKey      string
	AlipayPublicKey string
	ServerURL       string
	ProductCode     string
	Mode            string
	IsProd          bool
}

func ParseConfig(values map[string]any, env string) (Config, error) {
	cfg := Config{
		AppID:           strings.TrimSpace(stringValue(values["app_id"])),
		PrivateKey:      strings.TrimSpace(stringValue(values["private_key"])),
		AlipayPublicKey: strings.TrimSpace(stringValue(values["alipay_public_key"])),
		ServerURL:       strings.TrimSpace(stringValue(values["server_url"])),
		ProductCode:     strings.TrimSpace(stringValue(values["product_code"])),
		Mode:            strings.ToLower(strings.TrimSpace(stringValue(values["mode"]))),
		IsProd:          strings.EqualFold(strings.TrimSpace(env), "prod"),
	}
	if cfg.AppID == "" {
		return Config{}, ErrAppIDRequired
	}
	if cfg.PrivateKey == "" {
		return Config{}, ErrPrivateKeyRequired
	}
	if cfg.ProductCode == "" {
		cfg.ProductCode = DefaultProductCode
	}
	if cfg.Mode == "" {
		cfg.Mode = DefaultMode
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
