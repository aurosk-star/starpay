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
	assertRateLimitCode(t, secondGet, httpx.CodeRateLimited)
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
