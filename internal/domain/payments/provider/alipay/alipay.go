package alipay

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-pay/gopay"
	gopayalipay "github.com/go-pay/gopay/alipay"
	gopayalipayv3 "github.com/go-pay/gopay/alipay/v3"

	"payment-gateway/internal/domain/payments/provider"
)

type alipayClient interface {
	TradePagePay(ctx context.Context, body gopay.BodyMap) (string, error)
	TradeWapPay(ctx context.Context, body gopay.BodyMap) (string, error)
	TradePrecreate(ctx context.Context, body gopay.BodyMap) (string, error)
}

type v3PrecreateClient interface {
	TradePagePay(ctx context.Context, body gopay.BodyMap) (string, error)
	TradeWapPay(ctx context.Context, body gopay.BodyMap) (string, error)
	TradePrecreate(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradePrecreateRsp, error)
}

type clientFactory func(Config) (alipayClient, error)

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
	return "alipay"
}

func (p Provider) StartPayment(ctx context.Context, req provider.StartPaymentRequest) (*provider.StartPaymentResult, error) {
	cfg, err := ParseConfig(req.ChannelAccount.Config, req.ChannelAccount.Env)
	if err != nil {
		return nil, err
	}
	client, err := p.newClient(cfg)
	if err != nil {
		return nil, err
	}
	body := gopay.BodyMap{}
	body.Set("out_trade_no", req.Order.GatewayOrderNo)
	body.Set("subject", req.Order.Subject)
	body.Set("total_amount", formatAmount(req.Order.Amount))
	if strings.TrimSpace(req.NotifyURL) != "" {
		body.Set("notify_url", strings.TrimSpace(req.NotifyURL))
	}
	if strings.TrimSpace(req.ReturnURL) != "" {
		body.Set("return_url", strings.TrimSpace(req.ReturnURL))
	}

	result := &provider.StartPaymentResult{
		Status:          "pending",
		Channel:         req.Channel,
		PayMethod:       req.PayMethod,
		ProviderOrderNo: req.Order.GatewayOrderNo,
	}
	if cfg.Mode == "qr" {
		qrCode, err := client.TradePrecreate(ctx, body)
		if err != nil {
			return nil, err
		}
		result.QRCode = qrCode
		return result, nil
	}
	body.Set("product_code", cfg.ProductCode)
	if cfg.Mode == "wap" {
		payURL, err := client.TradeWapPay(ctx, body)
		if err != nil {
			return nil, err
		}
		result.PayURL = payURL
		return result, nil
	}
	payURL, err := client.TradePagePay(ctx, body)
	if err != nil {
		return nil, err
	}
	result.PayURL = payURL
	return result, nil
}

func (p Provider) ParseNotify(ctx context.Context, req provider.NotifyRequest) (*provider.NotifyResult, error) {
	_ = ctx
	cfg, err := ParseConfig(req.ChannelAccount.Config, req.ChannelAccount.Env)
	if err != nil {
		return nil, err
	}
	httpReq := &http.Request{
		Method: "POST",
		Header: http.Header{
			"Content-Type": {"application/x-www-form-urlencoded"},
		},
		Form:     url.Values(req.Form),
		PostForm: url.Values(req.Form),
	}
	bodyMap, err := gopayalipay.ParseNotifyToBodyMap(httpReq)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.AlipayPublicKey) == "" {
		return nil, fmt.Errorf("alipay notify signature verification requires alipay_public_key")
	}
	ok, err := gopayalipay.VerifySign(cfg.AlipayPublicKey, bodyMap)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("alipay notify signature verification failed")
	}
	tradeStatus := bodyMap.GetString("trade_status")
	status := normalizeTradeStatus(tradeStatus)
	return &provider.NotifyResult{
		Channel:        "alipay",
		GatewayOrderNo: bodyMap.GetString("out_trade_no"),
		ChannelTradeNo: bodyMap.GetString("trade_no"),
		Status:         status,
		Amount:         parseAlipayAmount(bodyMap.GetString("total_amount")),
		Currency:       "CNY",
		Raw:            bodyMapToMap(bodyMap),
	}, nil
}

func newGopayClient(cfg Config) (alipayClient, error) {
	client, err := gopayalipayv3.NewClientV3(cfg.AppID, cfg.PrivateKey, cfg.IsProd)
	if err != nil {
		return nil, err
	}
	if cfg.ServerURL != "" {
		client.SetProxyHost(strings.TrimRight(cfg.ServerURL, "/"))
	}
	return &gopayClient{client: client}, nil
}

type gopayClient struct {
	client v3PrecreateClient
}

func (c *gopayClient) TradePagePay(ctx context.Context, body gopay.BodyMap) (string, error) {
	return c.client.TradePagePay(ctx, body)
}

func (c *gopayClient) TradeWapPay(ctx context.Context, body gopay.BodyMap) (string, error) {
	return c.client.TradeWapPay(ctx, body)
}

func (c *gopayClient) TradePrecreate(ctx context.Context, body gopay.BodyMap) (string, error) {
	rsp, err := c.client.TradePrecreate(ctx, body)
	if err != nil {
		return "", err
	}
	if rsp != nil && rsp.StatusCode != http.StatusOK {
		return "", alipayV3ResponseError(rsp)
	}
	if rsp == nil || strings.TrimSpace(rsp.QrCode) == "" {
		return "", fmt.Errorf("alipay precreate response is empty")
	}
	return rsp.QrCode, nil
}

func alipayV3ResponseError(rsp *gopayalipayv3.TradePrecreateRsp) error {
	if rsp == nil {
		return fmt.Errorf("alipay precreate response is empty")
	}
	errResp := rsp.ErrResponse
	parts := []string{fmt.Sprintf("alipay precreate failed: status_code=%d", rsp.StatusCode)}
	if strings.TrimSpace(errResp.Code) != "" {
		parts = append(parts, "code="+strings.TrimSpace(errResp.Code))
	}
	if strings.TrimSpace(errResp.Message) != "" {
		parts = append(parts, "message="+strings.TrimSpace(errResp.Message))
	}
	for _, detail := range errResp.Details {
		if detail == nil {
			continue
		}
		if strings.TrimSpace(detail.Issue) != "" {
			parts = append(parts, "issue="+strings.TrimSpace(detail.Issue))
		}
		if strings.TrimSpace(detail.Description) != "" {
			parts = append(parts, "description="+strings.TrimSpace(detail.Description))
		}
	}
	return errors.New(strings.Join(parts, ", "))
}

func formatAmount(amount int64) string {
	return fmt.Sprintf("%d.%02d", amount/100, amount%100)
}

func normalizeTradeStatus(status string) string {
	switch status {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		return "paid"
	case "TRADE_CLOSED":
		return "closed"
	default:
		return "pending"
	}
}

func parseAlipayAmount(value string) int64 {
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int64(amount*100 + 0.5)
}

func bodyMapToMap(bodyMap gopay.BodyMap) map[string]any {
	raw := make(map[string]any, len(bodyMap))
	for key, value := range bodyMap {
		raw[key] = value
	}
	return raw
}
