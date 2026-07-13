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
	FailureReason  string
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
	ChannelAccount   *ent.ChannelAccount
	ChannelAccountID int
	ProviderOrderNo  string
	GatewayOrderNo   string
}

type CapturePaymentResult struct {
	Channel         string
	ProviderOrderNo string
	GatewayOrderNo  string
	ChannelTradeNo  string
	Status          string
	Amount          int64
	Currency        string
	Raw             map[string]any
}

type NotifyProvider interface {
	Provider
	ParseNotify(ctx context.Context, req NotifyRequest) (*NotifyResult, error)
}

type QueryPaymentRequest struct {
	ChannelAccount *ent.ChannelAccount
	Order          *ent.PaymentOrder
}

type QueryPaymentResult struct {
	Channel         string
	GatewayOrderNo  string
	ProviderOrderNo string
	ChannelTradeNo  string
	Status          string
	Amount          int64
	Currency        string
	FailureReason   string
	Raw             map[string]any
}

type QueryProvider interface {
	Provider
	QueryPayment(ctx context.Context, req QueryPaymentRequest) (*QueryPaymentResult, error)
}

type ClosePaymentRequest struct {
	ChannelAccount *ent.ChannelAccount
	Order          *ent.PaymentOrder
}

type CloseProvider interface {
	Provider
	ClosePayment(ctx context.Context, req ClosePaymentRequest) error
}

type CreateRefundRequest struct {
	ChannelAccount  *ent.ChannelAccount
	GatewayOrderNo  string
	ProviderOrderNo string
	ChannelTradeNo  string
	RefundNo        string
	Amount          int64
	OriginalAmount  int64
	Currency        string
	Reason          string
}

type QueryRefundRequest struct {
	ChannelAccount  *ent.ChannelAccount
	GatewayOrderNo  string
	ChannelTradeNo  string
	RefundNo        string
	ChannelRefundNo string
}

type RefundResult struct {
	Channel         string
	RefundNo        string
	ChannelRefundNo string
	Status          string
	Amount          int64
	Currency        string
	FailureReason   string
	Raw             map[string]any
}

type RefundProvider interface {
	Provider
	CreateRefund(ctx context.Context, req CreateRefundRequest) (*RefundResult, error)
}

type RefundQueryProvider interface {
	Provider
	QueryRefund(ctx context.Context, req QueryRefundRequest) (*RefundResult, error)
}
