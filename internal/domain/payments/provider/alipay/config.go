package alipay

import (
	"errors"
	"strings"

	"payment-gateway/internal/platform/configvalue"
)

const (
	DefaultPageProductCode = "FAST_INSTANT_TRADE_PAY"
	DefaultWapProductCode  = "QUICK_WAP_WAY"
	DefaultMode            = "page"
)

var (
	ErrAppIDRequired      = errors.New("alipay app_id is required")
	ErrPrivateKeyRequired = errors.New("alipay private_key is required")
	ErrModeUnsupported    = errors.New("alipay mode must be page, wap, or qr")
)

type Config struct {
	AppID           string
	PrivateKey      string
	AlipayPublicKey string
	ServerURL       string
	ProductCode     string
	Mode            string
	EnablePagePay   bool
	EnableWapPay    bool
	EnableQRPay     bool
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
		EnablePagePay:   configvalue.BoolDefault(values["enable_page_pay"], true),
		EnableWapPay:    configvalue.BoolDefault(values["enable_wap_pay"], true),
		EnableQRPay:     configvalue.BoolDefault(values["enable_qr_pay"], true),
		IsProd:          strings.EqualFold(strings.TrimSpace(env), "prod"),
	}
	if cfg.AppID == "" {
		return Config{}, ErrAppIDRequired
	}
	if cfg.PrivateKey == "" {
		return Config{}, ErrPrivateKeyRequired
	}
	if cfg.Mode == "" {
		cfg.Mode = DefaultMode
	}
	if cfg.Mode != "page" && cfg.Mode != "wap" && cfg.Mode != "qr" {
		return Config{}, ErrModeUnsupported
	}
	if cfg.ProductCode == "" {
		cfg.ProductCode = defaultProductCode(cfg.Mode)
	}
	return cfg, nil
}

func defaultProductCode(mode string) string {
	switch mode {
	case "wap":
		return DefaultWapProductCode
	case "qr":
		return ""
	default:
		return DefaultPageProductCode
	}
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
