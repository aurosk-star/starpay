# App Request Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add reusable HMAC-SHA256 application request authentication for future business endpoints.

**Architecture:** Store app secrets as a password hash plus AES-GCM encrypted signing secret. A Gin middleware validates signed request parameters, app status, IP allowlist, timestamp window, and Redis replay keys before business handlers run.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, Redis, HMAC-SHA256, AES-GCM.

## Global Constraints

- Use global response shape `{ code, message, data, error }`.
- Backend tests must live in each module or platform `test/` directory.
- After backend changes, run `go test ./...`, build `./server`, and restart the backend.
- Existing applications created before this change must reset their secret before they can use open API request signing.

---

### Task 1: App Secret Encryption

**Files:**
- Modify: `ent/schema/app.go`
- Modify: `internal/domain/apps/repository/repository.go`
- Modify: `internal/domain/apps/service/service.go`
- Modify: `internal/domain/apps/test/service_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `.env.example`

**Interfaces:**
- Produces: `App.AppSecretCiphertext`, `Service.VerifySigningSecret`, and encrypted secret persistence.

- [ ] Write failing tests proving create/reset stores encrypted signing secret and plaintext still verifies.
- [ ] Add `app_secret_ciphertext` to Ent schema and regenerate Ent code.
- [ ] Add `APP_SECRET_ENCRYPTION_KEY` config with a local development default.
- [ ] Encrypt the generated app secret when creating or resetting an app.
- [ ] Run app service tests and confirm they pass.

### Task 2: Signed Request Middleware

**Files:**
- Create: `internal/platform/httpx/app_auth_middleware.go`
- Create: `internal/platform/httpx/test/app_auth_middleware_test.go`
- Modify: `internal/domain/apps/repository/repository.go`

**Interfaces:**
- Consumes: app records by `app_id`, encrypted signing secret, Redis `SetNX`.
- Produces: `AppAuthMiddleware`, context keys for `app_id`, numeric app ID, and `request_id`.

- [ ] Write failing middleware tests for missing fields, invalid signature, expired timestamp, replay, disabled app, IP mismatch, and valid signature.
- [ ] Add repository lookup by public `app_id`.
- [ ] Implement canonical signing string using sorted business/query/form fields excluding `sign`.
- [ ] Validate `app_id`, `request_id`, `timestamp`, `nonce`, and `sign`.
- [ ] Use constant-time HMAC comparison and Redis replay keys.
- [ ] Run middleware tests and confirm they pass.

### Task 3: Router Wiring And Verification

**Files:**
- Modify: `internal/platform/http/router.go`
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: Redis client from `cmd/server/main.go`.
- Produces: `/v1/open` route group protected by app auth, starting with a signed `/v1/open/ping` probe.

- [ ] Wire Redis into router construction.
- [ ] Register `/v1/open/ping` behind app auth for integration testing.
- [ ] Run `go test ./...`.
- [ ] Build `./server`.
- [ ] Restart backend from the fresh binary and verify `/healthz` and `/v1/ping`.
