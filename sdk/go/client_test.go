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
		_, _ = writer.Write([]byte(`{"code":"INVALID_SIGNATURE","message":"invalid signature","data":null,"error":{"code":"INVALID_SIGNATURE","message":"invalid signature","details":{"field":"sign","retryable":false}}}`))
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
	if apiErr.Code != CodeInvalidSignature {
		t.Fatalf("apiErr.Code = %q, want %s", apiErr.Code, CodeInvalidSignature)
	}
	if apiErr.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusUnauthorized)
	}
	if apiErr.Details["field"] != "sign" || apiErr.Details["retryable"] != false {
		t.Fatalf("apiErr.Details = %#v, want structured error details", apiErr.Details)
	}
}

func TestClientAcceptsUppercaseOKCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":"OK","message":"ok","data":{"order":{"gateway_order_no":"pay_001","merchant_order_no":"biz_001","subject":"Pro","amount":9900,"currency":"CNY","status":"pending"}},"error":null}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, AppID: "snsgo", AppSecret: "secret"})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	order, err := client.GetOrder(context.Background(), "pay_001")
	if err != nil {
		t.Fatalf("GetOrder() error = %v", err)
	}
	if order.GatewayOrderNo != "pay_001" {
		t.Fatalf("order.GatewayOrderNo = %q, want pay_001", order.GatewayOrderNo)
	}
}

func TestClientReturnsStructuredErrorForNonJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("upstream unavailable"))
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
	if apiErr.Code != CodeInvalidResponse || apiErr.HTTPStatus != http.StatusBadGateway {
		t.Fatalf("apiErr = %#v, want invalid response with HTTP 502", apiErr)
	}
	if apiErr.ResponseBody != "upstream unavailable" {
		t.Fatalf("apiErr.ResponseBody = %q, want upstream response", apiErr.ResponseBody)
	}
}

func TestClientCreateRefundUsesOpenRefundPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/open/refunds" || request.Method != http.MethodPost {
			t.Fatalf("request = %s %s, want POST /v1/open/refunds", request.Method, request.URL.Path)
		}
		var input CreateRefundRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.Amount != 1234 || input.MerchantRefundNo != "mrf_1" {
			t.Fatalf("input = %#v", input)
		}
		_, _ = writer.Write([]byte(`{"code":"ok","message":"ok","data":{"created":true,"refund":{"refund_no":"rf_1","merchant_refund_no":"mrf_1","gateway_order_no":"gw_1","amount":1234,"currency":"CNY","status":"pending"}},"error":null}`))
	}))
	defer server.Close()
	client, _ := NewClient(Config{BaseURL: server.URL, AppID: "snsgo", AppSecret: "secret"})
	result, err := client.CreateRefund(context.Background(), CreateRefundRequest{GatewayOrderNo: "gw_1", MerchantRefundNo: "mrf_1", Amount: 1234, Currency: "CNY"})
	if err != nil || !result.Created || result.Refund.RefundNo != "rf_1" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
