package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type CasbinRule struct {
	ent.Schema
}

func (CasbinRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("ptype"),
		field.String("v0").Optional().Nillable(),
		field.String("v1").Optional().Nillable(),
		field.String("v2").Optional().Nillable(),
		field.String("v3").Optional().Nillable(),
		field.String("v4").Optional().Nillable(),
		field.String("v5").Optional().Nillable(),
	}
}

func (CasbinRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ptype", "v0", "v1", "v2").Unique(),
	}
}
