package paygateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVerifyWebhookSignatureAcceptsValidSignature(t *testing.T) {
	body := []byte(`{"event_type":"payment.succeeded","app_id":"snsgo"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := webhookSignature("secret", timestamp, body)

	if err := VerifyWebhookSignature("secret", timestamp, body, signature); err != nil {
		t.Fatalf("VerifyWebhookSignature() error = %v", err)
	}
}

func TestVerifyWebhookSignatureRejectsInvalidSignature(t *testing.T) {
	body := []byte(`{"event_type":"payment.succeeded"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	if err := VerifyWebhookSignature("secret", timestamp, body, "bad"); err == nil {
		t.Fatal("VerifyWebhookSignature() error = nil, want error")
	}
}

func TestVerifyWebhookSignatureRejectsExpiredTimestamp(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	timestamp := strconv.FormatInt(now.Add(-6*time.Minute).Unix(), 10)
	body := []byte(`{"event_type":"payment.succeeded"}`)
	signature := webhookSignature("secret", timestamp, body)

	err := VerifyWebhookSignatureWithOptions("secret", timestamp, body, signature, WebhookVerificationOptions{
		Now: func() time.Time { return now },
	})
	if err == nil {
		t.Fatal("VerifyWebhookSignatureWithOptions() error = nil, want expired timestamp error")
	}
}

func TestParseWebhookRequestVerifiesAndParsesEvent(t *testing.T) {
	body := `{"event_type":"order.expired","app_id":"snsgo","gateway_order_no":"pay_001","merchant_order_no":"biz_001","amount":9900,"currency":"CNY","status":"closed"}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	request := httptest.NewRequest("POST", "/webhook", strings.NewReader(body))
	request.Header.Set(WebhookEventIDHeader, "evt_001")
	request.Header.Set(WebhookTimestampHeader, timestamp)
	request.Header.Set(WebhookDeliveryNoHeader, "whd_001")
	request.Header.Set(WebhookSignatureHeader, webhookSignature("secret", timestamp, []byte(body)))

	event, err := ParseWebhookRequest(request, "secret")
	if err != nil {
		t.Fatalf("ParseWebhookRequest() error = %v", err)
	}
	if event.EventID != "evt_001" || event.EventType != "order.expired" || event.GatewayOrderNo != "pay_001" {
		t.Fatalf("event = %#v, want parsed order.expired event", event)
	}
	if event.DeliveryNo != "whd_001" || event.Timestamp != mustParseInt64(t, timestamp) {
		t.Fatalf("event delivery metadata = %#v, want delivery number and timestamp", event)
	}
}

func TestParsePaymentFailedWebhookEvent(t *testing.T) {
	event, err := ParseWebhookEvent([]byte(`{"event_type":"payment.failed","gateway_order_no":"pay_001","failure_reason":"PAYERROR","failed_at":"2026-07-13T10:00:00Z"}`))
	if err != nil {
		t.Fatalf("ParseWebhookEvent() error = %v", err)
	}
	if event.EventType != "payment.failed" || event.FailureReason != "PAYERROR" || event.FailedAt == "" {
		t.Fatalf("event = %#v, want payment.failed fields", event)
	}
}

func TestParseRefundWebhookResourceFields(t *testing.T) {
	event, err := ParseWebhookEvent([]byte(`{"event_type":"refund.succeeded","resource_type":"refund","resource_id":"rf_1","refund_no":"rf_1","merchant_refund_no":"mrf_1","channel_refund_no":"provider_rf_1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if event.ResourceType != "refund" || event.ResourceID != "rf_1" || event.RefundNo != "rf_1" || event.ChannelRefundNo != "provider_rf_1" {
		t.Fatalf("event = %#v", event)
	}
}

func mustParseInt64(t *testing.T, value string) int64 {
	t.Helper()
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		t.Fatalf("ParseInt(%q) error = %v", value, err)
	}
	return parsed
}

func webhookSignature(secret string, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
