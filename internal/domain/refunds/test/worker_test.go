package refundstest

import (
	"testing"

	"github.com/redis/go-redis/v9"

	refundsvc "payment-gateway/internal/domain/refunds/service"
)

func TestRefundIDFromMessage(t *testing.T) {
	id, err := refundsvc.RefundIDFromMessage(redis.XMessage{Values: map[string]any{"refund_id": "42"}})
	if err != nil || id != 42 {
		t.Fatalf("id=%d err=%v, want 42", id, err)
	}
}

func TestRefundIDFromMessageRejectsMissingID(t *testing.T) {
	if _, err := refundsvc.RefundIDFromMessage(redis.XMessage{Values: map[string]any{}}); err == nil {
		t.Fatal("error=nil, want missing id error")
	}
}
