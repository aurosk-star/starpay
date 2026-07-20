package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"payment-gateway/ent"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	reconciliationrepo "payment-gateway/internal/domain/reconciliations/repository"
)

const maxAttempts = 8

var ErrOrderNotEligible = errors.New("payment order is not eligible for reconciliation")

type PaymentGateway interface {
	QueryPayment(context.Context, paymentsvc.QueryPaymentInput) (*paymentsvc.NotifyResult, error)
	ClosePayment(context.Context, paymentsvc.ClosePaymentInput) error
}

type OrderService interface {
	FindOrder(context.Context, int) (*ent.PaymentOrder, error)
	ApplyPaymentResult(context.Context, int, ordersvc.PaymentResultInput) (*ent.PaymentOrder, error)
	CloseExpiredPendingOrder(context.Context, int) (bool, error)
}

type Service struct {
	reconciliations reconciliationrepo.Repository
	payments        PaymentGateway
	orders          OrderService
	now             func() time.Time
	enqueuer        Enqueuer
}

type ListInput struct {
	Status         string
	Channel        string
	GatewayOrderNo string
	Page           int
	PageSize       int
}

type ListResult struct {
	Items    []*ent.PaymentReconciliation `json:"items"`
	Total    int                          `json:"total"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
}

type Option func(*Service)

func WithPaymentGateway(gateway PaymentGateway) Option {
	return func(s *Service) { s.payments = gateway }
}

func WithOrderService(orders OrderService) Option {
	return func(s *Service) { s.orders = orders }
}

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func WithEnqueuer(enqueuer Enqueuer) Option {
	return func(s *Service) { s.enqueuer = enqueuer }
}

func New(client *ent.Client, options ...Option) Service {
	s := Service{reconciliations: reconciliationrepo.New(client), now: time.Now}
	for _, option := range options {
		option(&s)
	}
	return s
}

func (s Service) EnsureForOrder(ctx context.Context, order *ent.PaymentOrder) (*ent.PaymentReconciliation, error) {
	if order == nil || order.Status != "pending" || strings.TrimSpace(order.Channel) == "" || order.ChannelAccountID <= 0 || strings.TrimSpace(order.ProviderOrderNo) == "" {
		return nil, ErrOrderNotEligible
	}
	next := s.now().Add(2 * time.Minute)
	if order.ExpiresAt != nil && order.ExpiresAt.Before(next) {
		next = *order.ExpiresAt
	}
	return s.reconciliations.EnsureForOrder(ctx, order, next)
}

func (s Service) RequestForOrder(ctx context.Context, order *ent.PaymentOrder) (*ent.PaymentReconciliation, error) {
	if order == nil {
		return nil, ErrOrderNotEligible
	}
	if order.Status != "pending" {
		item, err := s.reconciliations.FindByOrderID(ctx, order.ID)
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return item, err
	}
	item, err := s.EnsureForOrder(ctx, order)
	if err != nil {
		return nil, err
	}
	if item.Status != "pending" {
		return item, nil
	}
	requestedAt := s.now()
	item, changed, err := s.reconciliations.RequestActiveQuery(ctx, item.ID, requestedAt)
	if err != nil {
		return nil, err
	}
	if changed && s.enqueuer != nil {
		if err := s.enqueuer.EnqueuePaymentReconciliation(ctx, item.ID); err != nil {
			_ = s.reconciliations.ReleaseActiveQueryRequest(ctx, item.ID, requestedAt)
			return nil, err
		}
	}
	return item, nil
}

func (s Service) Process(ctx context.Context, id int) (*ent.PaymentReconciliation, error) {
	now := s.now()
	reconciliation, claimed, err := s.reconciliations.Claim(ctx, id, now)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return s.reconciliations.FindByID(ctx, id)
	}
	order, err := s.orders.FindOrder(ctx, reconciliation.PaymentOrderID)
	if err != nil {
		return s.recordFailure(ctx, reconciliation, "", nil, err)
	}
	if order.Status != "pending" {
		return s.resolve(ctx, reconciliation, order.Status, nil)
	}
	if s.payments == nil {
		return s.recordFailure(ctx, reconciliation, "", nil, paymentsvc.ErrQueryUnsupported)
	}
	result, err := s.payments.QueryPayment(ctx, paymentsvc.QueryPaymentInput{Order: order})
	if err != nil {
		return s.recordFailure(ctx, reconciliation, "", nil, err)
	}
	providerStatus := strings.ToLower(strings.TrimSpace(result.Status))
	snapshot := result.Raw
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	switch providerStatus {
	case "paid", "failed", "closed":
		if _, err := s.orders.ApplyPaymentResult(ctx, order.ID, ordersvc.PaymentResultInput{
			Channel: result.Channel, ChannelAccountID: result.ChannelAccountID,
			ChannelTradeNo: result.ChannelTradeNo, Status: providerStatus,
			Amount: result.Amount, Currency: result.Currency, FailureReason: result.FailureReason,
		}); err != nil {
			return s.recordFailure(ctx, reconciliation, providerStatus, snapshot, err)
		}
		return s.resolve(ctx, reconciliation, providerStatus, snapshot)
	default:
		if order.ExpiresAt != nil && !order.ExpiresAt.After(now) {
			closeErr := s.payments.ClosePayment(ctx, paymentsvc.ClosePaymentInput{Order: order})
			if closeErr != nil && !(errors.Is(closeErr, paymentsvc.ErrCloseUnsupported) && strings.EqualFold(order.Channel, "paypal")) {
				return s.recordFailure(ctx, reconciliation, providerStatus, snapshot, closeErr)
			}
			closed, err := s.orders.CloseExpiredPendingOrder(ctx, order.ID)
			if err != nil {
				return s.recordFailure(ctx, reconciliation, providerStatus, snapshot, err)
			}
			if closed {
				return s.resolve(ctx, reconciliation, "closed", snapshot)
			}
		}
		return s.reschedule(ctx, reconciliation, providerStatus, snapshot, "")
	}
}

func (s Service) Retry(ctx context.Context, id int) (*ent.PaymentReconciliation, error) {
	item, err := s.reconciliations.ResetForRetry(ctx, id, s.now())
	if err != nil {
		return nil, err
	}
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueuePaymentReconciliation(ctx, item.ID); err != nil {
			return nil, err
		}
	}
	return item, nil
}

func (s Service) Get(ctx context.Context, id int) (*ent.PaymentReconciliation, error) {
	return s.reconciliations.FindByID(ctx, id)
}

func (s Service) List(ctx context.Context, input ListInput) (*ListResult, error) {
	repoInput := reconciliationrepo.ListInput{Status: strings.ToLower(strings.TrimSpace(input.Status)), Channel: strings.ToLower(strings.TrimSpace(input.Channel)), GatewayOrderNo: strings.TrimSpace(input.GatewayOrderNo), Page: input.Page, PageSize: input.PageSize}
	items, total, err := s.reconciliations.List(ctx, repoInput)
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

func (s Service) ScanDue(ctx context.Context, limit int) (int, error) {
	now := s.now()
	if _, err := s.reconciliations.RecoverStale(ctx, now.Add(-5*time.Minute)); err != nil {
		return 0, err
	}
	orders, err := s.reconciliations.ListEligibleOrders(ctx, limit)
	if err != nil {
		return 0, err
	}
	for _, order := range orders {
		if _, err := s.EnsureForOrder(ctx, order); err != nil && !errors.Is(err, ErrOrderNotEligible) {
			return 0, err
		}
	}
	items, err := s.reconciliations.ListDue(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range items {
		if s.enqueuer != nil {
			if err := s.enqueuer.EnqueuePaymentReconciliation(ctx, item.ID); err != nil {
				return count, err
			}
		} else if _, err := s.Process(ctx, item.ID); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s Service) resolve(ctx context.Context, item *ent.PaymentReconciliation, providerStatus string, snapshot map[string]any) (*ent.PaymentReconciliation, error) {
	now := s.now()
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	return s.reconciliations.CompleteAttempt(ctx, item.ID, reconciliationrepo.CompleteAttemptInput{
		Status: "resolved", AttemptCount: item.AttemptCount + 1,
		LastProviderStatus: providerStatus, ProviderSnapshot: snapshot, ResolvedAt: &now,
	})
}

func (s Service) recordFailure(ctx context.Context, item *ent.PaymentReconciliation, providerStatus string, snapshot map[string]any, failure error) (*ent.PaymentReconciliation, error) {
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	return s.reschedule(ctx, item, providerStatus, snapshot, message)
}

func (s Service) reschedule(ctx context.Context, item *ent.PaymentReconciliation, providerStatus string, snapshot map[string]any, lastError string) (*ent.PaymentReconciliation, error) {
	attempts := item.AttemptCount + 1
	status := "pending"
	var next *time.Time
	if attempts >= maxAttempts {
		status = "manual_required"
	} else {
		value := s.now().Add(retryDelay(attempts))
		next = &value
	}
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	return s.reconciliations.CompleteAttempt(ctx, item.ID, reconciliationrepo.CompleteAttemptInput{
		Status: status, AttemptCount: attempts, NextAttemptAt: next,
		LastProviderStatus: providerStatus, LastError: lastError, ProviderSnapshot: snapshot,
	})
}

func retryDelay(attempt int) time.Duration {
	delays := []time.Duration{2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour, 6 * time.Hour, 24 * time.Hour}
	if attempt < 1 {
		attempt = 1
	}
	if attempt > len(delays) {
		attempt = len(delays)
	}
	return delays[attempt-1]
}
