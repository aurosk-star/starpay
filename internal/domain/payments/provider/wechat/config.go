package wechat

import (
	"errors"
	"strings"

	"payment-gateway/internal/platform/configvalue"
)

const DefaultMode = "native"

var (
	ErrAppIDRequired      = errors.New("wechat app_id is required")
	ErrMchIDRequired      = errors.New("wechat mch_id is required")
	ErrAPIV3KeyRequired   = errors.New("wechat api_v3_key is required")
	ErrSerialNoRequired   = errors.New("wechat serial_no is required")
	ErrPrivateKeyRequired = errors.New("wechat private_key is required")
)

type Config struct {
	AppID                string
	MchID                string
	APIV3Key             string
	SerialNo             string
	PrivateKey           string
	WechatPayPublicKey   string
	WechatPayPublicKeyID string
	Mode                 string
	EnableNativePay      bool
	EnableH5Pay          bool
	IsProd               bool
}

func ParseConfig(values map[string]any, env string) (Config, error) {
	cfg := Config{
		AppID:                strings.TrimSpace(stringValue(values["app_id"])),
		MchID:                strings.TrimSpace(stringValue(values["mch_id"])),
		APIV3Key:             strings.TrimSpace(stringValue(values["api_v3_key"])),
		SerialNo:             strings.TrimSpace(stringValue(values["serial_no"])),
		PrivateKey:           strings.TrimSpace(stringValue(values["private_key"])),
		WechatPayPublicKey:   strings.TrimSpace(stringValue(values["wechat_pay_public_key"])),
		WechatPayPublicKeyID: strings.TrimSpace(stringValue(values["wechat_pay_public_key_id"])),
		Mode:                 strings.ToLower(strings.TrimSpace(stringValue(values["mode"]))),
		EnableNativePay:      configvalue.BoolDefault(values["enable_native_pay"], legacyNativeDefault(values["mode"])),
		EnableH5Pay:          configvalue.BoolDefault(values["enable_h5_pay"], legacyH5Default(values["mode"])),
		IsProd:               strings.EqualFold(strings.TrimSpace(env), "prod"),
	}
	if cfg.AppID == "" {
		return Config{}, ErrAppIDRequired
	}
	if cfg.MchID == "" {
		return Config{}, ErrMchIDRequired
	}
	if cfg.APIV3Key == "" {
		return Config{}, ErrAPIV3KeyRequired
	}
	if cfg.SerialNo == "" {
		return Config{}, ErrSerialNoRequired
	}
	if cfg.PrivateKey == "" {
		return Config{}, ErrPrivateKeyRequired
	}
	if cfg.Mode == "" {
		cfg.Mode = DefaultMode
	}
	return cfg, nil
}

func legacyNativeDefault(value any) bool {
	mode := strings.ToLower(strings.TrimSpace(stringValue(value)))
	return mode == "" || mode == "native" || mode == "qr"
}

func legacyH5Default(value any) bool {
	return strings.EqualFold(strings.TrimSpace(stringValue(value)), "h5")
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
