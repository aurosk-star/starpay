package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type ChannelAccount struct {
	ent.Schema
}

func (ChannelAccount) Fields() []ent.Field {
	return []ent.Field{
		field.String("channel"),
		field.String("name"),
		field.Bool("enabled").Default(true),
		field.String("env").Default("sandbox"),
		field.JSON("config", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
