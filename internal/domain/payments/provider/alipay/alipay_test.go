package alipay

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/go-pay/gopay"
	gopayalipay "github.com/go-pay/gopay/alipay"
	gopayalipayv3 "github.com/go-pay/gopay/alipay/v3"

	"payment-gateway/ent"
	"payment-gateway/internal/domain/payments/provider"
)

type fakeClient struct {
	pageBody       gopay.BodyMap
	wapBody        gopay.BodyMap
	precreateBody  gopay.BodyMap
	pageCalls      int
	wapCalls       int
	precreateCalls int
}

func (c *fakeClient) TradePagePay(ctx context.Context, body gopay.BodyMap) (string, error) {
	_ = ctx
	c.pageBody = body
	c.pageCalls++
	return "https://openapi-sandbox.dl.alipaydev.com/gateway.do?test=1", nil
}

func (c *fakeClient) TradeWapPay(ctx context.Context, body gopay.BodyMap) (string, error) {
	_ = ctx
	c.wapBody = body
	c.wapCalls++
	return "https://openapi-sandbox.dl.alipaydev.com/gateway.do?wap=1", nil
}

func (c *fakeClient) TradePrecreate(ctx context.Context, body gopay.BodyMap) (string, error) {
	_ = ctx
	c.precreateBody = body
	c.precreateCalls++
	return "https://qr.alipay.com/test", nil
}

type fakeV3Transport struct {
	rsp *gopayalipayv3.TradePrecreateRsp
	err error
}

func (c *fakeV3Transport) TradePrecreate(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradePrecreateRsp, error) {
	_ = ctx
	return c.rsp, c.err
}

func (c *fakeV3Transport) TradePagePay(ctx context.Context, body gopay.BodyMap) (string, error) {
	_ = ctx
	return "https://openapi-sandbox.dl.alipaydev.com/gateway.do?test=1", c.err
}

func (c *fakeV3Transport) TradeWapPay(ctx context.Context, body gopay.BodyMap) (string, error) {
	_ = ctx
	return "https://openapi-sandbox.dl.alipaydev.com/gateway.do?wap=1", c.err
}

func TestParseConfigRequiresAppID(t *testing.T) {
	_, err := ParseConfig(map[string]any{"private_key": "private"}, "sandbox")
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want missing app_id error")
	}
}

func TestParseConfigRequiresPrivateKey(t *testing.T) {
	_, err := ParseConfig(map[string]any{"app_id": "app-1"}, "sandbox")
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want missing private_key error")
	}
}

func TestParseConfigDefaultsProductCodeAndMode(t *testing.T) {
	cfg, err := ParseConfig(map[string]any{
		"app_id":      "app-1",
		"private_key": "private",
	}, "sandbox")
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if cfg.ProductCode != DefaultPageProductCode {
		t.Fatalf("ProductCode = %q, want default", cfg.ProductCode)
	}
	if cfg.Mode != "page" {
		t.Fatalf("Mode = %q, want page", cfg.Mode)
	}
	if cfg.IsProd {
		t.Fatal("IsProd = true, want false for sandbox")
	}
}

func TestParseConfigDefaultsProductCodeByMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "page", mode: "page", want: DefaultPageProductCode},
		{name: "wap", mode: "wap", want: DefaultWapProductCode},
		{name: "qr", mode: "qr", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseConfig(map[string]any{
				"app_id":      "app-1",
				"private_key": "private",
				"mode":        tt.mode,
			}, "sandbox")
			if err != nil {
				t.Fatalf("ParseConfig() error = %v", err)
			}
			if cfg.ProductCode != tt.want {
				t.Fatalf("ProductCode = %q, want %q", cfg.ProductCode, tt.want)
			}
		})
	}
}

func TestParseConfigRejectsUnsupportedMode(t *testing.T) {
	_, err := ParseConfig(map[string]any{
		"app_id":      "app-1",
		"private_key": "private",
		"mode":        "facepay",
	}, "sandbox")
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want unsupported mode error")
	}
}

func TestStartPaymentRejectsDisabledMode(t *testing.T) {
	client := &fakeClient{}
	p := NewWithClientFactory(func(Config) (alipayClient, error) { return client, nil })
	_, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{GatewayOrderNo: "pay_disabled", Subject: "Pro", Amount: 9900, Currency: "CNY"},
		ChannelAccount: &ent.ChannelAccount{Env: "prod", Config: map[string]any{
			"app_id": "app", "private_key": "private", "mode": "page", "enable_page_pay": false,
		}},
		Channel: "alipay", PayMethod: "alipay",
	})
	if err == nil {
		t.Fatal("StartPayment() error = nil, want disabled mode error")
	}
	if client.pageCalls != 0 {
		t.Fatalf("page calls = %d, want 0", client.pageCalls)
	}
}

func TestParseConfigDefaultsAlipayCapabilities(t *testing.T) {
	cfg, err := ParseConfig(map[string]any{
		"app_id":      "app-1",
		"private_key": "private",
	}, "sandbox")
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if !cfg.EnablePagePay || !cfg.EnableWapPay || !cfg.EnableQRPay {
		t.Fatalf("capabilities = page:%v wap:%v qr:%v, want all enabled by default", cfg.EnablePagePay, cfg.EnableWapPay, cfg.EnableQRPay)
	}
}

func TestNormalizeAlipayV3ProxyHostStripsLegacyGatewayPath(t *testing.T) {
	tests := []struct {
		name      string
		serverURL string
		want      string
	}{
		{
			name:      "legacy sandbox gateway",
			serverURL: "https://openapi-sandbox.dl.alipaydev.com/gateway.do",
			want:      "https://openapi-sandbox.dl.alipaydev.com",
		},
		{
			name:      "host already valid",
			serverURL: "https://openapi-sandbox.dl.alipaydev.com",
			want:      "https://openapi-sandbox.dl.alipaydev.com",
		},
		{
			name:      "trailing slash",
			serverURL: "https://openapi-sandbox.dl.alipaydev.com/gateway.do/",
			want:      "https://openapi-sandbox.dl.alipaydev.com",
		},
		{
			name:      "empty",
			serverURL: " ",
			want:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeAlipayV3ProxyHost(tt.serverURL)
			if got != tt.want {
				t.Fatalf("normalizeAlipayV3ProxyHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProviderPageModeBuildsAlipayRequest(t *testing.T) {
	client := &fakeClient{}
	p := NewWithClientFactory(func(Config) (alipayClient, error) {
		return client, nil
	})
	account := &ent.ChannelAccount{
		Channel: "alipay",
		Env:     "sandbox",
		Config: map[string]any{
			"app_id":       "app-1",
			"private_key":  "private",
			"product_code": "FAST_INSTANT_TRADE_PAY",
			"mode":         "page",
		},
	}

	result, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_001",
			Subject:        "Pro 会员",
			Amount:         9900,
			Currency:       "CNY",
		},
		ChannelAccount: account,
		Channel:        "alipay",
		PayMethod:      "alipay",
		NotifyURL:      "https://pay.example.com/v1/channel/notify",
		ReturnURL:      "https://merchant.example.com/result",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if result.PayURL != "https://openapi-sandbox.dl.alipaydev.com/gateway.do?test=1" {
		t.Fatalf("PayURL = %q, want fake page pay URL", result.PayURL)
	}
	if client.pageCalls != 1 || client.precreateCalls != 0 {
		t.Fatalf("page/precreate calls = %d/%d, want 1/0", client.pageCalls, client.precreateCalls)
	}
	assertBody(t, client.pageBody, "out_trade_no", "pay_001")
	assertBody(t, client.pageBody, "subject", "Pro 会员")
	assertBody(t, client.pageBody, "total_amount", "99.00")
	assertBody(t, client.pageBody, "product_code", "FAST_INSTANT_TRADE_PAY")
	assertBody(t, client.pageBody, "notify_url", "https://pay.example.com/v1/channel/notify")
	assertBody(t, client.pageBody, "return_url", "https://merchant.example.com/result")
}

func TestProviderPageModeReturnsPayURL(t *testing.T) {
	client := &fakeClient{}
	p := NewWithClientFactory(func(Config) (alipayClient, error) {
		return client, nil
	})

	result, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_005",
			Subject:        "Pro 会员",
			Amount:         9900,
			Currency:       "CNY",
		},
		ChannelAccount: &ent.ChannelAccount{
			Channel: "alipay",
			Env:     "sandbox",
			Config: map[string]any{
				"app_id":       "app-1",
				"private_key":  "private",
				"product_code": "FAST_INSTANT_TRADE_PAY",
				"mode":         "page",
			},
		},
		Channel:   "alipay",
		PayMethod: "alipay",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if result.PayURL == "" {
		t.Fatalf("PayURL = %q, want page payment URL", result.PayURL)
	}
	if result.QRCode != "" {
		t.Fatalf("QRCode = %q, want empty for page mode", result.QRCode)
	}
	if client.pageCalls != 1 || client.precreateCalls != 0 {
		t.Fatalf("page/precreate calls = %d/%d, want 1/0", client.pageCalls, client.precreateCalls)
	}
}

func TestProviderQRCodeModeReturnsQRCode(t *testing.T) {
	client := &fakeClient{}
	p := NewWithClientFactory(func(Config) (alipayClient, error) {
		return client, nil
	})

	result, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_002",
			Subject:        "Pro 会员",
			Amount:         9900,
			Currency:       "CNY",
		},
		ChannelAccount: &ent.ChannelAccount{
			Channel: "alipay",
			Env:     "sandbox",
			Config: map[string]any{
				"app_id":      "app-1",
				"private_key": "private",
				"mode":        "qr",
			},
		},
		Channel:   "alipay",
		PayMethod: "alipay",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if result.QRCode != "https://qr.alipay.com/test" {
		t.Fatalf("QRCode = %q, want fake qr code", result.QRCode)
	}
	if result.PayURL != "" {
		t.Fatalf("PayURL = %q, want empty for qr mode", result.PayURL)
	}
	if client.pageCalls != 0 || client.precreateCalls != 1 {
		t.Fatalf("page/precreate calls = %d/%d, want 0/1", client.pageCalls, client.precreateCalls)
	}
	if got := client.precreateBody.GetString("product_code"); got != gopay.NULL {
		t.Fatalf("body[product_code] = %q, want omitted for v3 precreate", got)
	}
}

func TestProviderWapModeReturnsPayURL(t *testing.T) {
	client := &fakeClient{}
	p := NewWithClientFactory(func(Config) (alipayClient, error) {
		return client, nil
	})

	result, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_wap_001",
			Subject:        "Pro 会员",
			Amount:         9900,
			Currency:       "CNY",
		},
		ChannelAccount: &ent.ChannelAccount{
			Channel: "alipay",
			Env:     "sandbox",
			Config: map[string]any{
				"app_id":      "app-1",
				"private_key": "private",
				"mode":        "wap",
			},
		},
		Channel:   "alipay",
		PayMethod: "alipay",
		ReturnURL: "https://merchant.example.com/mobile-return",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if result.PayURL != "https://openapi-sandbox.dl.alipaydev.com/gateway.do?wap=1" {
		t.Fatalf("PayURL = %q, want fake wap pay URL", result.PayURL)
	}
	if result.QRCode != "" {
		t.Fatalf("QRCode = %q, want empty for wap mode", result.QRCode)
	}
	if client.wapCalls != 1 || client.pageCalls != 0 || client.precreateCalls != 0 {
		t.Fatalf("wap/page/precreate calls = %d/%d/%d, want 1/0/0", client.wapCalls, client.pageCalls, client.precreateCalls)
	}
	assertBody(t, client.wapBody, "return_url", "https://merchant.example.com/mobile-return")
	assertBody(t, client.wapBody, "product_code", DefaultWapProductCode)
}

func TestGopayClientReturnsPrecreateGatewayError(t *testing.T) {
	client := &gopayClient{
		client: &fakeV3Transport{
			rsp: &gopayalipayv3.TradePrecreateRsp{
				StatusCode: 400,
				ErrResponse: gopayalipayv3.ErrResponse{
					Code:    "PARAM_ERROR",
					Message: "invalid total_amount",
				},
			},
		},
	}

	_, err := client.TradePrecreate(context.Background(), gopay.BodyMap{
		"out_trade_no": "pay_003",
		"subject":      "Pro 会员",
		"total_amount": "99.00",
	})
	if err == nil {
		t.Fatal("TradePrecreate() error = nil, want gateway error")
	}
	if !strings.Contains(err.Error(), "PARAM_ERROR") || !strings.Contains(err.Error(), "invalid total_amount") {
		t.Fatalf("TradePrecreate() error = %q, want alipay error detail", err.Error())
	}
}

func TestGopayClientReturnsV3QRCode(t *testing.T) {
	client := &gopayClient{
		client: &fakeV3Transport{
			rsp: &gopayalipayv3.TradePrecreateRsp{
				StatusCode: http.StatusOK,
				OutTradeNo: "pay_004",
				QrCode:     "https://qr.alipay.com/v3-test",
			},
		},
	}

	qrCode, err := client.TradePrecreate(context.Background(), gopay.BodyMap{
		"out_trade_no": "pay_004",
		"subject":      "Pro 会员",
		"total_amount": "99.00",
	})
	if err != nil {
		t.Fatalf("TradePrecreate() error = %v", err)
	}
	if qrCode != "https://qr.alipay.com/v3-test" {
		t.Fatalf("qrCode = %q, want v3 qr code", qrCode)
	}
}

func TestParseNotifyMapsSuccessfulAlipayTrade(t *testing.T) {
	p := NewWithClientFactory(func(Config) (alipayClient, error) {
		t.Fatal("ParseNotify must not create a payment client")
		return nil, nil
	})
	privateKey, alipayPublicKey := testAlipayKeys(t)
	account := &ent.ChannelAccount{
		Channel: "alipay",
		Env:     "sandbox",
		Config: map[string]any{
			"app_id":            "app-1",
			"private_key":       "private-key",
			"alipay_public_key": alipayPublicKey,
		},
	}
	form := url.Values{}
	form.Set("out_trade_no", "GW202607010001")
	form.Set("trade_no", "2026070122000000001")
	form.Set("trade_status", "TRADE_SUCCESS")
	form.Set("total_amount", "99.00")
	signAlipayNotify(t, privateKey, form)

	result, err := p.ParseNotify(context.Background(), provider.NotifyRequest{
		ChannelAccount: account,
		Form:           form,
	})
	if err != nil {
		t.Fatalf("ParseNotify() error = %v", err)
	}
	if result.Status != "paid" || result.GatewayOrderNo != "GW202607010001" || result.ChannelTradeNo != "2026070122000000001" {
		t.Fatalf("result = %#v, want paid alipay notify", result)
	}
	if result.Amount != 9900 || result.Currency != "CNY" {
		t.Fatalf("amount/currency = %d/%s, want 9900/CNY", result.Amount, result.Currency)
	}
}

func testAlipayKeys(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey() error = %v", err)
	}
	return privateKey, base64.StdEncoding.EncodeToString(publicDER)
}

func signAlipayNotify(t *testing.T, privateKey *rsa.PrivateKey, values url.Values) {
	t.Helper()
	body := gopay.BodyMap{}
	for key, items := range values {
		if len(items) > 0 {
			body.Set(key, items[0])
		}
	}
	sign, err := gopayalipay.GetRsaSign(body, gopayalipay.RSA2, privateKey)
	if err != nil {
		t.Fatalf("GetRsaSign() error = %v", err)
	}
	values.Set("sign_type", gopayalipay.RSA2)
	values.Set("sign", sign)
}

func TestParseNotifyRequiresAlipayPublicKey(t *testing.T) {
	p := NewWithClientFactory(func(Config) (alipayClient, error) {
		t.Fatal("ParseNotify must not create a payment client")
		return nil, nil
	})
	account := &ent.ChannelAccount{
		Channel: "alipay",
		Env:     "sandbox",
		Config: map[string]any{
			"app_id":      "app-1",
			"private_key": "private-key",
		},
	}
	form := url.Values{}
	form.Set("out_trade_no", "GW202607010001")
	form.Set("trade_no", "2026070122000000001")
	form.Set("trade_status", "TRADE_SUCCESS")
	form.Set("total_amount", "99.00")

	if _, err := p.ParseNotify(context.Background(), provider.NotifyRequest{
		ChannelAccount: account,
		Form:           form,
	}); err == nil {
		t.Fatal("ParseNotify() error = nil, want missing alipay public key error")
	}
}

func TestParseNotifyRejectsInvalidAlipaySignature(t *testing.T) {
	_, alipayPublicKey := testAlipayKeys(t)
	p := NewWithClientFactory(func(Config) (alipayClient, error) {
		t.Fatal("ParseNotify must not create a payment client")
		return nil, nil
	})
	form := url.Values{}
	form.Set("out_trade_no", "GW202607010001")
	form.Set("trade_no", "2026070122000000001")
	form.Set("trade_status", "TRADE_SUCCESS")
	form.Set("total_amount", "99.00")
	form.Set("sign_type", gopayalipay.RSA2)
	form.Set("sign", "invalid-signature")

	_, err := p.ParseNotify(context.Background(), provider.NotifyRequest{
		ChannelAccount: &ent.ChannelAccount{Env: "prod", Config: map[string]any{
			"app_id": "app-1", "private_key": "private-key", "alipay_public_key": alipayPublicKey,
		}},
		Form: form,
	})
	if err == nil {
		t.Fatal("ParseNotify() error = nil, want invalid signature error")
	}
}

func TestParseAlipayAmountUsesExactMinorUnits(t *testing.T) {
	amount, err := parseAlipayAmount("90071992547409.91")
	if err != nil {
		t.Fatalf("parseAlipayAmount() error = %v", err)
	}
	if amount != 9007199254740991 {
		t.Fatalf("amount = %d, want 9007199254740991", amount)
	}
}

func assertBody(t *testing.T, body gopay.BodyMap, key string, want string) {
	t.Helper()
	if got := body.GetString(key); got != want {
		t.Fatalf("body[%s] = %q, want %q", key, got, want)
	}
}
