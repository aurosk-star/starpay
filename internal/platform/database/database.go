package database

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"payment-gateway/ent"
)

func Open(ctx context.Context, driverName string, dsn string) (*ent.Client, error) {
	entDialect, err := resolveDriver(driverName)
	if err != nil {
		return nil, err
	}

	drv, err := entsql.Open(entDialect, dsn)
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

func resolveDriver(driverName string) (string, error) {
	switch driverName {
	case "postgres", "postgresql":
		return dialect.Postgres, nil
	case "mysql":
		return dialect.MySQL, nil
	default:
		return "", fmt.Errorf("unsupported database driver %q", driverName)
	}
}
