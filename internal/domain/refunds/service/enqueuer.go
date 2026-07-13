package service

import (
	"context"
	"errors"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const refundStreamName = "refund:processing"

func RefundStreamName() string { return refundStreamName }

type Enqueuer interface {
	EnqueueRefund(context.Context, int) error
}
type redisEnqueuer struct{ client *redis.Client }

func NewRedisEnqueuer(client *redis.Client) Enqueuer {
	if client == nil {
		return nil
	}
	return redisEnqueuer{client: client}
}
func (e redisEnqueuer) EnqueueRefund(ctx context.Context, id int) error {
	if e.client == nil {
		return errors.New("redis client is required")
	}
	return e.client.XAdd(ctx, &redis.XAddArgs{Stream: refundStreamName, Values: map[string]any{"refund_id": strconv.Itoa(id)}}).Err()
}
