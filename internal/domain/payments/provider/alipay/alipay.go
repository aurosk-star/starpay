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

	"payment-gateway/ent"
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
	TradeQuery(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeQueryRsp, error)
	TradeClose(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeCloseRsp, error)
	TradeRefund(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeRefundRsp, error)
	TradeFastPayRefundQuery(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeFastPayRefundQueryRsp, error)
}

type alipayOperationsClient interface {
	TradeQuery(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeQueryRsp, error)
	TradeClose(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeCloseRsp, error)
	TradeRefund(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeRefundRsp, error)
	TradeFastPayRefundQuery(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeFastPayRefundQueryRsp, error)
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
	if !alipayModeEnabled(cfg) {
		return nil, fmt.Errorf("alipay %s payment mode is disabled", cfg.Mode)
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

func alipayModeEnabled(cfg Config) bool {
	switch cfg.Mode {
	case "page":
		return cfg.EnablePagePay
	case "wap":
		return cfg.EnableWapPay
	case "qr":
		return cfg.EnableQRPay
	default:
		return false
	}
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
	amount, err := parseAlipayAmount(bodyMap.GetString("total_amount"))
	if err != nil {
		return nil, err
	}
	return &provider.NotifyResult{
		Channel:        "alipay",
		GatewayOrderNo: bodyMap.GetString("out_trade_no"),
		ChannelTradeNo: bodyMap.GetString("trade_no"),
		Status:         status,
		Amount:         amount,
		Currency:       "CNY",
		FailureReason:  tradeStatus,
		Raw:            bodyMapToMap(bodyMap),
	}, nil
}

func (p Provider) QueryPayment(ctx context.Context, req provider.QueryPaymentRequest) (*provider.QueryPaymentResult, error) {
	if req.Order == nil {
		return nil, fmt.Errorf("alipay order is required")
	}
	client, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return nil, err
	}
	body := gopay.BodyMap{}
	body.Set("out_trade_no", req.Order.GatewayOrderNo)
	rsp, err := client.TradeQuery(ctx, body)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.StatusCode != http.StatusOK {
		if rsp != nil && isTradeNotExist(rsp.ErrResponse) {
			return &provider.QueryPaymentResult{
				Channel:         "alipay",
				GatewayOrderNo:  req.Order.GatewayOrderNo,
				ProviderOrderNo: req.Order.ProviderOrderNo,
				Status:          "pending",
				Amount:          req.Order.Amount,
				Currency:        req.Order.Currency,
				FailureReason:   strings.TrimSpace(rsp.ErrResponse.Code),
				Raw: map[string]any{
					"status_code": rsp.StatusCode,
					"code":        strings.TrimSpace(rsp.ErrResponse.Code),
					"message":     strings.TrimSpace(rsp.ErrResponse.Message),
				},
			}, nil
		}
		return nil, alipayAPIError("trade query", alipayQueryStatusCode(rsp), alipayQueryError(rsp))
	}
	amount, err := parseAlipayAmount(rsp.TotalAmount)
	if err != nil {
		return nil, err
	}
	return &provider.QueryPaymentResult{
		Channel:         "alipay",
		GatewayOrderNo:  strings.TrimSpace(rsp.OutTradeNo),
		ProviderOrderNo: strings.TrimSpace(req.Order.ProviderOrderNo),
		ChannelTradeNo:  strings.TrimSpace(rsp.TradeNo),
		Status:          normalizeTradeStatus(rsp.TradeStatus),
		Amount:          amount,
		Currency:        "CNY",
		FailureReason:   strings.TrimSpace(rsp.TradeStatus),
		Raw: map[string]any{
			"trade_no": rsp.TradeNo, "out_trade_no": rsp.OutTradeNo,
			"trade_status": rsp.TradeStatus, "total_amount": rsp.TotalAmount,
		},
	}, nil
}

func (p Provider) ClosePayment(ctx context.Context, req provider.ClosePaymentRequest) error {
	if req.Order == nil {
		return fmt.Errorf("alipay order is required")
	}
	client, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return err
	}
	body := gopay.BodyMap{}
	body.Set("out_trade_no", req.Order.GatewayOrderNo)
	if strings.TrimSpace(req.Order.ChannelTradeNo) != "" {
		body.Set("trade_no", req.Order.ChannelTradeNo)
	}
	rsp, err := client.TradeClose(ctx, body)
	if err != nil {
		return err
	}
	if rsp == nil || rsp.StatusCode != http.StatusOK {
		if rsp != nil && isTradeNotExist(rsp.ErrResponse) {
			return nil
		}
		return alipayAPIError("trade close", alipayCloseStatusCode(rsp), alipayCloseError(rsp))
	}
	return nil
}

func isTradeNotExist(errResp gopayalipayv3.ErrResponse) bool {
	return strings.EqualFold(strings.TrimSpace(errResp.Code), "ACQ.TRADE_NOT_EXIST")
}

func (p Provider) CreateRefund(ctx context.Context, req provider.CreateRefundRequest) (*provider.RefundResult, error) {
	client, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return nil, err
	}
	body := gopay.BodyMap{}
	body.Set("out_trade_no", req.GatewayOrderNo).
		Set("refund_amount", formatAmount(req.Amount)).
		Set("out_request_no", req.RefundNo)
	if strings.TrimSpace(req.ChannelTradeNo) != "" {
		body.Set("trade_no", req.ChannelTradeNo)
	}
	if strings.TrimSpace(req.Reason) != "" {
		body.Set("refund_reason", strings.TrimSpace(req.Reason))
	}
	rsp, err := client.TradeRefund(ctx, body)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.StatusCode != http.StatusOK {
		return nil, alipayAPIError("trade refund", alipayRefundStatusCode(rsp), alipayRefundError(rsp))
	}
	amount, err := parseAlipayAmount(rsp.RefundFee)
	if err != nil {
		return nil, err
	}
	return &provider.RefundResult{
		Channel: "alipay", RefundNo: req.RefundNo, ChannelRefundNo: req.RefundNo,
		Status: "succeeded", Amount: amount, Currency: "CNY",
		Raw: map[string]any{"trade_no": rsp.TradeNo, "out_trade_no": rsp.OutTradeNo, "fund_change": rsp.FundChange, "refund_fee": rsp.RefundFee},
	}, nil
}

func (p Provider) QueryRefund(ctx context.Context, req provider.QueryRefundRequest) (*provider.RefundResult, error) {
	client, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return nil, err
	}
	body := gopay.BodyMap{}
	body.Set("out_trade_no", req.GatewayOrderNo).Set("out_request_no", req.RefundNo)
	if strings.TrimSpace(req.ChannelTradeNo) != "" {
		body.Set("trade_no", req.ChannelTradeNo)
	}
	rsp, err := client.TradeFastPayRefundQuery(ctx, body)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.StatusCode != http.StatusOK {
		return nil, alipayAPIError("refund query", alipayRefundQueryStatusCode(rsp), alipayRefundQueryError(rsp))
	}
	amount, err := parseAlipayAmount(rsp.RefundAmount)
	if err != nil {
		return nil, err
	}
	status := "pending"
	if strings.EqualFold(strings.TrimSpace(rsp.RefundStatus), "REFUND_SUCCESS") {
		status = "succeeded"
	}
	return &provider.RefundResult{
		Channel: "alipay", RefundNo: strings.TrimSpace(rsp.OutRequestNo), ChannelRefundNo: req.RefundNo,
		Status: status, Amount: amount, Currency: "CNY", FailureReason: strings.TrimSpace(rsp.RefundStatus),
		Raw: map[string]any{"trade_no": rsp.TradeNo, "out_trade_no": rsp.OutTradeNo, "out_request_no": rsp.OutRequestNo, "refund_status": rsp.RefundStatus, "refund_amount": rsp.RefundAmount},
	}, nil
}

func (p Provider) operationsClient(account *ent.ChannelAccount) (alipayOperationsClient, error) {
	if account == nil {
		return nil, fmt.Errorf("alipay channel account is required")
	}
	cfg, err := ParseConfig(account.Config, account.Env)
	if err != nil {
		return nil, err
	}
	client, err := p.newClient(cfg)
	if err != nil {
		return nil, err
	}
	operations, ok := client.(alipayOperationsClient)
	if !ok {
		return nil, fmt.Errorf("alipay client does not support payment operations")
	}
	return operations, nil
}

func newGopayClient(cfg Config) (alipayClient, error) {
	client, err := gopayalipayv3.NewClientV3(cfg.AppID, cfg.PrivateKey, cfg.IsProd)
	if err != nil {
		return nil, err
	}
	if proxyHost := normalizeAlipayV3ProxyHost(cfg.ServerURL); proxyHost != "" {
		client.SetProxyHost(proxyHost)
	}
	return &gopayClient{client: client}, nil
}

func normalizeAlipayV3ProxyHost(serverURL string) string {
	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if serverURL == "" {
		return ""
	}
	return strings.TrimSuffix(serverURL, "/gateway.do")
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

func (c *gopayClient) TradeQuery(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeQueryRsp, error) {
	return c.client.TradeQuery(ctx, body)
}

func (c *gopayClient) TradeClose(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeCloseRsp, error) {
	return c.client.TradeClose(ctx, body)
}

func (c *gopayClient) TradeRefund(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeRefundRsp, error) {
	return c.client.TradeRefund(ctx, body)
}

func (c *gopayClient) TradeFastPayRefundQuery(ctx context.Context, body gopay.BodyMap) (*gopayalipayv3.TradeFastPayRefundQueryRsp, error) {
	return c.client.TradeFastPayRefundQuery(ctx, body)
}

func alipayAPIError(action string, status int, errResp gopayalipayv3.ErrResponse) error {
	parts := []string{fmt.Sprintf("alipay %s failed: status_code=%d", action, status)}
	if strings.TrimSpace(errResp.Code) != "" {
		parts = append(parts, "code="+strings.TrimSpace(errResp.Code))
	}
	if strings.TrimSpace(errResp.Message) != "" {
		parts = append(parts, "message="+strings.TrimSpace(errResp.Message))
	}
	return errors.New(strings.Join(parts, ", "))
}

func alipayQueryStatusCode(rsp *gopayalipayv3.TradeQueryRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.StatusCode
}
func alipayQueryError(rsp *gopayalipayv3.TradeQueryRsp) gopayalipayv3.ErrResponse {
	if rsp == nil {
		return gopayalipayv3.ErrResponse{}
	}
	return rsp.ErrResponse
}
func alipayCloseStatusCode(rsp *gopayalipayv3.TradeCloseRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.StatusCode
}
func alipayCloseError(rsp *gopayalipayv3.TradeCloseRsp) gopayalipayv3.ErrResponse {
	if rsp == nil {
		return gopayalipayv3.ErrResponse{}
	}
	return rsp.ErrResponse
}
func alipayRefundStatusCode(rsp *gopayalipayv3.TradeRefundRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.StatusCode
}
func alipayRefundError(rsp *gopayalipayv3.TradeRefundRsp) gopayalipayv3.ErrResponse {
	if rsp == nil {
		return gopayalipayv3.ErrResponse{}
	}
	return rsp.ErrResponse
}
func alipayRefundQueryStatusCode(rsp *gopayalipayv3.TradeFastPayRefundQueryRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.StatusCode
}
func alipayRefundQueryError(rsp *gopayalipayv3.TradeFastPayRefundQueryRsp) gopayalipayv3.ErrResponse {
	if rsp == nil {
		return gopayalipayv3.ErrResponse{}
	}
	return rsp.ErrResponse
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

func parseAlipayAmount(value string) (int64, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid alipay amount: %s", value)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, fmt.Errorf("invalid alipay amount: %s", value)
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return 0, fmt.Errorf("invalid alipay amount: %s", value)
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	cents := int64(0)
	if fraction != "" {
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid alipay amount: %s", value)
		}
	}
	return whole*100 + cents, nil
}

func bodyMapToMap(bodyMap gopay.BodyMap) map[string]any {
	raw := make(map[string]any, len(bodyMap))
	for key, value := range bodyMap {
		raw[key] = value
	}
	return raw
}
