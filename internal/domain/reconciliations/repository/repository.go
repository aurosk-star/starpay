package repository

import (
	"context"
	"time"

	"payment-gateway/ent"
	"payment-gateway/ent/paymentorder"
	"payment-gateway/ent/paymentreconciliation"
)

type Repository struct {
	client *ent.Client
}

func New(client *ent.Client) Repository {
	return Repository{client: client}
}

func (r Repository) FindByID(ctx context.Context, id int) (*ent.PaymentReconciliation, error) {
	return r.client.PaymentReconciliation.Get(ctx, id)
}

func (r Repository) FindByOrderID(ctx context.Context, orderID int) (*ent.PaymentReconciliation, error) {
	return r.client.PaymentReconciliation.Query().Where(paymentreconciliation.PaymentOrderID(orderID)).Only(ctx)
}

func (r Repository) EnsureForOrder(ctx context.Context, order *ent.PaymentOrder, next time.Time) (*ent.PaymentReconciliation, error) {
	if existing, err := r.FindByOrderID(ctx, order.ID); err == nil {
		return existing, nil
	} else if !ent.IsNotFound(err) {
		return nil, err
	}
	created, err := r.client.PaymentReconciliation.Create().
		SetPaymentOrderID(order.ID).
		SetGatewayOrderNo(order.GatewayOrderNo).
		SetChannel(order.Channel).
		SetChannelAccountID(order.ChannelAccountID).
		SetStatus("pending").
		SetNextAttemptAt(next).
		SetProviderSnapshot(map[string]any{}).
		Save(ctx)
	if err == nil {
		return created, nil
	}
	if existing, findErr := r.FindByOrderID(ctx, order.ID); findErr == nil {
		return existing, nil
	}
	return nil, err
}

func (r Repository) Claim(ctx context.Context, id int, now time.Time) (*ent.PaymentReconciliation, bool, error) {
	affected, err := r.client.PaymentReconciliation.Update().
		Where(paymentreconciliation.ID(id), paymentreconciliation.Status("pending")).
		SetStatus("processing").
		SetLastAttemptAt(now).
		Save(ctx)
	if err != nil || affected == 0 {
		return nil, false, err
	}
	item, err := r.FindByID(ctx, id)
	return item, true, err
}

type CompleteAttemptInput struct {
	Status             string
	AttemptCount       int
	NextAttemptAt      *time.Time
	LastProviderStatus string
	LastError          string
	ProviderSnapshot   map[string]any
	ResolvedAt         *time.Time
}

type ListInput struct {
	Status         string
	Channel        string
	GatewayOrderNo string
	Page           int
	PageSize       int
}

func (r Repository) List(ctx context.Context, input ListInput) ([]*ent.PaymentReconciliation, int, error) {
	query := r.client.PaymentReconciliation.Query()
	if input.Status != "" {
		query.Where(paymentreconciliation.Status(input.Status))
	}
	if input.Channel != "" {
		query.Where(paymentreconciliation.Channel(input.Channel))
	}
	if input.GatewayOrderNo != "" {
		query.Where(paymentreconciliation.GatewayOrderNoContains(input.GatewayOrderNo))
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
	items, err := query.Order(ent.Desc(paymentreconciliation.FieldUpdatedAt)).Offset((page - 1) * pageSize).Limit(pageSize).All(ctx)
	return items, total, err
}

func (r Repository) CompleteAttempt(ctx context.Context, id int, input CompleteAttemptInput) (*ent.PaymentReconciliation, error) {
	update := r.client.PaymentReconciliation.UpdateOneID(id).
		SetStatus(input.Status).
		SetAttemptCount(input.AttemptCount).
		SetProviderSnapshot(input.ProviderSnapshot)
	if input.NextAttemptAt != nil {
		update.SetNextAttemptAt(*input.NextAttemptAt)
	} else {
		update.ClearNextAttemptAt()
	}
	if input.LastProviderStatus != "" {
		update.SetLastProviderStatus(input.LastProviderStatus)
	} else {
		update.ClearLastProviderStatus()
	}
	if input.LastError != "" {
		update.SetLastError(input.LastError)
	} else {
		update.ClearLastError()
	}
	if input.ResolvedAt != nil {
		update.SetResolvedAt(*input.ResolvedAt)
	} else {
		update.ClearResolvedAt()
	}
	if _, err := update.Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) ListDue(ctx context.Context, now time.Time, limit int) ([]*ent.PaymentReconciliation, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return r.client.PaymentReconciliation.Query().
		Where(paymentreconciliation.Status("pending"), paymentreconciliation.NextAttemptAtNotNil(), paymentreconciliation.NextAttemptAtLTE(now)).
		Order(ent.Asc(paymentreconciliation.FieldNextAttemptAt)).
		Limit(limit).
		All(ctx)
}

func (r Repository) RecoverStale(ctx context.Context, cutoff time.Time) (int, error) {
	return r.client.PaymentReconciliation.Update().
		Where(paymentreconciliation.Status("processing"), paymentreconciliation.LastAttemptAtNotNil(), paymentreconciliation.LastAttemptAtLTE(cutoff)).
		SetStatus("pending").
		SetNextAttemptAt(cutoff.Add(5 * time.Minute)).
		Save(ctx)
}

func (r Repository) ResetForRetry(ctx context.Context, id int, now time.Time) (*ent.PaymentReconciliation, error) {
	if _, err := r.client.PaymentReconciliation.UpdateOneID(id).
		SetStatus("pending").
		SetAttemptCount(0).
		SetNextAttemptAt(now).
		ClearLastAttemptAt().
		ClearLastError().
		ClearResolvedAt().
		Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) RequestActiveQuery(ctx context.Context, id int, now time.Time) (*ent.PaymentReconciliation, bool, error) {
	affected, err := r.client.PaymentReconciliation.Update().
		Where(
			paymentreconciliation.ID(id),
			paymentreconciliation.Status("pending"),
			paymentreconciliation.ActiveQueryRequestedAtIsNil(),
		).
		SetActiveQueryRequestedAt(now).
		SetNextAttemptAt(now).
		Save(ctx)
	if err != nil || affected == 0 {
		if err != nil {
			return nil, false, err
		}
		item, findErr := r.FindByID(ctx, id)
		return item, false, findErr
	}
	item, err := r.FindByID(ctx, id)
	return item, true, err
}

func (r Repository) ReleaseActiveQueryRequest(ctx context.Context, id int, requestedAt time.Time) error {
	_, err := r.client.PaymentReconciliation.Update().
		Where(
			paymentreconciliation.ID(id),
			paymentreconciliation.Status("pending"),
			paymentreconciliation.ActiveQueryRequestedAtEQ(requestedAt),
		).
		ClearActiveQueryRequestedAt().
		Save(ctx)
	return err
}

func (r Repository) ListEligibleOrders(ctx context.Context, limit int) ([]*ent.PaymentOrder, error) {
	if limit < 1 {
		limit = 100
	}
	return r.client.PaymentOrder.Query().
		Where(
			paymentorder.Status("pending"),
			paymentorder.ChannelAccountIDNotNil(),
			paymentorder.ProviderOrderNoNotNil(),
		).
		Limit(limit).
		All(ctx)
}
