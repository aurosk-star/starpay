package orderstest

import (
	"testing"

	"github.com/redis/go-redis/v9"

	ordersvc "payment-gateway/internal/domain/orders/service"
)

func TestOrderExpirationIDFromMessageReadsOrderIDField(t *testing.T) {
	id, err := ordersvc.OrderExpirationIDFromMessage(redis.XMessage{
		Values: map[string]any{"order_id": "42"},
	})
	if err != nil {
		t.Fatalf("OrderExpirationIDFromMessage() error = %v", err)
	}
	if id != 42 {
		t.Fatalf("id = %d, want 42", id)
	}
}

func TestOrderExpirationIDFromMessageReadsPayload(t *testing.T) {
	id, err := ordersvc.OrderExpirationIDFromMessage(redis.XMessage{
		Values: map[string]any{"payload": `{"order_id":43}`},
	})
	if err != nil {
		t.Fatalf("OrderExpirationIDFromMessage() error = %v", err)
	}
	if id != 43 {
		t.Fatalf("id = %d, want 43", id)
	}
}

func TestOrderExpirationIDFromMessageRejectsMissingOrderID(t *testing.T) {
	if _, err := ordersvc.OrderExpirationIDFromMessage(redis.XMessage{Values: map[string]any{}}); err == nil {
		t.Fatal("OrderExpirationIDFromMessage() error = nil, want error")
	}
}
