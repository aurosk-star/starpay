package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RoutingRule struct {
	ent.Schema
}

func (RoutingRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.Bool("enabled").Default(true),
		field.Int("priority").Default(100),
		field.String("app_scope").Default("all"),
		field.JSON("app_ids", []string{}).Optional(),
		field.String("payment_method"),
		field.JSON("pay_modes", []string{}).Optional(),
		field.String("currency").Optional(),
		field.Int64("min_amount").Default(0),
		field.Int64("max_amount").Default(0),
		field.String("terminal").Default("any"),
		field.JSON("metadata", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RoutingRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled"),
		index.Fields("priority"),
		index.Fields("payment_method"),
		index.Fields("currency"),
		index.Fields("terminal"),
		index.Fields("created_at"),
	}
}
