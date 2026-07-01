# Gateway Config And Return URL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add admin-configurable gateway callback settings and persist order return URLs without mixing payment-platform notify URLs, app webhooks, and browser return URLs.

**Architecture:** Add a DB-backed singleton gateway config domain under `internal/domain/configs` and expose it through authenticated admin endpoints. Add `default_return_url` to apps and `return_url` to payment orders so checkout/payment initiation can resolve browser return URLs from order input first, then app default. Payment provider requests will receive runtime URLs: gateway config builds payment-platform `notify_url`, order/app data supplies `return_url`.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, React 19, TanStack Router, Bun, shadcn/ui, global `{ code, message, data, error }` responses.

## Global Constraints

- `notify_url` in app records remains the merchant webhook URL used after the gateway processes payment results.
- Payment-platform notify URLs are gateway-level runtime URLs, not channel account fields and not app fields.
- Browser `return_url` is order-level, falling back to app `default_return_url`.
- Channel account config must not include `notify_url` or `return_url`.
- Backend tests must live under each domain module's `test/` directory.
- All frontend UI must use shadcn/ui components and Chinese i18n keys.
- After backend changes, run tests, build `./server`, and restart the backend from the latest binary before handoff.

---

### Task 1: Gateway Config Domain And Admin Endpoint

**Files:**
- Create: `ent/schema/gateway_config.go`
- Create: `internal/domain/configs/repository/repository.go`
- Create: `internal/domain/configs/service/service.go`
- Create: `internal/domain/configs/handler/handler.go`
- Create: `internal/domain/configs/router/router.go`
- Create: `internal/domain/configs/test/service_test.go`
- Modify: `internal/platform/http/router.go`

**Interfaces:**
- Produces:
  - `GET /v1/admin/config/gateway`
  - `PUT /v1/admin/config/gateway`
  - `configs.Service.GetGatewayConfig(ctx)`
  - `configs.Service.UpdateGatewayConfig(ctx, input)`

- [ ] **Step 1: Write failing service tests**

Create `internal/domain/configs/test/service_test.go` with tests for:
- first read returns defaults;
- update trims base URL and paths;
- invalid base URL returns an error;
- empty notify paths reset to defaults.

Run: `go test ./internal/domain/configs/test -run TestGatewayConfig`

Expected: FAIL because schema and package do not exist.

- [ ] **Step 2: Add Ent schema**

Create singleton-style schema:

```go
field.String("gateway_base_url").Default("http://localhost:8080")
field.String("payment_notify_path").Default("/v1/channel/notify")
field.String("default_currency").Default("CNY")
field.String("default_locale").Default("zh-CN")
field.Bool("request_id_enabled").Default(true)
field.Bool("maintenance_mode").Default(false)
field.JSON("extra", map[string]any{}).Optional()
```

Run: `make ent-up`.

- [ ] **Step 3: Implement repository and service**

Repository must read the first row by `id ASC`; service creates a default row if none exists. Validate `gateway_base_url` with `net/url`: scheme must be `http` or `https`, host must be non-empty. Normalize paths to start with `/`.

- [ ] **Step 4: Implement admin handler/router**

Use `httpx.JSONOK` and `httpx.JSONError`. Register under `/v1/admin/config/gateway` with existing admin auth middleware.

- [ ] **Step 5: Verify**

Run:
- `go test ./internal/domain/configs/test`
- `go test ./internal/platform/httpx/test`

Expected: PASS.

### Task 2: Persist App Default Return URL

**Files:**
- Modify: `ent/schema/app.go`
- Modify: `internal/domain/apps/repository/repository.go`
- Modify: `internal/domain/apps/service/service.go`
- Modify: `internal/domain/apps/handler/handler.go`
- Modify: `internal/domain/apps/test/service_test.go`
- Modify: `web/src/features/apps/types.ts`
- Modify: `web/src/features/apps/apps-page.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Produces app field `default_return_url`.
- Keeps existing app field `notify_url` as merchant webhook URL.

- [ ] **Step 1: Write failing app tests**

Extend app service tests to assert:
- create app stores `default_return_url`;
- update app can clear `default_return_url`;
- `notify_url` remains independent.

Run: `go test ./internal/domain/apps/test -run TestApp`

Expected: FAIL because generated Ent app does not have `DefaultReturnURL`.

- [ ] **Step 2: Add schema field and regenerate**

Add to `ent/schema/app.go`:

```go
field.String("default_return_url").Optional(),
```

Run: `make ent-up`.

- [ ] **Step 3: Wire backend app layers**

Add `DefaultReturnURL string` to create/update inputs, request DTOs, response serialization, repository create/update calls, and service normalization.

- [ ] **Step 4: Update frontend app page**

Add a shadcn `Input` field labeled `默认支付返回地址`. Keep `notify_url` labeled as `业务通知地址` or `Webhook 通知地址` to avoid confusion.

- [ ] **Step 5: Verify**

Run:
- `go test ./internal/domain/apps/test`
- `bun run typecheck`

Expected: PASS.

### Task 3: Persist Order Return URL And Resolve Fallback

**Files:**
- Modify: `ent/schema/payment_order.go`
- Modify: `internal/domain/orders/repository/repository.go`
- Modify: `internal/domain/orders/service/service.go`
- Modify: `internal/domain/orders/handler/handler.go`
- Modify: `internal/domain/orders/handler/open_handler.go`
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Modify: `internal/domain/orders/test/service_test.go`
- Modify: `internal/domain/orders/test/open_handler_test.go`
- Modify: `internal/domain/orders/test/checkout_payment_test.go`

**Interfaces:**
- Produces order field `return_url`.
- Resolution order:
  1. open order request `return_url`;
  2. app `default_return_url`;
  3. empty string.

- [ ] **Step 1: Write failing order tests**

Add tests proving:
- open order stores request `return_url`;
- open order without request `return_url` stores app `default_return_url`;
- serialized admin/open/checkout order includes `return_url`;
- checkout pay uses stored order `return_url` when request body omits it.

Run: `go test ./internal/domain/orders/test -run 'Test.*ReturnURL'`

Expected: FAIL because order schema lacks `return_url`.

- [ ] **Step 2: Add schema field and regenerate**

Add to `ent/schema/payment_order.go`:

```go
field.String("return_url").Optional(),
```

Run: `make ent-up`.

- [ ] **Step 3: Wire repository and service**

Add `ReturnURL string` to `CreateOrderInput`, `ManageOrderInput`, and normalized order input. In `CreateOpenOrder`, if normalized return URL is empty, query app by `app_id` and use `DefaultReturnURL` when present.

- [ ] **Step 4: Update handlers**

Serialize `return_url` in admin, open, and checkout order responses. In checkout `StartPayment`, set payment input return URL to request `return_url` first, otherwise `order.ReturnURL`.

- [ ] **Step 5: Verify**

Run:
- `go test ./internal/domain/orders/test`
- `go test ./...`

Expected: PASS.

### Task 4: Frontend Gateway Config Page

**Files:**
- Create: `web/src/features/config/api.ts`
- Create: `web/src/features/config/types.ts`
- Create: `web/src/features/config/gateway-config-page.tsx`
- Create: `web/src/routes/config.tsx`
- Modify: `web/src/router.tsx`
- Modify: `web/src/routes/root.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Consumes:
  - `GET /v1/admin/config/gateway`
  - `PUT /v1/admin/config/gateway`
- Produces route `/config/gateway`.

- [ ] **Step 1: Add frontend API/types**

Define `GatewayConfig` and `UpdateGatewayConfigPayload` matching backend response names exactly.

- [ ] **Step 2: Add shadcn form page**

Use `Card`, `Input`, `Switch`, `Textarea`, `Button`, and `Alert`. Fields:
- 网关公网地址
- 统一异步通知路径
- 默认币种
- 默认语言
- 启用 Request ID
- 维护模式
- 扩展配置 JSON

- [ ] **Step 3: Add route and sidebar item**

Register `/config/gateway` and add sidebar label `网关配置`.

- [ ] **Step 4: Verify**

Run:
- `bun run typecheck`
- `bun run build`

Expected: PASS.

### Task 5: Plan Follow-Up Wiring For Alipay Provider

**Files:**
- Modify: `docs/superpowers/plans/2026-07-01-alipay-provider-first.md`

**Interfaces:**
- Consumes gateway config and order return URL work from this plan.
- Produces an updated provider plan where:
  - `NotifyURL` is `gateway_base_url + payment_notify_path`;
  - `ReturnURL` is checkout request return URL or persisted order return URL.

- [ ] **Step 1: Update Alipay provider plan dependencies**

Add this plan as a prerequisite and remove any wording that implies channel config stores callback URLs.

- [ ] **Step 2: Self-review plan wording**

Run:

```bash
rg -n "channel config.*notify_url|channel config.*return_url" docs/superpowers/plans/2026-07-01-alipay-provider-first.md
```

Expected: no matches.

### Task 6: Final Verification And Backend Restart

**Files:**
- Modify only files listed in earlier tasks.

**Interfaces:**
- Produces running backend with latest config/order/app changes.

- [ ] **Step 1: Run full verification**

Run:
- `go test ./...`
- `bun run typecheck`
- `bun run build`

Expected: PASS.

- [ ] **Step 2: Build latest backend**

Run:

```bash
go build -o ./server ./cmd/server
```

Expected: command exits 0 and updates `./server`.

- [ ] **Step 3: Restart backend**

Stop the old backend process, then run:

```bash
set -a; . ./.env; set +a; ./server
```

Expected: backend serves `/healthz` from the newly built binary.

- [ ] **Step 4: Smoke test**

Run:
- `curl -i http://127.0.0.1:8080/healthz`
- authenticated `GET /v1/admin/config/gateway`
- create/open order with `return_url`
- checkout pay without request `return_url`

Expected: gateway config returns saved values and checkout payment receives the persisted return URL.
