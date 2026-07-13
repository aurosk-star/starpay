package paypal

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/go-pay/gopay"
	gopaypaypal "github.com/go-pay/gopay/paypal"

	"payment-gateway/ent"
	"payment-gateway/internal/domain/payments/provider"
)

type fakeClient struct {
	body            gopay.BodyMap
	verifyBM        gopay.BodyMap
	rsp             *gopaypaypal.CreateOrderRsp
	captureRsp      *gopaypaypal.OrderCaptureRsp
	verifyRsp       *gopaypaypal.VerifyWebhookResponse
	detailRsp       *gopaypaypal.OrderDetailRsp
	refundRsp       *gopaypaypal.PaymentCaptureRefundRsp
	refundDetailRsp *gopaypaypal.PaymentRefundDetailRsp
	requestHeaders  map[string]string
}

func (c *fakeClient) CreateOrder(ctx context.Context, body gopay.BodyMap) (*gopaypaypal.CreateOrderRsp, error) {
	_ = ctx
	c.body = body
	return c.rsp, nil
}

func (c *fakeClient) OrderCapture(ctx context.Context, orderID string, body gopay.BodyMap) (*gopaypaypal.OrderCaptureRsp, error) {
	_ = ctx
	_ = orderID
	_ = body
	return c.captureRsp, nil
}

func (c *fakeClient) VerifyWebhookSignature(ctx context.Context, body gopay.BodyMap) (*gopaypaypal.VerifyWebhookResponse, error) {
	_ = ctx
	c.verifyBM = body
	return c.verifyRsp, nil
}

func (c *fakeClient) OrderDetail(ctx context.Context, orderID string, body gopay.BodyMap) (*gopaypaypal.OrderDetailRsp, error) {
	return c.detailRsp, nil
}

func (c *fakeClient) PaymentCaptureRefund(ctx context.Context, captureID string, body gopay.BodyMap) (*gopaypaypal.PaymentCaptureRefundRsp, error) {
	c.body = body
	return c.refundRsp, nil
}

func (c *fakeClient) PaymentRefundDetail(ctx context.Context, refundID string) (*gopaypaypal.PaymentRefundDetailRsp, error) {
	return c.refundDetailRsp, nil
}

func (c *fakeClient) SetRequestHeader(key string, values ...string) {
	if c.requestHeaders == nil {
		c.requestHeaders = map[string]string{}
	}
	value := ""
	if len(values) > 0 {
		value = values[0]
	}
	c.requestHeaders[key] = value
}

func (c *fakeClient) ClearRequestHeader() {}

func TestQueryPaymentNormalizesCompletedPaypalOrder(t *testing.T) {
	client := &fakeClient{detailRsp: &gopaypaypal.OrderDetailRsp{Code: gopaypaypal.Success, Response: &gopaypaypal.OrderDetail{Id: "PP_ORDER_1", Status: "COMPLETED", PurchaseUnits: []*gopaypaypal.PurchaseUnit{{CustomId: "gw_1", Payments: &gopaypaypal.Payments{Captures: []*gopaypaypal.Capture{{Id: "CAPTURE_1", Status: "COMPLETED", Amount: &gopaypaypal.Amount{CurrencyCode: "USD", Value: "99.00"}}}}}}}}}
	p := NewWithClientFactory(func(Config) (paypalClient, error) { return client, nil })
	result, err := p.QueryPayment(context.Background(), provider.QueryPaymentRequest{ChannelAccount: paypalAccount(), Order: &ent.PaymentOrder{GatewayOrderNo: "gw_1", ProviderOrderNo: "PP_ORDER_1", Amount: 9900, Currency: "USD"}})
	if err != nil {
		t.Fatalf("QueryPayment() error = %v", err)
	}
	if result.Status != "paid" || result.ChannelTradeNo != "CAPTURE_1" || result.Amount != 9900 {
		t.Fatalf("result = %#v, want completed PayPal capture", result)
	}
}

func TestCreateRefundSetsPaypalRequestID(t *testing.T) {
	client := &fakeClient{refundRsp: &gopaypaypal.PaymentCaptureRefundRsp{Code: gopaypaypal.Success, Response: &gopaypaypal.PaymentCaptureRefund{Id: "PP_REFUND_1", Status: "COMPLETED", Amount: &gopaypaypal.Amount{CurrencyCode: "USD", Value: "12.34"}}}}
	p := NewWithClientFactory(func(Config) (paypalClient, error) { return client, nil })
	result, err := p.CreateRefund(context.Background(), provider.CreateRefundRequest{ChannelAccount: paypalAccount(), GatewayOrderNo: "gw_1", ChannelTradeNo: "CAPTURE_1", RefundNo: "rf_1", Amount: 1234, Currency: "USD"})
	if err != nil {
		t.Fatalf("CreateRefund() error = %v", err)
	}
	if result.Status != "succeeded" || result.ChannelRefundNo != "PP_REFUND_1" || client.requestHeaders["PayPal-Request-Id"] != "rf_1" {
		t.Fatalf("result=%#v headers=%#v, want idempotent completed refund", result, client.requestHeaders)
	}
}

func TestQueryRefundNormalizesPendingPaypalRefund(t *testing.T) {
	client := &fakeClient{refundDetailRsp: &gopaypaypal.PaymentRefundDetailRsp{Code: gopaypaypal.Success, Response: &gopaypaypal.PaymentCaptureRefund{Id: "PP_REFUND_1", Status: "PENDING", InvoiceId: "rf_1", Amount: &gopaypaypal.Amount{CurrencyCode: "USD", Value: "12.34"}}}}
	p := NewWithClientFactory(func(Config) (paypalClient, error) { return client, nil })
	result, err := p.QueryRefund(context.Background(), provider.QueryRefundRequest{ChannelAccount: paypalAccount(), RefundNo: "rf_1", ChannelRefundNo: "PP_REFUND_1"})
	if err != nil {
		t.Fatalf("QueryRefund() error = %v", err)
	}
	if result.Status != "pending" || result.Amount != 1234 || result.ChannelRefundNo != "PP_REFUND_1" {
		t.Fatalf("result = %#v, want pending refund query", result)
	}
}

func paypalAccount() *ent.ChannelAccount {
	return &ent.ChannelAccount{Channel: "paypal", Env: "sandbox", Config: map[string]any{"client_id": "client", "client_secret": "secret"}}
}

func TestParseConfigRequiresClientID(t *testing.T) {
	_, err := ParseConfig(map[string]any{"client_secret": "secret"}, "sandbox")
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want missing client_id error")
	}
}

func TestParseConfigRequiresClientSecret(t *testing.T) {
	_, err := ParseConfig(map[string]any{"client_id": "client"}, "sandbox")
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want missing client_secret error")
	}
}

func TestParseConfigRejectsUnsupportedIntent(t *testing.T) {
	_, err := ParseConfig(map[string]any{"client_id": "client", "client_secret": "secret", "intent": "AUTHORIZE"}, "sandbox")
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want unsupported intent error")
	}
}

func TestProviderCreateOrderReturnsApproveURL(t *testing.T) {
	client := &fakeClient{
		rsp: &gopaypaypal.CreateOrderRsp{
			Code: gopaypaypal.Success,
			Response: &gopaypaypal.OrderDetail{
				Id:     "PAYPAL_ORDER_001",
				Status: "CREATED",
				Links: []*gopaypaypal.Link{
					{Rel: "self", Href: "https://api-m.sandbox.paypal.com/v2/checkout/orders/PAYPAL_ORDER_001"},
					{Rel: "approve", Href: "https://www.sandbox.paypal.com/checkoutnow?token=PAYPAL_ORDER_001"},
				},
			},
		},
	}
	p := NewWithClientFactory(func(Config) (paypalClient, error) {
		return client, nil
	})

	result, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_001",
			Subject:        "Pro 会员",
			Amount:         9900,
			Currency:       "USD",
		},
		ChannelAccount: &ent.ChannelAccount{
			Channel: "paypal",
			Env:     "sandbox",
			Config: map[string]any{
				"client_id":     "client",
				"client_secret": "secret",
				"brand_name":    "Payment Gateway",
				"intent":        "CAPTURE",
			},
		},
		Channel:   "paypal",
		PayMethod: "paypal",
		ReturnURL: "https://pay.example.com/v1/checkout/paypal/return?gateway_order_no=pay_001",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if result.ProviderOrderNo != "PAYPAL_ORDER_001" {
		t.Fatalf("ProviderOrderNo = %q, want PayPal order id", result.ProviderOrderNo)
	}
	if result.PayURL != "https://www.sandbox.paypal.com/checkoutnow?token=PAYPAL_ORDER_001" {
		t.Fatalf("PayURL = %q, want approve link", result.PayURL)
	}
	if client.body.GetString("intent") != "CAPTURE" {
		t.Fatalf("intent = %q, want CAPTURE", client.body.GetString("intent"))
	}
	source := client.body.GetAny("payment_source").(gopay.BodyMap)
	paypalSource := source.GetAny("paypal").(gopay.BodyMap)
	experience := paypalSource.GetAny("experience_context").(gopay.BodyMap)
	if experience.GetString("return_url") != "https://pay.example.com/v1/checkout/paypal/return?gateway_order_no=pay_001" {
		t.Fatalf("return_url = %q, want checkout return URL", experience.GetString("return_url"))
	}
	cancel, err := url.Parse(experience.GetString("cancel_url"))
	if err != nil {
		t.Fatalf("parse cancel_url: %v", err)
	}
	if cancel.Query().Get("gateway_order_no") != "pay_001" || cancel.Query().Get("cancel") != "1" {
		t.Fatalf("cancel_url = %q, want gateway_order_no and cancel params", experience.GetString("cancel_url"))
	}
}

func TestProviderCreateOrderReturnsPayerActionURL(t *testing.T) {
	client := &fakeClient{
		rsp: &gopaypaypal.CreateOrderRsp{
			Code: gopaypaypal.Success,
			Response: &gopaypaypal.OrderDetail{
				Id:     "PAYPAL_ORDER_001",
				Status: "PAYER_ACTION_REQUIRED",
				Links: []*gopaypaypal.Link{
					{Rel: "self", Href: "https://api-m.sandbox.paypal.com/v2/checkout/orders/PAYPAL_ORDER_001"},
					{Rel: "payer-action", Href: "https://www.sandbox.paypal.com/checkoutnow?token=PAYPAL_ORDER_001"},
				},
			},
		},
	}
	p := NewWithClientFactory(func(Config) (paypalClient, error) {
		return client, nil
	})

	result, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_001",
			Subject:        "Pro 会员",
			Amount:         9900,
			Currency:       "USD",
		},
		ChannelAccount: &ent.ChannelAccount{
			Channel: "paypal",
			Env:     "sandbox",
			Config: map[string]any{
				"client_id":     "client",
				"client_secret": "secret",
			},
		},
		Channel:   "paypal",
		PayMethod: "paypal",
		ReturnURL: "https://pay.example.com/v1/checkout/paypal/return?gateway_order_no=pay_001",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if result.PayURL != "https://www.sandbox.paypal.com/checkoutnow?token=PAYPAL_ORDER_001" {
		t.Fatalf("PayURL = %q, want payer-action link", result.PayURL)
	}
}

func TestCapturePaymentReturnsValidatedOrderIdentityAndAmount(t *testing.T) {
	client := &fakeClient{
		captureRsp: &gopaypaypal.OrderCaptureRsp{
			Code: gopaypaypal.Success,
			Response: &gopaypaypal.OrderDetail{
				Id:     "PAYPAL_ORDER_001",
				Status: "COMPLETED",
				PurchaseUnits: []*gopaypaypal.PurchaseUnit{
					{
						CustomId: "pay_001",
						Amount:   &gopaypaypal.Amount{CurrencyCode: "USD", Value: "99.00"},
						Payments: &gopaypaypal.Payments{Captures: []*gopaypaypal.Capture{
							{Id: "CAPTURE_001", Status: "COMPLETED", Amount: &gopaypaypal.Amount{CurrencyCode: "USD", Value: "99.00"}},
						}},
					},
				},
			},
		},
	}
	p := NewWithClientFactory(func(Config) (paypalClient, error) { return client, nil })

	result, err := p.CapturePayment(context.Background(), provider.CapturePaymentRequest{
		ChannelAccount:  &ent.ChannelAccount{Config: map[string]any{"client_id": "client", "client_secret": "secret"}},
		ProviderOrderNo: "PAYPAL_ORDER_001",
		GatewayOrderNo:  "pay_001",
	})
	if err != nil {
		t.Fatalf("CapturePayment() error = %v", err)
	}
	if result.Status != "paid" || result.ChannelTradeNo != "CAPTURE_001" {
		t.Fatalf("result = %#v, want completed capture", result)
	}
	if result.GatewayOrderNo != "pay_001" || result.Amount != 9900 || result.Currency != "USD" {
		t.Fatalf("result identity/amount = %#v, want pay_001 9900 USD", result)
	}
}

func TestNormalizeWebhookStatusKeepsApprovedOrderPending(t *testing.T) {
	if got := normalizeWebhookStatus("CHECKOUT.ORDER.APPROVED", "APPROVED"); got != "pending" {
		t.Fatalf("normalizeWebhookStatus() = %q, want pending", got)
	}
	if got := normalizeWebhookStatus("PAYMENT.CAPTURE.COMPLETED", "COMPLETED"); got != "paid" {
		t.Fatalf("normalizeWebhookStatus() = %q, want paid", got)
	}
}

func TestProviderCreateOrderOmitsEmptyExperienceFields(t *testing.T) {
	client := &fakeClient{
		rsp: &gopaypaypal.CreateOrderRsp{
			Code: gopaypaypal.Success,
			Response: &gopaypaypal.OrderDetail{
				Id:     "PAYPAL_ORDER_001",
				Status: "CREATED",
				Links: []*gopaypaypal.Link{
					{Rel: "approve", Href: "https://www.sandbox.paypal.com/checkoutnow?token=PAYPAL_ORDER_001"},
				},
			},
		},
	}
	p := NewWithClientFactory(func(Config) (paypalClient, error) {
		return client, nil
	})

	_, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_001",
			Subject:        "Pro 会员",
			Amount:         9900,
			Currency:       "USD",
		},
		ChannelAccount: &ent.ChannelAccount{
			Channel: "paypal",
			Env:     "sandbox",
			Config: map[string]any{
				"client_id":     "client",
				"client_secret": "secret",
				"brand_name":    "",
			},
		},
		Channel:   "paypal",
		PayMethod: "paypal",
		ReturnURL: "",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}

	source := client.body.GetAny("payment_source").(gopay.BodyMap)
	paypalSource := source.GetAny("paypal").(gopay.BodyMap)
	experience := paypalSource.GetAny("experience_context").(gopay.BodyMap)
	if _, exists := experience["brand_name"]; exists {
		t.Fatalf("brand_name exists with empty value: %#v", experience["brand_name"])
	}
	if _, exists := experience["return_url"]; exists {
		t.Fatalf("return_url exists with empty value: %#v", experience["return_url"])
	}
	if _, exists := experience["cancel_url"]; exists {
		t.Fatalf("cancel_url exists with empty value: %#v", experience["cancel_url"])
	}
}

func TestFormatAmountUsesNoDecimalsForZeroDecimalCurrency(t *testing.T) {
	if got := formatAmount(9900, "USD"); got != "99.00" {
		t.Fatalf("formatAmount(USD) = %q, want 99.00", got)
	}
	if got := formatAmount(9900, "JPY"); got != "9900" {
		t.Fatalf("formatAmount(JPY) = %q, want 9900", got)
	}
}

func TestProviderCreateOrderReportsPaypalErrorDetails(t *testing.T) {
	client := &fakeClient{
		rsp: &gopaypaypal.CreateOrderRsp{
			Code: 400,
			ErrorResponse: &gopaypaypal.ErrorResponse{
				Name:    "INVALID_REQUEST",
				Message: "Request is not well-formed",
				Details: []gopaypaypal.ErrorDetail{
					{
						Field:       "/purchase_units/0/amount/currency_code",
						Issue:       "INVALID_CURRENCY_CODE",
						Description: "Currency code is invalid.",
						Value:       "DOGE",
					},
				},
			},
		},
	}
	p := NewWithClientFactory(func(Config) (paypalClient, error) {
		return client, nil
	})

	_, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_001",
			Subject:        "Pro 会员",
			Amount:         9900,
			Currency:       "DOGE",
		},
		ChannelAccount: &ent.ChannelAccount{
			Channel: "paypal",
			Env:     "sandbox",
			Config: map[string]any{
				"client_id":     "client",
				"client_secret": "secret",
			},
		},
		Channel:   "paypal",
		PayMethod: "paypal",
		ReturnURL: "https://pay.example.com/v1/checkout/paypal/return?gateway_order_no=pay_001",
	})
	if err == nil {
		t.Fatal("StartPayment() error = nil, want paypal error")
	}
	message := err.Error()
	for _, want := range []string{"/purchase_units/0/amount/currency_code", "INVALID_CURRENCY_CODE", "DOGE"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error = %q, want detail %q", message, want)
		}
	}
}

func TestParseNotifyVerifiesPaypalWebhookSignature(t *testing.T) {
	client := &fakeClient{
		verifyRsp: &gopaypaypal.VerifyWebhookResponse{VerificationStatus: "SUCCESS"},
	}
	p := NewWithClientFactory(func(Config) (paypalClient, error) {
		return client, nil
	})

	result, err := p.ParseNotify(context.Background(), provider.NotifyRequest{
		ChannelAccount: &ent.ChannelAccount{
			Channel: "paypal",
			Env:     "sandbox",
			Config: map[string]any{
				"client_id":     "client",
				"client_secret": "secret",
				"webhook_id":    "WH-123",
			},
		},
		Header: map[string][]string{
			"Paypal-Auth-Algo":         {"SHA256withRSA"},
			"Paypal-Cert-Url":          {"https://api-m.sandbox.paypal.com/certs/CERT"},
			"Paypal-Transmission-Id":   {"transmission-id"},
			"Paypal-Transmission-Sig":  {"signature"},
			"Paypal-Transmission-Time": {"2026-07-01T00:00:00Z"},
		},
		RawBody: []byte(`{"event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"id":"CAPTURE_001","custom_id":"pay_001","status":"COMPLETED","amount":{"currency_code":"USD","value":"99.00"}}}`),
	})
	if err != nil {
		t.Fatalf("ParseNotify() error = %v", err)
	}
	if client.verifyBM.GetString("webhook_id") != "WH-123" || client.verifyBM.GetString("transmission_id") != "transmission-id" {
		t.Fatalf("verify body = %#v, want webhook id and transmission id", client.verifyBM)
	}
	if result.GatewayOrderNo != "pay_001" || result.ChannelTradeNo != "CAPTURE_001" || result.Status != "paid" || result.Amount != 9900 || result.Currency != "USD" {
		t.Fatalf("result = %#v, want paid PayPal notify", result)
	}
}

func TestParseNotifyRejectsUnverifiedPaypalWebhookSignature(t *testing.T) {
	client := &fakeClient{
		verifyRsp: &gopaypaypal.VerifyWebhookResponse{VerificationStatus: "FAILURE"},
	}
	p := NewWithClientFactory(func(Config) (paypalClient, error) {
		return client, nil
	})

	_, err := p.ParseNotify(context.Background(), provider.NotifyRequest{
		ChannelAccount: &ent.ChannelAccount{
			Channel: "paypal",
			Env:     "sandbox",
			Config: map[string]any{
				"client_id":     "client",
				"client_secret": "secret",
				"webhook_id":    "WH-123",
			},
		},
		Header: map[string][]string{
			"Paypal-Auth-Algo":         {"SHA256withRSA"},
			"Paypal-Cert-Url":          {"https://api-m.sandbox.paypal.com/certs/CERT"},
			"Paypal-Transmission-Id":   {"transmission-id"},
			"Paypal-Transmission-Sig":  {"signature"},
			"Paypal-Transmission-Time": {"2026-07-01T00:00:00Z"},
		},
		RawBody: []byte(`{"event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"id":"CAPTURE_001","custom_id":"pay_001","status":"COMPLETED","amount":{"currency_code":"USD","value":"99.00"}}}`),
	})
	if err == nil {
		t.Fatal("ParseNotify() error = nil, want unverified webhook error")
	}
}
