package service

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"payment-gateway/ent"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	"payment-gateway/internal/domain/payments/provider"
)

var (
	ErrOrderRequired          = errors.New("order is required")
	ErrOrderNotPayable        = errors.New("order is not payable")
	ErrOrderExpired           = errors.New("order is expired")
	ErrPayMethodRequired      = errors.New("pay_method is required")
	ErrProviderUnavailable    = errors.New("payment provider unavailable")
	ErrNotifyUnsupported      = errors.New("payment notify unsupported")
	ErrCaptureUnsupported     = errors.New("payment capture unsupported")
	ErrChannelAccountRequired = errors.New("channel_account_id is required when multiple channel accounts are enabled")
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
	Status           string         `json:"status"`
	Channel          string         `json:"channel"`
	ChannelAccountID int            `json:"channel_account_id"`
	PayMethod        string         `json:"pay_method"`
	ProviderOrderNo  string         `json:"provider_order_no"`
	PayURL           string         `json:"pay_url,omitempty"`
	QRCode           string         `json:"qr_code,omitempty"`
	FormHTML         string         `json:"form_html,omitempty"`
	Raw              map[string]any `json:"raw,omitempty"`
}

type NotifyInput struct {
	Channel          string
	ChannelAccountID int
	Header           map[string][]string
	Form             url.Values
	RawBody          []byte
}

type NotifyResult struct {
	Channel          string         `json:"channel"`
	ChannelAccountID int            `json:"channel_account_id"`
	ProviderOrderNo  string         `json:"provider_order_no,omitempty"`
	GatewayOrderNo   string         `json:"gateway_order_no"`
	ChannelTradeNo   string         `json:"channel_trade_no"`
	Status           string         `json:"status"`
	Amount           int64          `json:"amount"`
	Currency         string         `json:"currency"`
	FailureReason    string         `json:"failure_reason,omitempty"`
	Raw              map[string]any `json:"raw"`
}

type CapturePaymentInput struct {
	Channel          string
	ChannelAccountID int
	ProviderOrderNo  string
	GatewayOrderNo   string
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
		NotifyURL:      appendNotifyBinding(input.NotifyURL, channel, channelAccount.ID),
	})
	if err != nil {
		return nil, err
	}
	return &PaymentResult{
		Status:           result.Status,
		Channel:          result.Channel,
		ChannelAccountID: channelAccount.ID,
		PayMethod:        result.PayMethod,
		ProviderOrderNo:  result.ProviderOrderNo,
		PayURL:           result.PayURL,
		QRCode:           result.QRCode,
		FormHTML:         result.FormHTML,
		Raw:              result.Raw,
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
	channelAccount, err := s.resolveNotifyAccount(ctx, channel, input.ChannelAccountID)
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
		Channel:          result.Channel,
		ChannelAccountID: channelAccount.ID,
		GatewayOrderNo:   strings.TrimSpace(result.GatewayOrderNo),
		ChannelTradeNo:   strings.TrimSpace(result.ChannelTradeNo),
		Status:           result.Status,
		Amount:           result.Amount,
		Currency:         result.Currency,
		FailureReason:    result.FailureReason,
		Raw:              result.Raw,
	}, nil
}

func (s Service) resolveNotifyAccount(ctx context.Context, channel string, channelAccountID int) (*ent.ChannelAccount, error) {
	if channelAccountID > 0 {
		account, err := s.channels.FindEnabledByID(ctx, channelAccountID)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(strings.TrimSpace(account.Channel), channel) {
			return nil, ErrProviderUnavailable
		}
		return account, nil
	}
	accounts, err := s.channels.ListEnabledByChannel(ctx, channel)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, ErrProviderUnavailable
	}
	if len(accounts) > 1 {
		return nil, ErrChannelAccountRequired
	}
	return accounts[0], nil
}

func appendNotifyBinding(rawURL string, channel string, channelAccountID int) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || channelAccountID <= 0 {
		return trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	query := parsed.Query()
	query.Set("channel", channel)
	query.Set("channel_account_id", strconv.Itoa(channelAccountID))
	parsed.RawQuery = query.Encode()
	return parsed.String()
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
	var channelAccount *ent.ChannelAccount
	var err error
	if input.ChannelAccountID > 0 {
		channelAccount, err = s.channels.FindEnabledByID(ctx, input.ChannelAccountID)
	} else {
		channelAccount, err = s.channels.FindEnabledByChannel(ctx, channel)
	}
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(channelAccount.Channel, channel) {
		return nil, ErrProviderUnavailable
	}
	result, err := captureProvider.CapturePayment(ctx, provider.CapturePaymentRequest{
		ChannelAccount:   channelAccount,
		ChannelAccountID: channelAccount.ID,
		ProviderOrderNo:  strings.TrimSpace(input.ProviderOrderNo),
		GatewayOrderNo:   strings.TrimSpace(input.GatewayOrderNo),
	})
	if err != nil {
		return nil, err
	}
	return &NotifyResult{
		Channel:          result.Channel,
		ChannelAccountID: channelAccount.ID,
		ProviderOrderNo:  result.ProviderOrderNo,
		GatewayOrderNo:   result.GatewayOrderNo,
		ChannelTradeNo:   result.ChannelTradeNo,
		Status:           result.Status,
		Amount:           result.Amount,
		Currency:         result.Currency,
		Raw:              result.Raw,
	}, nil
}
