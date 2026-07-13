package http

import (
	"context"
	"net/http"
	"net/url"
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
	monitorhandler "payment-gateway/internal/domain/monitoring/handler"
	monitorrouter "payment-gateway/internal/domain/monitoring/router"
	monitorsvc "payment-gateway/internal/domain/monitoring/service"
	orderhandler "payment-gateway/internal/domain/orders/handler"
	orderrouter "payment-gateway/internal/domain/orders/router"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymenthandler "payment-gateway/internal/domain/payments/handler"
	alipayprovider "payment-gateway/internal/domain/payments/provider/alipay"
	paypalprovider "payment-gateway/internal/domain/payments/provider/paypal"
	wechatprovider "payment-gateway/internal/domain/payments/provider/wechat"
	paymentrouter "payment-gateway/internal/domain/payments/router"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	reconciliationhandler "payment-gateway/internal/domain/reconciliations/handler"
	reconciliationrouter "payment-gateway/internal/domain/reconciliations/router"
	reconciliationsvc "payment-gateway/internal/domain/reconciliations/service"
	refundhandler "payment-gateway/internal/domain/refunds/handler"
	refundrouter "payment-gateway/internal/domain/refunds/router"
	refundsvc "payment-gateway/internal/domain/refunds/service"
	routinghandler "payment-gateway/internal/domain/routing/handler"
	routingrouter "payment-gateway/internal/domain/routing/router"
	routingsvc "payment-gateway/internal/domain/routing/service"
	userhandler "payment-gateway/internal/domain/users/handler"
	userrouter "payment-gateway/internal/domain/users/router"
	usersvc "payment-gateway/internal/domain/users/service"
	webhookhandler "payment-gateway/internal/domain/webhooks/handler"
	webhookrouter "payment-gateway/internal/domain/webhooks/router"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
	"payment-gateway/internal/platform/config"
	"payment-gateway/internal/platform/httpx"
	"payment-gateway/internal/platform/rbac"
	"payment-gateway/internal/platform/webui"
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
	monitorService := monitorsvc.New(
		monitorsvc.WithEntClient(client),
		monitorsvc.WithRedis(redisClient),
		monitorsvc.WithStreams([]monitorsvc.StreamTarget{
			{Name: "orders", Stream: ordersvc.OrderExpirationStreamName(), Group: ordersvc.OrderExpirationWorkerGroup()},
			{Name: "webhooks", Stream: webhooksvc.WebhookStreamName(), Group: webhooksvc.WebhookWorkerGroup()},
			{Name: "reconciliations", Stream: reconciliationsvc.ReconciliationStreamName(), Group: reconciliationsvc.ReconciliationWorkerGroup()},
			{Name: "refunds", Stream: refundsvc.RefundStreamName(), Group: refundsvc.RefundWorkerGroup()},
		}),
	)
	monitorrouter.Register(router.Group("/v1/admin"), monitorhandler.New(monitorService), userService, enforcer)
	appService := appsvc.New(client, appsvc.WithSecretEncryptionKey(cfg.Auth.AppSecretEncryptionKey))
	appHandler := apphandler.New(appService)
	approuter.Register(router.Group("/v1/admin"), appHandler, userService, enforcer)
	channelService := channelsvc.New(client)
	channelHandler := channelhandler.New(channelService)
	channelrouter.Register(router.Group("/v1/admin"), channelHandler, userService, enforcer)
	routingService := routingsvc.New(client)
	routingrouter.Register(router.Group("/v1/admin"), routinghandler.New(routingService), userService, enforcer)
	configService := configsvc.New(client, configsvc.WithRuntimeDefaults(configsvc.RuntimeDefaults{
		OrderDefaultTTL:              cfg.Orders.DefaultTTL,
		OrderExpireScanInterval:      cfg.Orders.ExpireScanInterval,
		OrderExpireScanLimit:         cfg.Orders.ExpireScanLimit,
		OrderExpireWorkerConcurrency: cfg.Orders.ExpireWorkerConcurrency,
		OpenAPIRateLimitEnabled:      cfg.RateLimit.OpenAPIEnabled,
		OpenAPIRateLimit:             cfg.RateLimit.OpenAPILimit,
		OpenAPIRateLimitWindow:       cfg.RateLimit.OpenAPIWindow,
	}))
	configHandler := confighandler.New(configService)
	configrouter.Register(router.Group("/v1/admin"), configHandler, userService, enforcer)
	router.GET("/v1/public/site-config", configHandler.GetPublicSiteConfig)
	webhookService := webhooksvc.New(client,
		webhooksvc.WithRedis(redisClient),
		webhooksvc.WithSecretEncryptionKey(cfg.Auth.AppSecretEncryptionKey),
	)
	webhookHandler := webhookhandler.New(webhookService)
	webhookrouter.Register(router.Group("/v1/admin"), webhookHandler, userService, enforcer)
	orderService := ordersvc.New(client,
		ordersvc.WithWebhookService(webhookService),
		ordersvc.WithDefaultOrderTTL(cfg.Orders.DefaultTTL),
		ordersvc.WithDefaultOrderTTLResolver(configService.OrderDefaultTTL),
	)
	checkoutURLResolver := func(ctx *gin.Context, gatewayOrderNo string, token string) string {
		gatewayConfig, err := configService.GetGatewayConfig(ctx.Request.Context())
		if err != nil {
			return ""
		}
		return strings.TrimRight(gatewayConfig.GatewayBaseURL, "/") + "/checkout/" + url.PathEscape(gatewayOrderNo) + "?token=" + url.QueryEscape(token)
	}
	orderHandler := orderhandler.New(orderService, orderhandler.WithAdminCheckoutURLResolver(checkoutURLResolver))
	orderrouter.Register(router.Group("/v1/admin"), orderHandler, userService, enforcer)
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(client)),
		paymentsvc.WithProvider(alipayprovider.New()),
		paymentsvc.WithProvider(paypalprovider.New()),
		paymentsvc.WithProvider(wechatprovider.New()),
	)
	reconciliationService := reconciliationsvc.New(client,
		reconciliationsvc.WithPaymentGateway(paymentService),
		reconciliationsvc.WithOrderService(orderService),
		reconciliationsvc.WithEnqueuer(reconciliationsvc.NewRedisEnqueuer(redisClient)),
	)
	reconciliationrouter.Register(router.Group("/v1/admin"), reconciliationhandler.New(reconciliationService), userService, enforcer)
	refundService := refundsvc.New(client, refundsvc.WithDialect(cfg.Database.Driver), refundsvc.WithPaymentGateway(paymentService), refundsvc.WithWebhookService(webhookService), refundsvc.WithEnqueuer(refundsvc.NewRedisEnqueuer(redisClient)))
	refundrouter.Register(router.Group("/v1/admin"), refundhandler.NewAdmin(refundService), userService, enforcer)
	paymentrouter.RegisterNotify(router.Group("/v1/channel"), paymenthandler.NewNotify(paymentService, orderService))
	orderrouter.RegisterCheckout(router.Group("/v1/checkout"), orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithRoutingService(routingService),
		orderhandler.WithPaymentService(paymentService),
		orderhandler.WithReconciliationScheduler(reconciliationService),
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
		orderhandler.WithResultURLResolver(func(ctx *gin.Context, gatewayOrderNo string, token string) string {
			gatewayConfig, err := configService.GetGatewayConfig(ctx.Request.Context())
			if err != nil {
				return ""
			}
			target := strings.TrimRight(gatewayConfig.GatewayBaseURL, "/") + "/checkout/" + url.PathEscape(gatewayOrderNo) + "/result"
			if strings.TrimSpace(token) == "" {
				return target
			}
			return target + "?token=" + url.QueryEscape(token)
		}),
	))

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
		Resolver: func(ctx context.Context) (httpx.RateLimitRuntimeConfig, error) {
			enabled, limit, window, err := configService.OpenAPIRateLimitConfig(ctx)
			return httpx.RateLimitRuntimeConfig{
				Enabled: enabled,
				Limit:   limit,
				Window:  window,
			}, err
		},
	}))
	open.GET("/ping", func(ctx *gin.Context) {
		httpx.JSONOK(ctx, http.StatusOK, gin.H{
			"message":    "pong",
			"app_id":     ctx.GetString(httpx.ContextAppID),
			"request_id": ctx.GetString(httpx.ContextRequestID),
		})
	})
	orderrouter.RegisterOpen(open, orderhandler.NewOpen(orderService, orderhandler.WithCheckoutURLResolver(checkoutURLResolver)))
	refundrouter.RegisterOpen(open, refundhandler.NewOpen(refundService))
	assets, err := webui.Assets()
	if err != nil {
		panic(err)
	}
	if err := webui.Register(router, assets); err != nil {
		panic(err)
	}

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
