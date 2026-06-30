package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"payment-gateway/ent"
	apphandler "payment-gateway/internal/domain/apps/handler"
	approuter "payment-gateway/internal/domain/apps/router"
	appsvc "payment-gateway/internal/domain/apps/service"
	channelhandler "payment-gateway/internal/domain/channels/handler"
	channelrouter "payment-gateway/internal/domain/channels/router"
	channelsvc "payment-gateway/internal/domain/channels/service"
	orderhandler "payment-gateway/internal/domain/orders/handler"
	orderrouter "payment-gateway/internal/domain/orders/router"
	ordersvc "payment-gateway/internal/domain/orders/service"
	userhandler "payment-gateway/internal/domain/users/handler"
	userrouter "payment-gateway/internal/domain/users/router"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/config"
	"payment-gateway/internal/platform/httpx"
	"payment-gateway/internal/platform/rbac"
)

func NewRouter(client *ent.Client, redisClient *redis.Client, cfg config.Config) http.Handler {
	router := NewBaseRouter()

	enforcer, err := rbac.NewEnforcer()
	if err != nil {
		panic(err)
	}
	userService := usersvc.New(client, cfg.Auth)
	userHandler := userhandler.New(userService, cfg.Auth)
	userrouter.Register(router.Group("/v1/admin"), userHandler, userService, enforcer)
	appService := appsvc.New(client, appsvc.WithSecretEncryptionKey(cfg.Auth.AppSecretEncryptionKey))
	appHandler := apphandler.New(appService)
	approuter.Register(router.Group("/v1/admin"), appHandler, userService, enforcer)
	channelService := channelsvc.New(client)
	channelHandler := channelhandler.New(channelService)
	channelrouter.Register(router.Group("/v1/admin"), channelHandler, userService, enforcer)
	orderService := ordersvc.New(client)
	orderHandler := orderhandler.New(orderService)
	orderrouter.Register(router.Group("/v1/admin"), orderHandler, userService, enforcer)

	open := router.Group("/v1/open")
	open.Use(httpx.AppAuthMiddleware(httpx.AppAuthOptions{
		Client:              client,
		ReplayStore:         httpx.NewRedisReplayStore(redisClient),
		SecretEncryptionKey: cfg.Auth.AppSecretEncryptionKey,
	}))
	open.GET("/ping", func(ctx *gin.Context) {
		httpx.JSONOK(ctx, http.StatusOK, gin.H{
			"message":    "pong",
			"app_id":     ctx.GetString(httpx.ContextAppID),
			"request_id": ctx.GetString(httpx.ContextRequestID),
		})
	})

	return router
}

func NewBaseRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(ctx *gin.Context) {
		httpx.JSONOK(ctx, http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := router.Group("/v1")
	v1.GET("/ping", func(ctx *gin.Context) {
		httpx.JSONOK(ctx, http.StatusOK, gin.H{"message": "pong"})
	})

	return router
}
