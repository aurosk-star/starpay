package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"payment-gateway/ent"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	"payment-gateway/internal/domain/payments/provider"
)

var (
	ErrOrderRequired       = errors.New("order is required")
	ErrOrderNotPayable     = errors.New("order is not payable")
	ErrOrderExpired        = errors.New("order is expired")
	ErrPayMethodRequired   = errors.New("pay_method is required")
	ErrProviderUnavailable = errors.New("payment provider unavailable")
	ErrNotifyUnsupported   = errors.New("payment notify unsupported")
	ErrCaptureUnsupported  = errors.New("payment capture unsupported")
)

type Service struct {
	channels  channelrepo.Repository
	providers map[string]provider.Provider
}

type Option func(*Service)

func WithChannelRepository(channels channelrepo.Repository) Option {
	return func(s *Service) {
		s.channels = channels
	}
}

func WithProvider(paymentProvider provider.Provider) Option {
	return func(s *Service) {
		if paymentProvider == nil {
			return
		}
		s.providers[paymentProvider.Channel()] = paymentProvider
	}
}

func New(opts ...Option) Service {
	svc := Service{
		providers: map[string]provider.Provider{},
	}
	for _, opt := range opts {
		opt(&svc)
	}
	return svc
}

func ChannelSupportsCurrency(channel string, currency string) bool {
	normalizedChannel := strings.ToLower(strings.TrimSpace(channel))
	normalizedCurrency := strings.ToUpper(strings.TrimSpace(currency))
	switch normalizedChannel {
	case "alipay", "wechat":
		return normalizedCurrency == "CNY"
	case "paypal":
		switch normalizedCurrency {
		case "USD", "EUR", "HKD", "JPY", "GBP":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

type StartPaymentInput struct {
	Order            *ent.PaymentOrder
	PayMethod        string
	Channel          string
	ChannelAccountID int
	PayMode          string
	ClientIP         string
	ReturnURL        string
	NotifyURL        string
}

type PaymentResult struct {
	Status          string         `json:"status"`
	Channel         string         `json:"channel"`
	PayMethod       string         `json:"pay_method"`
	ProviderOrderNo string         `json:"provider_order_no"`
	PayURL          string         `json:"pay_url,omitempty"`
	QRCode          string         `json:"qr_code,omitempty"`
	FormHTML        string         `json:"form_html,omitempty"`
	Raw             map[string]any `json:"raw,omitempty"`
}

type NotifyInput struct {
	Channel string
	Header  map[string][]string
	Form    url.Values
	RawBody []byte
}

type NotifyResult struct {
	Channel        string         `json:"channel"`
	GatewayOrderNo string         `json:"gateway_order_no"`
	ChannelTradeNo string         `json:"channel_trade_no"`
	Status         string         `json:"status"`
	Amount         int64          `json:"amount"`
	Currency       string         `json:"currency"`
	Raw            map[string]any `json:"raw"`
}

type CapturePaymentInput struct {
	Channel         string
	ProviderOrderNo string
	GatewayOrderNo  string
}

func (s Service) StartPayment(ctx context.Context, input StartPaymentInput) (*PaymentResult, error) {
	if input.Order == nil {
		return nil, ErrOrderRequired
	}
	if input.Order.Status != "pending" {
		return nil, ErrOrderNotPayable
	}
	if input.Order.ExpiresAt != nil && !input.Order.ExpiresAt.After(time.Now()) {
		return nil, ErrOrderExpired
	}
	payMethod := strings.ToLower(strings.TrimSpace(input.PayMethod))
	if payMethod == "" {
		payMethod = strings.ToLower(strings.TrimSpace(input.Order.PayMethod))
	}
	if payMethod == "" {
		return nil, ErrPayMethodRequired
	}
	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	if channel == "" {
		channel = strings.ToLower(strings.TrimSpace(input.Order.Channel))
	}
	if channel == "" {
		channel = payMethod
	}

	paymentProvider := s.providers[channel]
	var channelAccount *ent.ChannelAccount
	if !s.channels.IsZero() {
		var account *ent.ChannelAccount
		var err error
		if input.ChannelAccountID > 0 {
			account, err = s.channels.FindEnabledByID(ctx, input.ChannelAccountID)
		} else {
			account, err = s.channels.FindEnabledByChannel(ctx, channel)
		}
		if err == nil {
			channelAccount = account
		} else if !ent.IsNotFound(err) {
			return nil, err
		}
	}
	if channelAccount == nil || paymentProvider == nil || !strings.EqualFold(channelAccount.Channel, channel) {
		return nil, ErrProviderUnavailable
	}
	channelAccount = withRuntimePayMode(channelAccount, input.PayMode)
	result, err := paymentProvider.StartPayment(ctx, provider.StartPaymentRequest{
		Order:          input.Order,
		ChannelAccount: channelAccount,
		PayMethod:      payMethod,
		Channel:        channel,
		ClientIP:       input.ClientIP,
		ReturnURL:      input.ReturnURL,
		NotifyURL:      input.NotifyURL,
	})
	if err != nil {
		return nil, err
	}
	return &PaymentResult{
		Status:          result.Status,
		Channel:         result.Channel,
		PayMethod:       result.PayMethod,
		ProviderOrderNo: result.ProviderOrderNo,
		PayURL:          result.PayURL,
		QRCode:          result.QRCode,
		FormHTML:        result.FormHTML,
		Raw:             result.Raw,
	}, nil
}

func withRuntimePayMode(account *ent.ChannelAccount, payMode string) *ent.ChannelAccount {
	mode := strings.ToLower(strings.TrimSpace(payMode))
	if account == nil || mode == "" {
		return account
	}
	next := *account
	next.Config = make(map[string]any, len(account.Config)+1)
	for key, value := range account.Config {
		next.Config[key] = value
	}
	next.Config["mode"] = mode
	return &next
}

func (s Service) HandleNotify(ctx context.Context, input NotifyInput) (*NotifyResult, error) {
	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	if channel == "" {
		channel = "alipay"
	}
	paymentProvider := s.providers[channel]
	notifyProvider, ok := paymentProvider.(provider.NotifyProvider)
	if !ok {
		return nil, ErrNotifyUnsupported
	}
	if s.channels.IsZero() {
		return nil, ErrProviderUnavailable
	}
	channelAccount, err := s.channels.FindEnabledByChannel(ctx, channel)
	if err != nil {
		return nil, err
	}
	result, err := notifyProvider.ParseNotify(ctx, provider.NotifyRequest{
		ChannelAccount: channelAccount,
		Header:         input.Header,
		Form:           input.Form,
		RawBody:        input.RawBody,
	})
	if err != nil {
		return nil, err
	}
	return &NotifyResult{
		Channel:        result.Channel,
		GatewayOrderNo: strings.TrimSpace(result.GatewayOrderNo),
		ChannelTradeNo: strings.TrimSpace(result.ChannelTradeNo),
		Status:         result.Status,
		Amount:         result.Amount,
		Currency:       result.Currency,
		Raw:            result.Raw,
	}, nil
}

func (s Service) CapturePayment(ctx context.Context, input CapturePaymentInput) (*NotifyResult, error) {
	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	if channel == "" {
		channel = "paypal"
	}
	paymentProvider := s.providers[channel]
	captureProvider, ok := paymentProvider.(provider.CaptureProvider)
	if !ok {
		return nil, ErrCaptureUnsupported
	}
	if s.channels.IsZero() {
		return nil, ErrProviderUnavailable
	}
	channelAccount, err := s.channels.FindEnabledByChannel(ctx, channel)
	if err != nil {
		return nil, err
	}
	result, err := captureProvider.CapturePayment(ctx, provider.CapturePaymentRequest{
		ChannelAccount:  channelAccount,
		ProviderOrderNo: strings.TrimSpace(input.ProviderOrderNo),
		GatewayOrderNo:  strings.TrimSpace(input.GatewayOrderNo),
	})
	if err != nil {
		return nil, err
	}
	return &NotifyResult{
		Channel:        result.Channel,
		GatewayOrderNo: result.GatewayOrderNo,
		ChannelTradeNo: result.ChannelTradeNo,
		Status:         result.Status,
		Raw:            result.Raw,
	}, nil
}
