package repository

import (
	"context"

	"payment-gateway/ent"
	"payment-gateway/ent/channelaccount"
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

type CreateChannelAccountInput struct {
	Channel string
	Name    string
	Enabled bool
	Env     string
	Config  map[string]any
}

type UpdateChannelAccountInput struct {
	Channel string
	Name    string
	Enabled bool
	Env     string
	Config  map[string]any
}

func (r Repository) List(ctx context.Context) ([]*ent.ChannelAccount, error) {
	return r.client.ChannelAccount.Query().
		Order(ent.Desc(channelaccount.FieldCreatedAt)).
		All(ctx)
}

func (r Repository) FindByID(ctx context.Context, id int) (*ent.ChannelAccount, error) {
	return r.client.ChannelAccount.Get(ctx, id)
}

func (r Repository) FindEnabledByID(ctx context.Context, id int) (*ent.ChannelAccount, error) {
	return r.client.ChannelAccount.Query().
		Where(channelaccount.ID(id), channelaccount.Enabled(true)).
		First(ctx)
}

func (r Repository) FindEnabledByChannel(ctx context.Context, channel string) (*ent.ChannelAccount, error) {
	return r.client.ChannelAccount.Query().
		Where(channelaccount.Channel(channel), channelaccount.Enabled(true)).
		Order(ent.Desc(channelaccount.FieldCreatedAt)).
		First(ctx)
}

func (r Repository) Create(ctx context.Context, input CreateChannelAccountInput) (*ent.ChannelAccount, error) {
	create := r.client.ChannelAccount.Create().
		SetChannel(input.Channel).
		SetName(input.Name).
		SetEnabled(input.Enabled).
		SetEnv(input.Env).
		SetConfig(input.Config)
	return create.Save(ctx)
}

func (r Repository) Update(ctx context.Context, id int, input UpdateChannelAccountInput) (*ent.ChannelAccount, error) {
	if _, err := r.client.ChannelAccount.UpdateOneID(id).
		SetChannel(input.Channel).
		SetName(input.Name).
		SetEnabled(input.Enabled).
		SetEnv(input.Env).
		SetConfig(input.Config).
		Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) SetEnabled(ctx context.Context, id int, enabled bool) (*ent.ChannelAccount, error) {
	if _, err := r.client.ChannelAccount.UpdateOneID(id).SetEnabled(enabled).Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
