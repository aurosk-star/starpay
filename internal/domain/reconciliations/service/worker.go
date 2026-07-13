package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	platformworker "payment-gateway/internal/platform/worker"
)

const reconciliationWorkerGroup = "payment-reconciliation-workers"

func ReconciliationWorkerGroup() string { return reconciliationWorkerGroup }

type Worker struct {
	service  Service
	redis    *redis.Client
	consumer string
	logger   *slog.Logger
}

func NewWorker(service Service, client *redis.Client, consumer string) Worker {
	if consumer == "" {
		consumer = "worker-1"
	}
	return Worker{service: service, redis: client, consumer: consumer, logger: slog.Default()}
}

func (w Worker) Run(ctx context.Context) {
	if w.redis == nil {
		return
	}
	if err := platformworker.WaitUntilReady(ctx, time.Second, w.ensureGroup, func(err error) {
		w.logger.Error("ensure payment reconciliation stream group", "error", err)
	}); err != nil {
		return
	}
	for {
		streams, err := w.redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: reconciliationWorkerGroup, Consumer: w.consumer, Streams: []string{reconciliationStreamName, ">"}, Count: 10, Block: time.Second}).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			w.logger.Error("read payment reconciliation stream", "error", err)
			time.Sleep(time.Second)
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				w.handle(ctx, message)
			}
		}
	}
}

func (w Worker) RunScanner(ctx context.Context, interval time.Duration, limit int) {
	if interval <= 0 {
		return
	}
	w.scan(ctx, limit)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.scan(ctx, limit)
		}
	}
}

func (w Worker) scan(ctx context.Context, limit int) {
	if count, err := w.service.ScanDue(ctx, limit); err != nil {
		w.logger.Error("scan payment reconciliations", "error", err)
	} else if count > 0 {
		w.logger.Info("enqueued payment reconciliations", "count", count)
	}
}

func (w Worker) ensureGroup(ctx context.Context) error {
	err := w.redis.XGroupCreateMkStream(ctx, reconciliationStreamName, reconciliationWorkerGroup, "0").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

func (w Worker) handle(ctx context.Context, message redis.XMessage) {
	id, err := ReconciliationIDFromMessage(message)
	if err != nil {
		_ = w.redis.XAck(ctx, reconciliationStreamName, reconciliationWorkerGroup, message.ID).Err()
		return
	}
	if _, err := w.service.Process(ctx, id); err != nil {
		w.logger.Error("process payment reconciliation", "id", id, "error", err)
		return
	}
	if err := w.redis.XAck(ctx, reconciliationStreamName, reconciliationWorkerGroup, message.ID).Err(); err != nil && !errors.Is(err, context.Canceled) {
		w.logger.Error("ack payment reconciliation", "id", id, "error", err)
	}
}

func ReconciliationIDFromMessage(message redis.XMessage) (int, error) {
	raw, ok := message.Values["reconciliation_id"]
	if !ok {
		return 0, fmt.Errorf("reconciliation_id is required")
	}
	id, err := strconv.Atoi(strings.TrimSpace(toString(raw)))
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid reconciliation_id")
	}
	return id, nil
}

func toString(value any) string {
	switch current := value.(type) {
	case string:
		return current
	case []byte:
		return string(current)
	default:
		return ""
	}
}
