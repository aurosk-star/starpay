package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const webhookWorkerGroup = "webhook-delivery-workers"

func WebhookWorkerGroup() string {
	return webhookWorkerGroup
}

type Worker struct {
	service  Service
	redis    *redis.Client
	consumer string
	logger   *slog.Logger
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
	if err := w.ensureGroup(ctx); err != nil && !errors.Is(err, context.Canceled) {
		w.logger.Error("ensure webhook stream group", "error", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		streams, err := w.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    webhookWorkerGroup,
			Consumer: w.consumer,
			Streams:  []string{webhookStreamName, ">"},
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
			w.logger.Error("read webhook delivery stream", "error", err)
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

func (w Worker) RunRetryScanner(ctx context.Context, interval time.Duration, limit int) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if limit < 1 {
		limit = 100
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := w.service.ScanDueDeliveries(ctx, limit); err != nil && !errors.Is(err, context.Canceled) {
				w.logger.Error("scan due webhook deliveries", "error", err)
			}
		}
	}
}

func (w Worker) ensureGroup(ctx context.Context) error {
	err := w.redis.XGroupCreateMkStream(ctx, webhookStreamName, webhookWorkerGroup, "0").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

func (w Worker) handleMessage(ctx context.Context, message redis.XMessage) {
	deliveryID, err := deliveryIDFromMessage(message)
	if err != nil {
		w.logger.Error("parse webhook delivery message", "message_id", message.ID, "error", err)
		_ = w.redis.XAck(ctx, webhookStreamName, webhookWorkerGroup, message.ID).Err()
		return
	}
	if _, err := w.service.DeliverWebhook(ctx, deliveryID); err != nil {
		w.logger.Error("deliver webhook", "delivery_id", deliveryID, "message_id", message.ID, "error", err)
		return
	}
	if err := w.redis.XAck(ctx, webhookStreamName, webhookWorkerGroup, message.ID).Err(); err != nil && !errors.Is(err, context.Canceled) {
		w.logger.Error("ack webhook delivery message", "delivery_id", deliveryID, "message_id", message.ID, "error", err)
	}
}

func deliveryIDFromMessage(message redis.XMessage) (int, error) {
	if value, ok := message.Values["delivery_id"]; ok {
		return strconv.Atoi(toString(value))
	}
	if value, ok := message.Values["payload"]; ok {
		var payload struct {
			DeliveryID int `json:"delivery_id"`
		}
		if err := json.Unmarshal([]byte(toString(value)), &payload); err != nil {
			return 0, err
		}
		if payload.DeliveryID > 0 {
			return payload.DeliveryID, nil
		}
	}
	return 0, errors.New("delivery_id is required")
}

func toString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
