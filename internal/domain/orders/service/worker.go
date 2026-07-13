package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	platformworker "payment-gateway/internal/platform/worker"
)

const orderExpirationWorkerGroup = "order-expiration-workers"

func OrderExpirationWorkerGroup() string {
	return orderExpirationWorkerGroup
}

type Worker struct {
	service  Service
	redis    *redis.Client
	consumer string
	logger   *slog.Logger
}

type ExpireScannerConfig struct {
	Interval time.Duration
	Limit    int
}

func NewWorker(service Service, redisClient *redis.Client, consumer string) Worker {
	if consumer == "" {
		consumer = "worker-1"
	}
	return Worker{
		service:  service,
		redis:    redisClient,
		consumer: consumer,
		logger:   slog.Default(),
	}
}

func (w Worker) Run(ctx context.Context) {
	if w.redis == nil {
		return
	}
	if err := platformworker.WaitUntilReady(ctx, time.Second, w.ensureGroup, func(err error) {
		w.logger.Error("ensure order expiration stream group", "error", err)
	}); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		streams, err := w.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    orderExpirationWorkerGroup,
			Consumer: w.consumer,
			Streams:  []string{orderExpirationStreamName, ">"},
			Count:    10,
			Block:    time.Second,
		}).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			w.logger.Error("read order expiration stream", "error", err)
			time.Sleep(time.Second)
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				w.handleMessage(ctx, message)
			}
		}
	}
}

func (w Worker) RunExpireScanner(ctx context.Context, interval time.Duration, limit int) {
	w.RunExpireScannerWithResolver(ctx, interval, limit, nil)
}

func (w Worker) RunExpireScannerWithResolver(ctx context.Context, interval time.Duration, limit int, resolver func(context.Context) (ExpireScannerConfig, error)) {
	if interval <= 0 {
		return
	}
	currentInterval := interval
	currentLimit := limit
	if resolver != nil {
		if resolved, err := resolver(ctx); err == nil {
			if resolved.Interval > 0 {
				currentInterval = resolved.Interval
			}
			if resolved.Limit > 0 {
				currentLimit = resolved.Limit
			}
		}
	}
	w.scanExpiredOrders(ctx, currentLimit)
	timer := time.NewTimer(currentInterval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if resolver != nil {
				if resolved, err := resolver(ctx); err == nil {
					if resolved.Interval > 0 {
						currentInterval = resolved.Interval
					}
					if resolved.Limit > 0 {
						currentLimit = resolved.Limit
					}
				} else {
					w.logger.Error("resolve order expiration scanner config", "error", err)
				}
			}
			w.scanExpiredOrders(ctx, currentLimit)
			timer.Reset(currentInterval)
		}
	}
}

func (w Worker) scanExpiredOrders(ctx context.Context, limit int) {
	enqueued, err := w.service.ScanExpiredPendingOrders(ctx, limit)
	if err != nil {
		w.logger.Error("scan expired payment orders", "error", err)
		return
	}
	if enqueued > 0 {
		w.logger.Info("enqueued expired payment orders", "count", enqueued)
	}
}

func (w Worker) ensureGroup(ctx context.Context) error {
	err := w.redis.XGroupCreateMkStream(ctx, orderExpirationStreamName, orderExpirationWorkerGroup, "0").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

func (w Worker) handleMessage(ctx context.Context, message redis.XMessage) {
	orderID, err := OrderExpirationIDFromMessage(message)
	if err != nil {
		w.logger.Error("parse order expiration message", "message_id", message.ID, "error", err)
		_ = w.redis.XAck(ctx, orderExpirationStreamName, orderExpirationWorkerGroup, message.ID).Err()
		return
	}
	if _, err := w.service.CloseExpiredPendingOrder(ctx, orderID); err != nil {
		w.logger.Error("close expired payment order", "order_id", orderID, "message_id", message.ID, "error", err)
		return
	}
	if err := w.redis.XAck(ctx, orderExpirationStreamName, orderExpirationWorkerGroup, message.ID).Err(); err != nil && !errors.Is(err, context.Canceled) {
		w.logger.Error("ack order expiration message", "order_id", orderID, "message_id", message.ID, "error", err)
	}
}
