package schema

import (
	"time"

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
		field.String("app_secret_ciphertext").Optional(),
		field.String("notify_url").Optional(),
		field.JSON("allowed_ips", []string{}).Optional(),
		field.String("status").Default("enabled"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
