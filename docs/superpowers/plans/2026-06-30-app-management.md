# App Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first complete multi-application management slice for the payment gateway admin console.

**Architecture:** Implement `apps` as a vertical domain under `internal/domain/apps`, following the existing `users` module structure. Admin APIs live under `/v1/admin/apps`, use Casbin RBAC, and return the global `{ code, message, data, error }` envelope. The frontend adds an Apps page under `/apps` using shadcn/ui and the shared Data Table factory.

**Tech Stack:** Go 1.26, Gin, Ent, PostgreSQL/MySQL-compatible Ent schema, Casbin, bcrypt, React 19, Rsbuild, Bun, shadcn/ui, TanStack Router.

## Global Constraints

- Keep backend business code inside `internal/domain/apps/{handler,router,service,repository,test}`.
- Backend tests for this domain must live in `internal/domain/apps/test/`.
- Use `internal/platform/httpx.JSONOK`, `JSONError`, and `JSONNoContent`; do not return raw Gin JSON envelopes.
- Store app secrets only as hashes. Return plaintext secrets only from create and reset responses.
- Preserve existing shadcn/ui defaults and use `web/src/components/data-table/` for tabular display.
- Default frontend language is Chinese. Add all new UI copy to `web/src/i18n/resources.ts`.
- Do not implement signed business-app payment APIs in this slice; this plan only covers admin app management.

---

## File Structure

- Modify `ent/schema/app.go`: add PRD fields for `allowed_ips` and timestamps.
- Generate Ent code under `ent/` with `make ent-up`.
- Create `internal/domain/apps/repository/repository.go`: Ent persistence for CRUD and secret hash updates.
- Create `internal/domain/apps/service/service.go`: validation, app ID generation rules, secret generation/hash/reset behavior.
- Create `internal/domain/apps/handler/handler.go`: admin HTTP request/response mapping.
- Create `internal/domain/apps/router/router.go`: register `/v1/admin/apps` routes.
- Create `internal/domain/apps/test/service_test.go`: module behavior tests.
- Modify `internal/platform/http/router.go`: instantiate and register app routes.
- Modify `internal/platform/rbac/defaults.go`: add app management policies.
- Create `web/src/features/apps/types.ts`: frontend app types.
- Create `web/src/features/apps/api.ts`: frontend admin app API functions.
- Create `web/src/features/apps/apps-page.tsx`: shadcn page with Data Table, dialogs, and reset secret flow.
- Create `web/src/routes/apps.tsx`: TanStack route.
- Modify `web/src/router.tsx`: add apps route to route tree.
- Modify `web/src/routes/root.tsx`: link sidebar item to `/apps`.
- Modify `web/src/i18n/resources.ts`: Chinese and minimal English translation keys.

## API Contract

Admin endpoints:

- `GET /v1/admin/apps`
- `POST /v1/admin/apps`
- `PUT /v1/admin/apps/:id`
- `POST /v1/admin/apps/:id/enable`
- `POST /v1/admin/apps/:id/disable`
- `POST /v1/admin/apps/:id/reset-secret`

Create request:

```json
{
  "app_id": "snsgo",
  "name": "snsgo",
  "notify_url": "https://snsgo.example.com/payment/webhook",
  "allowed_ips": ["10.0.0.1"],
  "status": "enabled"
}
```

Create response data:

```json
{
  "app": {
    "id": 1,
    "app_id": "snsgo",
    "name": "snsgo",
    "notify_url": "https://snsgo.example.com/payment/webhook",
    "allowed_ips": ["10.0.0.1"],
    "status": "enabled",
    "created_at": "2026-06-30T12:00:00Z",
    "updated_at": "2026-06-30T12:00:00Z"
  },
  "app_secret": "pg_app_secret_plaintext_once"
}
```

List/update/status responses return `app` or `items` without `app_secret`.

### Task 1: Extend App Schema

**Files:**
- Modify: `ent/schema/app.go`
- Generated: `ent/*`

**Interfaces:**
- Produces Ent fields used by later tasks: `allowed_ips []string`, `created_at time.Time`, `updated_at time.Time`.

- [ ] **Step 1: Update schema**

Replace `ent/schema/app.go` with:

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type App struct {
	ent.Schema
}

func (App) Fields() []ent.Field {
	return []ent.Field{
		field.String("app_id").Unique(),
		field.String("name"),
		field.String("app_secret_hash"),
		field.String("notify_url").Optional(),
		field.JSON("allowed_ips", []string{}).Optional(),
		field.String("status").Default("enabled"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
```

- [ ] **Step 2: Generate Ent code**

Run: `make ent-up`

Expected: generated `ent/app*.go` files include `AllowedIps`, `CreatedAt`, and `UpdatedAt` support.

- [ ] **Step 3: Verify generation**

Run: `go test ./internal/domain/apps/... ./internal/platform/http/...`

Expected: PASS.

### Task 2: Implement App Repository

**Files:**
- Create: `internal/domain/apps/repository/repository.go`

**Interfaces:**
- Produces `Repository` methods consumed by service:
  - `New(client *ent.Client) Repository`
  - `List(ctx context.Context) ([]*ent.App, error)`
  - `FindByID(ctx context.Context, id int) (*ent.App, error)`
  - `Create(ctx context.Context, input CreateAppInput) (*ent.App, error)`
  - `Update(ctx context.Context, id int, input UpdateAppInput) (*ent.App, error)`
  - `SetStatus(ctx context.Context, id int, status string) (*ent.App, error)`
  - `SetSecretHash(ctx context.Context, id int, hash string) (*ent.App, error)`

- [ ] **Step 1: Create repository**

```go
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

type CreateAppInput struct {
	AppID         string
	Name          string
	AppSecretHash string
	NotifyURL     string
	AllowedIPs    []string
	Status        string
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

func (r Repository) Create(ctx context.Context, input CreateAppInput) (*ent.App, error) {
	create := r.client.App.Create().
		SetAppID(input.AppID).
		SetName(input.Name).
		SetAppSecretHash(input.AppSecretHash).
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

func (r Repository) SetSecretHash(ctx context.Context, id int, hash string) (*ent.App, error) {
	if _, err := r.client.App.UpdateOneID(id).SetAppSecretHash(hash).Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
```

- [ ] **Step 2: Run targeted compile**

Run: `go test ./internal/domain/apps/...`

Expected: PASS or no test files.

### Task 3: Implement App Service With Secret Rules

**Files:**
- Create: `internal/domain/apps/service/service.go`
- Test: `internal/domain/apps/test/service_test.go`

**Interfaces:**
- Produces service methods consumed by handler:
  - `New(client *ent.Client) Service`
  - `ListApps(ctx context.Context) ([]*ent.App, error)`
  - `CreateApp(ctx context.Context, input ManageAppInput) (*AppWithSecret, error)`
  - `UpdateApp(ctx context.Context, id int, input ManageAppInput) (*ent.App, error)`
  - `EnableApp(ctx context.Context, id int) (*ent.App, error)`
  - `DisableApp(ctx context.Context, id int) (*ent.App, error)`
  - `ResetSecret(ctx context.Context, id int) (*AppWithSecret, error)`

- [ ] **Step 1: Write failing tests**

Create `internal/domain/apps/test/service_test.go`:

```go
package appstest

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent"
	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	platformauth "payment-gateway/internal/platform/auth"
)

func TestCreateAppHashesSecretAndReturnsPlaintextOnce(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:create_app?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := appsvc.New(client)
	result, err := svc.CreateApp(ctx, appsvc.ManageAppInput{
		AppID:      "snsgo",
		Name:       "snsgo",
		NotifyURL:  "https://snsgo.example.com/payment/webhook",
		AllowedIPs: []string{"10.0.0.1"},
		Status:     "enabled",
	})
	if err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	if result.AppSecret == "" {
		t.Fatal("AppSecret is empty")
	}
	if result.App.AppSecretHash == "" || result.App.AppSecretHash == result.AppSecret {
		t.Fatalf("secret hash = %q, plaintext = %q", result.App.AppSecretHash, result.AppSecret)
	}
	if !platformauth.CheckPassword(result.App.AppSecretHash, result.AppSecret) {
		t.Fatal("stored hash does not match returned secret")
	}
}

func TestUpdateAppChangesMetadataWithoutChangingSecret(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:update_app?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := appsvc.New(client)
	created, err := svc.CreateApp(ctx, appsvc.ManageAppInput{AppID: "billing", Name: "Billing", Status: "enabled"})
	if err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	originalHash := created.App.AppSecretHash

	updated, err := svc.UpdateApp(ctx, created.App.ID, appsvc.ManageAppInput{
		Name:       "Billing API",
		NotifyURL:  "https://billing.example.com/webhook",
		AllowedIPs: []string{"192.168.1.10"},
		Status:     "disabled",
	})
	if err != nil {
		t.Fatalf("UpdateApp() error = %v", err)
	}
	if updated.Name != "Billing API" || updated.Status != "disabled" {
		t.Fatalf("updated app = %#v", updated)
	}
	if updated.AppSecretHash != originalHash {
		t.Fatal("UpdateApp changed app secret hash")
	}
}

func TestResetSecretRotatesSecretHash(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:reset_secret?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := appsvc.New(client)
	created, err := svc.CreateApp(ctx, appsvc.ManageAppInput{AppID: "ops", Name: "Ops", Status: "enabled"})
	if err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	originalHash := created.App.AppSecretHash

	reset, err := svc.ResetSecret(ctx, created.App.ID)
	if err != nil {
		t.Fatalf("ResetSecret() error = %v", err)
	}
	if reset.AppSecret == "" {
		t.Fatal("reset secret is empty")
	}
	if reset.App.AppSecretHash == originalHash {
		t.Fatal("secret hash did not change")
	}
	if !platformauth.CheckPassword(reset.App.AppSecretHash, reset.AppSecret) {
		t.Fatal("stored reset hash does not match returned secret")
	}
}

var _ *ent.App
```

- [ ] **Step 2: Run tests and confirm failure**

Run: `go test ./internal/domain/apps/test -run Test -count=1`

Expected: FAIL because `internal/domain/apps/service` is not implemented.

- [ ] **Step 3: Implement service**

Create `internal/domain/apps/service/service.go`:

```go
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
	ErrAppIDRequired = errors.New("app_id is required")
	ErrNameRequired  = errors.New("name is required")
)

type Service struct {
	apps apprepo.Repository
}

func New(client *ent.Client) Service {
	return Service{apps: apprepo.New(client)}
}

type ManageAppInput struct {
	AppID      string
	Name       string
	NotifyURL  string
	AllowedIPs []string
	Status     string
}

type AppWithSecret struct {
	App       *ent.App
	AppSecret string
}

func (s Service) ListApps(ctx context.Context) ([]*ent.App, error) {
	return s.apps.List(ctx)
}

func (s Service) CreateApp(ctx context.Context, input ManageAppInput) (*AppWithSecret, error) {
	appID := strings.TrimSpace(input.AppID)
	name := strings.TrimSpace(input.Name)
	if appID == "" {
		return nil, ErrAppIDRequired
	}
	if name == "" {
		return nil, ErrNameRequired
	}
	secret, hash, err := newAppSecret()
	if err != nil {
		return nil, err
	}
	created, err := s.apps.Create(ctx, apprepo.CreateAppInput{
		AppID:         appID,
		Name:          name,
		AppSecretHash: hash,
		NotifyURL:     strings.TrimSpace(input.NotifyURL),
		AllowedIPs:    normalizeAllowedIPs(input.AllowedIPs),
		Status:        normalizeStatus(input.Status),
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
		Name:       name,
		NotifyURL:  strings.TrimSpace(input.NotifyURL),
		AllowedIPs: normalizeAllowedIPs(input.AllowedIPs),
		Status:     normalizeStatus(input.Status),
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
	updated, err := s.apps.SetSecretHash(ctx, id, hash)
	if err != nil {
		return nil, err
	}
	return &AppWithSecret{App: updated, AppSecret: secret}, nil
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/domain/apps/test -run Test -count=1`

Expected: PASS.

### Task 4: Add Admin App HTTP Endpoints and RBAC

**Files:**
- Create: `internal/domain/apps/handler/handler.go`
- Create: `internal/domain/apps/router/router.go`
- Modify: `internal/platform/http/router.go`
- Modify: `internal/platform/rbac/defaults.go`

**Interfaces:**
- Consumes `apps/service.Service`.
- Produces admin API routes under `/v1/admin/apps`.

- [ ] **Step 1: Create handler**

Create `internal/domain/apps/handler/handler.go`:

```go
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"payment-gateway/ent"
	appsvc "payment-gateway/internal/domain/apps/service"
	"payment-gateway/internal/platform/httpx"
)

type Handler struct {
	service appsvc.Service
}

func New(service appsvc.Service) Handler {
	return Handler{service: service}
}

type manageAppRequest struct {
	AppID      string   `json:"app_id"`
	Name       string   `json:"name" binding:"required"`
	NotifyURL  string   `json:"notify_url"`
	AllowedIPs []string `json:"allowed_ips"`
	Status     string   `json:"status"`
}

func (h Handler) ListApps(ctx *gin.Context) {
	apps, err := h.service.ListApps(ctx.Request.Context())
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "list_apps_failed", err.Error())
		return
	}
	items := make([]gin.H, 0, len(apps))
	for _, item := range apps {
		items = append(items, serializeApp(item))
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"items": items})
}

func (h Handler) CreateApp(ctx *gin.Context) {
	var req manageAppRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := h.service.CreateApp(ctx.Request.Context(), appsvc.ManageAppInput{
		AppID:      req.AppID,
		Name:       req.Name,
		NotifyURL:  req.NotifyURL,
		AllowedIPs: req.AllowedIPs,
		Status:     req.Status,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "create_app_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusCreated, gin.H{
		"app":        serializeApp(result.App),
		"app_secret": result.AppSecret,
	})
}

func (h Handler) UpdateApp(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_app_id", "invalid app id")
		return
	}
	var req manageAppRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	app, err := h.service.UpdateApp(ctx.Request.Context(), id, appsvc.ManageAppInput{
		Name:       req.Name,
		NotifyURL:  req.NotifyURL,
		AllowedIPs: req.AllowedIPs,
		Status:     req.Status,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "update_app_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"app": serializeApp(app)})
}

func (h Handler) EnableApp(ctx *gin.Context) {
	h.setStatus(ctx, true)
}

func (h Handler) DisableApp(ctx *gin.Context) {
	h.setStatus(ctx, false)
}

func (h Handler) ResetSecret(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_app_id", "invalid app id")
		return
	}
	result, err := h.service.ResetSecret(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "reset_app_secret_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{
		"app":        serializeApp(result.App),
		"app_secret": result.AppSecret,
	})
}

func (h Handler) setStatus(ctx *gin.Context, enabled bool) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_app_id", "invalid app id")
		return
	}
	var app *ent.App
	if enabled {
		app, err = h.service.EnableApp(ctx.Request.Context(), id)
	} else {
		app, err = h.service.DisableApp(ctx.Request.Context(), id)
	}
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "update_app_status_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"app": serializeApp(app)})
}

func parseID(ctx *gin.Context) (int, error) {
	return strconv.Atoi(ctx.Param("id"))
}

func serializeApp(app *ent.App) gin.H {
	return gin.H{
		"id":          app.ID,
		"app_id":      app.AppID,
		"name":        app.Name,
		"notify_url":  app.NotifyURL,
		"allowed_ips": app.AllowedIps,
		"status":      app.Status,
		"created_at":  app.CreatedAt,
		"updated_at":  app.UpdatedAt,
	}
}
```

- [ ] **Step 2: Create router**

Create `internal/domain/apps/router/router.go`:

```go
package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	apphandler "payment-gateway/internal/domain/apps/handler"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler apphandler.Handler, userService usersvc.Service, enforcer *casbin.Enforcer) {
	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(userService, enforcer))
	protected.GET("/apps", handler.ListApps)
	protected.POST("/apps", handler.CreateApp)
	protected.PUT("/apps/:id", handler.UpdateApp)
	protected.POST("/apps/:id/enable", handler.EnableApp)
	protected.POST("/apps/:id/disable", handler.DisableApp)
	protected.POST("/apps/:id/reset-secret", handler.ResetSecret)
}
```

- [ ] **Step 3: Register routes in platform router**

Modify `internal/platform/http/router.go` imports and `NewRouter`:

```go
apphandler "payment-gateway/internal/domain/apps/handler"
approuter "payment-gateway/internal/domain/apps/router"
appsvc "payment-gateway/internal/domain/apps/service"
```

Add after user registration:

```go
appService := appsvc.New(client)
appHandler := apphandler.New(appService)
approuter.Register(router.Group("/v1/admin"), appHandler, userService, enforcer)
```

- [ ] **Step 4: Add RBAC policies**

Modify `internal/platform/rbac/defaults.go` policies:

```go
{"super_admin", "/v1/admin/apps", "*"},
{"super_admin", "/v1/admin/apps/:id", "*"},
{"super_admin", "/v1/admin/apps/:id/enable", "*"},
{"super_admin", "/v1/admin/apps/:id/disable", "*"},
{"super_admin", "/v1/admin/apps/:id/reset-secret", "*"},
{"operator", "/v1/admin/apps", "GET"},
{"viewer", "/v1/admin/apps", "GET"},
```

- [ ] **Step 5: Verify backend**

Run: `go test ./...`

Expected: PASS.

### Task 5: Add Frontend App API and Route

**Files:**
- Create: `web/src/features/apps/types.ts`
- Create: `web/src/features/apps/api.ts`
- Create: `web/src/routes/apps.tsx`
- Modify: `web/src/router.tsx`
- Modify: `web/src/routes/root.tsx`

**Interfaces:**
- Produces frontend API used by `AppsPage`.

- [ ] **Step 1: Add types**

Create `web/src/features/apps/types.ts`:

```ts
export type GatewayApp = {
  id: number;
  app_id: string;
  name: string;
  notify_url?: string;
  allowed_ips: string[];
  status: string;
  created_at: string;
  updated_at: string;
};

export type ManageAppPayload = {
  app_id?: string;
  name: string;
  notify_url?: string;
  allowed_ips: string[];
  status: string;
};
```

- [ ] **Step 2: Add API functions**

Create `web/src/features/apps/api.ts`:

```ts
import { apiRequest } from "@/lib/api";

import type { GatewayApp, ManageAppPayload } from "./types";

export function listApps(accessToken: string) {
  return apiRequest<{ items: GatewayApp[] }>("/v1/admin/apps", {
    accessToken,
  });
}

export function createApp(accessToken: string, payload: ManageAppPayload) {
  return apiRequest<{ app: GatewayApp; app_secret: string }>("/v1/admin/apps", {
    method: "POST",
    accessToken,
    body: JSON.stringify(payload),
  });
}

export function updateApp(
  accessToken: string,
  id: number,
  payload: ManageAppPayload,
) {
  return apiRequest<{ app: GatewayApp }>(`/v1/admin/apps/${id}`, {
    method: "PUT",
    accessToken,
    body: JSON.stringify(payload),
  });
}

export function enableApp(accessToken: string, id: number) {
  return apiRequest<{ app: GatewayApp }>(`/v1/admin/apps/${id}/enable`, {
    method: "POST",
    accessToken,
  });
}

export function disableApp(accessToken: string, id: number) {
  return apiRequest<{ app: GatewayApp }>(`/v1/admin/apps/${id}/disable`, {
    method: "POST",
    accessToken,
  });
}

export function resetAppSecret(accessToken: string, id: number) {
  return apiRequest<{ app: GatewayApp; app_secret: string }>(
    `/v1/admin/apps/${id}/reset-secret`,
    {
      method: "POST",
      accessToken,
    },
  );
}
```

- [ ] **Step 3: Add route file**

Create `web/src/routes/apps.tsx`:

```tsx
import { createRoute } from "@tanstack/react-router";

import { AppsPage } from "@/features/apps/apps-page";

import { rootRoute } from "./root";

export const appsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/apps",
  component: AppsPage,
});
```

- [ ] **Step 4: Register route**

Modify `web/src/router.tsx`:

```ts
import { appsRoute } from "./routes/apps";
```

Change route tree:

```ts
const routeTree = rootRoute.addChildren([indexRoute, usersRoute, appsRoute]);
```

- [ ] **Step 5: Link sidebar**

Modify `web/src/routes/root.tsx` nav item:

```ts
{ label: t("nav.apps"), icon: Boxes, to: "/apps" },
```

- [ ] **Step 6: Verify route compile fails until page exists**

Run: `cd web && bun run typecheck`

Expected: FAIL because `AppsPage` is not implemented.

### Task 6: Build Apps Page With shadcn Data Table

**Files:**
- Create: `web/src/features/apps/apps-page.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Consumes app API from Task 5.
- UI uses shadcn Dialog for create/edit and AlertDialog for secret reset confirmation.

- [ ] **Step 1: Add translation keys**

Add Chinese keys under `translation.apps`:

```ts
apps: {
  title: "接入应用",
  description: "管理内部业务应用、回调地址、IP 白名单和应用密钥。",
  create: "新建应用",
  edit: "编辑",
  enable: "启用",
  disable: "禁用",
  resetSecret: "重置密钥",
  createTitle: "新建接入应用",
  editTitle: "编辑接入应用",
  formDescription: "应用 ID 创建后不可修改，应用密钥只会显示一次。",
  appId: "应用 ID",
  name: "应用名称",
  notifyUrl: "Webhook 地址",
  allowedIps: "IP 白名单",
  allowedIpsHint: "用英文逗号或换行分隔，留空表示不限制。",
  status: "状态",
  enabled: "启用",
  disabled: "禁用",
  save: "保存",
  cancel: "取消",
  loading: "正在加载应用...",
  empty: "暂无接入应用。",
  loadFailed: "加载应用失败。",
  saveFailed: "保存应用失败。",
  statusFailed: "更新应用状态失败。",
  resetFailed: "重置应用密钥失败。",
  secretTitle: "应用密钥",
  secretDescription: "请立即保存该密钥。关闭后将无法再次查看明文。",
  resetConfirmTitle: "确认重置应用密钥？",
  resetConfirmDescription: "旧密钥会立即失效，业务应用必须改用新密钥。",
  table: {
    app: "应用",
    notifyUrl: "Webhook",
    allowedIps: "IP 白名单",
    status: "状态",
    updatedAt: "更新时间",
  },
}
```

Add minimal English keys under `en.translation.apps` with equivalent labels.

- [ ] **Step 2: Create AppsPage**

Implement `web/src/features/apps/apps-page.tsx` using this structure:

```tsx
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { KeyRound, MoreHorizontal, Plus, RefreshCw } from "lucide-react";

import { createDataTable, type DataTableColumn } from "@/components/data-table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { APIError } from "@/lib/api";
import { useAuthStore } from "@/features/auth/store";

import {
  createApp,
  disableApp,
  enableApp,
  listApps,
  resetAppSecret,
  updateApp,
} from "./api";
import type { GatewayApp } from "./types";

const AppsDataTable = createDataTable<GatewayApp>();

const emptyForm = {
  appId: "",
  name: "",
  notifyUrl: "",
  allowedIps: "",
  status: "enabled",
};

type FormState = typeof emptyForm;

export function AppsPage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [apps, setApps] = useState<GatewayApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingApp, setEditingApp] = useState<GatewayApp | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [secret, setSecret] = useState<string | null>(null);
  const [resetTarget, setResetTarget] = useState<GatewayApp | null>(null);

  const columns = useMemo<DataTableColumn<GatewayApp>[]>(
    () => [
      {
        accessorKey: "app_id",
        header: t("apps.table.app"),
        cell: ({ row }) => (
          <div className="flex flex-col">
            <span className="font-medium">{row.original.name}</span>
            <span className="font-mono text-xs text-muted-foreground">
              {row.original.app_id}
            </span>
          </div>
        ),
      },
      {
        accessorKey: "notify_url",
        header: t("apps.table.notifyUrl"),
        cell: ({ row }) => (
          <span className="block max-w-[320px] truncate font-mono text-xs">
            {row.original.notify_url || "-"}
          </span>
        ),
      },
      {
        accessorKey: "allowed_ips",
        header: t("apps.table.allowedIps"),
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            {row.original.allowed_ips.length > 0
              ? row.original.allowed_ips.map((ip) => (
                  <Badge key={ip} variant="outline">
                    {ip}
                  </Badge>
                ))
              : "-"}
          </div>
        ),
      },
      {
        accessorKey: "status",
        header: t("apps.table.status"),
        cell: ({ row }) => (
          <Badge
            variant={
              row.original.status === "enabled" ? "secondary" : "outline"
            }
          >
            {row.original.status === "enabled"
              ? t("apps.enabled")
              : t("apps.disabled")}
          </Badge>
        ),
      },
      {
        accessorKey: "updated_at",
        header: t("apps.table.updatedAt"),
        cell: ({ row }) => (
          <span className="font-mono text-xs text-muted-foreground">
            {new Date(row.original.updated_at).toLocaleString()}
          </span>
        ),
      },
      {
        id: "actions",
        header: () => (
          <div className="text-right">{t("common.moreActions")}</div>
        ),
        cell: ({ row }) => (
          <div className="text-right">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-sm">
                  <MoreHorizontal />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => openEdit(row.original)}>
                  {t("apps.edit")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => toggleStatus(row.original)}>
                  {row.original.status === "enabled"
                    ? t("apps.disable")
                    : t("apps.enable")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setResetTarget(row.original)}>
                  <KeyRound />
                  {t("apps.resetSecret")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        ),
      },
    ],
    [t],
  );

  async function load() {
    if (!accessToken) return;
    setLoading(true);
    setError(null);
    try {
      const result = await listApps(accessToken);
      setApps(result.items);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("apps.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [accessToken]);

  function openCreate() {
    setEditingApp(null);
    setForm(emptyForm);
    setDialogOpen(true);
  }

  function openEdit(app: GatewayApp) {
    setEditingApp(app);
    setForm({
      appId: app.app_id,
      name: app.name,
      notifyUrl: app.notify_url || "",
      allowedIps: app.allowed_ips.join("\n"),
      status: app.status,
    });
    setDialogOpen(true);
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken) return;
    setSaving(true);
    setError(null);
    try {
      const payload = {
        app_id: editingApp ? undefined : form.appId,
        name: form.name,
        notify_url: form.notifyUrl || undefined,
        allowed_ips: form.allowedIps
          .split(/[\n,]/)
          .map((item) => item.trim())
          .filter(Boolean),
        status: form.status,
      };
      const result = editingApp
        ? await updateApp(accessToken, editingApp.id, payload)
        : await createApp(accessToken, payload);
      setApps((current) =>
        editingApp
          ? current.map((item) =>
              item.id === result.app.id ? result.app : item,
            )
          : [result.app, ...current],
      );
      if ("app_secret" in result) {
        setSecret(result.app_secret);
      }
      setDialogOpen(false);
      setEditingApp(null);
      setForm(emptyForm);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("apps.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function toggleStatus(app: GatewayApp) {
    if (!accessToken) return;
    try {
      const result =
        app.status === "enabled"
          ? await disableApp(accessToken, app.id)
          : await enableApp(accessToken, app.id);
      setApps((current) =>
        current.map((item) => (item.id === result.app.id ? result.app : item)),
      );
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("apps.statusFailed"));
    }
  }

  async function confirmResetSecret() {
    if (!accessToken || !resetTarget) return;
    try {
      const result = await resetAppSecret(accessToken, resetTarget.id);
      setApps((current) =>
        current.map((item) => (item.id === result.app.id ? result.app : item)),
      );
      setSecret(result.app_secret);
      setResetTarget(null);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("apps.resetFailed"));
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("apps.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("apps.description")}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={load}>
            <RefreshCw />
            {t("common.refresh")}
          </Button>
          <Button onClick={openCreate}>
            <Plus />
            {t("apps.create")}
          </Button>
        </div>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertTitle>{t("apps.loadFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{t("apps.title")}</CardTitle>
          <CardDescription>{t("apps.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <p className="text-sm text-muted-foreground">
              {t("apps.loading")}
            </p>
          ) : apps.length > 0 ? (
            <AppsDataTable columns={columns} data={apps} />
          ) : (
            <p className="text-sm text-muted-foreground">{t("apps.empty")}</p>
          )}
        </CardContent>
      </Card>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingApp ? t("apps.editTitle") : t("apps.createTitle")}
            </DialogTitle>
            <DialogDescription>{t("apps.formDescription")}</DialogDescription>
          </DialogHeader>
          <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="app_id">{t("apps.appId")}</FieldLabel>
                <Input
                  id="app_id"
                  value={form.appId}
                  disabled={Boolean(editingApp)}
                  required
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      appId: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="name">{t("apps.name")}</FieldLabel>
                <Input
                  id="name"
                  value={form.name}
                  required
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="notify_url">
                  {t("apps.notifyUrl")}
                </FieldLabel>
                <Input
                  id="notify_url"
                  value={form.notifyUrl}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      notifyUrl: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="allowed_ips">
                  {t("apps.allowedIps")}
                </FieldLabel>
                <Input
                  id="allowed_ips"
                  value={form.allowedIps}
                  placeholder={t("apps.allowedIpsHint")}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      allowedIps: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel>{t("apps.status")}</FieldLabel>
                <Select
                  value={form.status}
                  onValueChange={(value) =>
                    setForm((current) => ({ ...current, status: value }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="enabled">{t("apps.enabled")}</SelectItem>
                      <SelectItem value="disabled">{t("apps.disabled")}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
            </FieldGroup>
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setDialogOpen(false)}
              >
                {t("apps.cancel")}
              </Button>
              <Button type="submit" disabled={saving}>
                {t("apps.save")}
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(secret)} onOpenChange={() => setSecret(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("apps.secretTitle")}</DialogTitle>
            <DialogDescription>{t("apps.secretDescription")}</DialogDescription>
          </DialogHeader>
          <div className="rounded-md border bg-muted p-3 font-mono text-sm">
            {secret}
          </div>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(resetTarget)}
        onOpenChange={(open) => {
          if (!open) setResetTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("apps.resetConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("apps.resetConfirmDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("apps.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmResetSecret}>
              {t("apps.resetSecret")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
```

- [ ] **Step 3: Run frontend checks**

Run:

```bash
cd web && bun run typecheck
cd web && bun run build
cd web && bun run format:check
```

Expected: all PASS.

### Task 7: Final Verification

**Files:**
- All files changed by previous tasks.

**Interfaces:**
- Confirms backend and frontend work together.

- [ ] **Step 1: Run backend tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Run frontend checks**

Run:

```bash
cd web && bun run typecheck
cd web && bun run build
cd web && bun run format:check
```

Expected: all PASS.

- [ ] **Step 3: Manual smoke test**

Run:

```bash
make db-up
make dev
make web-dev
```

Expected:

- Admin can log in.
- Sidebar “接入应用” opens `/apps`.
- Creating an app returns a one-time secret dialog.
- Editing an app does not show a secret.
- Resetting a secret shows a new one-time secret.
- Enable/disable updates table status.

## Self-Review

- Spec coverage: Covers PRD section 13.1 “应用管理” and data model section 10.1 `App`. It intentionally does not implement business-app signed payment APIs from PRD section 11.1.
- Placeholder scan: No task uses TBD/TODO/later placeholders.
- Type consistency: Backend service, handler, and frontend API names align on `app`, `app_secret`, `allowed_ips`, `enabled`, and `disabled`.
