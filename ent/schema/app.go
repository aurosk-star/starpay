package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type App struct {
	ent.Schema
}

func (App) Fields() []ent.Field {
	return []ent.Field{
		field.String("app_id").Unique(),
		field.String("name"),
		field.String("app_secret_hash"),
		field.String("notify_url").Optional(),
		field.String("status").Default("enabled"),
	}
}
