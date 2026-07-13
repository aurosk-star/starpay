package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"reflect"
	"strings"
	"time"

	"payment-gateway/ent"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	refundrepo "payment-gateway/internal/domain/refunds/repository"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
)

var (
	ErrAppIDRequired            = errors.New("app_id is required")
	ErrGatewayOrderNoRequired   = errors.New("gateway_order_no is required")
	ErrMerchantRefundNoRequired = errors.New("merchant_refund_no is required")
	ErrInvalidRefundAmount      = errors.New("refund amount must be greater than zero")
	ErrRefundAmountExceedsPaid  = errors.New("refund amount exceeds paid amount")
	ErrRefundStatusNotAllowed   = errors.New("payment order status does not allow refund")
	ErrRefundCurrencyMismatch   = errors.New("refund currency does not match payment order")
	ErrIdempotencyConflict      = errors.New("refund idempotency conflict")
)

type PaymentGateway interface {
	CreateRefund(context.Context, paymentsvc.CreateRefundInput) (*paymentsvc.RefundResult, error)
	QueryRefund(context.Context, paymentsvc.QueryRefundInput) (*paymentsvc.RefundResult, error)
}

type Service struct {
	client   *ent.Client
	refunds  refundrepo.Repository
	payments PaymentGateway
	webhooks webhooksvc.Service
	now      func() time.Time
	enqueuer Enqueuer
}

type Option func(*Service)

func WithPaymentGateway(gateway PaymentGateway) Option {
	return func(s *Service) { s.payments = gateway }
}
func WithWebhookService(webhooks webhooksvc.Service) Option {
	return func(s *Service) { s.webhooks = webhooks }
}
func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}
func WithDialect(name string) Option {
	return func(s *Service) { s.refunds = refundrepo.New(s.client, name) }
}
func WithEnqueuer(enqueuer Enqueuer) Option { return func(s *Service) { s.enqueuer = enqueuer } }

func New(client *ent.Client, options ...Option) Service {
	s := Service{client: client, refunds: refundrepo.New(client, "sqlite"), now: time.Now}
	for _, option := range options {
		option(&s)
	}
	return s
}

type CreateInput struct {
	AppID, GatewayOrderNo, MerchantRefundNo, Currency, Reason string
	Amount                                                    int64
	Metadata                                                  map[string]any
}

type ListInput struct {
	AppID, Status, Channel, GatewayOrderNo, MerchantRefundNo string
	Page, PageSize                                           int
}
type ListResult struct {
	Items                 []*ent.Refund `json:"items"`
	Total, Page, PageSize int
}

func (s Service) Create(ctx context.Context, input CreateInput) (*ent.Refund, bool, error) {
	input.AppID = strings.TrimSpace(input.AppID)
	input.GatewayOrderNo = strings.TrimSpace(input.GatewayOrderNo)
	input.MerchantRefundNo = strings.TrimSpace(input.MerchantRefundNo)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if input.AppID == "" {
		return nil, false, ErrAppIDRequired
	}
	if input.GatewayOrderNo == "" {
		return nil, false, ErrGatewayOrderNoRequired
	}
	if input.MerchantRefundNo == "" {
		return nil, false, ErrMerchantRefundNoRequired
	}
	if input.Amount <= 0 {
		return nil, false, ErrInvalidRefundAmount
	}
	if existing, err := s.refunds.FindByMerchant(ctx, input.AppID, input.MerchantRefundNo); err == nil {
		if sameRequest(existing, input) {
			return existing, false, nil
		}
		return nil, false, ErrIdempotencyConflict
	} else if !ent.IsNotFound(err) {
		return nil, false, err
	}
	refundNo, err := newRefundNo(s.now())
	if err != nil {
		return nil, false, err
	}
	created, order, err := s.refunds.Reserve(ctx, refundrepo.CreateInput{RefundNo: refundNo, AppID: input.AppID, GatewayOrderNo: input.GatewayOrderNo, MerchantRefundNo: input.MerchantRefundNo, Amount: input.Amount, Currency: input.Currency, Reason: input.Reason, Metadata: input.Metadata})
	if err != nil {
		switch {
		case errors.Is(err, refundrepo.ErrAmountUnavailable):
			return nil, false, ErrRefundAmountExceedsPaid
		case errors.Is(err, refundrepo.ErrOrderNotPaid), errors.Is(err, refundrepo.ErrOrderUnbound):
			return nil, false, ErrRefundStatusNotAllowed
		case errors.Is(err, refundrepo.ErrCurrencyMismatch):
			return nil, false, ErrRefundCurrencyMismatch
		}
		if existing, findErr := s.refunds.FindByMerchant(ctx, input.AppID, input.MerchantRefundNo); findErr == nil {
			if sameRequest(existing, input) {
				return existing, false, nil
			}
			return nil, false, ErrIdempotencyConflict
		}
		return nil, false, err
	}
	if s.payments == nil {
		return created, true, nil
	}
	result, providerErr := s.payments.CreateRefund(ctx, paymentsvc.CreateRefundInput{Channel: created.Channel, ChannelAccountID: created.ChannelAccountID, GatewayOrderNo: created.GatewayOrderNo, ProviderOrderNo: created.ProviderOrderNo, ChannelTradeNo: created.ChannelTradeNo, RefundNo: created.RefundNo, Amount: created.Amount, OriginalAmount: order.Amount, Currency: created.Currency, Reason: created.Reason})
	if providerErr != nil {
		updated, err := s.refunds.ApplyProviderResult(ctx, created.ID, refundrepo.ProviderResultInput{Status: "pending", LastError: providerErr.Error(), Snapshot: map[string]any{}, Now: s.now()})
		return updated, true, err
	}
	updated, err := s.applyResult(ctx, created, result)
	return updated, true, err
}

func (s Service) applyResult(ctx context.Context, current *ent.Refund, result *paymentsvc.RefundResult) (*ent.Refund, error) {
	if result == nil {
		return current, nil
	}
	status := strings.ToLower(strings.TrimSpace(result.Status))
	if status == "succeeded" && (result.Amount != current.Amount || !strings.EqualFold(result.Currency, current.Currency)) {
		return s.refunds.ApplyProviderResult(ctx, current.ID, refundrepo.ProviderResultInput{Status: "pending", LastError: "provider refund amount or currency mismatch", Snapshot: result.Raw, Now: s.now()})
	}
	updated, err := s.refunds.ApplyProviderResult(ctx, current.ID, refundrepo.ProviderResultInput{Status: status, ChannelRefundNo: result.ChannelRefundNo, FailureReason: result.FailureReason, Amount: result.Amount, Currency: result.Currency, Snapshot: result.Raw, Now: s.now()})
	if err != nil {
		return nil, err
	}
	if s.webhooks.IsZero() {
		return updated, nil
	}
	if status == "succeeded" {
		_, err = s.webhooks.RecordRefundSucceeded(ctx, updated)
	}
	if status == "failed" {
		_, err = s.webhooks.RecordRefundFailed(ctx, updated)
	}
	return updated, err
}

func (s Service) Get(ctx context.Context, id int) (*ent.Refund, error) {
	return s.refunds.FindByID(ctx, id)
}
func (s Service) GetForApp(ctx context.Context, appID, refundNo string) (*ent.Refund, error) {
	return s.refunds.FindByRefundNoForApp(ctx, strings.TrimSpace(appID), strings.TrimSpace(refundNo))
}
func (s Service) GetByMerchantForApp(ctx context.Context, appID, merchantRefundNo string) (*ent.Refund, error) {
	return s.refunds.FindByMerchant(ctx, strings.TrimSpace(appID), strings.TrimSpace(merchantRefundNo))
}

func (s Service) List(ctx context.Context, input ListInput) (*ListResult, error) {
	repoInput := refundrepo.ListInput{AppID: strings.TrimSpace(input.AppID), Status: strings.ToLower(strings.TrimSpace(input.Status)), Channel: strings.ToLower(strings.TrimSpace(input.Channel)), GatewayOrderNo: strings.TrimSpace(input.GatewayOrderNo), MerchantRefundNo: strings.TrimSpace(input.MerchantRefundNo), Page: input.Page, PageSize: input.PageSize}
	items, total, err := s.refunds.List(ctx, repoInput)
	if err != nil {
		return nil, err
	}
	page, pageSize := input.Page, input.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return &ListResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s Service) Process(ctx context.Context, id int) (*ent.Refund, error) {
	current, err := s.refunds.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Status != "pending" || s.payments == nil {
		return current, nil
	}
	order, err := s.client.PaymentOrder.Get(ctx, current.PaymentOrderID)
	if err != nil {
		return nil, err
	}
	var result *paymentsvc.RefundResult
	if current.ChannelRefundNo == "" {
		result, err = s.payments.CreateRefund(ctx, paymentsvc.CreateRefundInput{Channel: current.Channel, ChannelAccountID: current.ChannelAccountID, GatewayOrderNo: current.GatewayOrderNo, ProviderOrderNo: current.ProviderOrderNo, ChannelTradeNo: current.ChannelTradeNo, RefundNo: current.RefundNo, Amount: current.Amount, OriginalAmount: order.Amount, Currency: current.Currency, Reason: current.Reason})
	} else {
		result, err = s.payments.QueryRefund(ctx, paymentsvc.QueryRefundInput{Channel: current.Channel, ChannelAccountID: current.ChannelAccountID, GatewayOrderNo: current.GatewayOrderNo, ChannelTradeNo: current.ChannelTradeNo, RefundNo: current.RefundNo, ChannelRefundNo: current.ChannelRefundNo})
	}
	if err != nil {
		attempts := current.AttemptCount + 1
		next := s.now().Add(refundRetryDelay(attempts))
		updated, updateErr := s.refunds.ApplyProviderResult(ctx, current.ID, refundrepo.ProviderResultInput{Status: "pending", LastError: err.Error(), Snapshot: current.ProviderSnapshot, Now: s.now()})
		if updateErr == nil && attempts >= 8 {
			updated, updateErr = s.refunds.StopAutomaticRetry(ctx, current.ID, attempts, err.Error(), s.now())
		} else if updateErr == nil {
			updated, updateErr = s.refunds.SetRetry(ctx, current.ID, attempts, next, err.Error(), s.now())
		}
		return updated, updateErr
	}
	return s.applyResult(ctx, current, result)
}

func (s Service) Retry(ctx context.Context, id int) (*ent.Refund, error) {
	item, err := s.refunds.ResetForRetry(ctx, id, s.now())
	if err != nil {
		return nil, err
	}
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueRefund(ctx, item.ID); err != nil {
			return nil, err
		}
	}
	return item, nil
}

func (s Service) ScanDue(ctx context.Context, limit int) (int, error) {
	items, err := s.refunds.ListDue(ctx, s.now(), limit)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range items {
		if s.enqueuer != nil {
			if err := s.enqueuer.EnqueueRefund(ctx, item.ID); err != nil {
				return count, err
			}
		} else if _, err := s.Process(ctx, item.ID); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func refundRetryDelay(attempt int) time.Duration {
	delays := []time.Duration{2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour, 6 * time.Hour, 24 * time.Hour}
	if attempt < 1 {
		attempt = 1
	}
	if attempt > len(delays) {
		attempt = len(delays)
	}
	return delays[attempt-1]
}

func sameRequest(existing *ent.Refund, input CreateInput) bool {
	return existing.AppID == input.AppID && existing.GatewayOrderNo == input.GatewayOrderNo && existing.MerchantRefundNo == input.MerchantRefundNo && existing.Amount == input.Amount && strings.EqualFold(existing.Currency, input.Currency) && existing.Reason == input.Reason && reflect.DeepEqual(existing.Metadata, input.Metadata)
}

func newRefundNo(now time.Time) (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	encoded := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw[:]))
	return "rf_" + now.UTC().Format("20060102") + "_" + encoded, nil
}
