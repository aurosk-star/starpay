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
	SiteName                       string
	GatewayBaseURL                 string
	PaymentNotifyPath              string
	DefaultCurrency                string
	DefaultLocale                  string
	RequestIDEnabled               bool
	MaintenanceMode                bool
	OrderDefaultTTLSeconds         int
	OrderExpireScanIntervalSeconds int
	OrderExpireScanLimit           int
	OrderExpireWorkerConcurrency   int
	OpenAPIRateLimitEnabled        bool
	OpenAPIRateLimit               int
	OpenAPIRateLimitWindowSeconds  int
	Extra                          map[string]any
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
		SetSiteName(input.SiteName).
		SetGatewayBaseURL(input.GatewayBaseURL).
		SetPaymentNotifyPath(input.PaymentNotifyPath).
		SetDefaultCurrency(input.DefaultCurrency).
		SetDefaultLocale(input.DefaultLocale).
		SetRequestIDEnabled(input.RequestIDEnabled).
		SetMaintenanceMode(input.MaintenanceMode).
		SetOrderDefaultTTLSeconds(input.OrderDefaultTTLSeconds).
		SetOrderExpireScanIntervalSeconds(input.OrderExpireScanIntervalSeconds).
		SetOrderExpireScanLimit(input.OrderExpireScanLimit).
		SetOrderExpireWorkerConcurrency(input.OrderExpireWorkerConcurrency).
		SetOpenAPIRateLimitEnabled(input.OpenAPIRateLimitEnabled).
		SetOpenAPIRateLimit(input.OpenAPIRateLimit).
		SetOpenAPIRateLimitWindowSeconds(input.OpenAPIRateLimitWindowSeconds).
		SetExtra(input.Extra).
		Save(ctx); err != nil {
		return nil, err
	}
	return r.client.GatewayConfig.Get(ctx, id)
}
