package paygateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCreateOrderSignsRequestAndDecodesResponse(t *testing.T) {
	var gotSign string
	var gotAppID string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotSign = request.URL.Query().Get("sign")
		gotAppID = request.URL.Query().Get("app_id")
		if request.URL.Path != "/v1/open/orders" {
			t.Fatalf("path = %q, want /v1/open/orders", request.URL.Path)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", request.Header.Get("Content-Type"))
		}
		var input CreateOrderRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if input.MerchantOrderNo != "biz_001" || input.Amount != 9900 {
			t.Fatalf("request body = %#v, want order request", input)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":"ok","message":"ok","data":{"created":true,"order":{"gateway_order_no":"pay_001","merchant_order_no":"biz_001","subject":"Pro","amount":9900,"currency":"CNY","status":"pending"},"payment":{"status":"pending","pay_url":"https://pay.example.com/checkout/pay_001?token=abc"}},"error":null}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, AppID: "snsgo", AppSecret: "secret"})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	result, err := client.CreateOrder(context.Background(), CreateOrderRequest{
		MerchantOrderNo: "biz_001",
		Subject:         "Pro",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if gotAppID != "snsgo" || gotSign == "" {
		t.Fatalf("auth query app_id=%q sign=%q, want signed request", gotAppID, gotSign)
	}
	if !result.Created || result.Order.GatewayOrderNo != "pay_001" {
		t.Fatalf("result = %#v, want created order", result)
	}
	if result.Payment.PayURL != "https://pay.example.com/checkout/pay_001?token=abc" {
		t.Fatalf("payment.pay_url = %q, want checkout url", result.Payment.PayURL)
	}
}

func TestClientReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"code":"invalid_signature","message":"invalid signature","data":null,"error":{"code":"invalid_signature","message":"invalid signature"}}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, AppID: "snsgo", AppSecret: "secret"})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.GetOrder(context.Background(), "pay_001")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiErr.Code != "invalid_signature" {
		t.Fatalf("apiErr.Code = %q, want invalid_signature", apiErr.Code)
	}
}
