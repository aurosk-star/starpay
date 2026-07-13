package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type WebhookDelivery struct {
	ent.Schema
}

func (WebhookDelivery) Fields() []ent.Field {
	return []ent.Field{
		field.String("delivery_no").Unique(),
		field.Int("event_id"),
		field.String("app_id"),
		field.String("event_type"),
		field.String("resource_type").Default("payment_order"),
		field.String("resource_id"),
		field.String("gateway_order_no"),
		field.String("refund_no").Optional(),
		field.String("target_url"),
		field.String("status").Default("pending"),
		field.Int("attempt_count").Default(0),
		field.Time("next_attempt_at").Optional().Nillable(),
		field.Time("last_attempt_at").Optional().Nillable(),
		field.Int("last_status_code").Optional(),
		field.String("last_response_body").Optional(),
		field.String("last_error").Optional(),
		field.Time("succeeded_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (WebhookDelivery) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("delivery_no"),
		index.Fields("event_id").Unique(),
		index.Fields("app_id"),
		index.Fields("resource_type"),
		index.Fields("resource_id"),
		index.Fields("refund_no"),
		index.Fields("status"),
		index.Fields("next_attempt_at"),
		index.Fields("created_at"),
	}
}
