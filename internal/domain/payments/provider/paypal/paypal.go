package paypal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-pay/gopay"
	gopaypaypal "github.com/go-pay/gopay/paypal"

	"payment-gateway/internal/domain/payments/provider"
)

type paypalClient interface {
	CreateOrder(ctx context.Context, body gopay.BodyMap) (*gopaypaypal.CreateOrderRsp, error)
	OrderCapture(ctx context.Context, orderID string, body gopay.BodyMap) (*gopaypaypal.OrderCaptureRsp, error)
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
	status := "pending"
	if strings.EqualFold(rsp.Response.Status, "COMPLETED") {
		status = "paid"
	}
	channelTradeNo := captureID(rsp.Response)
	if channelTradeNo == "" {
		channelTradeNo = orderID
	}
	return &provider.CapturePaymentResult{
		Channel:        "paypal",
		GatewayOrderNo: req.GatewayOrderNo,
		ChannelTradeNo: channelTradeNo,
		Status:         status,
		Raw: map[string]any{
			"paypal_order_id": orderID,
			"paypal_status":   rsp.Response.Status,
		},
	}, nil
}

func newGopayClient(cfg Config) (paypalClient, error) {
	return gopaypaypal.NewClient(cfg.ClientID, cfg.ClientSecret, cfg.IsProd)
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

func captureID(detail *gopaypaypal.OrderDetail) string {
	if detail == nil {
		return ""
	}
	for _, unit := range detail.PurchaseUnits {
		if unit == nil || unit.Payments == nil {
			continue
		}
		for _, capture := range unit.Payments.Captures {
			if capture != nil && strings.TrimSpace(capture.Id) != "" {
				return strings.TrimSpace(capture.Id)
			}
		}
	}
	return ""
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
