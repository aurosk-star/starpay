package service

import (
	"context"
	"errors"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const reconciliationStreamName = "payment:reconciliations"

func ReconciliationStreamName() string { return reconciliationStreamName }

type Enqueuer interface {
	EnqueuePaymentReconciliation(context.Context, int) error
}

type redisEnqueuer struct{ client *redis.Client }

func NewRedisEnqueuer(client *redis.Client) Enqueuer {
	if client == nil {
		return nil
	}
	return redisEnqueuer{client: client}
}

func (e redisEnqueuer) EnqueuePaymentReconciliation(ctx context.Context, id int) error {
	if e.client == nil {
		return errors.New("redis client is required")
	}
	return e.client.XAdd(ctx, &redis.XAddArgs{Stream: reconciliationStreamName, Values: map[string]any{"reconciliation_id": strconv.Itoa(id)}}).Err()
}
