package wechat

import (
	"bytes"
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-pay/crypto/xpem"
	"github.com/go-pay/gopay"
	wechatv3 "github.com/go-pay/gopay/wechat/v3"

	"payment-gateway/internal/domain/payments/provider"
)

type wechatClient interface {
	V3TransactionNative(ctx context.Context, body gopay.BodyMap) (*wechatv3.NativeRsp, error)
	V3TransactionH5(ctx context.Context, body gopay.BodyMap) (*wechatv3.H5Rsp, error)
}

type clientFactory func(Config) (wechatClient, error)

type Provider struct {
	newClient clientFactory
}

func New() Provider {
	return NewWithClientFactory(newGopayClient)
}

func NewWithClientFactory(factory clientFactory) Provider {
	return Provider{newClient: factory}
}

func (Provider) Channel() string {
	return "wechat"
}

func (p Provider) StartPayment(ctx context.Context, req provider.StartPaymentRequest) (*provider.StartPaymentResult, error) {
	cfg, err := ParseConfig(req.ChannelAccount.Config, req.ChannelAccount.Env)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(req.Order.Currency), "CNY") {
		return nil, fmt.Errorf("wechat only supports CNY currency")
	}
	client, err := p.newClient(cfg)
	if err != nil {
		return nil, err
	}

	body := basePaymentBody(cfg, req)
	result := &provider.StartPaymentResult{
		Status:          "pending",
		Channel:         req.Channel,
		PayMethod:       req.PayMethod,
		ProviderOrderNo: req.Order.GatewayOrderNo,
	}
	switch cfg.Mode {
	case "h5":
		body.SetBodyMap("scene_info", func(scene gopay.BodyMap) {
			scene.Set("payer_client_ip", payerClientIP(req.ClientIP)).
				SetBodyMap("h5_info", func(info gopay.BodyMap) {
					info.Set("type", "Wap")
				})
		})
		rsp, err := client.V3TransactionH5(ctx, body)
		if err != nil {
			return nil, err
		}
		if rsp == nil || rsp.Code != wechatv3.Success {
			return nil, wechatResponseError("h5 order", h5StatusCode(rsp), h5ErrorText(rsp))
		}
		if rsp.Response == nil || strings.TrimSpace(rsp.Response.H5Url) == "" {
			return nil, fmt.Errorf("wechat h5 order response is empty")
		}
		result.PayURL = rsp.Response.H5Url
		result.Raw = map[string]any{"wechat_trade_type": "MWEB"}
		return result, nil
	case "native", "qr":
		rsp, err := client.V3TransactionNative(ctx, body)
		if err != nil {
			return nil, err
		}
		if rsp == nil || rsp.Code != wechatv3.Success {
			return nil, wechatResponseError("native order", nativeStatusCode(rsp), nativeErrorText(rsp))
		}
		if rsp.Response == nil || strings.TrimSpace(rsp.Response.CodeUrl) == "" {
			return nil, fmt.Errorf("wechat native order response is empty")
		}
		result.QRCode = rsp.Response.CodeUrl
		result.Raw = map[string]any{"wechat_trade_type": "NATIVE"}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported wechat payment mode: %s", cfg.Mode)
	}
}

func (p Provider) ParseNotify(ctx context.Context, req provider.NotifyRequest) (*provider.NotifyResult, error) {
	_ = ctx
	cfg, err := ParseConfig(req.ChannelAccount.Config, req.ChannelAccount.Env)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, "https://gateway.local/v1/channel/notify", bytes.NewReader(req.RawBody))
	if err != nil {
		return nil, err
	}
	for key, values := range req.Header {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}
	notifyReq, err := wechatv3.V3ParseNotify(httpReq)
	if err != nil {
		return nil, err
	}
	publicKeys, err := wechatPublicKeyMap(cfg)
	if err != nil {
		return nil, err
	}
	if err := notifyReq.VerifySignByPKMap(publicKeys); err != nil {
		return nil, err
	}
	payResult, err := notifyReq.DecryptPayCipherText(cfg.APIV3Key)
	if err != nil {
		return nil, err
	}
	amount := int64(0)
	currency := "CNY"
	if payResult.Amount != nil {
		amount = int64(payResult.Amount.Total)
		if strings.TrimSpace(payResult.Amount.Currency) != "" {
			currency = strings.TrimSpace(payResult.Amount.Currency)
		}
	}
	return &provider.NotifyResult{
		Channel:        "wechat",
		GatewayOrderNo: strings.TrimSpace(payResult.OutTradeNo),
		ChannelTradeNo: strings.TrimSpace(payResult.TransactionId),
		Status:         mapWechatTradeState(payResult.TradeState),
		Amount:         amount,
		Currency:       currency,
		Raw: map[string]any{
			"event_type":  notifyReq.EventType,
			"trade_state": payResult.TradeState,
			"summary":     notifyReq.Summary,
		},
	}, nil
}

func wechatPublicKeyMap(cfg Config) (map[string]*rsa.PublicKey, error) {
	publicKeyID := strings.TrimSpace(cfg.WechatPayPublicKeyID)
	publicKeyContent := strings.TrimSpace(cfg.WechatPayPublicKey)
	if publicKeyID == "" || publicKeyContent == "" {
		return nil, fmt.Errorf("wechat notify public key and public key id are required")
	}
	publicKey, err := xpem.DecodePublicKey([]byte(publicKeyContent))
	if err != nil {
		return nil, err
	}
	return map[string]*rsa.PublicKey{publicKeyID: publicKey}, nil
}

func mapWechatTradeState(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "SUCCESS":
		return "paid"
	case "CLOSED", "REVOKED":
		return "closed"
	case "PAYERROR":
		return "failed"
	default:
		return "pending"
	}
}

func newGopayClient(cfg Config) (wechatClient, error) {
	client, err := wechatv3.NewClientV3(cfg.MchID, cfg.SerialNo, cfg.APIV3Key, cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.WechatPayPublicKey) != "" && strings.TrimSpace(cfg.WechatPayPublicKeyID) != "" {
		if err := client.AutoVerifySignByPublicKey([]byte(cfg.WechatPayPublicKey), cfg.WechatPayPublicKeyID); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func basePaymentBody(cfg Config, req provider.StartPaymentRequest) gopay.BodyMap {
	body := gopay.BodyMap{}
	body.Set("appid", cfg.AppID).
		Set("mchid", cfg.MchID).
		Set("description", req.Order.Subject).
		Set("out_trade_no", req.Order.GatewayOrderNo).
		SetBodyMap("amount", func(amount gopay.BodyMap) {
			amount.Set("total", req.Order.Amount).
				Set("currency", "CNY")
		})
	if strings.TrimSpace(req.NotifyURL) != "" {
		body.Set("notify_url", strings.TrimSpace(req.NotifyURL))
	}
	return body
}

func payerClientIP(value string) string {
	if strings.TrimSpace(value) == "" {
		return "127.0.0.1"
	}
	return strings.TrimSpace(value)
}

func nativeStatusCode(rsp *wechatv3.NativeRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.Code
}

func h5StatusCode(rsp *wechatv3.H5Rsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.Code
}

func nativeErrorText(rsp *wechatv3.NativeRsp) string {
	if rsp == nil {
		return "empty response"
	}
	return formatWechatError(rsp.ErrResponse, rsp.Error)
}

func h5ErrorText(rsp *wechatv3.H5Rsp) string {
	if rsp == nil {
		return "empty response"
	}
	return formatWechatError(rsp.ErrResponse, rsp.Error)
}

func formatWechatError(errResp wechatv3.ErrResponse, raw string) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(errResp.Code) != "" {
		parts = append(parts, "code="+strings.TrimSpace(errResp.Code))
	}
	if strings.TrimSpace(errResp.Message) != "" {
		parts = append(parts, "message="+strings.TrimSpace(errResp.Message))
	}
	if strings.TrimSpace(raw) != "" {
		parts = append(parts, "raw="+strings.TrimSpace(raw))
	}
	if len(parts) == 0 {
		return "empty error detail"
	}
	return strings.Join(parts, ", ")
}

func wechatResponseError(action string, code int, detail string) error {
	return fmt.Errorf("wechat %s failed: code=%d, %s", action, code, detail)
}
