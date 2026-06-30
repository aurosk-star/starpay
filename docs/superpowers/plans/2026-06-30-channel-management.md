# Channel Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first payment channel layer for WeChat Pay, Alipay, and PayPal using `github.com/go-pay/gopay@latest`.

**Architecture:** Implement channel account management as a vertical domain under `internal/domain/channels`, following the existing `apps` and `users` module style. Persist provider credentials in `ChannelAccount.config` as JSON, return masked config to the admin UI, and define channel adapter interfaces without wiring them into orders yet.

**Tech Stack:** Go 1.26, Gin, Ent, Casbin, PostgreSQL/MySQL-compatible Ent schema, `github.com/go-pay/gopay v1.5.121`, React 19, Rsbuild, Bun, shadcn/ui, TanStack Router.

## Global Constraints

- Use `github.com/go-pay/gopay@latest`; current latest resolves to `v1.5.121`.
- First version supports `wechat`, `alipay`, and `paypal`.
- Prefer WeChat Pay API v3 and Alipay v3 boundaries; do not add legacy v2-specific behavior.
- Store sensitive channel config in the database, but never return secrets in list/detail responses.
- Keep backend code under `internal/domain/channels/{handler,router,service,repository,test}`.
- Backend tests for channels must live under `internal/domain/channels/test/`.
- Use global response helpers from `internal/platform/httpx`.
- Frontend UI must use shadcn/ui and the shared Data Table factory.
- This slice does not implement real order payment, refund, or callback processing. It only creates the channel account model, admin management APIs, and adapter interfaces.

---

## File Structure

- Modify `go.mod` / `go.sum`: add `github.com/go-pay/gopay@latest`.
- Create `ent/schema/channel_account.go`: channel account schema.
- Generate Ent code with `make ent-up`.
- Create `internal/domain/channels/repository/repository.go`: Ent persistence.
- Create `internal/domain/channels/service/service.go`: validation, masking, CRUD/status behavior.
- Create `internal/domain/channels/service/adapter.go`: adapter interfaces and gopay-oriented config types.
- Create `internal/domain/channels/handler/handler.go`: admin HTTP endpoints.
- Create `internal/domain/channels/router/router.go`: route registration.
- Create `internal/domain/channels/test/service_test.go`: behavior tests.
- Modify `internal/platform/http/router.go`: register channel routes.
- Modify `internal/platform/rbac/defaults.go`: add channel policies.
- Create `web/src/features/channels/types.ts`: frontend types.
- Create `web/src/features/channels/api.ts`: frontend API.
- Create `web/src/features/channels/channels-page.tsx`: admin UI.
- Create `web/src/routes/channels.tsx`: route.
- Modify `web/src/router.tsx`: add route.
- Modify `web/src/routes/root.tsx`: link sidebar channel item to `/channels`.
- Modify `web/src/i18n/resources.ts`: translations.

## API Contract

Admin endpoints:

- `GET /v1/admin/channels`
- `POST /v1/admin/channels`
- `PUT /v1/admin/channels/:id`
- `POST /v1/admin/channels/:id/enable`
- `POST /v1/admin/channels/:id/disable`

Create/update request:

```json
{
  "channel": "wechat",
  "name": "微信支付生产商户号",
  "env": "prod",
  "enabled": true,
  "config": {
    "app_id": "wx_app_id",
    "mch_id": "merchant_id",
    "api_v3_key": "secret",
    "serial_no": "certificate_serial",
    "private_key": "pem_or_path"
  }
}
```

List/update response masks sensitive values:

```json
{
  "channel_account": {
    "id": 1,
    "channel": "wechat",
    "name": "微信支付生产商户号",
    "env": "prod",
    "enabled": true,
    "config": {
      "app_id": "wx_app_id",
      "mch_id": "merchant_id",
      "api_v3_key": "********",
      "serial_no": "certificate_serial",
      "private_key": "********"
    },
    "created_at": "2026-06-30T12:00:00Z",
    "updated_at": "2026-06-30T12:00:00Z"
  }
}
```

## Task 1: Add go-pay and ChannelAccount Schema

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `ent/schema/channel_account.go`
- Generated: `ent/*`

**Interfaces:**
- Produces Ent type `ent.ChannelAccount`.
- Supports fields: `channel`, `name`, `enabled`, `env`, `config`, `created_at`, `updated_at`.

- [ ] **Step 1: Add dependency**

Run:

```bash
go get github.com/go-pay/gopay@latest
```

Expected: `go.mod` includes `github.com/go-pay/gopay v1.5.121` or newer.

- [ ] **Step 2: Create schema**

Create `ent/schema/channel_account.go`:

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type ChannelAccount struct {
	ent.Schema
}

func (ChannelAccount) Fields() []ent.Field {
	return []ent.Field{
		field.String("channel"),
		field.String("name"),
		field.Bool("enabled").Default(true),
		field.String("env").Default("sandbox"),
		field.JSON("config", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
```

- [ ] **Step 3: Generate Ent code**

Run: `make ent-up`

Expected: generated Ent files include `ChannelAccount`.

### Task 2: Implement Repository and Service With Masking

**Files:**
- Create: `internal/domain/channels/repository/repository.go`
- Create: `internal/domain/channels/service/service.go`
- Test: `internal/domain/channels/test/service_test.go`

**Interfaces:**
- `service.New(client *ent.Client) Service`
- `Service.ListChannelAccounts(ctx context.Context) ([]ChannelAccountView, error)`
- `Service.CreateChannelAccount(ctx context.Context, input ManageChannelAccountInput) (*ChannelAccountView, error)`
- `Service.UpdateChannelAccount(ctx context.Context, id int, input ManageChannelAccountInput) (*ChannelAccountView, error)`
- `Service.EnableChannelAccount(ctx context.Context, id int) (*ChannelAccountView, error)`
- `Service.DisableChannelAccount(ctx context.Context, id int) (*ChannelAccountView, error)`

- [ ] **Step 1: Write failing tests**

Create `internal/domain/channels/test/service_test.go` with tests for:

- creating a WeChat channel stores config but masks `api_v3_key` and `private_key` in returned view.
- updating an Alipay channel masks `private_key` and `alipay_public_key`.
- disabling a PayPal channel flips `enabled` to false.
- invalid channel values are rejected.

- [ ] **Step 2: Verify tests fail**

Run:

```bash
go test ./internal/domain/channels/test -run Test -count=1
```

Expected: FAIL because service is not implemented.

- [ ] **Step 3: Implement repository**

Repository methods:

- `List(ctx context.Context) ([]*ent.ChannelAccount, error)`
- `FindByID(ctx context.Context, id int) (*ent.ChannelAccount, error)`
- `Create(ctx context.Context, input CreateChannelAccountInput) (*ent.ChannelAccount, error)`
- `Update(ctx context.Context, id int, input UpdateChannelAccountInput) (*ent.ChannelAccount, error)`
- `SetEnabled(ctx context.Context, id int, enabled bool) (*ent.ChannelAccount, error)`

- [ ] **Step 4: Implement service**

Rules:

- valid channels: `wechat`, `alipay`, `paypal`
- valid env values: `sandbox`, `prod`
- config is stored as provided.
- returned view masks these keys when present:
  - `api_key`
  - `api_v3_key`
  - `secret`
  - `client_secret`
  - `private_key`
  - `alipay_public_key`
  - `wechat_pay_public_key`
  - `cert`
  - `cert_key`
- masking value is `"********"`.

- [ ] **Step 5: Verify service tests**

Run:

```bash
go test ./internal/domain/channels/test -run Test -count=1
```

Expected: PASS.

### Task 3: Add Adapter Interfaces and gopay Boundary

**Files:**
- Create: `internal/domain/channels/service/adapter.go`

**Interfaces:**
- Produces channel adapter abstractions for later orders/refunds tasks.

- [ ] **Step 1: Create adapter types**

Create `internal/domain/channels/service/adapter.go`:

```go
package service

import (
	"context"
	"time"
)

type PaymentRequest struct {
	GatewayOrderNo string
	MerchantOrderNo string
	Amount int64
	Currency string
	Subject string
	Description string
	NotifyURL string
	ReturnURL string
	Metadata map[string]any
}

type PaymentResponse struct {
	Channel string
	ChannelTradeNo string
	PayURL string
	QRCode string
	ClientToken string
	ExpiresAt *time.Time
	Raw map[string]any
}

type QueryPaymentResponse struct {
	ChannelTradeNo string
	Status string
	PaidAt *time.Time
	Raw map[string]any
}

type RefundRequest struct {
	RefundNo string
	GatewayOrderNo string
	ChannelTradeNo string
	Amount int64
	Currency string
	Reason string
}

type RefundResponse struct {
	ChannelRefundNo string
	Status string
	Raw map[string]any
}

type NotifyResult struct {
	ChannelTradeNo string
	Status string
	Amount int64
	Currency string
	PaidAt *time.Time
	Raw map[string]any
}

type Adapter interface {
	CreatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)
	QueryPayment(ctx context.Context, gatewayOrderNo string, channelTradeNo string) (*QueryPaymentResponse, error)
	ClosePayment(ctx context.Context, gatewayOrderNo string, channelTradeNo string) error
	CreateRefund(ctx context.Context, req RefundRequest) (*RefundResponse, error)
	VerifyNotify(ctx context.Context, headers map[string]string, body []byte) (*NotifyResult, error)
}

type AdapterFactory interface {
	Build(account ChannelAccountView) (Adapter, error)
}
```

- [ ] **Step 2: Add gopay import guard**

Add a compile-time gopay dependency reference in the same file or a small `gopay.go` file:

```go
package service

import "github.com/go-pay/gopay"

var _ = gopay.SUCCESS
```

Use the actual exported symbol confirmed by compilation. If `SUCCESS` does not exist, use a real exported symbol from `gopay`.

- [ ] **Step 3: Verify compile**

Run:

```bash
go test ./internal/domain/channels/...
```

Expected: PASS.

### Task 4: Add Admin HTTP Endpoints and RBAC

**Files:**
- Create: `internal/domain/channels/handler/handler.go`
- Create: `internal/domain/channels/router/router.go`
- Modify: `internal/platform/http/router.go`
- Modify: `internal/platform/rbac/defaults.go`

**Interfaces:**
- Produces admin APIs under `/v1/admin/channels`.

- [ ] **Step 1: Implement handler**

Handler methods:

- `ListChannelAccounts`
- `CreateChannelAccount`
- `UpdateChannelAccount`
- `EnableChannelAccount`
- `DisableChannelAccount`

All methods use `httpx.JSONOK` / `httpx.JSONError`.

- [ ] **Step 2: Implement router**

Routes:

- `GET /channels`
- `POST /channels`
- `PUT /channels/:id`
- `POST /channels/:id/enable`
- `POST /channels/:id/disable`

All routes use `AdminAuthMiddleware`.

- [ ] **Step 3: Register in platform router**

Instantiate channel service/handler in `internal/platform/http/router.go` and register under `router.Group("/v1/admin")`.

- [ ] **Step 4: Add RBAC policies**

Policies:

- `super_admin`: full access to `/v1/admin/channels` and `/v1/admin/channels/*`
- `operator`: `GET /v1/admin/channels`
- `viewer`: `GET /v1/admin/channels`

- [ ] **Step 5: Verify backend**

Run:

```bash
go test ./...
```

Expected: PASS.

### Task 5: Add Frontend Channel API and Route

**Files:**
- Create: `web/src/features/channels/types.ts`
- Create: `web/src/features/channels/api.ts`
- Create: `web/src/routes/channels.tsx`
- Modify: `web/src/router.tsx`
- Modify: `web/src/routes/root.tsx`

**Interfaces:**
- Produces frontend API used by `ChannelsPage`.

- [ ] **Step 1: Add frontend types**

Types:

- `PaymentChannel = "wechat" | "alipay" | "paypal"`
- `ChannelEnv = "sandbox" | "prod"`
- `ChannelAccount`
- `ManageChannelAccountPayload`

- [ ] **Step 2: Add API functions**

Functions:

- `listChannelAccounts`
- `createChannelAccount`
- `updateChannelAccount`
- `enableChannelAccount`
- `disableChannelAccount`

- [ ] **Step 3: Add route**

Create `web/src/routes/channels.tsx` and add it to `web/src/router.tsx`.

- [ ] **Step 4: Link sidebar**

Set the existing channel nav item to `to: "/channels"`.

### Task 6: Build Channels Page

**Files:**
- Create: `web/src/features/channels/channels-page.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Uses shared Data Table factory and shadcn components.

- [ ] **Step 1: Add translations**

Add `channels` keys for Chinese and English:

- title, description, create, edit, enable, disable
- channel, name, env, enabled, disabled, config
- wechat, alipay, paypal, sandbox, prod
- loading, empty, loadFailed, saveFailed, statusFailed
- table columns

- [ ] **Step 2: Implement page**

Page behavior:

- loads `/v1/admin/channels`
- displays accounts in Data Table
- shows masked config as compact key/value badges or monospace text
- creates/edits accounts in shadcn Dialog
- uses channel select to switch helper text:
  - WeChat: `app_id, mch_id, api_v3_key, serial_no, private_key`
  - Alipay: `app_id, private_key, alipay_public_key, server_url`
  - PayPal: `client_id, client_secret`
- config entry can be a JSON textarea-like input if no textarea component exists; otherwise use the simplest shadcn-compatible input pattern available in the project.
- enables/disables accounts from row actions.

- [ ] **Step 3: Verify frontend**

Run:

```bash
cd web && bun run typecheck
cd web && bun run build
cd web && bun run format:check
```

Expected: all PASS.

### Task 7: Final Verification

**Files:**
- All files changed by prior tasks.

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

Expected:

- `/channels` opens from sidebar.
- Can create WeChat, Alipay, and PayPal channel accounts.
- Sensitive config values display as `********`.
- Edit preserves admin workflow.
- Enable/disable updates row status.

## Self-Review

- Scope is limited to PRD `ChannelAccount` management and adapter boundaries.
- Does not implement payment order creation, routing, refunds, or callback processing.
- Uses gopay latest version discovered from Go module metadata.
- No placeholders remain in task requirements.
