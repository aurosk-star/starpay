# Orders CRUD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add admin order CRUD for payment orders with payment-specific constraints and list/detail workflows.

**Architecture:** Payment orders are managed as mutable records only for safe fields; money, app binding, merchant order numbers, and lifecycle status rules stay in the service layer. Admin HTTP handlers expose list/detail/create/update/close endpoints, while delete is modeled as close to preserve auditability.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, Casbin, global JSON response helpers.

## Global Constraints

- Use global response shape `{ code, message, data, error }`.
- Amounts must stay in integer minor units.
- `app_id + merchant_order_no` must be unique.
- Payment order states must be explicit, with `pending`, `paid`, `failed`, and `closed` supported in the first cut.
- Backend tests must live in each module `test/` directory.
- After backend changes, run `go test ./...`, build `./server`, and restart the backend.

---

### Task 1: Payment Order Schema And Ent Generation

**Files:**
- Create: `ent/schema/payment_order.go`
- Modify: generated `ent/*` files via `go run entgo.io/ent/cmd/ent generate ./ent/schema`

**Interfaces:**
- Produces: `ent.PaymentOrder` with fields for gateway order number, app binding, merchant order number, amount, currency, lifecycle status, timestamps, and metadata.

- [ ] Write the failing order service test that references `ent.PaymentOrder` and the new order fields.
- [ ] Add the `PaymentOrder` Ent schema with unique `gateway_order_no` and composite uniqueness on `app_id + merchant_order_no`.
- [ ] Regenerate Ent code and verify the new entity compiles.

### Task 2: Order Repository And Service Rules

**Files:**
- Create: `internal/domain/orders/repository/repository.go`
- Create: `internal/domain/orders/service/service.go`
- Create: `internal/domain/orders/test/service_test.go`

**Interfaces:**
- Consumes: `CreateOrderInput`, `UpdateOrderInput`, `ListOrdersInput`.
- Produces: `CreateOrder`, `ListOrders`, `FindOrder`, `UpdateOrder`, `CloseOrder`, `MarkPaid`.

- [ ] Write tests for minor-unit amount storage, currency allowlist, duplicate merchant order rejection, list filtering, close semantics, and immutable financial fields.
- [ ] Implement repository CRUD for create, list, lookup, update-safe-fields, close, and mark-paid transitions.
- [ ] Implement service validation for app ID, merchant order number, subject, amount, currency, and close-state guards.
- [ ] Run `go test ./internal/domain/orders/test` until green.

### Task 3: Admin HTTP Handler And Router

**Files:**
- Create: `internal/domain/orders/handler/handler.go`
- Create: `internal/domain/orders/router/router.go`
- Modify: `internal/platform/http/router.go`
- Modify: `internal/platform/rbac/defaults.go`

**Interfaces:**
- Consumes: order service methods and admin auth middleware.
- Produces: `/v1/admin/orders`, `/v1/admin/orders/:id`, `/v1/admin/orders/:id/close`.

- [ ] Add handler tests or service-backed request tests for list, detail, create, update, and close behavior.
- [ ] Register order routes under the admin group.
- [ ] Add Casbin rules for super_admin, operator, and viewer order access.
- [ ] Run `go test ./...`.

### Task 4: Backend Rebuild And Restart

**Files:**
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: latest compiled backend binary.
- Produces: fresh running server process for manual verification.

- [ ] Build `./server` from the latest code.
- [ ] Restart the backend process.
- [ ] Verify `/healthz` and `/v1/ping` return 200.
- [ ] Capture the latest running PID and log path for handoff.
