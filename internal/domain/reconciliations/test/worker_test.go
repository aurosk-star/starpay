package reconciliationstest

import (
	"testing"

	"github.com/redis/go-redis/v9"

	reconciliationsvc "payment-gateway/internal/domain/reconciliations/service"
)

func TestReconciliationIDFromMessage(t *testing.T) {
	id, err := reconciliationsvc.ReconciliationIDFromMessage(redis.XMessage{Values: map[string]any{"reconciliation_id": "42"}})
	if err != nil || id != 42 {
		t.Fatalf("id=%d err=%v, want 42", id, err)
	}
}

func TestReconciliationIDFromMessageRejectsMissingID(t *testing.T) {
	if _, err := reconciliationsvc.ReconciliationIDFromMessage(redis.XMessage{Values: map[string]any{}}); err == nil {
		t.Fatal("error = nil, want missing id error")
	}
}
