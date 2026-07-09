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
	Store    RateLimitStore
	Enabled  bool
	Limit    int
	Window   time.Duration
	Scope    string
	Resolver func(context.Context) (RateLimitRuntimeConfig, error)
}

type RateLimitRuntimeConfig struct {
	Enabled bool
	Limit   int
	Window  time.Duration
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
		config := RateLimitRuntimeConfig{
			Enabled: options.Enabled,
			Limit:   options.Limit,
			Window:  window,
		}
		if options.Resolver != nil {
			resolved, err := options.Resolver(ctx.Request.Context())
			if err != nil {
				JSONError(ctx, http.StatusServiceUnavailable, CodeServiceUnavailable, "rate limit config is unavailable")
				ctx.Abort()
				return
			}
			config = resolved
			if config.Window <= 0 {
				config.Window = window
			}
		}
		if !config.Enabled || options.Store == nil || config.Limit <= 0 {
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
		count, err := options.Store.Incr(ctx.Request.Context(), key, config.Window)
		if err != nil {
			JSONError(ctx, http.StatusServiceUnavailable, CodeServiceUnavailable, "rate limit service is unavailable")
			ctx.Abort()
			return
		}
		if count > int64(config.Limit) {
			JSONError(ctx, http.StatusTooManyRequests, CodeRateLimited, "rate limit exceeded")
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}
