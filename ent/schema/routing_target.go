package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RoutingTarget struct {
	ent.Schema
}

func (RoutingTarget) Fields() []ent.Field {
	return []ent.Field{
		field.Int("routing_rule_id"),
		field.Int("channel_account_id"),
		field.Bool("enabled").Default(true),
		field.Int("priority").Default(100),
		field.Int("weight").Default(100),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RoutingTarget) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("routing_rule_id"),
		index.Fields("channel_account_id"),
		index.Fields("enabled"),
		index.Fields("priority"),
	}
}
