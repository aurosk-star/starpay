package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type RefreshToken struct {
	ent.Schema
}

func (RefreshToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("token_hash").Unique(),
		field.Time("expires_at"),
		field.Time("revoked_at").Optional().Nillable(),
		field.String("replaced_by_hash").Optional().Nillable(),
		field.String("user_agent").Optional(),
		field.String("ip_address").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (RefreshToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("refresh_tokens").Unique().Required(),
	}
}
