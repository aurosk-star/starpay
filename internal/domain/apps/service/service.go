package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"payment-gateway/ent"
	apprepo "payment-gateway/internal/domain/apps/repository"
	platformauth "payment-gateway/internal/platform/auth"
)

var (
	ErrNameRequired  = errors.New("name is required")
)

type Service struct {
	apps                apprepo.Repository
	secretEncryptionKey string
}

type Option func(*Service)

func WithSecretEncryptionKey(key string) Option {
	return func(s *Service) {
		s.secretEncryptionKey = key
	}
}

func New(client *ent.Client, opts ...Option) Service {
	svc := Service{apps: apprepo.New(client)}
	for _, opt := range opts {
		opt(&svc)
	}
	if svc.secretEncryptionKey == "" {
		svc.secretEncryptionKey = "0123456789abcdef0123456789abcdef"
	}
	return svc
}

type ManageAppInput struct {
	AppID            string
	Name             string
	NotifyURL        string
	DefaultReturnURL string
	AllowedIPs       []string
	Status           string
}

type AppWithSecret struct {
	App       *ent.App
	AppSecret string
}

func (s Service) ListApps(ctx context.Context) ([]*ent.App, error) {
	return s.apps.List(ctx)
}

func (s Service) GetApp(ctx context.Context, id int) (*ent.App, error) {
	return s.apps.FindByID(ctx, id)
}

func (s Service) CreateApp(ctx context.Context, input ManageAppInput) (*AppWithSecret, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	appID := strings.TrimSpace(input.AppID)
	if appID == "" {
		generated, err := newAppID()
		if err != nil {
			return nil, err
		}
		appID = generated
	}
	secret, hash, err := newAppSecret()
	if err != nil {
		return nil, err
	}
	created, err := s.apps.Create(ctx, apprepo.CreateAppInput{
		AppID:               appID,
		Name:                name,
		AppSecretHash:       hash,
		AppSecretCiphertext: encryptSecretOrPanic(s.secretEncryptionKey, secret),
		NotifyURL:           strings.TrimSpace(input.NotifyURL),
		DefaultReturnURL:    strings.TrimSpace(input.DefaultReturnURL),
		AllowedIPs:          normalizeAllowedIPs(input.AllowedIPs),
		Status:              normalizeStatus(input.Status),
	})
	if err != nil {
		return nil, err
	}
	return &AppWithSecret{App: created, AppSecret: secret}, nil
}

func (s Service) UpdateApp(ctx context.Context, id int, input ManageAppInput) (*ent.App, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	return s.apps.Update(ctx, id, apprepo.UpdateAppInput{
		Name:             name,
		NotifyURL:        strings.TrimSpace(input.NotifyURL),
		DefaultReturnURL: strings.TrimSpace(input.DefaultReturnURL),
		AllowedIPs:       normalizeAllowedIPs(input.AllowedIPs),
		Status:           normalizeStatus(input.Status),
	})
}

func (s Service) EnableApp(ctx context.Context, id int) (*ent.App, error) {
	return s.apps.SetStatus(ctx, id, "enabled")
}

func (s Service) DisableApp(ctx context.Context, id int) (*ent.App, error) {
	return s.apps.SetStatus(ctx, id, "disabled")
}

func (s Service) ResetSecret(ctx context.Context, id int) (*AppWithSecret, error) {
	secret, hash, err := newAppSecret()
	if err != nil {
		return nil, err
	}
	ciphertext, err := platformauth.EncryptSecret(s.secretEncryptionKey, secret)
	if err != nil {
		return nil, err
	}
	updated, err := s.apps.SetSecret(ctx, id, hash, ciphertext)
	if err != nil {
		return nil, err
	}
	return &AppWithSecret{App: updated, AppSecret: secret}, nil
}

func encryptSecretOrPanic(key string, secret string) string {
	ciphertext, err := platformauth.EncryptSecret(key, secret)
	if err != nil {
		panic(err)
	}
	return ciphertext
}

func newAppSecret() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	secret := "pgsec_" + base64.RawURLEncoding.EncodeToString(raw)
	hash, err := platformauth.HashPassword(secret)
	if err != nil {
		return "", "", err
	}
	return secret, hash, nil
}

func newAppID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "app_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func normalizeStatus(status string) string {
	if strings.EqualFold(status, "disabled") {
		return "disabled"
	}
	return "enabled"
}

func normalizeAllowedIPs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
