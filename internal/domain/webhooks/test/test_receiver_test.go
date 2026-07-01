package webhookstest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	webhookhandler "payment-gateway/internal/domain/webhooks/handler"
)

func TestWebhookTestReceiverStoresRequests(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	receiver := webhookhandler.NewTestReceiver()
	router := gin.New()
	router.POST("/v1/test/webhook/ping", receiver.Ping)
	router.GET("/v1/test/webhook/requests", receiver.List)

	req := httptest.NewRequest(http.MethodPost, "/v1/test/webhook/ping", bytes.NewBufferString(`{"event_type":"payment.succeeded"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pay-Gateway-Event-Id", "evt_test")
	req.Header.Set("X-Pay-Gateway-Signature", "sig_test")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d body = %s", rec.Code, rec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/test/webhook/requests", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d body = %s", listRec.Code, listRec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	data := body["data"].(map[string]any)
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	item := items[0].(map[string]any)
	if item["method"] != "POST" || item["path"] != "/v1/test/webhook/ping" || item["body"] != `{"event_type":"payment.succeeded"}` {
		t.Fatalf("stored item = %#v, want saved request", item)
	}
	headers := item["headers"].(map[string]any)
	if headers["X-Pay-Gateway-Event-Id"].([]any)[0] != "evt_test" {
		t.Fatalf("headers = %#v, want event id header", headers)
	}
}
