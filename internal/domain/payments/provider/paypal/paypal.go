package paypal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-pay/gopay"
	gopaypaypal "github.com/go-pay/gopay/paypal"

	"payment-gateway/ent"
	"payment-gateway/internal/domain/payments/provider"
)

type paypalClient interface {
	CreateOrder(ctx context.Context, body gopay.BodyMap) (*gopaypaypal.CreateOrderRsp, error)
	OrderCapture(ctx context.Context, orderID string, body gopay.BodyMap) (*gopaypaypal.OrderCaptureRsp, error)
	VerifyWebhookSignature(ctx context.Context, body gopay.BodyMap) (*gopaypaypal.VerifyWebhookResponse, error)
}

type paypalOperationsClient interface {
	OrderDetail(ctx context.Context, orderID string, body gopay.BodyMap) (*gopaypaypal.OrderDetailRsp, error)
	PaymentCaptureRefund(ctx context.Context, captureID string, body gopay.BodyMap) (*gopaypaypal.PaymentCaptureRefundRsp, error)
	PaymentRefundDetail(ctx context.Context, refundID string) (*gopaypaypal.PaymentRefundDetailRsp, error)
}

type paypalRequestHeaderClient interface {
	SetRequestHeader(key string, defaultValue ...string)
	ClearRequestHeader()
}

type clientFactory func(Config) (paypalClient, error)

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
	return "paypal"
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
	body.Set("intent", cfg.Intent)
	body.Set("purchase_units", []*gopaypaypal.PurchaseUnit{
		{
			ReferenceId: req.Order.GatewayOrderNo,
			CustomId:    req.Order.GatewayOrderNo,
			Description: req.Order.Subject,
			Amount: &gopaypaypal.Amount{
				CurrencyCode: strings.ToUpper(req.Order.Currency),
				Value:        formatAmount(req.Order.Amount, req.Order.Currency),
			},
		},
	})
	body.SetBodyMap("payment_source", func(source gopay.BodyMap) {
		source.SetBodyMap("paypal", func(paypal gopay.BodyMap) {
			paypal.SetBodyMap("experience_context", func(experience gopay.BodyMap) {
				setIfNotEmpty(experience, "brand_name", cfg.BrandName)
				setIfNotEmpty(experience, "locale", cfg.Locale)
				setIfNotEmpty(experience, "return_url", req.ReturnURL)
				setIfNotEmpty(experience, "cancel_url", cancelURL(req.ReturnURL))
				experience.Set("shipping_preference", "NO_SHIPPING").
					Set("user_action", "PAY_NOW").
					Set("landing_page", "LOGIN")
			})
		})
	})

	rsp, err := client.CreateOrder(ctx, body)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.Code != gopaypaypal.Success {
		return nil, paypalResponseError("create order", statusCode(rsp), errorText(rsp))
	}
	if rsp.Response == nil || strings.TrimSpace(rsp.Response.Id) == "" {
		return nil, fmt.Errorf("paypal create order response is empty")
	}
	approveURL := findCheckoutLink(rsp.Response.Links)
	if approveURL == "" {
		return nil, fmt.Errorf("paypal approve link is empty: links=%s", formatLinks(rsp.Response.Links))
	}
	return &provider.StartPaymentResult{
		Status:          "pending",
		Channel:         req.Channel,
		PayMethod:       req.PayMethod,
		ProviderOrderNo: rsp.Response.Id,
		PayURL:          approveURL,
		Raw: map[string]any{
			"paypal_order_id": rsp.Response.Id,
			"paypal_status":   rsp.Response.Status,
		},
	}, nil
}

func (p Provider) CapturePayment(ctx context.Context, req provider.CapturePaymentRequest) (*provider.CapturePaymentResult, error) {
	cfg, err := ParseConfig(req.ChannelAccount.Config, req.ChannelAccount.Env)
	if err != nil {
		return nil, err
	}
	client, err := p.newClient(cfg)
	if err != nil {
		return nil, err
	}
	orderID := strings.TrimSpace(req.ProviderOrderNo)
	if orderID == "" {
		return nil, fmt.Errorf("paypal order id is required")
	}
	rsp, err := client.OrderCapture(ctx, orderID, nil)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.Code != gopaypaypal.Success {
		return nil, fmt.Errorf("paypal capture failed: code=%d, %s", captureStatusCode(rsp), captureErrorText(rsp))
	}
	if rsp.Response == nil {
		return nil, fmt.Errorf("paypal capture response is empty")
	}
	if responseOrderID := strings.TrimSpace(rsp.Response.Id); responseOrderID != "" && responseOrderID != orderID {
		return nil, fmt.Errorf("paypal capture order mismatch: got %s, want %s", responseOrderID, orderID)
	}
	status := "pending"
	if strings.EqualFold(rsp.Response.Status, "COMPLETED") {
		status = "paid"
	}
	gatewayOrderNo, amount, currency, channelTradeNo, err := captureDetails(rsp.Response)
	if err != nil {
		return nil, err
	}
	if channelTradeNo == "" {
		channelTradeNo = orderID
	}
	if gatewayOrderNo == "" {
		gatewayOrderNo = strings.TrimSpace(req.GatewayOrderNo)
	}
	return &provider.CapturePaymentResult{
		Channel:         "paypal",
		ProviderOrderNo: orderID,
		GatewayOrderNo:  gatewayOrderNo,
		ChannelTradeNo:  channelTradeNo,
		Status:          status,
		Amount:          amount,
		Currency:        currency,
		Raw: map[string]any{
			"paypal_order_id": orderID,
			"paypal_status":   rsp.Response.Status,
		},
	}, nil
}

func (p Provider) ParseNotify(ctx context.Context, req provider.NotifyRequest) (*provider.NotifyResult, error) {
	cfg, err := ParseConfig(req.ChannelAccount.Config, req.ChannelAccount.Env)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.WebhookID) == "" {
		return nil, fmt.Errorf("paypal webhook signature verification requires webhook_id")
	}
	client, err := p.newClient(cfg)
	if err != nil {
		return nil, err
	}
	var event paypalWebhookEvent
	if err := json.Unmarshal(req.RawBody, &event); err != nil {
		return nil, err
	}
	verifyBody := gopay.BodyMap{}
	verifyBody.Set("auth_algo", headerValue(req.Header, "Paypal-Auth-Algo")).
		Set("cert_url", headerValue(req.Header, "Paypal-Cert-Url")).
		Set("transmission_id", headerValue(req.Header, "Paypal-Transmission-Id")).
		Set("transmission_sig", headerValue(req.Header, "Paypal-Transmission-Sig")).
		Set("transmission_time", headerValue(req.Header, "Paypal-Transmission-Time")).
		Set("webhook_id", cfg.WebhookID)
	var rawEvent any
	if err := json.Unmarshal(req.RawBody, &rawEvent); err != nil {
		return nil, err
	}
	verifyBody.Set("webhook_event", rawEvent)
	verifyRsp, err := client.VerifyWebhookSignature(ctx, verifyBody)
	if err != nil {
		return nil, err
	}
	if verifyRsp == nil || !strings.EqualFold(strings.TrimSpace(verifyRsp.VerificationStatus), "SUCCESS") {
		return nil, fmt.Errorf("paypal webhook signature verification failed")
	}
	gatewayOrderNo := strings.TrimSpace(event.Resource.CustomID)
	if gatewayOrderNo == "" {
		gatewayOrderNo = strings.TrimSpace(event.Resource.InvoiceID)
	}
	status := normalizeWebhookStatus(event.EventType, event.Resource.Status)
	amount, err := parsePayPalAmount(event.Resource.Amount.Value, event.Resource.Amount.CurrencyCode)
	if err != nil {
		return nil, err
	}
	return &provider.NotifyResult{
		Channel:        "paypal",
		GatewayOrderNo: gatewayOrderNo,
		ChannelTradeNo: strings.TrimSpace(event.Resource.ID),
		Status:         status,
		Amount:         amount,
		Currency:       strings.ToUpper(strings.TrimSpace(event.Resource.Amount.CurrencyCode)),
		FailureReason:  strings.TrimSpace(event.Resource.Status),
		Raw: map[string]any{
			"event_id":     event.ID,
			"event_type":   event.EventType,
			"resource_id":  event.Resource.ID,
			"resource_raw": event.Resource,
		},
	}, nil
}

func (p Provider) QueryPayment(ctx context.Context, req provider.QueryPaymentRequest) (*provider.QueryPaymentResult, error) {
	if req.Order == nil {
		return nil, fmt.Errorf("paypal order is required")
	}
	client, _, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return nil, err
	}
	providerOrderNo := strings.TrimSpace(req.Order.ProviderOrderNo)
	if providerOrderNo == "" {
		return nil, fmt.Errorf("paypal order id is required")
	}
	rsp, err := client.OrderDetail(ctx, providerOrderNo, nil)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.Code != gopaypaypal.Success {
		return nil, paypalOrderDetailError(rsp)
	}
	if rsp.Response == nil {
		return nil, fmt.Errorf("paypal order detail response is empty")
	}
	gatewayOrderNo, amount, currency, channelTradeNo, err := captureDetails(rsp.Response)
	if err != nil {
		return nil, err
	}
	if gatewayOrderNo == "" {
		gatewayOrderNo = strings.TrimSpace(req.Order.GatewayOrderNo)
	}
	if channelTradeNo == "" {
		channelTradeNo = strings.TrimSpace(req.Order.ChannelTradeNo)
	}
	return &provider.QueryPaymentResult{
		Channel: "paypal", GatewayOrderNo: gatewayOrderNo, ProviderOrderNo: providerOrderNo,
		ChannelTradeNo: channelTradeNo, Status: normalizePaypalOrderStatus(rsp.Response.Status),
		Amount: amount, Currency: currency, FailureReason: strings.TrimSpace(rsp.Response.Status),
		Raw: map[string]any{"paypal_order_id": rsp.Response.Id, "paypal_status": rsp.Response.Status},
	}, nil
}

func (p Provider) CreateRefund(ctx context.Context, req provider.CreateRefundRequest) (*provider.RefundResult, error) {
	client, headers, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return nil, err
	}
	captureID := strings.TrimSpace(req.ChannelTradeNo)
	if captureID == "" {
		return nil, fmt.Errorf("paypal capture id is required")
	}
	refundNo := strings.TrimSpace(req.RefundNo)
	if refundNo == "" {
		return nil, fmt.Errorf("refund number is required")
	}
	headers.SetRequestHeader("PayPal-Request-Id", refundNo)
	defer headers.ClearRequestHeader()
	body := gopay.BodyMap{}
	body.Set("amount", &gopaypaypal.Amount{CurrencyCode: strings.ToUpper(req.Currency), Value: formatAmount(req.Amount, req.Currency)}).
		Set("invoice_id", refundNo)
	if strings.TrimSpace(req.Reason) != "" {
		body.Set("note_to_payer", strings.TrimSpace(req.Reason))
	}
	rsp, err := client.PaymentCaptureRefund(ctx, captureID, body)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.Code != gopaypaypal.Success {
		return nil, paypalRefundError("create refund", refundResponseCode(rsp), refundErrorText(rsp))
	}
	if rsp.Response == nil {
		return nil, fmt.Errorf("paypal refund response is empty")
	}
	return normalizePaypalRefund(refundNo, rsp.Response)
}

func (p Provider) QueryRefund(ctx context.Context, req provider.QueryRefundRequest) (*provider.RefundResult, error) {
	client, _, err := p.operationsClient(req.ChannelAccount)
	if err != nil {
		return nil, err
	}
	channelRefundNo := strings.TrimSpace(req.ChannelRefundNo)
	if channelRefundNo == "" {
		return nil, fmt.Errorf("paypal refund id is required")
	}
	rsp, err := client.PaymentRefundDetail(ctx, channelRefundNo)
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.Code != gopaypaypal.Success {
		return nil, paypalRefundError("query refund", refundDetailResponseCode(rsp), refundDetailErrorText(rsp))
	}
	if rsp.Response == nil {
		return nil, fmt.Errorf("paypal refund detail response is empty")
	}
	return normalizePaypalRefund(req.RefundNo, rsp.Response)
}

func (p Provider) operationsClient(account *ent.ChannelAccount) (paypalOperationsClient, paypalRequestHeaderClient, error) {
	if account == nil {
		return nil, nil, fmt.Errorf("paypal channel account is required")
	}
	cfg, err := ParseConfig(account.Config, account.Env)
	if err != nil {
		return nil, nil, err
	}
	client, err := p.newClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	operations, ok := client.(paypalOperationsClient)
	if !ok {
		return nil, nil, fmt.Errorf("paypal client does not support payment operations")
	}
	headers, ok := client.(paypalRequestHeaderClient)
	if !ok {
		return nil, nil, fmt.Errorf("paypal client does not support idempotency headers")
	}
	return operations, headers, nil
}

func normalizePaypalOrderStatus(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "COMPLETED":
		return "paid"
	case "VOIDED", "CANCELLED":
		return "closed"
	case "DENIED", "FAILED":
		return "failed"
	default:
		return "pending"
	}
}

func normalizePaypalRefund(fallbackRefundNo string, response *gopaypaypal.PaymentCaptureRefund) (*provider.RefundResult, error) {
	amount := int64(0)
	currency := ""
	var err error
	if response.Amount != nil {
		currency = strings.ToUpper(strings.TrimSpace(response.Amount.CurrencyCode))
		amount, err = parsePayPalAmount(response.Amount.Value, currency)
		if err != nil {
			return nil, err
		}
	}
	refundNo := strings.TrimSpace(response.InvoiceId)
	if refundNo == "" {
		refundNo = strings.TrimSpace(fallbackRefundNo)
	}
	return &provider.RefundResult{
		Channel: "paypal", RefundNo: refundNo, ChannelRefundNo: strings.TrimSpace(response.Id),
		Status: normalizePaypalRefundStatus(response.Status), Amount: amount, Currency: currency,
		FailureReason: strings.TrimSpace(response.Status),
		Raw:           map[string]any{"paypal_refund_id": response.Id, "paypal_status": response.Status, "invoice_id": response.InvoiceId},
	}, nil
}

func normalizePaypalRefundStatus(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "COMPLETED":
		return "succeeded"
	case "CANCELLED", "FAILED", "DENIED":
		return "failed"
	default:
		return "pending"
	}
}

func paypalOrderDetailError(rsp *gopaypaypal.OrderDetailRsp) error {
	if rsp == nil {
		return fmt.Errorf("paypal order detail failed: empty response")
	}
	message := strings.TrimSpace(rsp.Error)
	if rsp.ErrorResponse != nil {
		message = strings.TrimSpace(rsp.ErrorResponse.Name + ": " + rsp.ErrorResponse.Message + formatErrorDetails(rsp.ErrorResponse.Details))
	}
	return fmt.Errorf("paypal order detail failed: code=%d, %s", rsp.Code, message)
}

func paypalRefundError(action string, code int, detail string) error {
	return fmt.Errorf("paypal %s failed: code=%d, %s", action, code, strings.TrimSpace(detail))
}

func refundResponseCode(rsp *gopaypaypal.PaymentCaptureRefundRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.Code
}
func refundErrorText(rsp *gopaypaypal.PaymentCaptureRefundRsp) string {
	if rsp == nil {
		return "empty response"
	}
	if rsp.ErrorResponse != nil {
		return strings.TrimSpace(rsp.ErrorResponse.Name + ": " + rsp.ErrorResponse.Message + formatErrorDetails(rsp.ErrorResponse.Details))
	}
	return strings.TrimSpace(rsp.Error)
}
func refundDetailResponseCode(rsp *gopaypaypal.PaymentRefundDetailRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.Code
}
func refundDetailErrorText(rsp *gopaypaypal.PaymentRefundDetailRsp) string {
	if rsp == nil {
		return "empty response"
	}
	if rsp.ErrorResponse != nil {
		return strings.TrimSpace(rsp.ErrorResponse.Name + ": " + rsp.ErrorResponse.Message + formatErrorDetails(rsp.ErrorResponse.Details))
	}
	return strings.TrimSpace(rsp.Error)
}

func newGopayClient(cfg Config) (paypalClient, error) {
	return gopaypaypal.NewClient(cfg.ClientID, cfg.ClientSecret, cfg.IsProd)
}

type paypalWebhookEvent struct {
	ID        string                `json:"id"`
	EventType string                `json:"event_type"`
	Resource  paypalWebhookResource `json:"resource"`
}

type paypalWebhookResource struct {
	ID        string              `json:"id"`
	CustomID  string              `json:"custom_id"`
	InvoiceID string              `json:"invoice_id"`
	Status    string              `json:"status"`
	Amount    paypalWebhookAmount `json:"amount"`
}

type paypalWebhookAmount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

func normalizeWebhookStatus(eventType string, resourceStatus string) string {
	normalizedEventType := strings.ToUpper(strings.TrimSpace(eventType))
	normalizedStatus := strings.ToUpper(strings.TrimSpace(resourceStatus))
	if normalizedEventType == "PAYMENT.CAPTURE.COMPLETED" ||
		(strings.HasPrefix(normalizedEventType, "PAYMENT.CAPTURE.") && normalizedStatus == "COMPLETED") {
		return "paid"
	}
	if normalizedEventType == "CHECKOUT.ORDER.VOIDED" ||
		strings.Contains(normalizedEventType, "CANCELLED") ||
		normalizedStatus == "VOIDED" ||
		normalizedStatus == "CANCELLED" {
		return "closed"
	}
	if normalizedEventType == "PAYMENT.CAPTURE.DENIED" || normalizedStatus == "DENIED" || normalizedStatus == "FAILED" {
		return "failed"
	}
	return "pending"
}

func headerValue(headers map[string][]string, key string) string {
	for currentKey, values := range headers {
		if !strings.EqualFold(currentKey, key) || len(values) == 0 {
			continue
		}
		return strings.TrimSpace(values[0])
	}
	return ""
}

func cancelURL(returnURL string) string {
	trimmed := strings.TrimSpace(returnURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	query := parsed.Query()
	query.Set("cancel", "1")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func findLink(links []*gopaypaypal.Link, rel string) string {
	for _, link := range links {
		if link == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(link.Rel), rel) {
			return strings.TrimSpace(link.Href)
		}
	}
	return ""
}

func findCheckoutLink(links []*gopaypaypal.Link) string {
	if href := findLink(links, "approve"); href != "" {
		return href
	}
	return findLink(links, "payer-action")
}

func formatLinks(links []*gopaypaypal.Link) string {
	if len(links) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(links))
	for _, link := range links {
		if link == nil {
			continue
		}
		parts = append(parts, strings.TrimSpace(link.Rel)+"="+strings.TrimSpace(link.Href))
	}
	if len(parts) == 0 {
		return "[]"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func setIfNotEmpty(body gopay.BodyMap, key string, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		body.Set(key, trimmed)
	}
}

func statusCode(rsp *gopaypaypal.CreateOrderRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.Code
}

func errorText(rsp *gopaypaypal.CreateOrderRsp) string {
	if rsp == nil {
		return ""
	}
	if rsp.ErrorResponse != nil {
		return strings.TrimSpace(rsp.ErrorResponse.Name + ": " + rsp.ErrorResponse.Message + formatErrorDetails(rsp.ErrorResponse.Details))
	}
	return strings.TrimSpace(rsp.Error)
}

func captureStatusCode(rsp *gopaypaypal.OrderCaptureRsp) int {
	if rsp == nil {
		return 0
	}
	return rsp.Code
}

func captureErrorText(rsp *gopaypaypal.OrderCaptureRsp) string {
	if rsp == nil {
		return ""
	}
	if rsp.ErrorResponse != nil {
		return strings.TrimSpace(rsp.ErrorResponse.Name + ": " + rsp.ErrorResponse.Message + formatErrorDetails(rsp.ErrorResponse.Details))
	}
	return strings.TrimSpace(rsp.Error)
}

func formatErrorDetails(details []gopaypaypal.ErrorDetail) string {
	if len(details) == 0 {
		return ""
	}
	parts := make([]string, 0, len(details))
	for _, detail := range details {
		fields := make([]string, 0, 4)
		if strings.TrimSpace(detail.Field) != "" {
			fields = append(fields, "field="+strings.TrimSpace(detail.Field))
		}
		if strings.TrimSpace(detail.Issue) != "" {
			fields = append(fields, "issue="+strings.TrimSpace(detail.Issue))
		}
		if strings.TrimSpace(detail.Description) != "" {
			fields = append(fields, "description="+strings.TrimSpace(detail.Description))
		}
		if strings.TrimSpace(detail.Value) != "" {
			fields = append(fields, "value="+strings.TrimSpace(detail.Value))
		}
		if len(fields) > 0 {
			parts = append(parts, strings.Join(fields, ", "))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " [" + strings.Join(parts, "; ") + "]"
}

func captureDetails(detail *gopaypaypal.OrderDetail) (string, int64, string, string, error) {
	if detail == nil {
		return "", 0, "", "", nil
	}
	for _, unit := range detail.PurchaseUnits {
		if unit == nil {
			continue
		}
		gatewayOrderNo := strings.TrimSpace(unit.CustomId)
		if gatewayOrderNo == "" {
			gatewayOrderNo = strings.TrimSpace(unit.InvoiceId)
		}
		amountValue := ""
		currency := ""
		channelTradeNo := ""
		if unit.Payments != nil {
			for _, capture := range unit.Payments.Captures {
				if capture == nil {
					continue
				}
				channelTradeNo = strings.TrimSpace(capture.Id)
				if capture.Amount != nil {
					amountValue = capture.Amount.Value
					currency = capture.Amount.CurrencyCode
				}
				break
			}
		}
		if amountValue == "" && unit.Amount != nil {
			amountValue = unit.Amount.Value
			currency = unit.Amount.CurrencyCode
		}
		amount, err := parsePayPalAmount(amountValue, currency)
		if err != nil {
			return "", 0, "", "", err
		}
		return gatewayOrderNo, amount, strings.ToUpper(strings.TrimSpace(currency)), channelTradeNo, nil
	}
	return "", 0, "", "", nil
}

func parsePayPalAmount(value string, currency string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if zeroDecimalCurrency(currency) {
		amount, err := strconv.ParseInt(value, 10, 64)
		if err != nil || amount < 0 {
			return 0, fmt.Errorf("invalid paypal amount: %s", value)
		}
		return amount, nil
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid paypal amount: %s", value)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, fmt.Errorf("invalid paypal amount: %s", value)
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return 0, fmt.Errorf("invalid paypal amount: %s", value)
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	cents := int64(0)
	if fraction != "" {
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid paypal amount: %s", value)
		}
	}
	return whole*100 + cents, nil
}

func paypalResponseError(action string, code int, detail string) error {
	if detail == "" {
		detail = http.StatusText(code)
	}
	return fmt.Errorf("paypal %s failed: code=%d, %s", action, code, detail)
}

func formatAmount(amount int64, currency string) string {
	if zeroDecimalCurrency(currency) {
		return fmt.Sprintf("%d", amount)
	}
	return fmt.Sprintf("%d.%02d", amount/100, amount%100)
}

func zeroDecimalCurrency(currency string) bool {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "JPY":
		return true
	default:
		return false
	}
}
