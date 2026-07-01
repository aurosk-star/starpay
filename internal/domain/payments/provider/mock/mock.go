package mock

import (
	"context"
	"net/url"
	"strings"

	"payment-gateway/internal/domain/payments/provider"
)

type Provider struct{}

func New() Provider {
	return Provider{}
}

func (Provider) Channel() string {
	return "mock"
}

func (Provider) StartPayment(ctx context.Context, req provider.StartPaymentRequest) (*provider.StartPaymentResult, error) {
	_ = ctx
	payMethod := strings.ToLower(strings.TrimSpace(req.PayMethod))
	if payMethod == "" {
		payMethod = "mock"
	}
	values := url.Values{}
	values.Set("method", payMethod)
	if req.ReturnURL != "" {
		values.Set("return_url", req.ReturnURL)
	}
	return &provider.StartPaymentResult{
		Status:          "pending",
		Channel:         req.Channel,
		PayMethod:       payMethod,
		ProviderOrderNo: "mock_" + req.Order.GatewayOrderNo,
		PayURL:          "/checkout/mock-pay/" + req.Order.GatewayOrderNo + "?" + values.Encode(),
	}, nil
}
