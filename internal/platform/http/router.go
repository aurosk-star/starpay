package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"payment-gateway/ent"
	apphandler "payment-gateway/internal/domain/apps/handler"
	approuter "payment-gateway/internal/domain/apps/router"
	appsvc "payment-gateway/internal/domain/apps/service"
	channelhandler "payment-gateway/internal/domain/channels/handler"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	channelrouter "payment-gateway/internal/domain/channels/router"
	channelsvc "payment-gateway/internal/domain/channels/service"
	confighandler "payment-gateway/internal/domain/configs/handler"
	configrouter "payment-gateway/internal/domain/configs/router"
	configsvc "payment-gateway/internal/domain/configs/service"
	orderhandler "payment-gateway/internal/domain/orders/handler"
	orderrouter "payment-gateway/internal/domain/orders/router"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymenthandler "payment-gateway/internal/domain/payments/handler"
	alipayprovider "payment-gateway/internal/domain/payments/provider/alipay"
	paypalprovider "payment-gateway/internal/domain/payments/provider/paypal"
	paymentrouter "payment-gateway/internal/domain/payments/router"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	userhandler "payment-gateway/internal/domain/users/handler"
	userrouter "payment-gateway/internal/domain/users/router"
	usersvc "payment-gateway/internal/domain/users/service"
	webhookhandler "payment-gateway/internal/domain/webhooks/handler"
	webhookrouter "payment-gateway/internal/domain/webhooks/router"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
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
	configService := configsvc.New(client)
	configHandler := confighandler.New(configService)
	configrouter.Register(router.Group("/v1/admin"), configHandler, userService, enforcer)
	webhookService := webhooksvc.New(client,
		webhooksvc.WithRedis(redisClient),
		webhooksvc.WithSecretEncryptionKey(cfg.Auth.AppSecretEncryptionKey),
	)
	webhookHandler := webhookhandler.New(webhookService)
	webhookrouter.Register(router.Group("/v1/admin"), webhookHandler, userService, enforcer)
	orderService := ordersvc.New(client,
		ordersvc.WithWebhookService(webhookService),
		ordersvc.WithDefaultOrderTTL(cfg.Orders.DefaultTTL),
	)
	orderHandler := orderhandler.New(orderService)
	orderrouter.Register(router.Group("/v1/admin"), orderHandler, userService, enforcer)
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(client)),
		paymentsvc.WithProvider(alipayprovider.New()),
		paymentsvc.WithProvider(paypalprovider.New()),
	)
	paymentrouter.RegisterNotify(router.Group("/v1/channel"), paymenthandler.NewNotify(paymentService, orderService))
	orderrouter.RegisterCheckout(router.Group("/v1/checkout"), orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithPaymentService(paymentService),
		orderhandler.WithNotifyURLResolver(func(ctx *gin.Context) string {
			gatewayConfig, err := configService.GetGatewayConfig(ctx.Request.Context())
			if err != nil {
				return ""
			}
			return strings.TrimRight(gatewayConfig.GatewayBaseURL, "/") + gatewayConfig.PaymentNotifyPath
		}),
		orderhandler.WithPaypalReturnURLResolver(func(ctx *gin.Context, gatewayOrderNo string) string {
			gatewayConfig, err := configService.GetGatewayConfig(ctx.Request.Context())
			if err != nil {
				return ""
			}
			return strings.TrimRight(gatewayConfig.GatewayBaseURL, "/") + "/v1/checkout/paypal/return?gateway_order_no=" + gatewayOrderNo
		}),
	))

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
	orderrouter.RegisterOpen(open, orderhandler.NewOpen(orderService))

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
	webhookrouter.RegisterTest(v1, webhookhandler.NewTestReceiver())

	return router
}
