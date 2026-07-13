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

	"payment-gateway/ent"
	"payment-gateway/internal/domain/payments/provider"
)

type wechatClient interface {
	V3TransactionNative(ctx context.Context, body gopay.BodyMap) (*wechatv3.NativeRsp, error)
	V3TransactionH5(ctx context.Context, body gopay.BodyMap) (*wechatv3.H5Rsp, error)
}

type wechatOperationsClient interface {
	V3TransactionQueryOrder(ctx context.Context, orderNoType wechatv3.OrderNoType, orderNo string) (*wechatv3.QueryOrderRsp, error)
	V3TransactionCloseOrder(ctx context.Context, tradeNo string) (*wechatv3.EmptyRsp, error)
	V3Refund(ctx context.Context, body gopay.BodyMap) (*wechatv3.RefundRsp, error)
	V3RefundQuery(ctx context.Context, outRefundNo string, body gopay.BodyMap) (*wechatv3.RefundQueryRsp, error)
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
	if !wechatModeEnabled(cfg) {
		return nil, fmt.Errorf("wechat %s payment mode is disabled", cfg.Mode)
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

func wechatModeEnabled(cfg Config) bool {
	switch cfg.Mode {
	case "native", "qr":
		return cfg.EnableNativePay
	case "h5":
		return cfg.EnableH5Pay
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
		FailureReason:  strings.TrimSpace(payResult.TradeState),
		Raw: map[string]any{
			"event_type":  notifyReq.EventType,
			"trade_state": payResult.TradeState,
			"summary":     notifyReq.Summary,
		},
	}, nil
}

func (p Provider) QueryPayment(ctx context.Context, req provider.QueryPaymentRequest) (*provider.QueryPaymentResult, error) {
	if req.Order == nil {
		return nil, fmt.Errorf("wechat order is required")
	}
	client, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return nil, err
	}
	rsp, err := client.V3TransactionQueryOrder(ctx, wechatv3.OutTradeNo, req.Order.GatewayOrderNo)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.Code != wechatv3.Success {
		return nil, wechatResponseError("query order", wechatQueryCode(rsp), wechatQueryError(rsp))
	}
	if rsp.Response == nil {
		return nil, fmt.Errorf("wechat query order response is empty")
	}
	amount := int64(0)
	currency := "CNY"
	if rsp.Response.Amount != nil {
		amount = int64(rsp.Response.Amount.Total)
		if strings.TrimSpace(rsp.Response.Amount.Currency) != "" {
			currency = strings.ToUpper(strings.TrimSpace(rsp.Response.Amount.Currency))
		}
	}
	return &provider.QueryPaymentResult{
		Channel: "wechat", GatewayOrderNo: strings.TrimSpace(rsp.Response.OutTradeNo),
		ProviderOrderNo: strings.TrimSpace(req.Order.ProviderOrderNo), ChannelTradeNo: strings.TrimSpace(rsp.Response.TransactionId),
		Status: mapWechatTradeState(rsp.Response.TradeState), Amount: amount, Currency: currency,
		FailureReason: strings.TrimSpace(rsp.Response.TradeStateDesc),
		Raw:           map[string]any{"out_trade_no": rsp.Response.OutTradeNo, "transaction_id": rsp.Response.TransactionId, "trade_state": rsp.Response.TradeState, "trade_state_desc": rsp.Response.TradeStateDesc},
	}, nil
}

func (p Provider) ClosePayment(ctx context.Context, req provider.ClosePaymentRequest) error {
	if req.Order == nil {
		return fmt.Errorf("wechat order is required")
	}
	client, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return err
	}
	rsp, err := client.V3TransactionCloseOrder(ctx, req.Order.GatewayOrderNo)
	if err != nil {
		return err
	}
	if rsp == nil || rsp.Code != wechatv3.Success {
		return wechatResponseError("close order", wechatEmptyCode(rsp), wechatEmptyError(rsp))
	}
	return nil
}

func (p Provider) CreateRefund(ctx context.Context, req provider.CreateRefundRequest) (*provider.RefundResult, error) {
	if req.Amount <= 0 || req.OriginalAmount <= 0 {
		return nil, fmt.Errorf("wechat refund and original amounts must be positive")
	}
	if !strings.EqualFold(strings.TrimSpace(req.Currency), "CNY") {
		return nil, fmt.Errorf("wechat refunds only support CNY currency")
	}
	client, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return nil, err
	}
	body := gopay.BodyMap{}
	body.Set("out_trade_no", req.GatewayOrderNo).
		Set("out_refund_no", req.RefundNo).
		SetBodyMap("amount", func(amount gopay.BodyMap) {
			amount.Set("refund", req.Amount).Set("total", req.OriginalAmount).Set("currency", "CNY")
		})
	if strings.TrimSpace(req.ChannelTradeNo) != "" {
		body.Set("transaction_id", req.ChannelTradeNo)
	}
	if strings.TrimSpace(req.Reason) != "" {
		body.Set("reason", strings.TrimSpace(req.Reason))
	}
	rsp, err := client.V3Refund(ctx, body)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.Code != wechatv3.Success {
		return nil, wechatResponseError("refund", wechatRefundCode(rsp), wechatRefundError(rsp))
	}
	if rsp.Response == nil {
		return nil, fmt.Errorf("wechat refund response is empty")
	}
	return normalizeWechatRefund(req.RefundNo, rsp.Response), nil
}

func (p Provider) QueryRefund(ctx context.Context, req provider.QueryRefundRequest) (*provider.RefundResult, error) {
	client, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return nil, err
	}
	rsp, err := client.V3RefundQuery(ctx, req.RefundNo, nil)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.Code != wechatv3.Success {
		return nil, wechatResponseError("refund query", wechatRefundQueryCode(rsp), wechatRefundQueryError(rsp))
	}
	if rsp.Response == nil {
		return nil, fmt.Errorf("wechat refund query response is empty")
	}
	return normalizeWechatRefundQuery(req.RefundNo, rsp.Response), nil
}

func (p Provider) operationsClient(account *ent.ChannelAccount) (wechatOperationsClient, error) {
	if account == nil {
		return nil, fmt.Errorf("wechat channel account is required")
	}
	cfg, err := ParseConfig(account.Config, account.Env)
	if err != nil {
		return nil, err
	}
	client, err := p.newClient(cfg)
	if err != nil {
		return nil, err
	}
	operations, ok := client.(wechatOperationsClient)
	if !ok {
		return nil, fmt.Errorf("wechat client does not support payment operations")
	}
	return operations, nil
}

func normalizeWechatRefund(fallbackRefundNo string, response *wechatv3.RefundOrderResponse) *provider.RefundResult {
	amount, currency := wechatRefundAmount(response.Amount)
	refundNo := strings.TrimSpace(response.OutRefundNo)
	if refundNo == "" {
		refundNo = strings.TrimSpace(fallbackRefundNo)
	}
	return &provider.RefundResult{
		Channel: "wechat", RefundNo: refundNo, ChannelRefundNo: strings.TrimSpace(response.RefundId),
		Status: mapWechatRefundState(response.Status), Amount: amount, Currency: currency,
		FailureReason: strings.TrimSpace(response.Status),
		Raw:           map[string]any{"refund_id": response.RefundId, "out_refund_no": response.OutRefundNo, "out_trade_no": response.OutTradeNo, "transaction_id": response.TransactionId, "status": response.Status},
	}
}

func normalizeWechatRefundQuery(fallbackRefundNo string, response *wechatv3.RefundQueryResponse) *provider.RefundResult {
	amount, currency := wechatRefundAmount(response.Amount)
	refundNo := strings.TrimSpace(response.OutRefundNo)
	if refundNo == "" {
		refundNo = strings.TrimSpace(fallbackRefundNo)
	}
	return &provider.RefundResult{
		Channel: "wechat", RefundNo: refundNo, ChannelRefundNo: strings.TrimSpace(response.RefundId),
		Status: mapWechatRefundState(response.Status), Amount: amount, Currency: currency,
		FailureReason: strings.TrimSpace(response.Status),
		Raw:           map[string]any{"refund_id": response.RefundId, "out_refund_no": response.OutRefundNo, "out_trade_no": response.OutTradeNo, "transaction_id": response.TransactionId, "status": response.Status},
	}
}

func wechatRefundAmount(amount *wechatv3.RefundOrderAmount) (int64, string) {
	if amount == nil {
		return 0, "CNY"
	}
	currency := strings.ToUpper(strings.TrimSpace(amount.Currency))
	if currency == "" {
		currency = "CNY"
	}
	return int64(amount.Refund), currency
}

func mapWechatRefundState(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "SUCCESS":
		return "succeeded"
	case "CLOSED", "ABNORMAL":
		return "failed"
	default:
		return "pending"
	}
}

func wechatQueryCode(rsp *wechatv3.QueryOrderRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.Code
}
func wechatQueryError(rsp *wechatv3.QueryOrderRsp) string {
	if rsp == nil {
		return "empty response"
	}
	return formatWechatError(rsp.ErrResponse, rsp.Error)
}
func wechatEmptyCode(rsp *wechatv3.EmptyRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.Code
}
func wechatEmptyError(rsp *wechatv3.EmptyRsp) string {
	if rsp == nil {
		return "empty response"
	}
	return formatWechatError(rsp.ErrResponse, rsp.Error)
}
func wechatRefundCode(rsp *wechatv3.RefundRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.Code
}
func wechatRefundError(rsp *wechatv3.RefundRsp) string {
	if rsp == nil {
		return "empty response"
	}
	return formatWechatError(rsp.ErrResponse, rsp.Error)
}
func wechatRefundQueryCode(rsp *wechatv3.RefundQueryRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.Code
}
func wechatRefundQueryError(rsp *wechatv3.RefundQueryRsp) string {
	if rsp == nil {
		return "empty response"
	}
	return formatWechatError(rsp.ErrResponse, rsp.Error)
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
