package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type WebhookEvent struct {
	ent.Schema
}

func (WebhookEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("event_id").Unique(),
		field.String("event_type"),
		field.String("app_id"),
		field.String("gateway_order_no"),
		field.Int("payment_order_id").Optional(),
		field.JSON("payload", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (WebhookEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("event_type", "gateway_order_no").Unique(),
		index.Fields("event_id"),
		index.Fields("app_id"),
		index.Fields("gateway_order_no"),
		index.Fields("created_at"),
	}
}
