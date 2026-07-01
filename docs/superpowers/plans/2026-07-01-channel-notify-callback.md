# Channel Notify Callback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the unified payment-platform callback endpoint so Alipay sandbox notifications can mark orders paid.

**Architecture:** Keep `/v1/channel/notify` unauthenticated because payment platforms call it directly. The HTTP handler delegates parsing and signature verification to the payment provider, then updates orders through the order service. Return plain `success` / `fail` because Alipay requires text responses.

**Tech Stack:** Go, Gin, Ent, gopay Alipay, module-local tests under `internal/domain/payments/test/`.

---

### Task 1: Provider Notify Contract

**Files:**
- Modify: `internal/domain/payments/provider/provider.go`
- Modify: `internal/domain/payments/provider/alipay/alipay.go`
- Test: `internal/domain/payments/provider/alipay/alipay_test.go`

- [ ] Add a `NotifyRequest`, `NotifyResult`, and optional `NotifyProvider` interface.
- [ ] Implement Alipay notify parsing with `alipay.ParseNotifyToBodyMap`.
- [ ] Verify signatures with `alipay.VerifySign` when `alipay_public_key` is configured.
- [ ] Extract `out_trade_no`, `trade_no`, `trade_status`, and amount.

### Task 2: Notify Service

**Files:**
- Modify: `internal/domain/payments/service/service.go`
- Test: `internal/domain/payments/test/notify_test.go`

- [ ] Write failing tests for successful Alipay notify and missing provider errors.
- [ ] Add `HandleNotify(ctx, input)` to select the enabled channel account and provider.
- [ ] Return normalized order number, channel trade number, status, amount, and raw payload.

### Task 3: HTTP Callback Endpoint

**Files:**
- Create: `internal/domain/payments/handler/notify_handler.go`
- Create: `internal/domain/payments/router/router.go`
- Modify: `internal/platform/http/router.go`
- Test: `internal/domain/payments/test/notify_handler_test.go`

- [ ] Write failing test for `POST /v1/channel/notify`.
- [ ] Route callback requests without admin auth.
- [ ] On successful `TRADE_SUCCESS` or `TRADE_FINISHED`, mark the order paid idempotently.
- [ ] Return `text/plain` `success`; return `fail` for parse, verify, missing order, or provider errors.

### Task 4: Verification

**Files:**
- Build artifact: `./server`

- [ ] Run `go test ./...`.
- [ ] Run `go build -o ./server ./cmd/server`.
- [ ] Restart the backend from `./server` using `.env`.
- [ ] Smoke test `/healthz`.
