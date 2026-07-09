package wechat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-pay/gopay"
	wechatv3 "github.com/go-pay/gopay/wechat/v3"

	"payment-gateway/ent"
	"payment-gateway/internal/domain/payments/provider"
)

type fakeWechatClient struct {
	nativeRsp *wechatv3.NativeRsp
	h5Rsp     *wechatv3.H5Rsp
	err       error
	body      gopay.BodyMap
}

func (c *fakeWechatClient) V3TransactionNative(ctx context.Context, body gopay.BodyMap) (*wechatv3.NativeRsp, error) {
	c.body = body
	return c.nativeRsp, c.err
}

func (c *fakeWechatClient) V3TransactionH5(ctx context.Context, body gopay.BodyMap) (*wechatv3.H5Rsp, error) {
	c.body = body
	return c.h5Rsp, c.err
}

func TestParseConfigRequiresWechatCredentials(t *testing.T) {
	_, err := ParseConfig(map[string]any{
		"app_id":     "wx_app",
		"mch_id":     "mch_1",
		"api_v3_key": "v3-key",
		"serial_no":  "serial",
	}, "prod")
	if !errors.Is(err, ErrPrivateKeyRequired) {
		t.Fatalf("ParseConfig() error = %v, want ErrPrivateKeyRequired", err)
	}
}

func TestParseConfigDefaultsWechatCapabilities(t *testing.T) {
	cfg, err := ParseConfig(account(nil).Config, "prod")
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if !cfg.EnableNativePay || cfg.EnableH5Pay {
		t.Fatalf("capabilities = native:%v h5:%v, want legacy native default", cfg.EnableNativePay, cfg.EnableH5Pay)
	}
}

func TestParseConfigReadsWechatCapabilities(t *testing.T) {
	cfg, err := ParseConfig(account(map[string]any{
		"enable_native_pay": "false",
		"enable_h5_pay":     "true",
	}).Config, "prod")
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if cfg.EnableNativePay || !cfg.EnableH5Pay {
		t.Fatalf("capabilities = native:%v h5:%v, want h5 only", cfg.EnableNativePay, cfg.EnableH5Pay)
	}
}

func TestStartPaymentNativeReturnsQRCode(t *testing.T) {
	client := &fakeWechatClient{
		nativeRsp: &wechatv3.NativeRsp{
			Code:     wechatv3.Success,
			Response: &wechatv3.Native{CodeUrl: "weixin://wxpay/bizpayurl?pr=test"},
		},
	}
	p := NewWithClientFactory(func(Config) (wechatClient, error) {
		return client, nil
	})

	result, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_001",
			Subject:        "测试订单",
			Amount:         99,
			Currency:       "CNY",
		},
		ChannelAccount: account(map[string]any{"mode": "native"}),
		Channel:        "wechat",
		PayMethod:      "wechat",
		NotifyURL:      "https://gateway.example.com/v1/channel/notify",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if result.QRCode != "weixin://wxpay/bizpayurl?pr=test" {
		t.Fatalf("QRCode = %q", result.QRCode)
	}
	if result.PayURL != "" {
		t.Fatalf("PayURL = %q, want empty for native", result.PayURL)
	}
	if got := client.body.GetString("out_trade_no"); got != "pay_001" {
		t.Fatalf("out_trade_no = %q", got)
	}
	amount, ok := client.body.GetAny("amount").(gopay.BodyMap)
	if !ok {
		t.Fatalf("amount = %#v, want BodyMap", client.body.GetAny("amount"))
	}
	if got := amount.GetAny("total"); got != int64(99) {
		t.Fatalf("amount.total = %#v, want int64(99)", got)
	}
	if got := amount.GetString("currency"); got != "CNY" {
		t.Fatalf("amount.currency = %q", got)
	}
}

func TestStartPaymentH5ReturnsPayURLAndSceneInfo(t *testing.T) {
	client := &fakeWechatClient{
		h5Rsp: &wechatv3.H5Rsp{
			Code:     wechatv3.Success,
			Response: &wechatv3.H5Url{H5Url: "https://wx.tenpay.com/cgi-bin/mmpayweb-bin/checkmweb?prepay_id=test"},
		},
	}
	p := NewWithClientFactory(func(Config) (wechatClient, error) {
		return client, nil
	})

	result, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_h5",
			Subject:        "H5订单",
			Amount:         100,
			Currency:       "CNY",
		},
		ChannelAccount: account(map[string]any{"mode": "h5"}),
		Channel:        "wechat",
		PayMethod:      "wechat",
		ClientIP:       "203.0.113.9",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if !strings.HasPrefix(result.PayURL, "https://wx.tenpay.com/") {
		t.Fatalf("PayURL = %q", result.PayURL)
	}
	sceneInfo, ok := client.body.GetAny("scene_info").(gopay.BodyMap)
	if !ok {
		t.Fatalf("scene_info = %#v, want BodyMap", client.body.GetAny("scene_info"))
	}
	if got := sceneInfo.GetString("payer_client_ip"); got != "203.0.113.9" {
		t.Fatalf("payer_client_ip = %q", got)
	}
	h5Info, ok := sceneInfo.GetAny("h5_info").(gopay.BodyMap)
	if !ok {
		t.Fatalf("h5_info = %#v, want BodyMap", sceneInfo.GetAny("h5_info"))
	}
	if got := h5Info.GetString("type"); got != "Wap" {
		t.Fatalf("h5_info.type = %q", got)
	}
}

func TestStartPaymentRejectsUnsupportedCurrency(t *testing.T) {
	p := NewWithClientFactory(func(Config) (wechatClient, error) {
		t.Fatal("client factory should not be called for unsupported currency")
		return nil, nil
	})

	_, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_usd",
			Subject:        "USD订单",
			Amount:         100,
			Currency:       "USD",
		},
		ChannelAccount: account(nil),
		Channel:        "wechat",
		PayMethod:      "wechat",
	})
	if err == nil || !strings.Contains(err.Error(), "only supports CNY") {
		t.Fatalf("StartPayment() error = %v, want CNY error", err)
	}
}

func TestStartPaymentNativeResponseErrorIncludesWechatDetail(t *testing.T) {
	client := &fakeWechatClient{
		nativeRsp: &wechatv3.NativeRsp{
			Code: 400,
			ErrResponse: wechatv3.ErrResponse{
				Code:    "PARAM_ERROR",
				Message: "参数错误",
			},
		},
	}
	p := NewWithClientFactory(func(Config) (wechatClient, error) {
		return client, nil
	})

	_, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order: &ent.PaymentOrder{
			GatewayOrderNo: "pay_bad",
			Subject:        "坏订单",
			Amount:         100,
			Currency:       "CNY",
		},
		ChannelAccount: account(map[string]any{"mode": "native"}),
		Channel:        "wechat",
		PayMethod:      "wechat",
	})
	if err == nil || !strings.Contains(err.Error(), "PARAM_ERROR") {
		t.Fatalf("StartPayment() error = %v, want PARAM_ERROR", err)
	}
}

func account(config map[string]any) *ent.ChannelAccount {
	values := map[string]any{
		"app_id":     "wx_app",
		"mch_id":     "mch_1",
		"api_v3_key": "12345678901234567890123456789012",
		"serial_no":  "serial",
		"private_key": "-----BEGIN PRIVATE KEY-----\n" +
			"MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD\n" +
			"-----END PRIVATE KEY-----",
	}
	for key, value := range config {
		values[key] = value
	}
	return &ent.ChannelAccount{
		Channel: "wechat",
		Env:     "prod",
		Config:  values,
	}
}
