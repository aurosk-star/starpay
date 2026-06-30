package httpx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"payment-gateway/ent"
	apprepo "payment-gateway/internal/domain/apps/repository"
	platformauth "payment-gateway/internal/platform/auth"
)

type ReplayStore interface {
	SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

type RedisReplayStore struct {
	client *redis.Client
}

func NewRedisReplayStore(client *redis.Client) RedisReplayStore {
	return RedisReplayStore{client: client}
}

func (s RedisReplayStore) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, key, "1", ttl).Result()
}

type AppAuthOptions struct {
	Client              *ent.Client
	AppRepository       apprepo.Repository
	ReplayStore         ReplayStore
	SecretEncryptionKey string
	Window              time.Duration
}

func AppAuthMiddleware(options AppAuthOptions) gin.HandlerFunc {
	window := options.Window
	if window <= 0 {
		window = 5 * time.Minute
	}
	return func(ctx *gin.Context) {
		values, err := requestValues(ctx)
		if err != nil {
			JSONError(ctx, http.StatusBadRequest, "invalid_request", "invalid request parameters")
			ctx.Abort()
			return
		}

		appID := strings.TrimSpace(values.Get("app_id"))
		requestID := strings.TrimSpace(values.Get("request_id"))
		timestamp := strings.TrimSpace(values.Get("timestamp"))
		nonce := strings.TrimSpace(values.Get("nonce"))
		signature := strings.TrimSpace(values.Get("sign"))
		if appID == "" || requestID == "" || timestamp == "" || nonce == "" || signature == "" {
			JSONError(ctx, http.StatusUnauthorized, "invalid_signature", "missing signature parameters")
			ctx.Abort()
			return
		}

		signedAt, err := parseAppTimestamp(timestamp)
		if err != nil || time.Since(signedAt) > window || time.Until(signedAt) > window {
			JSONError(ctx, http.StatusUnauthorized, "invalid_signature", "expired signature")
			ctx.Abort()
			return
		}

		repository := options.AppRepository
		if repository.IsZero() {
			repository = apprepo.New(options.Client)
		}
		record, err := repository.FindByAppID(ctx.Request.Context(), appID)
		if err != nil {
			JSONError(ctx, http.StatusUnauthorized, "invalid_signature", "invalid app credentials")
			ctx.Abort()
			return
		}
		if record.Status != "enabled" {
			JSONError(ctx, http.StatusForbidden, "app_disabled", "app is disabled")
			ctx.Abort()
			return
		}
		if !appIPAllowed(record.AllowedIps, ctx.ClientIP()) {
			JSONError(ctx, http.StatusForbidden, "ip_not_allowed", "client ip is not allowed")
			ctx.Abort()
			return
		}
		if record.AppSecretCiphertext == "" {
			JSONError(ctx, http.StatusUnauthorized, "invalid_signature", "app secret must be reset")
			ctx.Abort()
			return
		}

		secret, err := platformauth.DecryptSecret(options.SecretEncryptionKey, record.AppSecretCiphertext)
		if err != nil {
			JSONError(ctx, http.StatusUnauthorized, "invalid_signature", "invalid app credentials")
			ctx.Abort()
			return
		}
		if !validAppSignature(secret, values, signature) {
			JSONError(ctx, http.StatusUnauthorized, "invalid_signature", "invalid signature")
			ctx.Abort()
			return
		}

		if options.ReplayStore != nil {
			if ok := reserveReplayKey(ctx.Request.Context(), options.ReplayStore, "request", appID, requestID, window); !ok {
				JSONError(ctx, http.StatusUnauthorized, "replayed_request", "request_id was already used")
				ctx.Abort()
				return
			}
			if ok := reserveReplayKey(ctx.Request.Context(), options.ReplayStore, "nonce", appID, nonce, window); !ok {
				JSONError(ctx, http.StatusUnauthorized, "replayed_request", "nonce was already used")
				ctx.Abort()
				return
			}
		}

		ctx.Set(ContextAppID, record.AppID)
		ctx.Set(ContextAppDBID, record.ID)
		ctx.Set(ContextRequestID, requestID)
		ctx.Next()
	}
}

func CanonicalAppSignString(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key == "sign" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		items := append([]string(nil), values[key]...)
		sort.Strings(items)
		for _, value := range items {
			parts = append(parts, key+"="+value)
		}
	}
	return strings.Join(parts, "&")
}

func requestValues(ctx *gin.Context) (url.Values, error) {
	values := url.Values{}
	for key, items := range ctx.Request.URL.Query() {
		for _, item := range items {
			values.Add(key, item)
		}
	}
	if ctx.Request.Method == http.MethodPost || ctx.Request.Method == http.MethodPut || ctx.Request.Method == http.MethodPatch {
		contentType := ctx.GetHeader("Content-Type")
		if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") || strings.HasPrefix(contentType, "multipart/form-data") {
			if err := ctx.Request.ParseForm(); err != nil {
				return nil, err
			}
			for key, items := range ctx.Request.PostForm {
				for _, item := range items {
					values.Add(key, item)
				}
			}
		}
	}
	return values, nil
}

func parseAppTimestamp(value string) (time.Time, error) {
	if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func validAppSignature(secret string, values url.Values, provided string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(CanonicalAppSignString(values)))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(provided)))
}

func reserveReplayKey(ctx context.Context, store ReplayStore, kind string, appID string, value string, ttl time.Duration) bool {
	ok, err := store.SetNX(ctx, "appauth:"+kind+":"+appID+":"+value, ttl)
	return err == nil && ok
}

func appIPAllowed(allowed []string, clientIP string) bool {
	if len(allowed) == 0 {
		return true
	}
	parsedClient := net.ParseIP(clientIP)
	if parsedClient == nil {
		return false
	}
	for _, item := range allowed {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, cidr, err := net.ParseCIDR(item); err == nil && cidr.Contains(parsedClient) {
			return true
		}
		if ip := net.ParseIP(item); ip != nil && ip.Equal(parsedClient) {
			return true
		}
	}
	return false
}
