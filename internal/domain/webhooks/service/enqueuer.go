package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const webhookStreamName = "webhook:deliveries"

type Enqueuer interface {
	EnqueueWebhookDelivery(ctx context.Context, deliveryID int) error
}

type redisEnqueuer struct {
	client *redis.Client
}

func newRedisEnqueuer(client *redis.Client) Enqueuer {
	if client == nil {
		return nil
	}
	return redisEnqueuer{client: client}
}

func (e redisEnqueuer) EnqueueWebhookDelivery(ctx context.Context, deliveryID int) error {
	if e.client == nil {
		return errors.New("redis client is required")
	}
	payload, err := json.Marshal(map[string]any{"delivery_id": deliveryID})
	if err != nil {
		return err
	}
	return e.client.XAdd(ctx, &redis.XAddArgs{
		Stream: webhookStreamName,
		Values: map[string]any{
			"delivery_id": fmt.Sprintf("%d", deliveryID),
			"payload":     string(payload),
		},
	}).Err()
}
