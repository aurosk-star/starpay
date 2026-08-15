package paymentstest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymenthandler "payment-gateway/internal/domain/payments/handler"
	paymentprovider "payment-gateway/internal/domain/payments/provider"
	wechatprovider "payment-gateway/internal/domain/payments/provider/wechat"
	paymentrouter "payment-gateway/internal/domain/payments/router"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	webhookrepo "payment-gateway/internal/domain/webhooks/repository"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"

	gopayaes "github.com/go-pay/crypto/aes"
	wechatv3 "github.com/go-pay/gopay/wechat/v3"
)

func TestNotifyHandlerMarksOrderPaidAndReturnsAlipaySuccess(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctx := t.Context()
	client := enttest.Open(t, dialect.SQLite, "file:notify_handler_paid?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledAppWithNotifyURL(t, client, "snsgo", "https://merchant.example.com/payment/webhook")

	orderService := ordersvc.New(client, ordersvc.WithWebhookService(webhooksvc.New(client)))
	order, err := orderService.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_notify_handler",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	channels := channelrepo.New(client)
	if _, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{
		Channel: "alipay",
		Name:    "Alipay Sandbox",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("Create channel account error = %v", err)
	}
	provider := &fakeNotifyProvider{
		channel: "alipay",
		result: &paymentprovider.NotifyResult{
			Channel:        "alipay",
			GatewayOrderNo: order.GatewayOrderNo,
			ChannelTradeNo: "2026070122000000001",
			Status:         "paid",
			Amount:         9900,
			Currency:       "CNY",
		},
	}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channels),
		paymentsvc.WithProvider(provider),
	)

	router := gin.New()
	paymentrouter.RegisterNotify(router.Group("/v1/channel"), paymenthandler.NewNotify(paymentService, orderService))

	form := url.Values{}
	form.Set("out_trade_no", order.GatewayOrderNo)
	req := httptest.NewRequest(http.MethodPost, "/v1/channel/notify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "success" {
		t.Fatalf("body = %q, want success", recorder.Body.String())
	}
	updated, err := orderService.FindOrder(ctx, order.ID)
	if err != nil {
		t.Fatalf("FindOrder() error = %v", err)
	}
	if updated.Status != "paid" || updated.ChannelTradeNo != "2026070122000000001" || updated.PaidAt == nil {
		t.Fatalf("updated order = %#v, want paid with channel trade no", updated)
	}
	_, totalEvents, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	deliveries, totalDeliveries, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if totalEvents != 1 || totalDeliveries != 1 {
		t.Fatalf("webhook totals events=%d deliveries=%d, want one payment.succeeded delivery", totalEvents, totalDeliveries)
	}
	if deliveries[0].EventType != webhooksvc.EventPaymentSucceeded || deliveries[0].TargetURL != "https://merchant.example.com/payment/webhook" {
		t.Fatalf("delivery = %#v, want payment.succeeded to merchant notify url", deliveries[0])
	}
}

func TestNotifyHandlerRejectsPaidAmountMismatch(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctx := t.Context()
	client := enttest.Open(t, dialect.SQLite, "file:notify_handler_amount_mismatch?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	createEnabledAppWithNotifyURL(t, client, "snsgo", "")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(ctx, ordersvc.ManageOrderInput{AppID: "snsgo", MerchantOrderNo: "biz_amount_mismatch", Subject: "Pro", Amount: 9900, Currency: "CNY", Channel: "alipay", PayMethod: "alipay"})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	channels := channelrepo.New(client)
	if _, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{Channel: "alipay", Name: "Alipay", Enabled: true, Env: "prod", Config: map[string]any{}}); err != nil {
		t.Fatalf("Create account error = %v", err)
	}
	provider := &fakeNotifyProvider{channel: "alipay", result: &paymentprovider.NotifyResult{Channel: "alipay", GatewayOrderNo: order.GatewayOrderNo, ChannelTradeNo: "trade_bad", Status: "paid", Amount: 9800, Currency: "CNY"}}
	paymentService := paymentsvc.New(paymentsvc.WithChannelRepository(channels), paymentsvc.WithProvider(provider))
	router := gin.New()
	paymentrouter.RegisterNotify(router.Group("/v1/channel"), paymenthandler.NewNotify(paymentService, orderService))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/channel/notify", strings.NewReader("out_trade_no="+order.GatewayOrderNo))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(recorder, req)

	if recorder.Body.String() != "fail" {
		t.Fatalf("body = %q, want fail", recorder.Body.String())
	}
	stored, _ := orderService.FindOrder(ctx, order.ID)
	if stored.Status != "pending" {
		t.Fatalf("Status = %q, want pending", stored.Status)
	}
}

func TestNotifyHandlerMarksOrderFailedAndRecordsWebhook(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctx := t.Context()
	client := enttest.Open(t, dialect.SQLite, "file:notify_handler_failed?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	createEnabledAppWithNotifyURL(t, client, "snsgo", "https://merchant.example.com/payment/webhook")

	orderService := ordersvc.New(client, ordersvc.WithWebhookService(webhooksvc.New(client)))
	order, err := orderService.CreateOrder(ctx, ordersvc.ManageOrderInput{AppID: "snsgo", MerchantOrderNo: "biz_failed", Subject: "Pro", Amount: 9900, Currency: "CNY", Channel: "wechat", PayMethod: "wechat"})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	channels := channelrepo.New(client)
	if _, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{Channel: "wechat", Name: "Wechat", Enabled: true, Env: "prod", Config: map[string]any{}}); err != nil {
		t.Fatalf("Create account error = %v", err)
	}
	provider := &fakeNotifyProvider{channel: "wechat", result: &paymentprovider.NotifyResult{Channel: "wechat", GatewayOrderNo: order.GatewayOrderNo, Status: "failed", FailureReason: "PAYERROR"}}
	paymentService := paymentsvc.New(paymentsvc.WithChannelRepository(channels), paymentsvc.WithProvider(provider))
	router := gin.New()
	paymentrouter.RegisterNotify(router.Group("/v1/channel"), paymenthandler.NewNotify(paymentService, orderService))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/channel/notify?channel=wechat", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != `{"code":"SUCCESS","message":"成功"}` {
		t.Fatalf("status/body = %d/%q", recorder.Code, recorder.Body.String())
	}
	stored, _ := orderService.FindOrder(ctx, order.ID)
	if stored.Status != "failed" || stored.FailureReason != "PAYERROR" || stored.FailedAt == nil {
		t.Fatalf("stored = %#v, want failed", stored)
	}
	_, total, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{EventType: webhooksvc.EventPaymentFailed})
	if err != nil || total != 1 {
		t.Fatalf("payment.failed events total=%d err=%v", total, err)
	}
}

func TestNotifyHandlerMarksOrderPaidAndReturnsWechatSuccess(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctx := t.Context()
	client := enttest.Open(t, dialect.SQLite, "file:notify_handler_wechat_paid?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledAppWithNotifyURL(t, client, "snsgo", "https://merchant.example.com/payment/webhook")

	orderService := ordersvc.New(client, ordersvc.WithWebhookService(webhooksvc.New(client)))
	order, err := orderService.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_notify_handler_wechat",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "wechat",
		Channel:         "wechat",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	publicKeyPEM, privateKey := generateWechatNotifyKey(t)
	channels := channelrepo.New(client)
	if _, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{
		Channel: "wechat",
		Name:    "Wechat Pay",
		Enabled: true,
		Env:     "prod",
		Config: map[string]any{
			"app_id":                   "wx_app",
			"mch_id":                   "mch_1",
			"api_v3_key":               testWechatAPIV3Key,
			"serial_no":                "merchant-serial",
			"private_key":              "test-private-key-not-used-by-notify-parser",
			"wechat_pay_public_key":    publicKeyPEM,
			"wechat_pay_public_key_id": "platform-serial-1",
			"mode":                     "native",
		},
	}); err != nil {
		t.Fatalf("Create channel account error = %v", err)
	}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channels),
		paymentsvc.WithProvider(wechatprovider.New()),
	)

	router := gin.New()
	paymentrouter.RegisterNotify(router.Group("/v1/channel"), paymenthandler.NewNotify(paymentService, orderService))

	body := signedWechatPayNotifyBody(t, privateKey, "platform-serial-1", "SUCCESS", order.GatewayOrderNo, "4200000000000000001", 9900)
	req := httptest.NewRequest(http.MethodPost, "/v1/channel/notify", strings.NewReader(string(body.raw)))
	for key, values := range body.header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.TrimSpace(recorder.Body.String()) != `{"code":"SUCCESS","message":"成功"}` {
		t.Fatalf("body = %q, want wechat success json", recorder.Body.String())
	}
	updated, err := orderService.FindOrder(ctx, order.ID)
	if err != nil {
		t.Fatalf("FindOrder() error = %v", err)
	}
	if updated.Status != "paid" || updated.ChannelTradeNo != "4200000000000000001" || updated.PaidAt == nil {
		t.Fatalf("updated order = %#v, want paid with wechat transaction id", updated)
	}
}

const testWechatAPIV3Key = "12345678901234567890123456789012"

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
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})), privateKey
}

func signedWechatPayNotifyBody(t *testing.T, privateKey *rsa.PrivateKey, serial string, tradeState string, outTradeNo string, transactionID string, amount int64) signedWechatNotify {
	t.Helper()
	resourceJSON, err := json.Marshal(map[string]any{
		"appid":          "wx_app",
		"mchid":          "mch_1",
		"out_trade_no":   outTradeNo,
		"transaction_id": transactionID,
		"trade_state":    tradeState,
		"amount": map[string]any{
			"total":    amount,
			"currency": "CNY",
		},
	})
	if err != nil {
		t.Fatalf("Marshal resource error = %v", err)
	}
	nonce := "notify-nonce"
	associatedData := "transaction"
	ciphertext, err := gopayaes.GCMEncrypt(resourceJSON, []byte(nonce), []byte(associatedData), []byte(testWechatAPIV3Key))
	if err != nil {
		t.Fatalf("GCMEncrypt() error = %v", err)
	}
	raw, err := json.Marshal(map[string]any{
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
	})
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
