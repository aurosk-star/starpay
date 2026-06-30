package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"payment-gateway/ent"
	channelrepo "payment-gateway/internal/domain/channels/repository"
)

const maskedValue = "********"

var (
	ErrInvalidChannel = errors.New("invalid channel")
	ErrInvalidEnv     = errors.New("invalid channel environment")
	ErrNameRequired   = errors.New("name is required")
)

type Service struct {
	channels channelrepo.Repository
}

func New(client *ent.Client) Service {
	return Service{channels: channelrepo.New(client)}
}

type ManageChannelAccountInput struct {
	Channel string
	Name    string
	Enabled bool
	Env     string
	Config  map[string]any
}

type ChannelAccountView struct {
	ID        int            `json:"id"`
	Channel   string         `json:"channel"`
	Name      string         `json:"name"`
	Enabled   bool           `json:"enabled"`
	Env       string         `json:"env"`
	Config    map[string]any `json:"config"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (s Service) ListChannelAccounts(ctx context.Context) ([]ChannelAccountView, error) {
	accounts, err := s.channels.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]ChannelAccountView, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, toView(account))
	}
	return items, nil
}

func (s Service) FindChannelAccount(ctx context.Context, id int) (*ChannelAccountView, error) {
	account, err := s.channels.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	view := toView(account)
	return &view, nil
}

func (s Service) CreateChannelAccount(ctx context.Context, input ManageChannelAccountInput) (*ChannelAccountView, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, err
	}
	created, err := s.channels.Create(ctx, channelrepo.CreateChannelAccountInput{
		Channel: normalized.Channel,
		Name:    normalized.Name,
		Enabled: normalized.Enabled,
		Env:     normalized.Env,
		Config:  normalized.Config,
	})
	if err != nil {
		return nil, err
	}
	view := toView(created)
	return &view, nil
}

func (s Service) UpdateChannelAccount(ctx context.Context, id int, input ManageChannelAccountInput) (*ChannelAccountView, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, err
	}
	existing, err := s.channels.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	normalized.Config = mergeSensitiveConfig(existing.Config, normalized.Config)
	updated, err := s.channels.Update(ctx, id, channelrepo.UpdateChannelAccountInput{
		Channel: normalized.Channel,
		Name:    normalized.Name,
		Enabled: normalized.Enabled,
		Env:     normalized.Env,
		Config:  normalized.Config,
	})
	if err != nil {
		return nil, err
	}
	view := toView(updated)
	return &view, nil
}

func (s Service) EnableChannelAccount(ctx context.Context, id int) (*ChannelAccountView, error) {
	updated, err := s.channels.SetEnabled(ctx, id, true)
	if err != nil {
		return nil, err
	}
	view := toView(updated)
	return &view, nil
}

func (s Service) DisableChannelAccount(ctx context.Context, id int) (*ChannelAccountView, error) {
	updated, err := s.channels.SetEnabled(ctx, id, false)
	if err != nil {
		return nil, err
	}
	view := toView(updated)
	return &view, nil
}

func normalizeInput(input ManageChannelAccountInput) (ManageChannelAccountInput, error) {
	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	if channel != "wechat" && channel != "alipay" && channel != "paypal" {
		return ManageChannelAccountInput{}, ErrInvalidChannel
	}
	env := strings.ToLower(strings.TrimSpace(input.Env))
	if env == "" {
		env = "sandbox"
	}
	if env != "sandbox" && env != "prod" {
		return ManageChannelAccountInput{}, ErrInvalidEnv
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ManageChannelAccountInput{}, ErrNameRequired
	}
	config := input.Config
	if config == nil {
		config = map[string]any{}
	}
	return ManageChannelAccountInput{
		Channel: channel,
		Name:    name,
		Enabled: input.Enabled,
		Env:     env,
		Config:  config,
	}, nil
}

func toView(account *ent.ChannelAccount) ChannelAccountView {
	return ChannelAccountView{
		ID:        account.ID,
		Channel:   account.Channel,
		Name:      account.Name,
		Enabled:   account.Enabled,
		Env:       account.Env,
		Config:    maskConfig(account.Config),
		CreatedAt: account.CreatedAt,
		UpdatedAt: account.UpdatedAt,
	}
}

func maskConfig(config map[string]any) map[string]any {
	masked := make(map[string]any, len(config))
	for key, value := range config {
		if isSensitiveConfigKey(key) {
			masked[key] = maskedValue
			continue
		}
		masked[key] = value
	}
	return masked
}

func mergeSensitiveConfig(existing map[string]any, next map[string]any) map[string]any {
	merged := make(map[string]any, len(existing))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range next {
		if isSensitiveConfigKey(key) && strings.TrimSpace(stringValue(value)) == "" {
			continue
		}
		merged[key] = value
	}
	return merged
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return ""
	}
}

func isSensitiveConfigKey(key string) bool {
	switch strings.ToLower(key) {
	case "api_key",
		"api_v3_key",
		"secret",
		"client_secret",
		"private_key",
		"alipay_public_key",
		"wechat_pay_public_key",
		"cert",
		"cert_key":
		return true
	default:
		return false
	}
}
