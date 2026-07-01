package provider

import (
	"context"
	"net/url"

	"payment-gateway/ent"
)

type StartPaymentRequest struct {
	Order          *ent.PaymentOrder
	ChannelAccount *ent.ChannelAccount
	PayMethod      string
	Channel        string
	ClientIP       string
	ReturnURL      string
	NotifyURL      string
}

type StartPaymentResult struct {
	Status          string
	Channel         string
	PayMethod       string
	ProviderOrderNo string
	PayURL          string
	QRCode          string
	FormHTML        string
	Raw             map[string]any
}

type NotifyRequest struct {
	ChannelAccount *ent.ChannelAccount
	Header         map[string][]string
	Form           url.Values
	RawBody        []byte
}

type NotifyResult struct {
	Channel        string
	GatewayOrderNo string
	ChannelTradeNo string
	Status         string
	Amount         int64
	Currency       string
	Raw            map[string]any
}

type Provider interface {
	Channel() string
	StartPayment(ctx context.Context, req StartPaymentRequest) (*StartPaymentResult, error)
}

type CaptureProvider interface {
	Provider
	CapturePayment(ctx context.Context, req CapturePaymentRequest) (*CapturePaymentResult, error)
}

type CapturePaymentRequest struct {
	ChannelAccount  *ent.ChannelAccount
	ProviderOrderNo string
	GatewayOrderNo  string
}

type CapturePaymentResult struct {
	Channel        string
	GatewayOrderNo string
	ChannelTradeNo string
	Status         string
	Raw            map[string]any
}

type NotifyProvider interface {
	Provider
	ParseNotify(ctx context.Context, req NotifyRequest) (*NotifyResult, error)
}
