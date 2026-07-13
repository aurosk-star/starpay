package repository

import (
	"context"
	"errors"
	"time"

	"payment-gateway/ent"
	"payment-gateway/ent/paymentorder"
)

var ErrStatusTransitionRejected = errors.New("payment order status transition rejected")

type Repository struct {
	client *ent.Client
}

func New(client *ent.Client) Repository {
	return Repository{client: client}
}

type CreateOrderInput struct {
	GatewayOrderNo     string
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
	CheckoutTokenHash  string
	Status             string
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

func (r Repository) Create(ctx context.Context, input CreateOrderInput) (*ent.PaymentOrder, error) {
	create := r.client.PaymentOrder.Create().
		SetGatewayOrderNo(input.GatewayOrderNo).
		SetAppID(input.AppID).
		SetMerchantOrderNo(input.MerchantOrderNo).
		SetSubject(input.Subject).
		SetAmount(input.Amount).
		SetCurrency(input.Currency).
		SetStatus(input.Status).
		SetMetadata(input.Metadata)
	if input.BusinessType != "" {
		create.SetBusinessType(input.BusinessType)
	}
	if input.Description != "" {
		create.SetDescription(input.Description)
	}
	if input.SettlementAmount > 0 {
		create.SetSettlementAmount(input.SettlementAmount)
	}
	if input.SettlementCurrency != "" {
		create.SetSettlementCurrency(input.SettlementCurrency)
	}
	if input.Channel != "" {
		create.SetChannel(input.Channel)
	}
	if input.PayMethod != "" {
		create.SetPayMethod(input.PayMethod)
	}
	if input.ReturnURL != "" {
		create.SetReturnURL(input.ReturnURL)
	}
	if input.CheckoutTokenHash != "" {
		create.SetCheckoutTokenHash(input.CheckoutTokenHash)
	}
	if input.ExpiresAt != nil {
		create.SetExpiresAt(*input.ExpiresAt)
	}
	return create.Save(ctx)
}

func (r Repository) FindByID(ctx context.Context, id int) (*ent.PaymentOrder, error) {
	return r.client.PaymentOrder.Get(ctx, id)
}

func (r Repository) FindByGatewayOrderNo(ctx context.Context, gatewayOrderNo string) (*ent.PaymentOrder, error) {
	return r.client.PaymentOrder.Query().Where(paymentorder.GatewayOrderNo(gatewayOrderNo)).Only(ctx)
}

func (r Repository) FindByGatewayOrderNoForApp(ctx context.Context, appID string, gatewayOrderNo string) (*ent.PaymentOrder, error) {
	return r.client.PaymentOrder.Query().
		Where(paymentorder.AppID(appID), paymentorder.GatewayOrderNo(gatewayOrderNo)).
		Only(ctx)
}

func (r Repository) FindByMerchantOrderNo(ctx context.Context, appID string, merchantOrderNo string) (*ent.PaymentOrder, error) {
	return r.client.PaymentOrder.Query().
		Where(paymentorder.AppID(appID), paymentorder.MerchantOrderNo(merchantOrderNo)).
		Only(ctx)
}

func (r Repository) List(ctx context.Context, input ListOrdersInput) ([]*ent.PaymentOrder, int, error) {
	query := r.client.PaymentOrder.Query()
	if input.AppID != "" {
		query.Where(paymentorder.AppID(input.AppID))
	}
	if input.Status != "" {
		query.Where(paymentorder.Status(input.Status))
	}
	if input.Channel != "" {
		query.Where(paymentorder.Channel(input.Channel))
	}
	if input.Currency != "" {
		query.Where(paymentorder.Currency(input.Currency))
	}
	if input.MerchantOrderNo != "" {
		query.Where(paymentorder.MerchantOrderNoContains(input.MerchantOrderNo))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	items, err := query.
		Order(ent.Desc(paymentorder.FieldCreatedAt)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	return items, total, err
}

func (r Repository) ListExpiredPending(ctx context.Context, now time.Time, limit int) ([]*ent.PaymentOrder, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return r.client.PaymentOrder.Query().
		Where(
			paymentorder.Status("pending"),
			paymentorder.ExpiresAtNotNil(),
			paymentorder.ExpiresAtLTE(now),
			paymentorder.Or(
				paymentorder.ChannelAccountIDIsNil(),
				paymentorder.ProviderOrderNoIsNil(),
				paymentorder.ProviderOrderNoEQ(""),
			),
		).
		Order(ent.Asc(paymentorder.FieldExpiresAt)).
		Limit(limit).
		All(ctx)
}

func (r Repository) Update(ctx context.Context, id int, input UpdateOrderInput) (*ent.PaymentOrder, error) {
	update := r.client.PaymentOrder.UpdateOneID(id).
		SetSubject(input.Subject).
		SetMetadata(input.Metadata)
	if input.BusinessType != "" {
		update.SetBusinessType(input.BusinessType)
	} else {
		update.ClearBusinessType()
	}
	if input.Description != "" {
		update.SetDescription(input.Description)
	} else {
		update.ClearDescription()
	}
	if input.Channel != "" {
		update.SetChannel(input.Channel)
	} else {
		update.ClearChannel()
	}
	if input.PayMethod != "" {
		update.SetPayMethod(input.PayMethod)
	} else {
		update.ClearPayMethod()
	}
	if _, err := update.Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) SetStatus(ctx context.Context, id int, status string, now time.Time) (*ent.PaymentOrder, error) {
	if status == "closed" {
		affected, err := r.client.PaymentOrder.Update().
			Where(paymentorder.ID(id), paymentorder.StatusIn("pending", "failed")).
			SetStatus("closed").
			SetClosedAt(now).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		if affected == 0 {
			current, findErr := r.FindByID(ctx, id)
			if findErr != nil {
				return nil, findErr
			}
			return current, ErrStatusTransitionRejected
		}
		return r.FindByID(ctx, id)
	}
	update := r.client.PaymentOrder.UpdateOneID(id).SetStatus(status)
	switch status {
	case "paid":
		update.SetPaidAt(now)
	}
	if _, err := update.Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) CloseExpiredPending(ctx context.Context, id int, now time.Time) (bool, error) {
	affected, err := r.client.PaymentOrder.Update().
		Where(
			paymentorder.ID(id),
			paymentorder.Status("pending"),
			paymentorder.ExpiresAtNotNil(),
			paymentorder.ExpiresAtLTE(now),
		).
		SetStatus("closed").
		SetClosedAt(now).
		Save(ctx)
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r Repository) MarkPaid(ctx context.Context, id int, channelTradeNo string, now time.Time) (*ent.PaymentOrder, error) {
	update := r.client.PaymentOrder.Update().
		Where(paymentorder.ID(id), paymentorder.StatusIn("pending", "failed")).
		SetStatus("paid").
		SetPaidAt(now)
	if channelTradeNo != "" {
		update.SetChannelTradeNo(channelTradeNo)
	}
	affected, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		current, findErr := r.FindByID(ctx, id)
		if findErr != nil {
			return nil, findErr
		}
		return current, ErrStatusTransitionRejected
	}
	return r.FindByID(ctx, id)
}

func (r Repository) MarkFailed(ctx context.Context, id int, reason string, now time.Time) (*ent.PaymentOrder, error) {
	update := r.client.PaymentOrder.Update().
		Where(paymentorder.ID(id), paymentorder.Status("pending")).
		SetStatus("failed").
		SetFailedAt(now)
	if reason != "" {
		update.SetFailureReason(reason)
	}
	affected, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		current, findErr := r.FindByID(ctx, id)
		if findErr != nil {
			return nil, findErr
		}
		return current, ErrStatusTransitionRejected
	}
	return r.FindByID(ctx, id)
}

func (r Repository) SetChannelTradeNo(ctx context.Context, id int, channelTradeNo string) (*ent.PaymentOrder, error) {
	if _, err := r.client.PaymentOrder.UpdateOneID(id).
		SetChannelTradeNo(channelTradeNo).
		Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) SetProviderOrderNo(ctx context.Context, id int, providerOrderNo string) (*ent.PaymentOrder, error) {
	if _, err := r.client.PaymentOrder.UpdateOneID(id).
		SetProviderOrderNo(providerOrderNo).
		Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) SetPaymentSelection(ctx context.Context, id int, channel string, payMethod string, channelAccountID int) (*ent.PaymentOrder, error) {
	update := r.client.PaymentOrder.UpdateOneID(id)
	if channel != "" {
		update.SetChannel(channel)
	}
	if payMethod != "" {
		update.SetPayMethod(payMethod)
	}
	if channelAccountID > 0 {
		update.SetChannelAccountID(channelAccountID)
	}
	if _, err := update.Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) SetCheckoutTokenHash(ctx context.Context, id int, tokenHash string) (*ent.PaymentOrder, error) {
	if _, err := r.client.PaymentOrder.UpdateOneID(id).
		SetCheckoutTokenHash(tokenHash).
		Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
