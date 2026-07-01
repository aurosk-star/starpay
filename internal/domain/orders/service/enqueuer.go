package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const orderExpirationStreamName = "order:expirations"

type ExpirationEnqueuer interface {
	EnqueueOrderExpiration(ctx context.Context, orderID int) error
}

type redisExpirationEnqueuer struct {
	client *redis.Client
}

func NewRedisExpirationEnqueuer(client *redis.Client) ExpirationEnqueuer {
	if client == nil {
		return nil
	}
	return redisExpirationEnqueuer{client: client}
}

func (e redisExpirationEnqueuer) EnqueueOrderExpiration(ctx context.Context, orderID int) error {
	if e.client == nil {
		return errors.New("redis client is required")
	}
	payload, err := json.Marshal(map[string]any{"order_id": orderID})
	if err != nil {
		return err
	}
	return e.client.XAdd(ctx, &redis.XAddArgs{
		Stream: orderExpirationStreamName,
		Values: map[string]any{
			"order_id": fmt.Sprintf("%d", orderID),
			"payload":  string(payload),
		},
	}).Err()
}
