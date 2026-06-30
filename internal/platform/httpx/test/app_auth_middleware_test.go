package httpxtest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	"payment-gateway/internal/platform/httpx"
)

const appAuthTestKey = "0123456789abcdef0123456789abcdef"

func TestAppAuthMiddlewareAcceptsValidSignedRequest(t *testing.T) {
	router, secret := newAppAuthRouter(t, "enabled", nil)
	values := signedValues(secret, map[string]string{"amount": "100"})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/open/ping?"+values.Encode(), nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestAppAuthMiddlewareRejectsMissingRequestID(t *testing.T) {
	router, secret := newAppAuthRouter(t, "enabled", nil)
	values := signedValues(secret, nil)
	values.Del("request_id")
	values.Set("sign", signValues(secret, values))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/open/ping?"+values.Encode(), nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want unauthorized", recorder.Code)
	}
}

func TestAppAuthMiddlewareRejectsInvalidSignature(t *testing.T) {
	router, secret := newAppAuthRouter(t, "enabled", nil)
	values := signedValues(secret, nil)
	values.Set("sign", "bad-signature")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/open/ping?"+values.Encode(), nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want unauthorized", recorder.Code)
	}
}

func TestAppAuthMiddlewareRejectsExpiredTimestamp(t *testing.T) {
	router, secret := newAppAuthRouter(t, "enabled", nil)
	values := signedValues(secret, nil)
	values.Set("timestamp", time.Now().Add(-10*time.Minute).Format(time.RFC3339))
	values.Set("sign", signValues(secret, values))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/open/ping?"+values.Encode(), nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want unauthorized", recorder.Code)
	}
}

func TestAppAuthMiddlewareRejectsReplayRequestID(t *testing.T) {
	router, secret := newAppAuthRouter(t, "enabled", nil)
	values := signedValues(secret, nil)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/open/ping?"+values.Encode(), nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/open/ping?"+values.Encode(), nil))
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("second status = %d, want unauthorized", second.Code)
	}
}

func TestAppAuthMiddlewareRejectsDisabledApp(t *testing.T) {
	router, secret := newAppAuthRouter(t, "disabled", nil)
	values := signedValues(secret, nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/open/ping?"+values.Encode(), nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want forbidden", recorder.Code)
	}
}

func TestAppAuthMiddlewareRejectsIPMismatch(t *testing.T) {
	router, secret := newAppAuthRouter(t, "enabled", []string{"203.0.113.10"})
	values := signedValues(secret, nil)
	request := httptest.NewRequest(http.MethodPost, "/open/ping?"+values.Encode(), nil)
	request.RemoteAddr = "198.51.100.7:12345"

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want forbidden", recorder.Code)
	}
}

func newAppAuthRouter(t *testing.T, status string, allowedIPs []string) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:app_auth_"+status+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })

	svc := appsvc.New(client, appsvc.WithSecretEncryptionKey(appAuthTestKey))
	created, err := svc.CreateApp(ctx, appsvc.ManageAppInput{
		AppID:      "app_123",
		Name:       "Open API App",
		Status:     status,
		AllowedIPs: allowedIPs,
	})
	if err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}

	router := gin.New()
	router.Use(httpx.AppAuthMiddleware(httpx.AppAuthOptions{
		Client:              client,
		ReplayStore:         newMemoryReplayStore(),
		SecretEncryptionKey: appAuthTestKey,
		Window:              5 * time.Minute,
	}))
	router.POST("/open/ping", func(ctx *gin.Context) {
		httpx.JSONOK(ctx, http.StatusOK, gin.H{
			"app_id":     ctx.GetString(httpx.ContextAppID),
			"request_id": ctx.GetString(httpx.ContextRequestID),
		})
	})
	return router, created.AppSecret
}

func signedValues(secret string, extra map[string]string) url.Values {
	values := url.Values{}
	values.Set("app_id", "app_123")
	values.Set("request_id", "req_123")
	values.Set("timestamp", time.Now().UTC().Format(time.RFC3339))
	values.Set("nonce", "nonce_123")
	for key, value := range extra {
		values.Set(key, value)
	}
	values.Set("sign", signValues(secret, values))
	return values
}

func signValues(secret string, values url.Values) string {
	canonical := httpx.CanonicalAppSignString(values)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

type memoryReplayStore struct {
	mu   sync.Mutex
	keys map[string]struct{}
}

func newMemoryReplayStore() *memoryReplayStore {
	return &memoryReplayStore{keys: map[string]struct{}{}}
}

func (s *memoryReplayStore) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keys[key]; ok {
		return false, nil
	}
	s.keys[key] = struct{}{}
	return true, nil
}
