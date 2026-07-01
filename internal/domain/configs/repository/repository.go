package repository

import (
	"context"

	"payment-gateway/ent"
	"payment-gateway/ent/gatewayconfig"
)

type Repository struct {
	client *ent.Client
}

func New(client *ent.Client) Repository {
	return Repository{client: client}
}

type UpdateGatewayConfigInput struct {
	GatewayBaseURL    string
	PaymentNotifyPath string
	DefaultCurrency   string
	DefaultLocale     string
	RequestIDEnabled  bool
	MaintenanceMode   bool
	Extra             map[string]any
}

func (r Repository) First(ctx context.Context) (*ent.GatewayConfig, error) {
	return r.client.GatewayConfig.Query().
		Order(ent.Asc(gatewayconfig.FieldID)).
		First(ctx)
}

func (r Repository) CreateDefault(ctx context.Context) (*ent.GatewayConfig, error) {
	return r.client.GatewayConfig.Create().Save(ctx)
}

func (r Repository) Update(ctx context.Context, id int, input UpdateGatewayConfigInput) (*ent.GatewayConfig, error) {
	if _, err := r.client.GatewayConfig.UpdateOneID(id).
		SetGatewayBaseURL(input.GatewayBaseURL).
		SetPaymentNotifyPath(input.PaymentNotifyPath).
		SetDefaultCurrency(input.DefaultCurrency).
		SetDefaultLocale(input.DefaultLocale).
		SetRequestIDEnabled(input.RequestIDEnabled).
		SetMaintenanceMode(input.MaintenanceMode).
		SetExtra(input.Extra).
		Save(ctx); err != nil {
		return nil, err
	}
	return r.client.GatewayConfig.Get(ctx, id)
}
