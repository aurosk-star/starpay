package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Refund struct{ ent.Schema }

func (Refund) Fields() []ent.Field {
	return []ent.Field{
		field.String("refund_no").Unique(),
		field.String("app_id"),
		field.Int("payment_order_id"),
		field.String("gateway_order_no"),
		field.String("merchant_order_no"),
		field.String("merchant_refund_no"),
		field.String("channel"),
		field.Int("channel_account_id"),
		field.String("provider_order_no").Optional(),
		field.String("channel_trade_no"),
		field.String("channel_refund_no").Optional(),
		field.Int64("amount"),
		field.String("currency"),
		field.String("reason").Optional(),
		field.String("status").Default("pending"),
		field.String("failure_reason").Optional(),
		field.JSON("metadata", map[string]any{}).Optional(),
		field.JSON("provider_snapshot", map[string]any{}).Optional(),
		field.Int("attempt_count").Default(0),
		field.Time("next_attempt_at").Optional().Nillable(),
		field.Time("last_attempt_at").Optional().Nillable(),
		field.String("last_error").Optional(),
		field.Time("succeeded_at").Optional().Nillable(),
		field.Time("failed_at").Optional().Nillable(),
		field.Time("closed_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Refund) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "merchant_refund_no").Unique(),
		index.Fields("app_id"),
		index.Fields("payment_order_id"),
		index.Fields("gateway_order_no"),
		index.Fields("merchant_order_no"),
		index.Fields("channel"),
		index.Fields("channel_account_id"),
		index.Fields("channel_refund_no"),
		index.Fields("status", "next_attempt_at"),
		index.Fields("created_at"),
	}
}
