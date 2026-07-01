package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"

	monitorsvc "payment-gateway/internal/domain/monitoring/service"
)

func TestOverviewReportsDatabaseRedisAndQueueMetrics(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", "file:monitoring_overview?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	svc := monitorsvc.New(
		monitorsvc.WithDB(db),
		monitorsvc.WithRedis(redisClient),
		monitorsvc.WithStreams([]monitorsvc.StreamTarget{
			{Name: "orders", Stream: "order:expirations", Group: "order-expiration-workers"},
			{Name: "webhooks", Stream: "webhook:deliveries", Group: "webhook-delivery-workers"},
		}),
		monitorsvc.WithClock(func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) }),
	)

	overview := svc.Overview(ctx)

	if overview.Database.Status != "ok" {
		t.Fatalf("database status = %q, want ok", overview.Database.Status)
	}
	if overview.Redis.Status != "degraded" {
		t.Fatalf("redis status = %q, want degraded for unavailable redis", overview.Redis.Status)
	}
	if overview.Runtime.Status != "ok" || overview.Runtime.CheckedAt.IsZero() {
		t.Fatalf("runtime = %#v, want ok with checked_at", overview.Runtime)
	}
	if len(overview.Queues) != 2 {
		t.Fatalf("queues len = %d, want 2", len(overview.Queues))
	}
	if overview.Queues[0].Name != "orders" || overview.Queues[0].Status != "degraded" {
		t.Fatalf("orders queue = %#v, want degraded orders queue", overview.Queues[0])
	}
}

func TestNewUsesEntSQLDriverDatabase(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:monitoring_ent_driver?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	driver := entsql.OpenDB("sqlite3", db)
	svc := monitorsvc.New(monitorsvc.WithEntDriver(driver))
	overview := svc.Overview(context.Background())

	if overview.Database.Status != "ok" {
		t.Fatalf("database status = %q error=%q, want ok", overview.Database.Status, overview.Database.Error)
	}
}
