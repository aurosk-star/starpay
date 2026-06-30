package repository

import (
	"context"

	"payment-gateway/ent"
	"payment-gateway/ent/app"
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

type CreateAppInput struct {
	AppID               string
	Name                string
	AppSecretHash       string
	AppSecretCiphertext string
	NotifyURL           string
	AllowedIPs          []string
	Status              string
}

type UpdateAppInput struct {
	Name       string
	NotifyURL  string
	AllowedIPs []string
	Status     string
}

func (r Repository) List(ctx context.Context) ([]*ent.App, error) {
	return r.client.App.Query().Order(ent.Desc(app.FieldCreatedAt)).All(ctx)
}

func (r Repository) FindByID(ctx context.Context, id int) (*ent.App, error) {
	return r.client.App.Get(ctx, id)
}

func (r Repository) FindByAppID(ctx context.Context, appID string) (*ent.App, error) {
	return r.client.App.Query().Where(app.AppID(appID)).Only(ctx)
}

func (r Repository) Create(ctx context.Context, input CreateAppInput) (*ent.App, error) {
	create := r.client.App.Create().
		SetAppID(input.AppID).
		SetName(input.Name).
		SetAppSecretHash(input.AppSecretHash).
		SetAppSecretCiphertext(input.AppSecretCiphertext).
		SetAllowedIps(input.AllowedIPs).
		SetStatus(input.Status)
	if input.NotifyURL != "" {
		create.SetNotifyURL(input.NotifyURL)
	}
	return create.Save(ctx)
}

func (r Repository) Update(ctx context.Context, id int, input UpdateAppInput) (*ent.App, error) {
	update := r.client.App.UpdateOneID(id).
		SetName(input.Name).
		SetAllowedIps(input.AllowedIPs).
		SetStatus(input.Status)
	if input.NotifyURL != "" {
		update.SetNotifyURL(input.NotifyURL)
	} else {
		update.ClearNotifyURL()
	}
	if _, err := update.Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) SetStatus(ctx context.Context, id int, status string) (*ent.App, error) {
	if _, err := r.client.App.UpdateOneID(id).SetStatus(status).Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) SetSecret(ctx context.Context, id int, hash string, ciphertext string) (*ent.App, error) {
	if _, err := r.client.App.UpdateOneID(id).SetAppSecretHash(hash).SetAppSecretCiphertext(ciphertext).Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
