package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type GatewayConfig struct {
	ent.Schema
}

func (GatewayConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("site_name").Default("starpay-支付网关"),
		field.String("gateway_base_url").Default("http://localhost:8080"),
		field.String("payment_notify_path").Default("/v1/channel/notify"),
		field.String("default_currency").Default("CNY"),
		field.String("default_locale").Default("zh-CN"),
		field.Bool("request_id_enabled").Default(true),
		field.Bool("maintenance_mode").Default(false),
		field.Int("order_default_ttl_seconds").Default(900),
		field.Int("order_expire_scan_interval_seconds").Default(30),
		field.Int("order_expire_scan_limit").Default(100),
		field.Int("order_expire_worker_concurrency").Default(2),
		field.Bool("open_api_rate_limit_enabled").Default(true),
		field.Int("open_api_rate_limit").Default(120),
		field.Int("open_api_rate_limit_window_seconds").Default(60),
		field.JSON("extra", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
