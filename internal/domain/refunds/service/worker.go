package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	platformworker "payment-gateway/internal/platform/worker"
)

const refundWorkerGroup = "refund-processing-workers"

func RefundWorkerGroup() string { return refundWorkerGroup }

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
		w.logger.Error("ensure refund stream group", "error", err)
	}); err != nil {
		return
	}
	for {
		streams, err := w.redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: refundWorkerGroup, Consumer: w.consumer, Streams: []string{refundStreamName, ">"}, Count: 10, Block: time.Second}).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
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
	if _, err := w.service.ScanDue(ctx, limit); err != nil {
		w.logger.Error("scan refunds", "error", err)
	}
}
func (w Worker) ensureGroup(ctx context.Context) error {
	err := w.redis.XGroupCreateMkStream(ctx, refundStreamName, refundWorkerGroup, "0").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}
func (w Worker) handle(ctx context.Context, message redis.XMessage) {
	id, err := RefundIDFromMessage(message)
	if err != nil {
		_ = w.redis.XAck(ctx, refundStreamName, refundWorkerGroup, message.ID).Err()
		return
	}
	if _, err := w.service.Process(ctx, id); err != nil {
		return
	}
	_ = w.redis.XAck(ctx, refundStreamName, refundWorkerGroup, message.ID).Err()
}
func RefundIDFromMessage(message redis.XMessage) (int, error) {
	raw, ok := message.Values["refund_id"]
	if !ok {
		return 0, errors.New("refund_id is required")
	}
	value, ok := raw.(string)
	if !ok {
		return 0, errors.New("invalid refund_id")
	}
	id, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || id <= 0 {
		return 0, errors.New("invalid refund_id")
	}
	return id, nil
}
