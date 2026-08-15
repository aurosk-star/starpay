package wechat

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"testing"

	gopayaes "github.com/go-pay/crypto/aes"
	"github.com/go-pay/gopay"
	wechatv3 "github.com/go-pay/gopay/wechat/v3"

	"payment-gateway/ent"
	"payment-gateway/internal/domain/payments/provider"
)

type fakeWechatClient struct {
	nativeRsp      *wechatv3.NativeRsp
	h5Rsp          *wechatv3.H5Rsp
	err            error
	body           gopay.BodyMap
	calls          int
	queryRsp       *wechatv3.QueryOrderRsp
	closeRsp       *wechatv3.EmptyRsp
	refundRsp      *wechatv3.RefundRsp
	refundQueryRsp *wechatv3.RefundQueryRsp
	queryNo        string
	closeNo        string
	refundNo       string
}

func (c *fakeWechatClient) V3TransactionNative(ctx context.Context, body gopay.BodyMap) (*wechatv3.NativeRsp, error) {
	c.calls++
	c.body = body
	return c.nativeRsp, c.err
}

func (c *fakeWechatClient) V3TransactionH5(ctx context.Context, body gopay.BodyMap) (*wechatv3.H5Rsp, error) {
	c.calls++
	c.body = body
	return c.h5Rsp, c.err
}

func (c *fakeWechatClient) V3TransactionQueryOrder(ctx context.Context, orderNoType wechatv3.OrderNoType, orderNo string) (*wechatv3.QueryOrderRsp, error) {
	c.queryNo = orderNo
	return c.queryRsp, c.err
}

func (c *fakeWechatClient) V3TransactionCloseOrder(ctx context.Context, tradeNo string) (*wechatv3.EmptyRsp, error) {
	c.closeNo = tradeNo
	return c.closeRsp, c.err
}

func (c *fakeWechatClient) V3Refund(ctx context.Context, body gopay.BodyMap) (*wechatv3.RefundRsp, error) {
	c.body = body
	return c.refundRsp, c.err
}

func (c *fakeWechatClient) V3RefundQuery(ctx context.Context, outRefundNo string, body gopay.BodyMap) (*wechatv3.RefundQueryRsp, error) {
	c.refundNo = outRefundNo
	return c.refundQueryRsp, c.err
}

func TestQueryPaymentNormalizesPaidWechatTrade(t *testing.T) {
	client := &fakeWechatClient{queryRsp: &wechatv3.QueryOrderRsp{Code: wechatv3.Success, Response: &wechatv3.QueryOrder{OutTradeNo: "gw_1", TransactionId: "wx_trade_1", TradeState: "SUCCESS", Amount: &wechatv3.Amount{Total: 9900, Currency: "CNY"}}}}
	p := NewWithClientFactory(func(Config) (wechatClient, error) { return client, nil })
	result, err := p.QueryPayment(context.Background(), provider.QueryPaymentRequest{ChannelAccount: account(nil), Order: &ent.PaymentOrder{GatewayOrderNo: "gw_1", Amount: 9900, Currency: "CNY"}})
	if err != nil {
		t.Fatalf("QueryPayment() error = %v", err)
	}
	if result.Status != "paid" || result.Amount != 9900 || result.ChannelTradeNo != "wx_trade_1" {
		t.Fatalf("result = %#v, want paid wechat trade", result)
	}
}

func TestCreateRefundUsesWechatRefundIdentityAndAmounts(t *testing.T) {
	client := &fakeWechatClient{refundRsp: &wechatv3.RefundRsp{Code: wechatv3.Success, Response: &wechatv3.RefundOrderResponse{RefundId: "wx_refund_1", OutRefundNo: "rf_1", Status: "PROCESSING", Amount: &wechatv3.RefundOrderAmount{Refund: 1234, Total: 9900, Currency: "CNY"}}}}
	p := NewWithClientFactory(func(Config) (wechatClient, error) { return client, nil })
	result, err := p.CreateRefund(context.Background(), provider.CreateRefundRequest{ChannelAccount: account(nil), GatewayOrderNo: "gw_1", ChannelTradeNo: "wx_trade_1", RefundNo: "rf_1", Amount: 1234, OriginalAmount: 9900, Currency: "CNY"})
	if err != nil {
		t.Fatalf("CreateRefund() error = %v", err)
	}
	if result.Status != "pending" || result.ChannelRefundNo != "wx_refund_1" || result.Amount != 1234 {
		t.Fatalf("result = %#v, want pending wechat refund", result)
	}
	if client.body.GetString("out_refund_no") != "rf_1" {
		t.Fatalf("out_refund_no = %q", client.body.GetString("out_refund_no"))
	}
}

func TestQueryRefundNormalizesSuccessfulWechatRefund(t *testing.T) {
	client := &fakeWechatClient{refundQueryRsp: &wechatv3.RefundQueryRsp{Code: wechatv3.Success, Response: &wechatv3.RefundQueryResponse{RefundId: "wx_refund_1", OutRefundNo: "rf_1", Status: "SUCCESS", Amount: &wechatv3.RefundOrderAmount{Refund: 1234, Currency: "CNY"}}}}
	p := NewWithClientFactory(func(Config) (wechatClient, error) { return client, nil })
	result, err := p.QueryRefund(context.Background(), provider.QueryRefundRequest{ChannelAccount: account(nil), RefundNo: "rf_1"})
	if err != nil {
		t.Fatalf("QueryRefund() error = %v", err)
	}
	if result.Status != "succeeded" || result.Amount != 1234 || client.refundNo != "rf_1" {
		t.Fatalf("result = %#v queryNo=%q, want succeeded refund", result, client.refundNo)
	}
}

func TestClosePaymentUsesGatewayOrderNumber(t *testing.T) {
	client := &fakeWechatClient{closeRsp: &wechatv3.EmptyRsp{Code: wechatv3.Success}}
	p := NewWithClientFactory(func(Config) (wechatClient, error) { return client, nil })
	err := p.ClosePayment(context.Background(), provider.ClosePaymentRequest{ChannelAccount: account(nil), Order: &ent.PaymentOrder{GatewayOrderNo: "gw_1"}})
	if err != nil {
		t.Fatalf("ClosePayment() error = %v", err)
	}
	if client.closeNo != "gw_1" {
		t.Fatalf("close order no = %q, want gw_1", client.closeNo)
	}
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

func TestParseConfigRejectsUnsupportedMode(t *testing.T) {
	_, err := ParseConfig(map[string]any{
		"app_id": "wx_app", "mch_id": "mch", "api_v3_key": testAPIV3Key, "serial_no": "serial", "private_key": "private", "mode": "jsapi",
	}, "prod")
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want unsupported mode error")
	}
}

func TestStartPaymentRejectsDisabledMode(t *testing.T) {
	client := &fakeWechatClient{}
	p := NewWithClientFactory(func(Config) (wechatClient, error) { return client, nil })
	_, err := p.StartPayment(context.Background(), provider.StartPaymentRequest{
		Order:          &ent.PaymentOrder{GatewayOrderNo: "pay_disabled", Subject: "Pro", Amount: 9900, Currency: "CNY"},
		ChannelAccount: account(map[string]any{"mode": "native", "enable_native_pay": false}),
		Channel:        "wechat", PayMethod: "wechat",
	})
	if err == nil {
		t.Fatal("StartPayment() error = nil, want disabled mode error")
	}
	if client.calls != 0 {
		t.Fatalf("client calls = %d, want 0", client.calls)
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
	if got := client.body.GetString("notify_url"); got != "https://gateway.example.com/v1/channel/notify" {
		t.Fatalf("notify_url = %q, want unchanged notify url", got)
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

func TestParseNotifyVerifiesAndDecryptsSuccessfulWechatPayNotify(t *testing.T) {
	publicKeyPEM, privateKey := generateWechatNotifyKey(t)
	body := signedWechatPayNotifyBody(t, privateKey, "wechat-serial-1", "SUCCESS", "pay_notify", "4200000000000000001", 9900)
	p := NewWithClientFactory(func(Config) (wechatClient, error) {
		t.Fatal("client factory should not be called for notify parsing")
		return nil, nil
	})

	result, err := p.ParseNotify(context.Background(), provider.NotifyRequest{
		ChannelAccount: account(map[string]any{
			"api_v3_key":                testAPIV3Key,
			"wechat_pay_public_key":     publicKeyPEM,
			"wechat_pay_public_key_id":  "wechat-serial-1",
			"wechat_pay_public_key_id2": "ignored",
		}),
		Header:  body.header,
		RawBody: body.raw,
	})
	if err != nil {
		t.Fatalf("ParseNotify() error = %v", err)
	}
	if result.Channel != "wechat" || result.GatewayOrderNo != "pay_notify" || result.ChannelTradeNo != "4200000000000000001" {
		t.Fatalf("result = %#v, want normalized wechat order ids", result)
	}
	if result.Status != "paid" || result.Amount != 9900 || result.Currency != "CNY" {
		t.Fatalf("result = %#v, want paid CNY 9900", result)
	}
}

func TestParseNotifyRejectsInvalidWechatSignature(t *testing.T) {
	publicKeyPEM, privateKey := generateWechatNotifyKey(t)
	body := signedWechatPayNotifyBody(t, privateKey, "wechat-serial-1", "SUCCESS", "pay_notify_bad", "4200000000000000002", 100)
	body.raw = []byte(strings.Replace(string(body.raw), "支付成功", "签名篡改", 1))
	p := NewWithClientFactory(func(Config) (wechatClient, error) {
		t.Fatal("client factory should not be called for notify parsing")
		return nil, nil
	})

	_, err := p.ParseNotify(context.Background(), provider.NotifyRequest{
		ChannelAccount: account(map[string]any{
			"api_v3_key":               testAPIV3Key,
			"wechat_pay_public_key":    publicKeyPEM,
			"wechat_pay_public_key_id": "wechat-serial-1",
		}),
		Header:  body.header,
		RawBody: body.raw,
	})
	if err == nil {
		t.Fatal("ParseNotify() error = nil, want invalid signature error")
	}
}

func account(config map[string]any) *ent.ChannelAccount {
	values := map[string]any{
		"app_id":     "wx_app",
		"mch_id":     "mch_1",
		"api_v3_key": testAPIV3Key,
		"serial_no":  "serial",
		"private_key": "test-private-key-not-used-by-notify-parser",
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

const testAPIV3Key = "12345678901234567890123456789012"

type signedWechatNotify struct {
	header map[string][]string
	raw    []byte
}

func generateWechatNotifyKey(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey() error = %v", err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	return publicPEM, privateKey
}

func signedWechatPayNotifyBody(t *testing.T, privateKey *rsa.PrivateKey, serial string, tradeState string, outTradeNo string, transactionID string, amount int64) signedWechatNotify {
	t.Helper()
	resource := map[string]any{
		"appid":            "wx_app",
		"mchid":            "mch_1",
		"out_trade_no":     outTradeNo,
		"transaction_id":   transactionID,
		"trade_state":      tradeState,
		"trade_state_desc": "支付成功",
		"amount": map[string]any{
			"total":    amount,
			"currency": "CNY",
		},
	}
	resourceJSON, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("Marshal resource error = %v", err)
	}
	nonce := "notify-nonce"
	associatedData := "transaction"
	ciphertext, err := gopayaes.GCMEncrypt(resourceJSON, []byte(nonce), []byte(associatedData), []byte(testAPIV3Key))
	if err != nil {
		t.Fatalf("GCMEncrypt() error = %v", err)
	}
	payload := map[string]any{
		"id":            "notify-id",
		"create_time":   "2026-07-09T12:00:00+08:00",
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"summary":       "支付成功",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      base64.StdEncoding.EncodeToString(ciphertext),
			"associated_data": associatedData,
			"nonce":           nonce,
			"original_type":   "transaction",
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal payload error = %v", err)
	}
	timestamp := "1783588800"
	headerNonce := "header-nonce"
	message := timestamp + "\n" + headerNonce + "\n" + string(raw) + "\n"
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15() error = %v", err)
	}
	return signedWechatNotify{
		header: map[string][]string{
			wechatv3.HeaderTimestamp: {timestamp},
			wechatv3.HeaderNonce:     {headerNonce},
			wechatv3.HeaderSignature: {base64.StdEncoding.EncodeToString(signature)},
			wechatv3.HeaderSerial:    {serial},
			"Content-Type":           {"application/json"},
		},
		raw: raw,
	}
}
