package database

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"
)

const legacyWebhookEventIndex = "webhookevent_event_type_gateway_order_no"

func prepareWebhookResourceMigration(ctx context.Context, db *sql.DB, driver string) error {
	if db == nil {
		return nil
	}
	switch driver {
	case dialect.Postgres:
		return preparePostgresWebhookResourceMigration(ctx, db)
	case dialect.MySQL:
		return prepareMySQLWebhookResourceMigration(ctx, db)
	default:
		return nil
	}
}

func preparePostgresWebhookResourceMigration(ctx context.Context, db *sql.DB) error {
	exists, err := postgresTableExists(ctx, db, "webhook_events")
	if err != nil || !exists {
		return err
	}
	statements := []string{
		`ALTER TABLE webhook_events ADD COLUMN IF NOT EXISTS resource_type character varying NOT NULL DEFAULT 'payment_order'`,
		`ALTER TABLE webhook_events ADD COLUMN IF NOT EXISTS resource_id character varying`,
		`ALTER TABLE webhook_events ADD COLUMN IF NOT EXISTS refund_no character varying`,
		`UPDATE webhook_events SET resource_type = 'payment_order' WHERE resource_type IS NULL OR resource_type = ''`,
		`UPDATE webhook_events SET resource_id = gateway_order_no WHERE resource_id IS NULL OR resource_id = ''`,
		`ALTER TABLE webhook_events ALTER COLUMN resource_id SET NOT NULL`,
		`DROP INDEX IF EXISTS ` + legacyWebhookEventIndex,
	}
	if err := execMigrationStatements(ctx, db, statements); err != nil {
		return err
	}
	deliveriesExist, err := postgresTableExists(ctx, db, "webhook_deliveries")
	if err != nil || !deliveriesExist {
		return err
	}
	return execMigrationStatements(ctx, db, []string{
		`ALTER TABLE webhook_deliveries ADD COLUMN IF NOT EXISTS resource_type character varying NOT NULL DEFAULT 'payment_order'`,
		`ALTER TABLE webhook_deliveries ADD COLUMN IF NOT EXISTS resource_id character varying`,
		`ALTER TABLE webhook_deliveries ADD COLUMN IF NOT EXISTS refund_no character varying`,
		`UPDATE webhook_deliveries SET resource_type = 'payment_order' WHERE resource_type IS NULL OR resource_type = ''`,
		`UPDATE webhook_deliveries SET resource_id = gateway_order_no WHERE resource_id IS NULL OR resource_id = ''`,
		`ALTER TABLE webhook_deliveries ALTER COLUMN resource_id SET NOT NULL`,
	})
}

func prepareMySQLWebhookResourceMigration(ctx context.Context, db *sql.DB) error {
	exists, err := mysqlTableExists(ctx, db, "webhook_events")
	if err != nil || !exists {
		return err
	}
	if err := ensureMySQLWebhookColumns(ctx, db, "webhook_events"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE webhook_events SET resource_type = 'payment_order' WHERE resource_type IS NULL OR resource_type = ''`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE webhook_events SET resource_id = gateway_order_no WHERE resource_id IS NULL OR resource_id = ''`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE webhook_events MODIFY COLUMN resource_id varchar(255) NOT NULL`); err != nil {
		return err
	}
	indexExists, err := mysqlIndexExists(ctx, db, "webhook_events", legacyWebhookEventIndex)
	if err != nil {
		return err
	}
	if indexExists {
		if _, err := db.ExecContext(ctx, `ALTER TABLE webhook_events DROP INDEX `+legacyWebhookEventIndex); err != nil {
			return err
		}
	}
	deliveriesExist, err := mysqlTableExists(ctx, db, "webhook_deliveries")
	if err != nil || !deliveriesExist {
		return err
	}
	if err := ensureMySQLWebhookColumns(ctx, db, "webhook_deliveries"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE webhook_deliveries SET resource_type = 'payment_order' WHERE resource_type IS NULL OR resource_type = ''`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE webhook_deliveries SET resource_id = gateway_order_no WHERE resource_id IS NULL OR resource_id = ''`); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `ALTER TABLE webhook_deliveries MODIFY COLUMN resource_id varchar(255) NOT NULL`)
	return err
}

func ensureMySQLWebhookColumns(ctx context.Context, db *sql.DB, table string) error {
	columns := []struct {
		name       string
		definition string
	}{
		{name: "resource_type", definition: `varchar(255) NOT NULL DEFAULT 'payment_order'`},
		{name: "resource_id", definition: `varchar(255) NULL`},
		{name: "refund_no", definition: `varchar(255) NULL`},
	}
	for _, column := range columns {
		exists, err := mysqlColumnExists(ctx, db, table, column.name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		statement := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column.name, column.definition)
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func postgresTableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = $1)`, table).Scan(&exists)
	return exists, err
}

func mysqlTableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&count)
	return count > 0, err
}

func mysqlColumnExists(ctx context.Context, db *sql.DB, table string, column string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, column).Scan(&count)
	return count > 0, err
}

func mysqlIndexExists(ctx context.Context, db *sql.DB, table string, index string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`, table, index).Scan(&count)
	return count > 0, err
}

func execMigrationStatements(ctx context.Context, db *sql.DB, statements []string) error {
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
