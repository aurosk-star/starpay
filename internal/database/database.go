package database

import (
	"context"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"

	"payment-gateway/ent"
)

func Open(ctx context.Context, databaseURL string) (*ent.Client, error) {
	drv, err := entsql.Open(dialect.Postgres, databaseURL)
	if err != nil {
		return nil, err
	}

	client := ent.NewClient(ent.Driver(drv))
	if err := client.Schema.Create(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}
