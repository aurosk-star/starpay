package paygateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifyWebhookSignatureAcceptsValidSignature(t *testing.T) {
	body := []byte(`{"event_type":"payment.succeeded","app_id":"snsgo"}`)
	signature := webhookSignature("secret", "1782921600", body)

	if err := VerifyWebhookSignature("secret", "1782921600", body, signature); err != nil {
		t.Fatalf("VerifyWebhookSignature() error = %v", err)
	}
}

func TestVerifyWebhookSignatureRejectsInvalidSignature(t *testing.T) {
	body := []byte(`{"event_type":"payment.succeeded"}`)
	if err := VerifyWebhookSignature("secret", "1782921600", body, "bad"); err == nil {
		t.Fatal("VerifyWebhookSignature() error = nil, want error")
	}
}

func TestParseWebhookRequestVerifiesAndParsesEvent(t *testing.T) {
	body := `{"event_type":"order.expired","app_id":"snsgo","gateway_order_no":"pay_001","merchant_order_no":"biz_001","amount":9900,"currency":"CNY","status":"closed"}`
	request := httptest.NewRequest("POST", "/webhook", strings.NewReader(body))
	request.Header.Set(WebhookEventIDHeader, "evt_001")
	request.Header.Set(WebhookTimestampHeader, "1782921600")
	request.Header.Set(WebhookSignatureHeader, webhookSignature("secret", "1782921600", []byte(body)))

	event, err := ParseWebhookRequest(request, "secret")
	if err != nil {
		t.Fatalf("ParseWebhookRequest() error = %v", err)
	}
	if event.EventID != "evt_001" || event.EventType != "order.expired" || event.GatewayOrderNo != "pay_001" {
		t.Fatalf("event = %#v, want parsed order.expired event", event)
	}
}

func webhookSignature(secret string, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
