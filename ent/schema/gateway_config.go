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
		field.String("gateway_base_url").Default("http://localhost:8080"),
		field.String("payment_notify_path").Default("/v1/channel/notify"),
		field.String("default_currency").Default("CNY"),
		field.String("default_locale").Default("zh-CN"),
		field.Bool("request_id_enabled").Default(true),
		field.Bool("maintenance_mode").Default(false),
		field.JSON("extra", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
