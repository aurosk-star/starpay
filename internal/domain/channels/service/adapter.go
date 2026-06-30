package service

import (
	"context"
	"time"

	"github.com/go-pay/gopay"
)

var _ gopay.BodyMap
var _ = gopay.NULL

type PaymentRequest struct {
	GatewayOrderNo  string
	MerchantOrderNo string
	Amount          int64
	Currency        string
	Subject         string
	Description     string
	NotifyURL       string
	ReturnURL       string
	Metadata        map[string]any
}

type PaymentResponse struct {
	Channel        string
	ChannelTradeNo string
	PayURL         string
	QRCode         string
	ClientToken    string
	ExpiresAt      *time.Time
	Raw            map[string]any
}

type QueryPaymentResponse struct {
	ChannelTradeNo string
	Status         string
	PaidAt         *time.Time
	Raw            map[string]any
}

type RefundRequest struct {
	RefundNo       string
	GatewayOrderNo string
	ChannelTradeNo string
	Amount         int64
	Currency       string
	Reason         string
}

type RefundResponse struct {
	ChannelRefundNo string
	Status          string
	Raw             map[string]any
}

type NotifyResult struct {
	ChannelTradeNo string
	Status         string
	Amount         int64
	Currency       string
	PaidAt         *time.Time
	Raw            map[string]any
}

type Adapter interface {
	CreatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)
	QueryPayment(ctx context.Context, gatewayOrderNo string, channelTradeNo string) (*QueryPaymentResponse, error)
	ClosePayment(ctx context.Context, gatewayOrderNo string, channelTradeNo string) error
	CreateRefund(ctx context.Context, req RefundRequest) (*RefundResponse, error)
	VerifyNotify(ctx context.Context, headers map[string]string, body []byte) (*NotifyResult, error)
}

type AdapterFactory interface {
	Build(account ChannelAccountView) (Adapter, error)
}
