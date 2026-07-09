# API Error Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Provide stable, documented API error responses with structured details and domain-specific error codes for merchant integration.

**Architecture:** Keep the existing global response envelope `{code,message,data,error}` for compatibility, but upgrade `error` to include `details`. Add a central error catalog and mapper so handlers stop returning generic codes like `create_order_failed` for known domain errors.

**Tech Stack:** Go, Gin, existing `internal/platform/httpx` response helpers, existing domain service sentinel errors, Markdown docs.

## Global Constraints

- Do not break the existing top-level response envelope.
- Error codes must use stable uppercase snake case.
- Do not expose secrets, certificates, signatures, app secrets, channel private keys, or raw provider responses in `details`.
- Add tests with every behavior change.
- Use `httpx.JSONOK`, `httpx.JSONError`, and related helpers; do not hand-roll JSON envelopes in business handlers.
- Backend changes must be followed by `go test ./...`, `go build -o .tmp/payment-gateway-server ./cmd/server`, and backend restart before handoff.

---

## File Structure

- Modify `internal/platform/httpx/context.go`
  - Add `ErrorDetail` / details support while keeping existing `JSONError(ctx, status, code, message)` callers working.
- Create `internal/platform/httpx/errors.go`
  - Own stable error code constants, `APIError`, and helper constructors.
- Create `internal/platform/httpx/error_mapper.go`
  - Map known domain errors to stable public error codes and HTTP status.
- Modify `internal/platform/httpx/test/response_test.go`
  - Verify `error.details` defaults to `{}` and custom details serialize correctly.
- Modify `internal/domain/orders/handler/open_handler.go`
  - Use mapper for merchant-facing order create/query/close errors.
- Modify `internal/domain/orders/test/open_handler_test.go`
  - Add integration tests for stable merchant API error codes.
- Modify `internal/domain/orders/handler/checkout_handler.go`
  - Map known checkout/payment errors to stable codes.
- Modify `internal/domain/orders/test/checkout_payment_test.go`
  - Assert stable checkout/payment error codes for currency and channel failures.
- Modify `internal/platform/httpx/app_auth_middleware.go`
  - Replace lower snake auth codes with stable constants.
- Modify `internal/platform/httpx/test/app_auth_middleware_test.go`
  - Assert auth error codes.
- Modify `docs/PAYMENT_GATEWAY_INTEGRATION.md`
  - Add full error response format and error code table.
- Modify `README.md`
  - Update short API response section to mention `error.details`.

---

### Task 1: Extend `httpx` Error Response Details

**Files:**
- Modify: `internal/platform/httpx/context.go`
- Create: `internal/platform/httpx/errors.go`
- Test: `internal/platform/httpx/test/response_test.go`

**Interfaces:**
- Consumes: existing `httpx.JSONError(ctx, status, code, message)`.
- Produces:
  - `type ErrorDetails map[string]any`
  - `func JSONErrorWithDetails(ctx *gin.Context, status int, code string, message string, details ErrorDetails)`
  - Existing `JSONError` delegates to `JSONErrorWithDetails` with empty details.

- [ ] **Step 1: Write failing response tests**

Add tests to `internal/platform/httpx/test/response_test.go`:

```go
func TestJSONErrorIncludesEmptyDetails(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/error", func(ctx *gin.Context) {
		httpx.JSONError(ctx, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	router.ServeHTTP(recorder, req)

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	errBody, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %#v, want object", body["error"])
	}
	details, ok := errBody["details"].(map[string]any)
	if !ok || len(details) != 0 {
		t.Fatalf("details = %#v, want empty object", errBody["details"])
	}
}

func TestJSONErrorWithDetailsIncludesStructuredDetails(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/error", func(ctx *gin.Context) {
		httpx.JSONErrorWithDetails(ctx, http.StatusBadGateway, "CHANNEL_RESPONSE_ERROR", "payment channel failed", httpx.ErrorDetails{
			"channel":             "wechat",
			"provider_error_code": "PARAM_ERROR",
			"retryable":           false,
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	router.ServeHTTP(recorder, req)

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["code"] != "CHANNEL_RESPONSE_ERROR" {
		t.Fatalf("code = %#v", body["code"])
	}
	errBody := body["error"].(map[string]any)
	details := errBody["details"].(map[string]any)
	if details["channel"] != "wechat" || details["provider_error_code"] != "PARAM_ERROR" || details["retryable"] != false {
		t.Fatalf("details = %#v", details)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/platform/httpx/test -run 'TestJSONError'
```

Expected: fail because `details` and `JSONErrorWithDetails` do not exist yet.

- [ ] **Step 3: Implement details support**

Create `internal/platform/httpx/errors.go`:

```go
package httpx

type ErrorDetails map[string]any

const (
	CodeOK                   = "OK"
	CodeInvalidRequest       = "INVALID_REQUEST"
	CodeInvalidSignature     = "INVALID_SIGNATURE"
	CodeTimestampExpired     = "TIMESTAMP_EXPIRED"
	CodeReplayedRequest      = "REPLAYED_REQUEST"
	CodeAppNotFound          = "APP_NOT_FOUND"
	CodeAppDisabled          = "APP_DISABLED"
	CodeIPNotAllowed         = "IP_NOT_ALLOWED"
	CodeMissingRequiredField = "MISSING_REQUIRED_FIELD"
	CodeInvalidAmount        = "INVALID_AMOUNT"
	CodeInvalidCurrency      = "INVALID_CURRENCY"
	CodeCurrencyNotSupported = "CURRENCY_NOT_SUPPORTED"
	CodeInvalidReturnURL     = "INVALID_RETURN_URL"
	CodeInvalidMetadata      = "INVALID_METADATA"
	CodeOrderNotFound        = "ORDER_NOT_FOUND"
	CodeOrderAlreadyPaid     = "ORDER_ALREADY_PAID"
	CodeOrderExpired         = "ORDER_EXPIRED"
	CodeOrderStatusNotAllowed = "ORDER_STATUS_NOT_ALLOWED"
	CodeIdempotencyConflict  = "IDEMPOTENCY_CONFLICT"
	CodeRefundNotFound       = "REFUND_NOT_FOUND"
	CodeRefundAmountExceedsPaid = "REFUND_AMOUNT_EXCEEDS_PAID"
	CodeRefundStatusNotAllowed = "REFUND_STATUS_NOT_ALLOWED"
	CodeChannelUnavailable   = "CHANNEL_UNAVAILABLE"
	CodeChannelConfigInvalid = "CHANNEL_CONFIG_INVALID"
	CodeChannelResponseError = "CHANNEL_RESPONSE_ERROR"
	CodeChannelTimeout       = "CHANNEL_TIMEOUT"
	CodeChannelNotifyVerifyFailed = "CHANNEL_NOTIFY_VERIFY_FAILED"
	CodeInternalError        = "INTERNAL_ERROR"
	CodeServiceUnavailable   = "SERVICE_UNAVAILABLE"
	CodeRateLimited          = "RATE_LIMITED"
)
```

Modify `internal/platform/httpx/context.go`:

```go
func JSONError(ctx *gin.Context, status int, code string, message string) {
	JSONErrorWithDetails(ctx, status, code, message, ErrorDetails{})
}

func JSONErrorWithDetails(ctx *gin.Context, status int, code string, message string, details ErrorDetails) {
	if details == nil {
		details = ErrorDetails{}
	}
	ctx.JSON(status, gin.H{
		"code":    code,
		"message": message,
		"data":    nil,
		"error": gin.H{
			"code":    code,
			"message": message,
			"details": details,
		},
	})
}
```

Keep `JSONOK` unchanged in this task.

- [ ] **Step 4: Run tests to verify pass**

Run:

```bash
go test ./internal/platform/httpx/test -run 'TestJSONError'
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/httpx/context.go internal/platform/httpx/errors.go internal/platform/httpx/test/response_test.go
git commit -m "Add structured API error details"
```

---

### Task 2: Add Domain Error Mapper for Merchant APIs

**Files:**
- Create: `internal/platform/httpx/error_mapper.go`
- Modify: `internal/domain/orders/handler/open_handler.go`
- Test: `internal/domain/orders/test/open_handler_test.go`

**Interfaces:**
- Consumes: sentinel errors from `internal/domain/orders/service`.
- Produces:
  - `type APIError struct { HTTPStatus int; Code string; Message string; Details ErrorDetails }`
  - `func OrderAPIError(err error, fallbackCode string, fallbackMessage string) APIError`
  - `func WriteAPIError(ctx *gin.Context, apiErr APIError)`

- [ ] **Step 1: Write failing open API error tests**

Add tests to `internal/domain/orders/test/open_handler_test.go`:

```go
func TestOpenCreateOrderReturnsStableInvalidAmountCode(t *testing.T) {
	router, _ := newOpenOrderTestRouter(t, "open_error_invalid_amount")
	body := strings.NewReader(`{"merchant_order_no":"biz_invalid_amount","subject":"Pro","amount":0,"currency":"CNY"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/open/orders", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-App-ID", "snsgo")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response["code"] != httpx.CodeInvalidAmount {
		t.Fatalf("code = %#v, want %s", response["code"], httpx.CodeInvalidAmount)
	}
}

func TestOpenCreateOrderReturnsStableCurrencyNotSupportedCode(t *testing.T) {
	router, _ := newOpenOrderTestRouter(t, "open_error_currency_not_supported")
	body := strings.NewReader(`{"merchant_order_no":"biz_currency","subject":"Pro","amount":9900,"currency":"USD","channel":"wechat","pay_method":"wechat"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/open/orders", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-App-ID", "snsgo")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response["code"] != httpx.CodeCurrencyNotSupported {
		t.Fatalf("code = %#v, want %s", response["code"], httpx.CodeCurrencyNotSupported)
	}
}
```

Use the existing test router helper. If it does not set `ContextAppID`, extend only the helper in the test file with a middleware that sets `httpx.ContextAppID` to `snsgo`.

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/domain/orders/test -run 'TestOpenCreateOrderReturnsStable'
```

Expected: fail because current handler returns `create_order_failed`.

- [ ] **Step 3: Implement mapper**

Create `internal/platform/httpx/error_mapper.go`:

```go
package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
)

type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
	Details    ErrorDetails
}

func WriteAPIError(ctx *gin.Context, err APIError) {
	JSONErrorWithDetails(ctx, err.HTTPStatus, err.Code, err.Message, err.Details)
}

func OrderAPIError(err error, fallbackCode string, fallbackMessage string) APIError {
	switch {
	case errors.Is(err, ordersvc.ErrMerchantOrderNoRequired):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeMissingRequiredField, Message: "merchant_order_no is required", Details: ErrorDetails{"field": "merchant_order_no"}}
	case errors.Is(err, ordersvc.ErrSubjectRequired):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeMissingRequiredField, Message: "subject is required", Details: ErrorDetails{"field": "subject"}}
	case errors.Is(err, ordersvc.ErrInvalidAmount):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeInvalidAmount, Message: "amount must be a positive integer in minor units", Details: ErrorDetails{"field": "amount"}}
	case errors.Is(err, ordersvc.ErrInvalidCurrency):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeInvalidCurrency, Message: "currency is invalid", Details: ErrorDetails{"field": "currency"}}
	case errors.Is(err, ordersvc.ErrUnsupportedCurrencyForChannel):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeCurrencyNotSupported, Message: "currency is not supported by channel", Details: ErrorDetails{}}
	case errors.Is(err, ordersvc.ErrIdempotencyConflict):
		return APIError{HTTPStatus: http.StatusConflict, Code: CodeIdempotencyConflict, Message: "merchant order already exists with different parameters", Details: ErrorDetails{}}
	case errors.Is(err, ordersvc.ErrOrderCannotBeClosed):
		return APIError{HTTPStatus: http.StatusConflict, Code: CodeOrderStatusNotAllowed, Message: "order status does not allow this operation", Details: ErrorDetails{}}
	case errors.Is(err, ordersvc.ErrAppNotFound):
		return APIError{HTTPStatus: http.StatusUnauthorized, Code: CodeAppNotFound, Message: "app not found", Details: ErrorDetails{}}
	case errors.Is(err, ordersvc.ErrAppDisabled):
		return APIError{HTTPStatus: http.StatusForbidden, Code: CodeAppDisabled, Message: "app is disabled", Details: ErrorDetails{}}
	case errors.Is(err, paymentsvc.ErrProviderUnavailable):
		return APIError{HTTPStatus: http.StatusServiceUnavailable, Code: CodeChannelUnavailable, Message: "payment channel is unavailable", Details: ErrorDetails{"retryable": true}}
	default:
		return APIError{HTTPStatus: http.StatusInternalServerError, Code: fallbackCode, Message: fallbackMessage, Details: ErrorDetails{}}
	}
}
```

- [ ] **Step 4: Wire open order handler**

Modify `internal/domain/orders/handler/open_handler.go`:

```go
if err != nil {
	apiErr := httpx.OrderAPIError(err, httpx.CodeInternalError, "failed to create order")
	httpx.WriteAPIError(ctx, apiErr)
	return
}
```

For `CloseOrder`, use:

```go
if err != nil {
	if ent.IsNotFound(err) {
		httpx.JSONErrorWithDetails(ctx, http.StatusNotFound, httpx.CodeOrderNotFound, "payment order not found", httpx.ErrorDetails{"gateway_order_no": ctx.Param("gateway_order_no")})
		return
	}
	apiErr := httpx.OrderAPIError(err, httpx.CodeInternalError, "failed to close order")
	httpx.WriteAPIError(ctx, apiErr)
	return
}
```

Import `payment-gateway/ent` only if needed for `ent.IsNotFound`.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/domain/orders/test -run 'TestOpenCreateOrderReturnsStable|TestOpen'
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add internal/platform/httpx/error_mapper.go internal/domain/orders/handler/open_handler.go internal/domain/orders/test/open_handler_test.go
git commit -m "Map merchant order errors to stable codes"
```

---

### Task 3: Stabilize Authentication Error Codes

**Files:**
- Modify: `internal/platform/httpx/app_auth_middleware.go`
- Test: `internal/platform/httpx/test/app_auth_middleware_test.go`
- Modify: `docs/PAYMENT_GATEWAY_INTEGRATION.md`

**Interfaces:**
- Consumes: constants from `internal/platform/httpx/errors.go`.
- Produces stable auth codes: `INVALID_SIGNATURE`, `TIMESTAMP_EXPIRED`, `APP_DISABLED`, `IP_NOT_ALLOWED`, `REPLAYED_REQUEST`.

- [ ] **Step 1: Write failing auth tests**

Add or update tests in `internal/platform/httpx/test/app_auth_middleware_test.go` to assert:

```go
if response["code"] != httpx.CodeInvalidSignature {
	t.Fatalf("code = %#v, want %s", response["code"], httpx.CodeInvalidSignature)
}
```

Cover missing signature parameters, expired timestamp, disabled app, and replayed request.

- [ ] **Step 2: Run auth tests to verify failure**

Run:

```bash
go test ./internal/platform/httpx/test -run 'AppAuth'
```

Expected: fail where old lower snake codes are returned.

- [ ] **Step 3: Replace middleware codes**

Modify `internal/platform/httpx/app_auth_middleware.go`:

```go
JSONError(ctx, http.StatusUnauthorized, CodeInvalidSignature, "missing signature parameters")
JSONError(ctx, http.StatusUnauthorized, CodeTimestampExpired, "timestamp is expired")
JSONError(ctx, http.StatusForbidden, CodeAppDisabled, "app is disabled")
JSONError(ctx, http.StatusForbidden, CodeIPNotAllowed, "client ip is not allowed")
JSONError(ctx, http.StatusUnauthorized, CodeReplayedRequest, "request_id was already used")
```

Do not change the signing algorithm in this task.

- [ ] **Step 4: Run auth tests**

Run:

```bash
go test ./internal/platform/httpx/test -run 'AppAuth'
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/httpx/app_auth_middleware.go internal/platform/httpx/test/app_auth_middleware_test.go
git commit -m "Stabilize app auth error codes"
```

---

### Task 4: Stabilize Checkout and Channel Error Codes

**Files:**
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Test: `internal/domain/orders/test/checkout_payment_test.go`
- Optional Modify: `internal/platform/httpx/error_mapper.go`

**Interfaces:**
- Consumes: `httpx.OrderAPIError`, payment service sentinel errors.
- Produces stable checkout/payment codes for currency, locked payment method, unavailable mode, channel failures.

- [ ] **Step 1: Write failing checkout tests**

Add tests in `internal/domain/orders/test/checkout_payment_test.go` for:

```go
if response["code"] != httpx.CodeCurrencyNotSupported {
	t.Fatalf("code = %#v, want %s", response["code"], httpx.CodeCurrencyNotSupported)
}
```

Also assert provider unavailable returns `CHANNEL_UNAVAILABLE` and payment mode unavailable returns `CHANNEL_UNAVAILABLE` or `ORDER_STATUS_NOT_ALLOWED` according to the PRD table. Use `CHANNEL_UNAVAILABLE` for mode unavailable in this task.

- [ ] **Step 2: Run checkout tests to verify failure**

Run:

```bash
go test ./internal/domain/orders/test -run 'Checkout.*Error|Currency|Unavailable'
```

Expected: fail where current codes are `unsupported_currency_for_channel`, `payment_mode_unavailable`, or `start_payment_failed`.

- [ ] **Step 3: Replace checkout error responses**

In `internal/domain/orders/handler/checkout_handler.go`, replace known error responses:

```go
httpx.JSONError(ctx, http.StatusBadRequest, httpx.CodeCurrencyNotSupported, "currency is not supported by channel")
httpx.JSONError(ctx, http.StatusConflict, httpx.CodeOrderStatusNotAllowed, "payment method is locked by the order")
httpx.JSONError(ctx, http.StatusServiceUnavailable, httpx.CodeChannelUnavailable, "payment method is not available for current terminal")
```

For `StartPayment` errors:

```go
apiErr := httpx.OrderAPIError(err, httpx.CodeChannelResponseError, "failed to start payment")
httpx.WriteAPIError(ctx, apiErr)
```

- [ ] **Step 4: Run checkout tests**

Run:

```bash
go test ./internal/domain/orders/test -run 'Checkout|Currency|Unavailable'
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/orders/handler/checkout_handler.go internal/domain/orders/test/checkout_payment_test.go internal/platform/httpx/error_mapper.go
git commit -m "Stabilize checkout payment error codes"
```

---

### Task 5: Document Error Codes for Integrators

**Files:**
- Modify: `docs/PAYMENT_GATEWAY_INTEGRATION.md`
- Modify: `README.md`
- Test: none, docs only

**Interfaces:**
- Consumes: PRD section `11.2 统一响应与错误模型`.
- Produces: Merchant-facing error response examples and error code table.

- [ ] **Step 1: Update integration response section**

In `docs/PAYMENT_GATEWAY_INTEGRATION.md`, replace the failure example with:

```json
{
  "code": "INVALID_SIGNATURE",
  "message": "invalid signature",
  "data": null,
  "error": {
    "code": "INVALID_SIGNATURE",
    "message": "invalid signature",
    "details": {}
  }
}
```

Add text:

```markdown
`message` 仅用于排障展示，接入方业务逻辑必须依赖 `error.code`。`error.details` 始终存在，默认为 `{}`。
```

- [ ] **Step 2: Replace common error section**

Replace `## 13. 常见错误` with a table containing the same codes from PRD section 11.2, including HTTP status and handling recommendation.

- [ ] **Step 3: Update README response example**

In `README.md`, add `details` under `error` in the failure response example and note stable uppercase error codes.

- [ ] **Step 4: Review docs**

Run:

```bash
rg -n "invalid_signature|create_order_failed|start_payment_failed|unsupported_currency_for_channel" docs/PAYMENT_GATEWAY_INTEGRATION.md README.md
```

Expected: no old lower snake merchant-facing error codes remain in integration-facing docs, except when explicitly marked as legacy.

- [ ] **Step 5: Commit**

```bash
git add docs/PAYMENT_GATEWAY_INTEGRATION.md README.md
git commit -m "Document stable API error codes"
```

---

### Task 6: Final Verification and Backend Restart

**Files:**
- No code changes expected.

**Interfaces:**
- Consumes: all previous tasks.
- Produces: verified backend and clean committed implementation.

- [ ] **Step 1: Run full test suite**

Run:

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Build backend**

Run:

```bash
go build -o .tmp/payment-gateway-server ./cmd/server
```

Expected: exit code 0.

- [ ] **Step 3: Restart local backend**

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

Expected: `HTTP/1.1 200 OK` and JSON body with `"status":"ok"`.

- [ ] **Step 4: Push**

Run:

```bash
git status --short --branch
git push origin main
```

Expected: branch pushed; only intentional unrelated local files, if any, remain unstaged.

---

## Self-Review

- Spec coverage: PRD requires structured error response, `details`, stable error code categories, and at least 20 common codes. Tasks 1-5 implement and document those requirements.
- Placeholder scan: no `TBD`, `TODO`, or "similar to" steps remain.
- Type consistency: `ErrorDetails`, `JSONErrorWithDetails`, `APIError`, `OrderAPIError`, and `WriteAPIError` are defined before use.
