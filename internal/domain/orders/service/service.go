package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"payment-gateway/ent"
	apprepo "payment-gateway/internal/domain/apps/repository"
	orderrepo "payment-gateway/internal/domain/orders/repository"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
)

var (
	ErrAppIDRequired                 = errors.New("app_id is required")
	ErrMerchantOrderNoRequired       = errors.New("merchant_order_no is required")
	ErrSubjectRequired               = errors.New("subject is required")
	ErrInvalidAmount                 = errors.New("amount must be greater than zero")
	ErrInvalidCurrency               = errors.New("invalid currency")
	ErrDuplicateOrder                = errors.New("merchant order already exists for app")
	ErrIdempotencyConflict           = errors.New("idempotency conflict")
	ErrOrderCannotBeClosed           = errors.New("order cannot be closed in current status")
	ErrOrderCannotBePaid             = errors.New("order cannot be paid in current status")
	ErrOrderCannotBeFailed           = errors.New("order cannot be failed in current status")
	ErrPaymentChannelMismatch        = errors.New("payment channel does not match order")
	ErrPaymentAccountMismatch        = errors.New("payment channel account does not match order")
	ErrPaymentAmountMismatch         = errors.New("payment amount does not match order")
	ErrPaymentCurrencyMismatch       = errors.New("payment currency does not match order")
	ErrAppNotFound                   = errors.New("app not found")
	ErrAppDisabled                   = errors.New("app is disabled")
	ErrUnsupportedCurrencyForChannel = errors.New("currency is not supported by channel")
)

type Service struct {
	orders                  orderrepo.Repository
	apps                    apprepo.Repository
	webhooks                webhooksvc.Service
	enqueuer                ExpirationEnqueuer
	now                     func() time.Time
	defaultOrderTTL         time.Duration
	defaultOrderTTLResolver func(context.Context) (time.Duration, error)
}

type Option func(*Service)

func WithWebhookService(webhooks webhooksvc.Service) Option {
	return func(s *Service) {
		s.webhooks = webhooks
	}
}

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func WithDefaultOrderTTL(ttl time.Duration) Option {
	return func(s *Service) {
		s.defaultOrderTTL = ttl
	}
}

func WithDefaultOrderTTLResolver(resolver func(context.Context) (time.Duration, error)) Option {
	return func(s *Service) {
		s.defaultOrderTTLResolver = resolver
	}
}

func WithExpirationEnqueuer(enqueuer ExpirationEnqueuer) Option {
	return func(s *Service) {
		s.enqueuer = enqueuer
	}
}

func New(client *ent.Client, opts ...Option) Service {
	svc := Service{
		orders:          orderrepo.New(client),
		apps:            apprepo.New(client),
		now:             time.Now,
		defaultOrderTTL: 15 * time.Minute,
	}
	for _, opt := range opts {
		opt(&svc)
	}
	return svc
}

type ManageOrderInput struct {
	AppID              string
	MerchantOrderNo    string
	BusinessType       string
	Subject            string
	Description        string
	Amount             int64
	Currency           string
	SettlementAmount   int64
	SettlementCurrency string
	Channel            string
	PayMethod          string
	ReturnURL          string
	ExpiresAt          *time.Time
	Metadata           map[string]any
}

type UpdateOrderInput struct {
	BusinessType string
	Subject      string
	Description  string
	Channel      string
	PayMethod    string
	Metadata     map[string]any
}

type OpenOrderInput struct {
	MerchantOrderNo  string
	BusinessType     string
	Subject          string
	Description      string
	Amount           int64
	Currency         string
	Channel          string
	PayMethod        string
	PreferredChannel string
	ClientIP         string
	ReturnURL        string
	Metadata         map[string]any
}

type ListOrdersInput struct {
	AppID           string
	Status          string
	Channel         string
	Currency        string
	MerchantOrderNo string
	Page            int
	PageSize        int
}

type ListOrdersResult struct {
	Items    []*ent.PaymentOrder `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type PaymentResultInput struct {
	Channel          string
	ChannelAccountID int
	ChannelTradeNo   string
	Status           string
	Amount           int64
	Currency         string
	FailureReason    string
}

type CreatedOrder struct {
	Order         *ent.PaymentOrder
	CheckoutToken string
}

func (s Service) CreateOrder(ctx context.Context, input ManageOrderInput) (*ent.PaymentOrder, error) {
	result, err := s.CreateOrderWithCheckoutToken(ctx, input)
	if err != nil {
		return nil, err
	}
	return result.Order, nil
}

func (s Service) CreateOrderWithCheckoutToken(ctx context.Context, input ManageOrderInput) (*CreatedOrder, error) {
	normalized, err := normalizeManageInput(input)
	if err != nil {
		return nil, err
	}
	if existing, err := s.orders.FindByMerchantOrderNo(ctx, normalized.AppID, normalized.MerchantOrderNo); err == nil && existing != nil {
		return nil, ErrDuplicateOrder
	}
	app, err := s.resolveEnabledApp(ctx, normalized.AppID)
	if err != nil {
		return nil, err
	}
	returnURL := normalized.ReturnURL
	if returnURL == "" {
		returnURL = strings.TrimSpace(app.DefaultReturnURL)
	}
	if err := validateChannelCurrency(normalized.Channel, normalized.PayMethod, normalized.Currency); err != nil {
		return nil, err
	}
	expiresAt := normalized.ExpiresAt
	if expiresAt == nil {
		expiresAt = s.defaultExpiresAt(ctx)
	}
	gatewayOrderNo, err := newGatewayOrderNo(s.now())
	if err != nil {
		return nil, err
	}
	checkoutToken, err := NewCheckoutToken()
	if err != nil {
		return nil, err
	}
	order, err := s.orders.Create(ctx, orderrepo.CreateOrderInput{
		GatewayOrderNo:     gatewayOrderNo,
		AppID:              normalized.AppID,
		MerchantOrderNo:    normalized.MerchantOrderNo,
		BusinessType:       normalized.BusinessType,
		Subject:            normalized.Subject,
		Description:        normalized.Description,
		Amount:             normalized.Amount,
		Currency:           normalized.Currency,
		SettlementAmount:   normalized.SettlementAmount,
		SettlementCurrency: normalized.SettlementCurrency,
		Channel:            normalized.Channel,
		PayMethod:          normalized.PayMethod,
		ReturnURL:          returnURL,
		CheckoutTokenHash:  HashCheckoutToken(checkoutToken),
		Status:             "pending",
		ExpiresAt:          expiresAt,
		Metadata:           normalized.Metadata,
	})
	if err != nil {
		return nil, err
	}
	return &CreatedOrder{Order: order, CheckoutToken: checkoutToken}, nil
}

func (s Service) CreateOpenOrder(ctx context.Context, appID string, input OpenOrderInput) (*ent.PaymentOrder, bool, error) {
	result, created, err := s.CreateOpenOrderWithCheckoutToken(ctx, appID, input)
	if err != nil {
		return nil, false, err
	}
	return result.Order, created, nil
}

func (s Service) CreateOpenOrderWithCheckoutToken(ctx context.Context, appID string, input OpenOrderInput) (*CreatedOrder, bool, error) {
	normalized, err := normalizeOpenOrderInput(input)
	if err != nil {
		return nil, false, err
	}
	existing, err := s.orders.FindByMerchantOrderNo(ctx, appID, normalized.MerchantOrderNo)
	if err == nil {
		if sameOpenOrder(existing, appID, normalized) {
			token, err := NewCheckoutToken()
			if err != nil {
				return nil, false, err
			}
			updated, err := s.orders.SetCheckoutTokenHash(ctx, existing.ID, HashCheckoutToken(token))
			if err != nil {
				return nil, false, err
			}
			return &CreatedOrder{Order: updated, CheckoutToken: token}, false, nil
		}
		return nil, false, ErrIdempotencyConflict
	}
	if !isNotFoundError(err) {
		return nil, false, err
	}
	app, err := s.resolveEnabledApp(ctx, appID)
	if err != nil {
		return nil, false, err
	}
	returnURL := normalized.ReturnURL
	if returnURL == "" {
		returnURL = strings.TrimSpace(app.DefaultReturnURL)
	}
	if err := validateChannelCurrency(normalized.Channel, normalized.PayMethod, normalized.Currency); err != nil {
		return nil, false, err
	}
	created, err := s.CreateOrderWithCheckoutToken(ctx, ManageOrderInput{
		AppID:           appID,
		MerchantOrderNo: normalized.MerchantOrderNo,
		BusinessType:    normalized.BusinessType,
		Subject:         normalized.Subject,
		Description:     normalized.Description,
		Amount:          normalized.Amount,
		Currency:        normalized.Currency,
		Channel:         normalized.Channel,
		PayMethod:       normalized.PayMethod,
		ReturnURL:       returnURL,
		Metadata:        normalized.Metadata,
	})
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

func (s Service) VerifyCheckoutToken(ctx context.Context, gatewayOrderNo string, token string) (*ent.PaymentOrder, bool, error) {
	order, err := s.FindOrderByGatewayOrderNo(ctx, gatewayOrderNo)
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(token) == "" || order.CheckoutTokenHash == "" {
		return order, false, nil
	}
	return order, constantTimeStringEqual(order.CheckoutTokenHash, HashCheckoutToken(token)), nil
}

func (s Service) SetCheckoutTokenHash(ctx context.Context, id int, tokenHash string) (*ent.PaymentOrder, error) {
	return s.orders.SetCheckoutTokenHash(ctx, id, tokenHash)
}

func (s Service) resolveAppDefaultReturnURL(ctx context.Context, appID string) (string, error) {
	app, err := s.apps.FindByAppID(ctx, appID)
	if err == nil {
		return strings.TrimSpace(app.DefaultReturnURL), nil
	}
	if isNotFoundError(err) {
		return "", nil
	}
	return "", err
}

func (s Service) resolveEnabledApp(ctx context.Context, appID string) (*ent.App, error) {
	app, err := s.apps.FindByAppID(ctx, appID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrAppNotFound
		}
		return nil, err
	}
	if app.Status != "enabled" {
		return nil, ErrAppDisabled
	}
	return app, nil
}

func (s Service) ListOrders(ctx context.Context, input ListOrdersInput) (*ListOrdersResult, error) {
	repoInput := orderrepo.ListOrdersInput{
		AppID:           strings.TrimSpace(input.AppID),
		Status:          strings.ToLower(strings.TrimSpace(input.Status)),
		Channel:         strings.ToLower(strings.TrimSpace(input.Channel)),
		Currency:        strings.ToUpper(strings.TrimSpace(input.Currency)),
		MerchantOrderNo: strings.TrimSpace(input.MerchantOrderNo),
		Page:            input.Page,
		PageSize:        input.PageSize,
	}
	items, total, err := s.orders.List(ctx, repoInput)
	if err != nil {
		return nil, err
	}
	page := repoInput.Page
	if page < 1 {
		page = 1
	}
	pageSize := repoInput.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return &ListOrdersResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s Service) FindOrder(ctx context.Context, id int) (*ent.PaymentOrder, error) {
	return s.orders.FindByID(ctx, id)
}

func (s Service) FindOrderByGatewayOrderNo(ctx context.Context, gatewayOrderNo string) (*ent.PaymentOrder, error) {
	return s.orders.FindByGatewayOrderNo(ctx, gatewayOrderNo)
}

func (s Service) FindOrderByGatewayOrderNoForApp(ctx context.Context, appID string, gatewayOrderNo string) (*ent.PaymentOrder, error) {
	return s.orders.FindByGatewayOrderNoForApp(ctx, appID, gatewayOrderNo)
}

func (s Service) FindOrderByMerchantOrderNoForApp(ctx context.Context, appID string, merchantOrderNo string) (*ent.PaymentOrder, error) {
	return s.orders.FindByMerchantOrderNo(ctx, appID, merchantOrderNo)
}

func (s Service) UpdateOrder(ctx context.Context, id int, input UpdateOrderInput) (*ent.PaymentOrder, error) {
	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		return nil, ErrSubjectRequired
	}
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	return s.orders.Update(ctx, id, orderrepo.UpdateOrderInput{
		BusinessType: strings.TrimSpace(input.BusinessType),
		Subject:      subject,
		Description:  strings.TrimSpace(input.Description),
		Channel:      strings.ToLower(strings.TrimSpace(input.Channel)),
		PayMethod:    strings.ToLower(strings.TrimSpace(input.PayMethod)),
		Metadata:     metadata,
	})
}

func (s Service) CloseOrder(ctx context.Context, id int) (*ent.PaymentOrder, error) {
	existing, err := s.orders.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Status != "pending" && existing.Status != "failed" {
		return nil, ErrOrderCannotBeClosed
	}
	return s.orders.SetStatus(ctx, id, "closed", s.now())
}

func (s Service) CloseOrderForApp(ctx context.Context, appID string, gatewayOrderNo string) (*ent.PaymentOrder, error) {
	existing, err := s.orders.FindByGatewayOrderNoForApp(ctx, appID, gatewayOrderNo)
	if err != nil {
		return nil, err
	}
	if existing.Status != "pending" && existing.Status != "failed" {
		return nil, ErrOrderCannotBeClosed
	}
	return s.orders.SetStatus(ctx, existing.ID, "closed", s.now())
}

func (s Service) ScanExpiredPendingOrders(ctx context.Context, limit int) (int, error) {
	now := s.now()
	orders, err := s.orders.ListExpiredPending(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	enqueued := 0
	for _, order := range orders {
		if order.Status != "pending" {
			continue
		}
		if order.ExpiresAt == nil || order.ExpiresAt.After(now) {
			continue
		}
		if err := s.enqueueOrderExpiration(ctx, order.ID); err != nil {
			return enqueued, err
		}
		enqueued++
	}
	return enqueued, nil
}

func (s Service) CloseExpiredPendingOrder(ctx context.Context, id int) (bool, error) {
	closed, err := s.orders.CloseExpiredPending(ctx, id, s.now())
	if err != nil || !closed {
		return closed, err
	}
	if s.webhooks.IsZero() {
		return true, nil
	}
	order, err := s.orders.FindByID(ctx, id)
	if err != nil {
		return true, err
	}
	if _, err := s.webhooks.RecordOrderExpired(ctx, order); err != nil {
		return true, err
	}
	return true, nil
}

func (s Service) MarkPaid(ctx context.Context, id int, channelTradeNo string) (*ent.PaymentOrder, error) {
	existing, err := s.orders.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Status == "paid" {
		if !s.webhooks.IsZero() {
			if _, err := s.webhooks.RecordPaymentSucceeded(ctx, existing); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}
	if existing.Status != "pending" && existing.Status != "failed" {
		return nil, ErrOrderCannotBePaid
	}
	paid, err := s.orders.MarkPaid(ctx, id, strings.TrimSpace(channelTradeNo), s.now())
	if err != nil {
		return nil, err
	}
	if existing.Status != "paid" && !s.webhooks.IsZero() {
		if _, err := s.webhooks.RecordPaymentSucceeded(ctx, paid); err != nil {
			return nil, err
		}
	}
	return paid, nil
}

func (s Service) MarkFailed(ctx context.Context, id int, reason string) (*ent.PaymentOrder, error) {
	existing, err := s.orders.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Status == "failed" {
		if !s.webhooks.IsZero() {
			if _, err := s.webhooks.RecordPaymentFailed(ctx, existing); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}
	if existing.Status != "pending" {
		return nil, ErrOrderCannotBeFailed
	}
	failed, err := s.orders.MarkFailed(ctx, id, strings.TrimSpace(reason), s.now())
	if err != nil {
		return nil, err
	}
	if !s.webhooks.IsZero() {
		if _, err := s.webhooks.RecordPaymentFailed(ctx, failed); err != nil {
			return nil, err
		}
	}
	return failed, nil
}

func (s Service) ApplyPaymentResult(ctx context.Context, id int, input PaymentResultInput) (*ent.PaymentOrder, error) {
	existing, err := s.orders.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	if existing.Channel != "" && !strings.EqualFold(existing.Channel, channel) {
		return nil, ErrPaymentChannelMismatch
	}
	if existing.ChannelAccountID > 0 && existing.ChannelAccountID != input.ChannelAccountID {
		return nil, ErrPaymentAccountMismatch
	}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status == "paid" {
		if existing.Amount != input.Amount {
			return nil, ErrPaymentAmountMismatch
		}
		if !strings.EqualFold(existing.Currency, strings.TrimSpace(input.Currency)) {
			return nil, ErrPaymentCurrencyMismatch
		}
	}
	if existing.Channel == "" || existing.PayMethod == "" || existing.ChannelAccountID == 0 {
		payMethod := existing.PayMethod
		if payMethod == "" {
			payMethod = channel
		}
		existing, err = s.orders.SetPaymentSelection(ctx, id, channel, payMethod, input.ChannelAccountID)
		if err != nil {
			return nil, err
		}
	}
	switch status {
	case "paid":
		return s.MarkPaid(ctx, id, input.ChannelTradeNo)
	case "failed":
		return s.MarkFailed(ctx, id, input.FailureReason)
	case "closed":
		if existing.Status == "closed" {
			return existing, nil
		}
		return s.CloseOrder(ctx, id)
	default:
		return existing, nil
	}
}

func (s Service) defaultExpiresAt(ctx context.Context) *time.Time {
	ttl := s.defaultOrderTTL
	if s.defaultOrderTTLResolver != nil {
		if resolved, err := s.defaultOrderTTLResolver(ctx); err == nil {
			ttl = resolved
		}
	}
	if ttl <= 0 {
		return nil
	}
	expiresAt := s.now().Add(ttl)
	return &expiresAt
}

func (s Service) enqueueOrderExpiration(ctx context.Context, orderID int) error {
	if s.enqueuer == nil {
		_, err := s.CloseExpiredPendingOrder(ctx, orderID)
		return err
	}
	return s.enqueuer.EnqueueOrderExpiration(ctx, orderID)
}

func (s Service) SetChannelTradeNo(ctx context.Context, id int, channelTradeNo string) (*ent.PaymentOrder, error) {
	return s.orders.SetChannelTradeNo(ctx, id, strings.TrimSpace(channelTradeNo))
}

func (s Service) SetProviderOrderNo(ctx context.Context, id int, providerOrderNo string) (*ent.PaymentOrder, error) {
	return s.orders.SetProviderOrderNo(ctx, id, strings.TrimSpace(providerOrderNo))
}

func (s Service) SetPaymentSelection(ctx context.Context, id int, channel string, payMethod string, channelAccountID int) (*ent.PaymentOrder, error) {
	return s.orders.SetPaymentSelection(ctx, id, strings.ToLower(strings.TrimSpace(channel)), strings.ToLower(strings.TrimSpace(payMethod)), channelAccountID)
}

func normalizeManageInput(input ManageOrderInput) (ManageOrderInput, error) {
	appID := strings.TrimSpace(input.AppID)
	if appID == "" {
		return ManageOrderInput{}, ErrAppIDRequired
	}
	merchantOrderNo := strings.TrimSpace(input.MerchantOrderNo)
	if merchantOrderNo == "" {
		return ManageOrderInput{}, ErrMerchantOrderNoRequired
	}
	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		return ManageOrderInput{}, ErrSubjectRequired
	}
	if input.Amount <= 0 {
		return ManageOrderInput{}, ErrInvalidAmount
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if !supportedCurrency(currency) {
		return ManageOrderInput{}, ErrInvalidCurrency
	}
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	return ManageOrderInput{
		AppID:              appID,
		MerchantOrderNo:    merchantOrderNo,
		BusinessType:       strings.TrimSpace(input.BusinessType),
		Subject:            subject,
		Description:        strings.TrimSpace(input.Description),
		Amount:             input.Amount,
		Currency:           currency,
		SettlementAmount:   input.SettlementAmount,
		SettlementCurrency: strings.ToUpper(strings.TrimSpace(input.SettlementCurrency)),
		Channel:            strings.ToLower(strings.TrimSpace(input.Channel)),
		PayMethod:          strings.ToLower(strings.TrimSpace(input.PayMethod)),
		ReturnURL:          strings.TrimSpace(input.ReturnURL),
		ExpiresAt:          input.ExpiresAt,
		Metadata:           metadata,
	}, nil
}

func normalizeOpenOrderInput(input OpenOrderInput) (OpenOrderInput, error) {
	merchantOrderNo := strings.TrimSpace(input.MerchantOrderNo)
	if merchantOrderNo == "" {
		return OpenOrderInput{}, ErrMerchantOrderNoRequired
	}
	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		return OpenOrderInput{}, ErrSubjectRequired
	}
	if input.Amount <= 0 {
		return OpenOrderInput{}, ErrInvalidAmount
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if !supportedCurrency(currency) {
		return OpenOrderInput{}, ErrInvalidCurrency
	}
	return OpenOrderInput{
		MerchantOrderNo:  merchantOrderNo,
		BusinessType:     strings.TrimSpace(input.BusinessType),
		Subject:          subject,
		Description:      strings.TrimSpace(input.Description),
		Amount:           input.Amount,
		Currency:         currency,
		Channel:          strings.ToLower(strings.TrimSpace(input.Channel)),
		PayMethod:        strings.ToLower(strings.TrimSpace(input.PayMethod)),
		PreferredChannel: strings.ToLower(strings.TrimSpace(input.PreferredChannel)),
		ClientIP:         strings.TrimSpace(input.ClientIP),
		ReturnURL:        strings.TrimSpace(input.ReturnURL),
		Metadata:         input.Metadata,
	}, nil
}

func sameOpenOrder(existing *ent.PaymentOrder, appID string, input OpenOrderInput) bool {
	if existing.AppID != appID {
		return false
	}
	return existing.MerchantOrderNo == input.MerchantOrderNo &&
		existing.Amount == input.Amount &&
		existing.Currency == input.Currency &&
		existing.Subject == input.Subject &&
		existing.BusinessType == input.BusinessType &&
		existing.PayMethod == input.PayMethod &&
		existing.Channel == input.Channel
}

func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}

func supportedCurrency(currency string) bool {
	switch currency {
	case "CNY", "USD", "EUR", "HKD", "JPY", "GBP":
		return true
	default:
		return false
	}
}

func validateChannelCurrency(channel string, payMethod string, currency string) error {
	paymentChannel := strings.ToLower(strings.TrimSpace(channel))
	if paymentChannel == "" {
		paymentChannel = strings.ToLower(strings.TrimSpace(payMethod))
	}
	if paymentChannel == "" {
		return nil
	}
	if !paymentsvc.ChannelSupportsCurrency(paymentChannel, currency) {
		return ErrUnsupportedCurrencyForChannel
	}
	return nil
}

func newGatewayOrderNo(now time.Time) (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	suffix := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))
	return "pay_" + now.UTC().Format("20060102") + "_" + suffix, nil
}

func NewCheckoutToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)), nil
}

func HashCheckoutToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func constantTimeStringEqual(left string, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
