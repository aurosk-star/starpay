package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PaymentReconciliation struct {
	ent.Schema
}

func (PaymentReconciliation) Fields() []ent.Field {
	return []ent.Field{
		field.Int("payment_order_id").Unique(),
		field.String("gateway_order_no"),
		field.String("channel"),
		field.Int("channel_account_id"),
		field.String("status").Default("pending"),
		field.Int("attempt_count").Default(0),
		field.Time("next_attempt_at").Optional().Nillable(),
		field.Time("last_attempt_at").Optional().Nillable(),
		field.String("last_provider_status").Optional(),
		field.String("last_error").Optional(),
		field.JSON("provider_snapshot", map[string]any{}).Optional(),
		field.Time("resolved_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (PaymentReconciliation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("gateway_order_no"),
		index.Fields("status", "next_attempt_at"),
		index.Fields("channel"),
		index.Fields("channel_account_id"),
		index.Fields("updated_at"),
	}
}
