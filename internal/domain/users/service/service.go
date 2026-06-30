package service

import (
	"context"
	"errors"
	"time"

	"payment-gateway/ent"
	"payment-gateway/ent/refreshtoken"
	"payment-gateway/internal/domain/roles/repository"
	userrepo "payment-gateway/internal/domain/users/repository"
	platformauth "payment-gateway/internal/platform/auth"
	"payment-gateway/internal/platform/config"
)

var (
	ErrSetupAlreadyDone  = errors.New("setup already completed")
	ErrInvalidCredential = errors.New("invalid username or password")
	ErrInvalidRefresh    = errors.New("invalid refresh token")
	ErrPasswordRequired  = errors.New("password is required")
)

type Service struct {
	client  *ent.Client
	users   userrepo.Repository
	roles   repository.Repository
	tokens  platformauth.TokenService
	authCfg config.AuthConfig
}

func New(client *ent.Client, authCfg config.AuthConfig) Service {
	return Service{
		client:  client,
		users:   userrepo.New(client),
		roles:   repository.New(client),
		tokens:  platformauth.NewTokenService(authCfg.JWTSecret),
		authCfg: authCfg,
	}
}

type SetupInput struct {
	Username    string
	Email       string
	Password    string
	DisplayName string
	UserAgent   string
	IPAddress   string
}

type LoginInput struct {
	Username  string
	Password  string
	UserAgent string
	IPAddress string
}

type ManageUserInput struct {
	Username    string
	Email       string
	Password    string
	DisplayName string
	Status      string
	RoleIDs     []int
}

type TokenPair struct {
	User                  *ent.User
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
	Roles                 []string
}

func (s Service) Setup(ctx context.Context, input SetupInput) (*TokenPair, error) {
	hasSuperAdmin, err := s.users.HasSuperAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if hasSuperAdmin {
		return nil, ErrSetupAlreadyDone
	}

	roles, err := s.roles.EnsureDefaults(ctx)
	if err != nil {
		return nil, err
	}
	var superAdmin *ent.Role
	for _, item := range roles {
		if item.Code == "super_admin" {
			superAdmin = item
			break
		}
	}
	if superAdmin == nil {
		return nil, errors.New("super_admin role missing")
	}

	passwordHash, err := platformauth.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	user, err := s.users.Create(ctx, userrepo.CreateUserInput{
		Username:     input.Username,
		Email:        input.Email,
		DisplayName:  input.DisplayName,
		PasswordHash: passwordHash,
		RoleIDs:      []int{superAdmin.ID},
	})
	if err != nil {
		return nil, err
	}
	user.Edges.Roles = []*ent.Role{superAdmin}

	return s.issueTokens(ctx, user, input.UserAgent, input.IPAddress)
}

func (s Service) Login(ctx context.Context, input LoginInput) (*TokenPair, error) {
	user, err := s.users.FindByUsername(ctx, input.Username)
	if err != nil {
		return nil, ErrInvalidCredential
	}
	if user.Status != "enabled" || !platformauth.CheckPassword(user.PasswordHash, input.Password) {
		return nil, ErrInvalidCredential
	}
	_, _ = s.client.User.UpdateOneID(user.ID).SetLastLoginAt(time.Now()).Save(ctx)
	return s.issueTokens(ctx, user, input.UserAgent, input.IPAddress)
}

func (s Service) Refresh(ctx context.Context, refreshToken string, userAgent string, ipAddress string) (*TokenPair, error) {
	hash := platformauth.HashRefreshToken(refreshToken)
	existing, err := s.client.RefreshToken.Query().
		Where(refreshtoken.TokenHash(hash)).
		WithUser(func(q *ent.UserQuery) { q.WithRoles() }).
		Only(ctx)
	if err != nil {
		return nil, ErrInvalidRefresh
	}
	if existing.RevokedAt != nil || time.Now().After(existing.ExpiresAt) {
		return nil, ErrInvalidRefresh
	}

	user := existing.Edges.User
	if user == nil || user.Status != "enabled" {
		return nil, ErrInvalidRefresh
	}

	pair, err := s.issueTokens(ctx, user, userAgent, ipAddress)
	if err != nil {
		return nil, err
	}
	_, err = s.client.RefreshToken.UpdateOne(existing).
		SetRevokedAt(time.Now()).
		SetReplacedByHash(platformauth.HashRefreshToken(pair.RefreshToken)).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return pair, nil
}

func (s Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	hash := platformauth.HashRefreshToken(refreshToken)
	_, err := s.client.RefreshToken.Update().
		Where(refreshtoken.TokenHash(hash)).
		SetRevokedAt(time.Now()).
		Save(ctx)
	return err
}

func (s Service) FindByID(ctx context.Context, id int) (*ent.User, error) {
	return s.users.FindByID(ctx, id)
}

func (s Service) CreateUser(ctx context.Context, input ManageUserInput) (*ent.User, error) {
	if input.Password == "" {
		return nil, ErrPasswordRequired
	}
	if _, err := s.roles.EnsureDefaults(ctx); err != nil {
		return nil, err
	}
	passwordHash, err := platformauth.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	status := normalizeStatus(input.Status)
	created, err := s.users.Create(ctx, userrepo.CreateUserInput{
		Username:     input.Username,
		Email:        input.Email,
		DisplayName:  input.DisplayName,
		PasswordHash: passwordHash,
		RoleIDs:      input.RoleIDs,
	})
	if err != nil {
		return nil, err
	}
	if status != "enabled" {
		return s.users.Update(ctx, created.ID, userrepo.UpdateUserInput{
			Username:    input.Username,
			Email:       input.Email,
			DisplayName: input.DisplayName,
			Status:      status,
			RoleIDs:     input.RoleIDs,
		})
	}
	return s.users.FindByID(ctx, created.ID)
}

func (s Service) UpdateUser(ctx context.Context, id int, input ManageUserInput) (*ent.User, error) {
	if _, err := s.roles.EnsureDefaults(ctx); err != nil {
		return nil, err
	}
	passwordHash := ""
	if input.Password != "" {
		var err error
		passwordHash, err = platformauth.HashPassword(input.Password)
		if err != nil {
			return nil, err
		}
	}
	return s.users.Update(ctx, id, userrepo.UpdateUserInput{
		Username:     input.Username,
		Email:        input.Email,
		DisplayName:  input.DisplayName,
		Status:       normalizeStatus(input.Status),
		PasswordHash: passwordHash,
		RoleIDs:      input.RoleIDs,
	})
}

func (s Service) DeleteUser(ctx context.Context, id int) error {
	return s.users.Disable(ctx, id)
}

func (s Service) ListUsers(ctx context.Context) ([]*ent.User, error) {
	return s.users.List(ctx)
}

func (s Service) ListRoles(ctx context.Context) ([]*ent.Role, error) {
	if _, err := s.roles.EnsureDefaults(ctx); err != nil {
		return nil, err
	}
	return s.roles.List(ctx)
}

func (s Service) ParseAccessToken(value string) (*platformauth.Claims, error) {
	return s.tokens.ParseAccessToken(value)
}

func normalizeStatus(status string) string {
	if status == "disabled" {
		return "disabled"
	}
	return "enabled"
}

func (s Service) issueTokens(ctx context.Context, user *ent.User, userAgent string, ipAddress string) (*TokenPair, error) {
	roles := roleCodes(user)
	accessToken, accessExpiresAt, err := s.tokens.IssueAccessToken(user.ID, roles, s.authCfg.AccessTokenTTL)
	if err != nil {
		return nil, err
	}
	refreshToken, refreshHash, err := platformauth.NewRefreshToken()
	if err != nil {
		return nil, err
	}
	refreshExpiresAt := time.Now().Add(s.authCfg.RefreshTokenTTL)
	_, err = s.client.RefreshToken.Create().
		SetUser(user).
		SetTokenHash(refreshHash).
		SetExpiresAt(refreshExpiresAt).
		SetUserAgent(userAgent).
		SetIPAddress(ipAddress).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		User:                  user,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshExpiresAt,
		Roles:                 roles,
	}, nil
}

func roleCodes(user *ent.User) []string {
	roles := make([]string, 0, len(user.Edges.Roles))
	for _, item := range user.Edges.Roles {
		if item.Status == "enabled" {
			roles = append(roles, item.Code)
		}
	}
	return roles
}
