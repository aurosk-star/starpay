# Security Hardening Gaps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fill the remaining payment-gateway security gaps by adding app-level rate limiting and documenting/implementing an operational app secret rotation and leak-response process.

**Architecture:** Keep the current open API authentication, replay protection, IP whitelist, and webhook signing behavior unchanged. Add a Redis-backed rate limiter as a small `httpx` middleware that runs after app authentication so it can limit by authenticated `app_id` and route. Document the existing secret behavior plus the supported rotation/leak-response workflow without introducing dual-key compatibility in V1.

**Tech Stack:** Go, Gin, Redis, Ent, existing `internal/platform/httpx`, existing app service secret reset flow, Markdown docs.

## Global Constraints

- Keep current webhook headers: `X-Pay-Gateway-Timestamp` and `X-Pay-Gateway-Signature`.
- Keep current webhook signature content: `timestamp + "." + raw_body`.
- Keep current open API replay protection: 5-minute timestamp window and Redis `SetNX` for `request_id` and `nonce`.
- Keep current IP whitelist behavior.
- V1 rate limit must be optional and configurable; disabling rate limit must not block requests.
- All API responses must use the global `{ code, message, data, error }` shape.
- Rate limit rejection must return HTTP `429` with `RATE_LIMITED`.
- Add tests with every behavior change.
- After backend changes, run `go test ./...`, build `.tmp/payment-gateway-server`, and restart the backend from that binary.

## Non-Goals

- Do not rename webhook signature headers to `X-Gateway-Signature`.
- Do not change app open API signing rules.
- Do not add dual-active app secrets in V1.
- Do not add a frontend secret-rotation wizard in this pass.
- Do not rate limit admin pages in this pass.

---

## File Structure

- Create `internal/platform/httpx/rate_limiter.go`
  - Owns a small Redis/fallback interface, fixed-window limiter logic, and Gin middleware.
- Modify `internal/platform/httpx/errors.go`
  - Reuse existing `CodeRateLimited`; no new code expected unless missing.
- Create `internal/platform/httpx/test/rate_limiter_test.go`
  - Tests app+route scoped limiting, disabled mode, and stable error response.
- Modify `internal/platform/http/router.go`
  - Adds rate limit middleware to `/v1/open` after `AppAuthMiddleware`.
- Modify `internal/platform/config/config.go`
  - Adds `RateLimitConfig` with env-driven settings.
- Modify `internal/platform/config/config_test.go`
  - Tests rate limit config defaults and env parsing.
- Modify `.env.example`
  - Documents V1 rate limit settings.
- Modify `docs/PAYMENT_GATEWAY_PRD.md`
  - Adds security subsections for rate limiting and app secret operation process.
- Modify `docs/PAYMENT_GATEWAY_INTEGRATION.md`
  - Adds merchant-facing rate limit behavior and secret leak/rotation guidance.
- Modify `README.md`
  - Adds short operational summary.

---

### Task 1: Add Configurable Open API Rate Limit Config

**Files:**
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `.env.example`

**Interfaces:**
- Produces:
  - `type RateLimitConfig struct { OpenAPIEnabled bool; OpenAPILimit int; OpenAPIWindow time.Duration }`
  - `Config.RateLimit RateLimitConfig`

- [ ] **Step 1: Write failing config tests**

Add to `internal/platform/config/config_test.go`:

```go
func TestLoadRateLimitDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.RateLimit.OpenAPIEnabled {
		t.Fatalf("RateLimit.OpenAPIEnabled = false, want true")
	}
	if cfg.RateLimit.OpenAPILimit != 120 {
		t.Fatalf("RateLimit.OpenAPILimit = %d, want 120", cfg.RateLimit.OpenAPILimit)
	}
	if cfg.RateLimit.OpenAPIWindow != time.Minute {
		t.Fatalf("RateLimit.OpenAPIWindow = %v, want 1m", cfg.RateLimit.OpenAPIWindow)
	}
}

func TestLoadRateLimitEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("OPEN_API_RATE_LIMIT_ENABLED", "false")
	t.Setenv("OPEN_API_RATE_LIMIT", "30")
	t.Setenv("OPEN_API_RATE_LIMIT_WINDOW", "10s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RateLimit.OpenAPIEnabled {
		t.Fatalf("RateLimit.OpenAPIEnabled = true, want false")
	}
	if cfg.RateLimit.OpenAPILimit != 30 {
		t.Fatalf("RateLimit.OpenAPILimit = %d, want 30", cfg.RateLimit.OpenAPILimit)
	}
	if cfg.RateLimit.OpenAPIWindow != 10*time.Second {
		t.Fatalf("RateLimit.OpenAPIWindow = %v, want 10s", cfg.RateLimit.OpenAPIWindow)
	}
}
```

Add these keys to `clearConfigEnv(t)`:

```go
"OPEN_API_RATE_LIMIT_ENABLED",
"OPEN_API_RATE_LIMIT",
"OPEN_API_RATE_LIMIT_WINDOW",
```

- [ ] **Step 2: Run config tests to verify failure**

Run:

```bash
go test ./internal/platform/config -run RateLimit
```

Expected: FAIL because `Config.RateLimit` does not exist.

- [ ] **Step 3: Implement config**

In `internal/platform/config/config.go`, add:

```go
type RateLimitConfig struct {
	OpenAPIEnabled bool
	OpenAPILimit   int
	OpenAPIWindow  time.Duration
}
```

Add field to `Config`:

```go
RateLimit RateLimitConfig
```

Add to `Load()`:

```go
RateLimit: RateLimitConfig{
	OpenAPIEnabled: boolEnv("OPEN_API_RATE_LIMIT_ENABLED", true),
	OpenAPILimit:   intEnv("OPEN_API_RATE_LIMIT", 120),
	OpenAPIWindow:  durationEnv("OPEN_API_RATE_LIMIT_WINDOW", time.Minute),
},
```

- [ ] **Step 4: Document env settings**

Add to `.env.example` near Redis/Auth settings:

```dotenv
# Open API rate limit, scoped by app_id + HTTP method + route.
OPEN_API_RATE_LIMIT_ENABLED=true
OPEN_API_RATE_LIMIT=120
OPEN_API_RATE_LIMIT_WINDOW=1m
```

- [ ] **Step 5: Run config tests to verify pass**

Run:

```bash
go test ./internal/platform/config -run RateLimit
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/platform/config/config.go internal/platform/config/config_test.go .env.example
git commit -m "Add open API rate limit config"
```

---

### Task 2: Add Redis-Backed App Route Rate Limiter

**Files:**
- Create: `internal/platform/httpx/rate_limiter.go`
- Create: `internal/platform/httpx/test/rate_limiter_test.go`
- Modify: `internal/platform/httpx/errors.go` only if `CodeRateLimited` is missing.

**Interfaces:**
- Consumes:
  - `ContextAppID`
  - `CodeRateLimited`
  - `JSONError(ctx, status, code, message)`
- Produces:
  - `type RateLimitStore interface { Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) }`
  - `type RedisRateLimitStore struct { client *redis.Client }`
  - `func NewRedisRateLimitStore(client *redis.Client) RedisRateLimitStore`
  - `type RateLimitOptions struct { Store RateLimitStore; Enabled bool; Limit int; Window time.Duration; Scope string }`
  - `func RateLimitMiddleware(options RateLimitOptions) gin.HandlerFunc`

- [ ] **Step 1: Write failing middleware tests**

Create `internal/platform/httpx/test/rate_limiter_test.go`:

```go
package httpxtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"payment-gateway/internal/platform/httpx"
)

func TestRateLimitMiddlewareLimitsByAppMethodAndRoute(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	store := newMemoryRateLimitStore()
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(httpx.ContextAppID, "app_123")
		ctx.Next()
	})
	router.Use(httpx.RateLimitMiddleware(httpx.RateLimitOptions{
		Store:   store,
		Enabled: true,
		Limit:   2,
		Window:  time.Minute,
		Scope:   "open_api",
	}))
	router.POST("/v1/open/orders", func(ctx *gin.Context) {
		httpx.JSONOK(ctx, http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/open/orders", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, body = %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/open/orders", nil))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("third status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	assertRateLimitCode(t, recorder, httpx.CodeRateLimited)
}

func TestRateLimitMiddlewareDisabledAllowsRequests(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	store := newMemoryRateLimitStore()
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(httpx.ContextAppID, "app_123")
		ctx.Next()
	})
	router.Use(httpx.RateLimitMiddleware(httpx.RateLimitOptions{
		Store:   store,
		Enabled: false,
		Limit:   1,
		Window:  time.Minute,
		Scope:   "open_api",
	}))
	router.POST("/v1/open/orders", func(ctx *gin.Context) {
		httpx.JSONOK(ctx, http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 3; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/open/orders", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, body = %s", i+1, recorder.Code, recorder.Body.String())
		}
	}
}

func assertRateLimitCode(t *testing.T, recorder *httptest.ResponseRecorder, want string) {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["code"] != want {
		t.Fatalf("code = %#v, want %s; body = %s", response["code"], want, recorder.Body.String())
	}
}

type memoryRateLimitStore struct {
	mu     sync.Mutex
	counts map[string]int64
}

func newMemoryRateLimitStore() *memoryRateLimitStore {
	return &memoryRateLimitStore{counts: map[string]int64{}}
}

func (s *memoryRateLimitStore) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[key]++
	return s.counts[key], nil
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/platform/httpx/test -run RateLimit
```

Expected: FAIL because rate limit types and middleware do not exist.

- [ ] **Step 3: Implement middleware**

Create `internal/platform/httpx/rate_limiter.go`:

```go
package httpx

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type RateLimitStore interface {
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

type RedisRateLimitStore struct {
	client *redis.Client
}

func NewRedisRateLimitStore(client *redis.Client) RedisRateLimitStore {
	return RedisRateLimitStore{client: client}
}

func (s RedisRateLimitStore) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := s.client.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

type RateLimitOptions struct {
	Store   RateLimitStore
	Enabled bool
	Limit   int
	Window  time.Duration
	Scope   string
}

func RateLimitMiddleware(options RateLimitOptions) gin.HandlerFunc {
	window := options.Window
	if window <= 0 {
		window = time.Minute
	}
	scope := strings.TrimSpace(options.Scope)
	if scope == "" {
		scope = "api"
	}
	return func(ctx *gin.Context) {
		if !options.Enabled || options.Store == nil || options.Limit <= 0 {
			ctx.Next()
			return
		}
		appID := strings.TrimSpace(ctx.GetString(ContextAppID))
		if appID == "" {
			ctx.Next()
			return
		}
		route := ctx.FullPath()
		if route == "" {
			route = ctx.Request.URL.Path
		}
		key := "rate_limit:" + scope + ":" + appID + ":" + ctx.Request.Method + ":" + route
		count, err := options.Store.Incr(ctx.Request.Context(), key, window)
		if err != nil {
			JSONError(ctx, http.StatusServiceUnavailable, CodeServiceUnavailable, "rate limit service is unavailable")
			ctx.Abort()
			return
		}
		if count > int64(options.Limit) {
			JSONError(ctx, http.StatusTooManyRequests, CodeRateLimited, "rate limit exceeded")
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}
```

- [ ] **Step 4: Run tests to verify pass**

Run:

```bash
go test ./internal/platform/httpx/test -run RateLimit
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/httpx/rate_limiter.go internal/platform/httpx/test/rate_limiter_test.go
git commit -m "Add open API rate limiting middleware"
```

---

### Task 3: Wire Rate Limiter Into Open API Routes

**Files:**
- Modify: `internal/platform/http/router.go`
- Test: `internal/platform/httpx/test/rate_limiter_test.go`

**Interfaces:**
- Consumes:
  - `config.Config.RateLimit`
  - `httpx.NewRedisRateLimitStore(redisClient)`
  - `httpx.RateLimitMiddleware(...)`

- [ ] **Step 1: Add a router wiring test**

If there is no existing router integration test for full `NewRouter`, keep this as a focused middleware test by adding to `internal/platform/httpx/test/rate_limiter_test.go`:

```go
func TestRateLimitMiddlewareUsesDifferentCountersForDifferentRoutes(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	store := newMemoryRateLimitStore()
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(httpx.ContextAppID, "app_123")
		ctx.Next()
	})
	router.Use(httpx.RateLimitMiddleware(httpx.RateLimitOptions{
		Store:   store,
		Enabled: true,
		Limit:   1,
		Window:  time.Minute,
		Scope:   "open_api",
	}))
	router.GET("/v1/open/orders/:gateway_order_no", func(ctx *gin.Context) {
		httpx.JSONOK(ctx, http.StatusOK, gin.H{"ok": true})
	})
	router.POST("/v1/open/orders", func(ctx *gin.Context) {
		httpx.JSONOK(ctx, http.StatusOK, gin.H{"ok": true})
	})

	firstGet := httptest.NewRecorder()
	router.ServeHTTP(firstGet, httptest.NewRequest(http.MethodGet, "/v1/open/orders/pay_123", nil))
	if firstGet.Code != http.StatusOK {
		t.Fatalf("first GET status = %d", firstGet.Code)
	}

	firstPost := httptest.NewRecorder()
	router.ServeHTTP(firstPost, httptest.NewRequest(http.MethodPost, "/v1/open/orders", nil))
	if firstPost.Code != http.StatusOK {
		t.Fatalf("first POST status = %d", firstPost.Code)
	}

	secondGet := httptest.NewRecorder()
	router.ServeHTTP(secondGet, httptest.NewRequest(http.MethodGet, "/v1/open/orders/pay_456", nil))
	if secondGet.Code != http.StatusTooManyRequests {
		t.Fatalf("second GET status = %d, body = %s", secondGet.Code, secondGet.Body.String())
	}
}
```

- [ ] **Step 2: Run route-scope test**

Run:

```bash
go test ./internal/platform/httpx/test -run RateLimit
```

Expected: PASS after Task 2.

- [ ] **Step 3: Wire middleware after app auth**

In `internal/platform/http/router.go`, change the open group wiring to:

```go
open := router.Group("/v1/open")
open.Use(httpx.AppAuthMiddleware(httpx.AppAuthOptions{
	Client:              client,
	ReplayStore:         httpx.NewRedisReplayStore(redisClient),
	SecretEncryptionKey: cfg.Auth.AppSecretEncryptionKey,
}))
open.Use(httpx.RateLimitMiddleware(httpx.RateLimitOptions{
	Store:   httpx.NewRedisRateLimitStore(redisClient),
	Enabled: cfg.RateLimit.OpenAPIEnabled,
	Limit:   cfg.RateLimit.OpenAPILimit,
	Window:  cfg.RateLimit.OpenAPIWindow,
	Scope:   "open_api",
}))
```

- [ ] **Step 4: Run package tests**

Run:

```bash
go test ./internal/platform/httpx/test ./internal/platform/http ./cmd/server
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/http/router.go internal/platform/httpx/test/rate_limiter_test.go
git commit -m "Apply rate limiting to open API routes"
```

---

### Task 4: Document Secret Rotation and Leak Response

**Files:**
- Modify: `docs/PAYMENT_GATEWAY_PRD.md`
- Modify: `docs/PAYMENT_GATEWAY_INTEGRATION.md`
- Modify: `README.md`

**Interfaces:**
- Consumes existing behavior:
  - App secret is generated by gateway.
  - Secret is returned only on create/reset.
  - Secret is encrypted at rest in `app_secret_ciphertext`.
  - `POST /v1/admin/apps/:id/reset-secret` resets the secret.

- [ ] **Step 1: Update PRD security section**

Add under the security/authentication section in `docs/PAYMENT_GATEWAY_PRD.md`:

```markdown
### 密钥管理与泄露处理

- `app_secret` 由网关生成，创建应用和重置密钥时仅返回一次。
- 网关保存 `app_secret_hash` 和加密后的 `app_secret_ciphertext`，不得明文展示密钥。
- 加密密钥由 `APP_SECRET_ENCRYPTION_KEY` 提供，生产环境必须使用独立强随机值并纳入密钥管理系统。
- V1 不支持双密钥并行验证；重置密钥会立即使旧密钥失效。
- 业务方怀疑密钥泄露时，必须立即禁用应用或重置密钥，并更新业务系统配置。
- 密钥轮换建议按季度或重大人员/系统变更时执行；V1 轮换流程为：新建维护窗口、重置密钥、业务方更新配置、用 `/v1/open/ping` 验签、恢复业务流量。
```

Add under API security controls:

```markdown
### 请求频率限制

- V1 对开放 API 预留并实现按 `app_id + HTTP method + route` 的固定窗口限流。
- 默认限制为每个应用每个接口每分钟 120 次，可通过环境变量关闭或调整。
- 命中限流返回 HTTP `429` 和错误码 `RATE_LIMITED`。
- 限流状态存储在 Redis；Redis 不可用时返回 `SERVICE_UNAVAILABLE`，避免资金接口在风控组件失效时继续放量。
```

- [ ] **Step 2: Update integration docs**

Add to `docs/PAYMENT_GATEWAY_INTEGRATION.md` near authentication:

```markdown
### 密钥轮换和泄露处理

`app_secret` 只会在应用创建或后台重置密钥时展示一次。业务方应存入自己的密钥管理系统，不应写入代码仓库或前端包。

V1 重置密钥会立即使旧密钥失效。建议流程：

1. 在低峰期由网关管理员重置应用密钥。
2. 业务方更新服务端配置。
3. 业务方调用 `/v1/open/ping` 验证新密钥签名。
4. 验证通过后恢复正常支付请求。

如果怀疑密钥泄露，应先禁用应用或重置密钥，再排查泄露来源。
```

Add near common errors:

```markdown
开放 API 默认按 `app_id + HTTP method + route` 限流。命中限流时返回：

```json
{
  "code": "RATE_LIMITED",
  "message": "rate limit exceeded",
  "data": null,
  "error": {
    "code": "RATE_LIMITED",
    "message": "rate limit exceeded",
    "details": {}
  }
}
```
```

- [ ] **Step 3: Update README**

Add under API or security summary:

```markdown
开放 API 已启用应用级安全控制：签名鉴权、5 分钟时间窗口、`request_id`/`nonce` Redis 去重、IP 白名单、按 `app_id + method + route` 的限流。业务 Webhook 使用 `X-Pay-Gateway-Timestamp` 和 `X-Pay-Gateway-Signature` 签名，业务方应使用 `app_secret` 验签。

应用密钥由网关生成，只在创建和重置时展示一次。V1 重置密钥会立即使旧密钥失效，业务方应在维护窗口内完成配置更新和 `/v1/open/ping` 验证。
```

- [ ] **Step 4: Review docs for forbidden changes**

Run:

```bash
rg -n "X-Gateway-Signature|timestamp \\+ body|双密钥|dual" docs/PAYMENT_GATEWAY_PRD.md docs/PAYMENT_GATEWAY_INTEGRATION.md README.md
```

Expected:
- No `X-Gateway-Signature`.
- No `timestamp + body` wording without the dot separator.
- No promise of dual-key support.

- [ ] **Step 5: Commit**

```bash
git add docs/PAYMENT_GATEWAY_PRD.md docs/PAYMENT_GATEWAY_INTEGRATION.md README.md
git commit -m "Document remaining payment security controls"
```

---

### Task 5: Final Verification, Build, and Restart

**Files:**
- No new files expected.

**Interfaces:**
- Consumes all prior tasks.
- Produces verified backend process running from `.tmp/payment-gateway-server`.

- [ ] **Step 1: Run all Go tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Build latest backend**

Run:

```bash
mkdir -p .tmp
go build -o .tmp/payment-gateway-server ./cmd/server
```

Expected: exit 0.

- [ ] **Step 3: Restart backend from latest binary**

Run:

```bash
if [ -f .tmp/payment-gateway-server.pid ]; then kill "$(cat .tmp/payment-gateway-server.pid)" 2>/dev/null || true; fi
sleep 1
if ss -ltnp 'sport = :8080' | rg -q ':8080'; then pkill -x payment-gateway-server 2>/dev/null || true; fi
set -a
. ./.env
set +a
setsid ./.tmp/payment-gateway-server > .tmp/payment-gateway-server.log 2>&1 < /dev/null &
echo $! > .tmp/payment-gateway-server.pid
sleep 1
curl -sS -i http://127.0.0.1:8080/healthz | sed -n '1,12p'
```

Expected: HTTP `200 OK`.

- [ ] **Step 4: Check working tree**

Run:

```bash
git status --short --branch
```

Expected: only intentional changes remain.

- [ ] **Step 5: Commit final verification marker if needed**

If Task 5 required code/document fixes, commit them:

```bash
git add <changed-files>
git commit -m "Verify security hardening gaps"
```

---

## Self-Review

- Spec coverage:
  - Replay attack protection: kept current implementation, explicitly documented as non-goal for changes.
  - Webhook signature: kept current `X-Pay-Gateway-*` headers and dot-separated signature rule, explicitly documented as non-goal for changes.
  - Secret management: Task 4 documents generation, encrypted storage, rotation, and leak response; current reset-secret implementation remains the V1 mechanism.
  - Rate limiting: Tasks 1-3 add config, middleware, Redis backing, route wiring, and tests.
- Placeholder scan:
  - No `TBD`, `TODO`, or unspecified test instructions remain.
- Type consistency:
  - `RateLimitConfig`, `RateLimitOptions`, `RateLimitStore`, and middleware names are defined before use.
