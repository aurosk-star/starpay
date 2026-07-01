package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PaymentOrder struct {
	ent.Schema
}

func (PaymentOrder) Fields() []ent.Field {
	return []ent.Field{
		field.String("gateway_order_no").Unique(),
		field.String("app_id"),
		field.String("merchant_order_no"),
		field.String("business_type").Optional(),
		field.String("subject"),
		field.String("description").Optional(),
		field.Int64("amount"),
		field.String("currency"),
		field.Int64("settlement_amount").Optional(),
		field.String("settlement_currency").Optional(),
		field.String("channel").Optional(),
		field.String("pay_method").Optional(),
		field.String("channel_trade_no").Optional(),
		field.String("return_url").Optional(),
		field.String("status").Default("pending"),
		field.Time("expires_at").Optional().Nillable(),
		field.Time("paid_at").Optional().Nillable(),
		field.Time("closed_at").Optional().Nillable(),
		field.JSON("metadata", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (PaymentOrder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "merchant_order_no").Unique(),
		index.Fields("gateway_order_no"),
		index.Fields("app_id"),
		index.Fields("status"),
		index.Fields("channel"),
		index.Fields("currency"),
		index.Fields("created_at"),
	}
}
