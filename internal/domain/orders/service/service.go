package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
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
	ErrAppNotFound                   = errors.New("app not found")
	ErrAppDisabled                   = errors.New("app is disabled")
	ErrUnsupportedCurrencyForChannel = errors.New("currency is not supported by channel")
)

type Service struct {
	orders          orderrepo.Repository
	apps            apprepo.Repository
	webhooks        webhooksvc.Service
	enqueuer        ExpirationEnqueuer
	now             func() time.Time
	defaultOrderTTL time.Duration
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

func (s Service) CreateOrder(ctx context.Context, input ManageOrderInput) (*ent.PaymentOrder, error) {
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
		expiresAt = s.defaultExpiresAt()
	}
	gatewayOrderNo, err := newGatewayOrderNo(s.now())
	if err != nil {
		return nil, err
	}
	return s.orders.Create(ctx, orderrepo.CreateOrderInput{
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
		Status:             "pending",
		ExpiresAt:          expiresAt,
		Metadata:           normalized.Metadata,
	})
}

func (s Service) CreateOpenOrder(ctx context.Context, appID string, input OpenOrderInput) (*ent.PaymentOrder, bool, error) {
	normalized, err := normalizeOpenOrderInput(input)
	if err != nil {
		return nil, false, err
	}
	existing, err := s.orders.FindByMerchantOrderNo(ctx, appID, normalized.MerchantOrderNo)
	if err == nil {
		if sameOpenOrder(existing, appID, normalized) {
			return existing, false, nil
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
	created, err := s.CreateOrder(ctx, ManageOrderInput{
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

func (s Service) defaultExpiresAt() *time.Time {
	if s.defaultOrderTTL <= 0 {
		return nil
	}
	expiresAt := s.now().Add(s.defaultOrderTTL)
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

func (s Service) SetPaymentSelection(ctx context.Context, id int, channel string, payMethod string) (*ent.PaymentOrder, error) {
	return s.orders.SetPaymentSelection(ctx, id, strings.ToLower(strings.TrimSpace(channel)), strings.ToLower(strings.TrimSpace(payMethod)))
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
