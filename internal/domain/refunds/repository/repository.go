package repository

import (
	"context"
	"errors"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	"payment-gateway/ent"
	"payment-gateway/ent/paymentorder"
	"payment-gateway/ent/predicate"
	"payment-gateway/ent/refund"
)

var ErrAmountUnavailable = errors.New("refund amount exceeds available paid amount")
var ErrOrderNotPaid = errors.New("payment order is not paid")
var ErrOrderUnbound = errors.New("payment order has no refundable provider transaction")
var ErrCurrencyMismatch = errors.New("refund currency does not match payment order")

type Repository struct {
	client  *ent.Client
	dialect string
}

func New(client *ent.Client, dialect string) Repository {
	return Repository{client: client, dialect: dialect}
}

type CreateInput struct {
	RefundNo, AppID, GatewayOrderNo, MerchantRefundNo, Currency, Reason string
	Amount                                                              int64
	Metadata                                                            map[string]any
}

func (r Repository) FindByMerchant(ctx context.Context, appID, merchantRefundNo string) (*ent.Refund, error) {
	return r.client.Refund.Query().Where(refund.AppID(appID), refund.MerchantRefundNo(merchantRefundNo)).Only(ctx)
}

func (r Repository) FindByID(ctx context.Context, id int) (*ent.Refund, error) {
	return r.client.Refund.Get(ctx, id)
}

func (r Repository) FindByRefundNoForApp(ctx context.Context, appID, refundNo string) (*ent.Refund, error) {
	return r.client.Refund.Query().Where(refund.AppID(appID), refund.RefundNo(refundNo)).Only(ctx)
}

type ListInput struct {
	AppID, Status, Channel, GatewayOrderNo, MerchantRefundNo string
	Page, PageSize                                           int
}

func (r Repository) List(ctx context.Context, input ListInput) ([]*ent.Refund, int, error) {
	query := r.client.Refund.Query()
	if input.AppID != "" {
		query.Where(refund.AppID(input.AppID))
	}
	if input.Status != "" {
		query.Where(refund.Status(input.Status))
	}
	if input.Channel != "" {
		query.Where(refund.Channel(input.Channel))
	}
	if input.GatewayOrderNo != "" {
		query.Where(refund.GatewayOrderNoContains(input.GatewayOrderNo))
	}
	if input.MerchantRefundNo != "" {
		query.Where(refund.MerchantRefundNoContains(input.MerchantRefundNo))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
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
	items, err := query.Order(ent.Desc(refund.FieldCreatedAt)).Offset((page - 1) * pageSize).Limit(pageSize).All(ctx)
	return items, total, err
}

func (r Repository) ListDue(ctx context.Context, now time.Time, limit int) ([]*ent.Refund, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return r.client.Refund.Query().Where(refund.Status("pending"), refund.NextAttemptAtNotNil(), refund.NextAttemptAtLTE(now)).Order(ent.Asc(refund.FieldNextAttemptAt)).Limit(limit).All(ctx)
}

func (r Repository) ResetForRetry(ctx context.Context, id int, now time.Time) (*ent.Refund, error) {
	if _, err := r.client.Refund.UpdateOneID(id).SetStatus("pending").SetAttemptCount(0).SetNextAttemptAt(now).ClearLastAttemptAt().ClearLastError().ClearFailureReason().ClearFailedAt().Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) SetRetry(ctx context.Context, id, attempts int, next time.Time, lastError string, now time.Time) (*ent.Refund, error) {
	if _, err := r.client.Refund.UpdateOneID(id).SetStatus("pending").SetAttemptCount(attempts).SetNextAttemptAt(next).SetLastAttemptAt(now).SetLastError(lastError).Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) StopAutomaticRetry(ctx context.Context, id, attempts int, lastError string, now time.Time) (*ent.Refund, error) {
	if _, err := r.client.Refund.UpdateOneID(id).SetStatus("pending").SetAttemptCount(attempts).ClearNextAttemptAt().SetLastAttemptAt(now).SetLastError(lastError).Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) Reserve(ctx context.Context, input CreateInput) (*ent.Refund, *ent.PaymentOrder, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, nil, err
	}
	rollback := func(cause error) (*ent.Refund, *ent.PaymentOrder, error) { _ = tx.Rollback(); return nil, nil, cause }
	predicates := []predicate.PaymentOrder{paymentorder.AppID(input.AppID), paymentorder.GatewayOrderNo(input.GatewayOrderNo)}
	if r.dialect != "sqlite3" && r.dialect != "sqlite" && r.dialect != "" {
		predicates = append(predicates, func(selector *entsql.Selector) { selector.ForUpdate() })
	}
	order, err := tx.PaymentOrder.Query().Where(predicates...).Only(ctx)
	if err != nil {
		return rollback(err)
	}
	if order.Status != "paid" {
		return rollback(ErrOrderNotPaid)
	}
	if order.ChannelAccountID <= 0 || order.ChannelTradeNo == "" || order.Channel == "" {
		return rollback(ErrOrderUnbound)
	}
	if order.Currency != input.Currency {
		return rollback(ErrCurrencyMismatch)
	}
	reservedRefunds, err := tx.Refund.Query().Where(refund.PaymentOrderID(order.ID), refund.StatusIn("pending", "succeeded")).All(ctx)
	if err != nil {
		return rollback(err)
	}
	reserved := int64(0)
	for _, item := range reservedRefunds {
		reserved += item.Amount
	}
	if input.Amount > order.Amount-reserved {
		return rollback(ErrAmountUnavailable)
	}
	created, err := tx.Refund.Create().
		SetRefundNo(input.RefundNo).SetAppID(input.AppID).SetPaymentOrderID(order.ID).
		SetGatewayOrderNo(order.GatewayOrderNo).SetMerchantOrderNo(order.MerchantOrderNo).
		SetMerchantRefundNo(input.MerchantRefundNo).SetChannel(order.Channel).
		SetChannelAccountID(order.ChannelAccountID).SetProviderOrderNo(order.ProviderOrderNo).
		SetChannelTradeNo(order.ChannelTradeNo).SetAmount(input.Amount).SetCurrency(input.Currency).
		SetReason(input.Reason).SetStatus("pending").SetMetadata(input.Metadata).
		SetProviderSnapshot(map[string]any{}).SetNextAttemptAt(time.Now()).Save(ctx)
	if err != nil {
		return rollback(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	created.Unwrap()
	order.Unwrap()
	return created, order, nil
}

type ProviderResultInput struct {
	Status, ChannelRefundNo, FailureReason, LastError string
	Amount                                            int64
	Currency                                          string
	Snapshot                                          map[string]any
	Now                                               time.Time
}

func (r Repository) ApplyProviderResult(ctx context.Context, id int, input ProviderResultInput) (*ent.Refund, error) {
	update := r.client.Refund.UpdateOneID(id).SetProviderSnapshot(input.Snapshot).SetLastAttemptAt(input.Now).SetAttemptCount(1)
	if input.ChannelRefundNo != "" {
		update.SetChannelRefundNo(input.ChannelRefundNo)
	}
	if input.LastError != "" {
		update.SetLastError(input.LastError).SetNextAttemptAt(input.Now.Add(2 * time.Minute))
	} else {
		update.ClearLastError()
	}
	switch input.Status {
	case "succeeded":
		update.SetStatus("succeeded").SetSucceededAt(input.Now).ClearNextAttemptAt().ClearFailureReason()
	case "failed":
		update.SetStatus("failed").SetFailedAt(input.Now).SetFailureReason(input.FailureReason).ClearNextAttemptAt()
	default:
		update.SetStatus("pending").SetNextAttemptAt(input.Now.Add(2 * time.Minute))
	}
	if _, err := update.Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
