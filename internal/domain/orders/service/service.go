package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
	"time"

	"payment-gateway/ent"
	orderrepo "payment-gateway/internal/domain/orders/repository"
)

var (
	ErrAppIDRequired           = errors.New("app_id is required")
	ErrMerchantOrderNoRequired = errors.New("merchant_order_no is required")
	ErrSubjectRequired         = errors.New("subject is required")
	ErrInvalidAmount           = errors.New("amount must be greater than zero")
	ErrInvalidCurrency         = errors.New("invalid currency")
	ErrDuplicateOrder          = errors.New("merchant order already exists for app")
	ErrOrderCannotBeClosed     = errors.New("order cannot be closed in current status")
)

type Service struct {
	orders orderrepo.Repository
	now    func() time.Time
}

func New(client *ent.Client) Service {
	return Service{orders: orderrepo.New(client), now: time.Now}
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
		Status:             "pending",
		ExpiresAt:          normalized.ExpiresAt,
		Metadata:           normalized.Metadata,
	})
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

func (s Service) MarkPaid(ctx context.Context, id int, channelTradeNo string) (*ent.PaymentOrder, error) {
	return s.orders.MarkPaid(ctx, id, strings.TrimSpace(channelTradeNo), s.now())
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
		ExpiresAt:          input.ExpiresAt,
		Metadata:           metadata,
	}, nil
}

func supportedCurrency(currency string) bool {
	switch currency {
	case "CNY", "USD", "EUR", "HKD", "JPY", "GBP":
		return true
	default:
		return false
	}
}

func newGatewayOrderNo(now time.Time) (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	suffix := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))
	return "pay_" + now.UTC().Format("20060102") + "_" + suffix, nil
}
