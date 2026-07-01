package repository

import (
	"context"
	"time"

	"payment-gateway/ent"
	"payment-gateway/ent/webhookdelivery"
	"payment-gateway/ent/webhookevent"
)

type Repository struct {
	client *ent.Client
}

func New(client *ent.Client) Repository {
	return Repository{client: client}
}

func (r Repository) IsZero() bool {
	return r.client == nil
}

type CreateEventInput struct {
	EventID        string
	EventType      string
	AppID          string
	GatewayOrderNo string
	PaymentOrderID int
	Payload        map[string]any
}

type CreateDeliveryInput struct {
	DeliveryNo     string
	EventID        int
	AppID          string
	EventType      string
	GatewayOrderNo string
	TargetURL      string
	Status         string
	NextAttemptAt  time.Time
}

type ListEventsInput struct {
	AppID          string
	EventType      string
	GatewayOrderNo string
	Page           int
	PageSize       int
}

type ListDeliveriesInput struct {
	AppID          string
	EventType      string
	Status         string
	GatewayOrderNo string
	Page           int
	PageSize       int
}

type DeliveryAttemptInput struct {
	Status           string
	AttemptCount     int
	NextAttemptAt    *time.Time
	LastAttemptAt    time.Time
	LastStatusCode   int
	LastResponseBody string
	LastError        string
	SucceededAt      *time.Time
}

func (r Repository) FindEventByTypeAndOrder(ctx context.Context, eventType string, gatewayOrderNo string) (*ent.WebhookEvent, error) {
	return r.client.WebhookEvent.Query().
		Where(webhookevent.EventType(eventType), webhookevent.GatewayOrderNo(gatewayOrderNo)).
		Only(ctx)
}

func (r Repository) CreateEvent(ctx context.Context, input CreateEventInput) (*ent.WebhookEvent, error) {
	create := r.client.WebhookEvent.Create().
		SetEventID(input.EventID).
		SetEventType(input.EventType).
		SetAppID(input.AppID).
		SetGatewayOrderNo(input.GatewayOrderNo).
		SetPayload(input.Payload)
	if input.PaymentOrderID > 0 {
		create.SetPaymentOrderID(input.PaymentOrderID)
	}
	return create.Save(ctx)
}

func (r Repository) FindDeliveryByEventID(ctx context.Context, eventID int) (*ent.WebhookDelivery, error) {
	return r.client.WebhookDelivery.Query().Where(webhookdelivery.EventID(eventID)).Only(ctx)
}

func (r Repository) FindDeliveryByID(ctx context.Context, id int) (*ent.WebhookDelivery, error) {
	return r.client.WebhookDelivery.Get(ctx, id)
}

func (r Repository) FindEventByID(ctx context.Context, id int) (*ent.WebhookEvent, error) {
	return r.client.WebhookEvent.Get(ctx, id)
}

func (r Repository) CreateDelivery(ctx context.Context, input CreateDeliveryInput) (*ent.WebhookDelivery, error) {
	status := input.Status
	if status == "" {
		status = "pending"
	}
	return r.client.WebhookDelivery.Create().
		SetDeliveryNo(input.DeliveryNo).
		SetEventID(input.EventID).
		SetAppID(input.AppID).
		SetEventType(input.EventType).
		SetGatewayOrderNo(input.GatewayOrderNo).
		SetTargetURL(input.TargetURL).
		SetStatus(status).
		SetNextAttemptAt(input.NextAttemptAt).
		Save(ctx)
}

func (r Repository) ListEvents(ctx context.Context, input ListEventsInput) ([]*ent.WebhookEvent, int, error) {
	query := r.client.WebhookEvent.Query()
	if input.AppID != "" {
		query.Where(webhookevent.AppID(input.AppID))
	}
	if input.EventType != "" {
		query.Where(webhookevent.EventType(input.EventType))
	}
	if input.GatewayOrderNo != "" {
		query.Where(webhookevent.GatewayOrderNo(input.GatewayOrderNo))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	items, err := query.Order(ent.Desc(webhookevent.FieldCreatedAt)).
		Offset(offset(input.Page, input.PageSize)).
		Limit(limit(input.PageSize)).
		All(ctx)
	return items, total, err
}

func (r Repository) ListDeliveries(ctx context.Context, input ListDeliveriesInput) ([]*ent.WebhookDelivery, int, error) {
	query := r.client.WebhookDelivery.Query()
	if input.AppID != "" {
		query.Where(webhookdelivery.AppID(input.AppID))
	}
	if input.EventType != "" {
		query.Where(webhookdelivery.EventType(input.EventType))
	}
	if input.Status != "" {
		query.Where(webhookdelivery.Status(input.Status))
	}
	if input.GatewayOrderNo != "" {
		query.Where(webhookdelivery.GatewayOrderNo(input.GatewayOrderNo))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	items, err := query.Order(ent.Desc(webhookdelivery.FieldCreatedAt)).
		Offset(offset(input.Page, input.PageSize)).
		Limit(limit(input.PageSize)).
		All(ctx)
	return items, total, err
}

func (r Repository) ListDueDeliveries(ctx context.Context, now time.Time, limitValue int) ([]*ent.WebhookDelivery, error) {
	if limitValue < 1 {
		limitValue = 100
	}
	return r.client.WebhookDelivery.Query().
		Where(
			webhookdelivery.Status("pending"),
			webhookdelivery.NextAttemptAtLTE(now),
		).
		Order(ent.Asc(webhookdelivery.FieldNextAttemptAt)).
		Limit(limitValue).
		All(ctx)
}

func (r Repository) UpdateDeliveryAttempt(ctx context.Context, id int, input DeliveryAttemptInput) (*ent.WebhookDelivery, error) {
	update := r.client.WebhookDelivery.UpdateOneID(id).
		SetStatus(input.Status).
		SetAttemptCount(input.AttemptCount).
		SetLastAttemptAt(input.LastAttemptAt)
	if input.LastStatusCode > 0 {
		update.SetLastStatusCode(input.LastStatusCode)
	} else {
		update.ClearLastStatusCode()
	}
	if input.LastResponseBody != "" {
		update.SetLastResponseBody(input.LastResponseBody)
	} else {
		update.ClearLastResponseBody()
	}
	if input.LastError != "" {
		update.SetLastError(input.LastError)
	} else {
		update.ClearLastError()
	}
	if input.NextAttemptAt != nil {
		update.SetNextAttemptAt(*input.NextAttemptAt)
	} else {
		update.ClearNextAttemptAt()
	}
	if input.SucceededAt != nil {
		update.SetSucceededAt(*input.SucceededAt)
	} else {
		update.ClearSucceededAt()
	}
	if _, err := update.Save(ctx); err != nil {
		return nil, err
	}
	return r.FindDeliveryByID(ctx, id)
}

func (r Repository) ResetDeliveryForRetry(ctx context.Context, id int, nextAttemptAt time.Time) (*ent.WebhookDelivery, error) {
	if _, err := r.client.WebhookDelivery.UpdateOneID(id).
		SetStatus("pending").
		SetAttemptCount(0).
		SetNextAttemptAt(nextAttemptAt).
		ClearLastAttemptAt().
		ClearLastStatusCode().
		ClearLastResponseBody().
		ClearLastError().
		ClearSucceededAt().
		Save(ctx); err != nil {
		return nil, err
	}
	return r.FindDeliveryByID(ctx, id)
}

func offset(page int, pageSize int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * limit(pageSize)
}

func limit(pageSize int) int {
	if pageSize < 1 {
		return 20
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}
