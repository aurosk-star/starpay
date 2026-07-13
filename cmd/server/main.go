package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	channelrepo "payment-gateway/internal/domain/channels/repository"
	configsvc "payment-gateway/internal/domain/configs/service"
	ordersvc "payment-gateway/internal/domain/orders/service"
	alipayprovider "payment-gateway/internal/domain/payments/provider/alipay"
	paypalprovider "payment-gateway/internal/domain/payments/provider/paypal"
	wechatprovider "payment-gateway/internal/domain/payments/provider/wechat"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	reconciliationsvc "payment-gateway/internal/domain/reconciliations/service"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
	"payment-gateway/internal/platform/cache"
	"payment-gateway/internal/platform/config"
	"payment-gateway/internal/platform/database"
	httpserver "payment-gateway/internal/platform/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	redisClient := cache.New(cfg.Redis)
	defer redisClient.Close()

	webhookService := webhooksvc.New(db,
		webhooksvc.WithRedis(redisClient),
		webhooksvc.WithSecretEncryptionKey(cfg.Auth.AppSecretEncryptionKey),
	)
	webhookWorker := webhooksvc.NewWorker(webhookService, redisClient, cfg.App.Name)
	go webhookWorker.Run(ctx)
	go webhookWorker.RunRetryScanner(ctx, 30*time.Second, 100)

	configService := configsvc.New(db, configsvc.WithRuntimeDefaults(configsvc.RuntimeDefaults{
		OrderDefaultTTL:              cfg.Orders.DefaultTTL,
		OrderExpireScanInterval:      cfg.Orders.ExpireScanInterval,
		OrderExpireScanLimit:         cfg.Orders.ExpireScanLimit,
		OrderExpireWorkerConcurrency: cfg.Orders.ExpireWorkerConcurrency,
		OpenAPIRateLimitEnabled:      cfg.RateLimit.OpenAPIEnabled,
		OpenAPIRateLimit:             cfg.RateLimit.OpenAPILimit,
		OpenAPIRateLimitWindow:       cfg.RateLimit.OpenAPIWindow,
	}))

	orderService := ordersvc.New(db,
		ordersvc.WithWebhookService(webhookService),
		ordersvc.WithDefaultOrderTTL(cfg.Orders.DefaultTTL),
		ordersvc.WithDefaultOrderTTLResolver(configService.OrderDefaultTTL),
		ordersvc.WithExpirationEnqueuer(ordersvc.NewRedisExpirationEnqueuer(redisClient)),
	)
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(db)),
		paymentsvc.WithProvider(alipayprovider.New()),
		paymentsvc.WithProvider(paypalprovider.New()),
		paymentsvc.WithProvider(wechatprovider.New()),
	)
	reconciliationService := reconciliationsvc.New(db,
		reconciliationsvc.WithPaymentGateway(paymentService),
		reconciliationsvc.WithOrderService(orderService),
		reconciliationsvc.WithEnqueuer(reconciliationsvc.NewRedisEnqueuer(redisClient)),
	)
	reconciliationWorker := reconciliationsvc.NewWorker(reconciliationService, redisClient, cfg.App.Name+"-reconciliation")
	go reconciliationWorker.Run(ctx)
	go reconciliationWorker.RunScanner(ctx, 30*time.Second, 100)
	orderWorker := ordersvc.NewWorker(orderService, redisClient, cfg.App.Name)
	go orderWorker.RunExpireScannerWithResolver(ctx, cfg.Orders.ExpireScanInterval, cfg.Orders.ExpireScanLimit, func(ctx context.Context) (ordersvc.ExpireScannerConfig, error) {
		interval, limit, err := configService.OrderExpireScanConfig(ctx)
		return ordersvc.ExpireScannerConfig{Interval: interval, Limit: limit}, err
	})
	orderWorkerConcurrency := cfg.Orders.ExpireWorkerConcurrency
	if configuredConcurrency, err := configService.OrderExpireWorkerConcurrency(ctx); err == nil && configuredConcurrency > 0 {
		orderWorkerConcurrency = configuredConcurrency
	}
	if orderWorkerConcurrency < 1 {
		orderWorkerConcurrency = 1
	}
	for i := 0; i < orderWorkerConcurrency; i++ {
		consumer := cfg.App.Name + "-order-expiration-" + strconv.Itoa(i+1)
		go ordersvc.NewWorker(orderService, redisClient, consumer).Run(ctx)
	}

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           httpserver.NewRouter(db, redisClient, cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("http server listening", "addr", cfg.HTTP.Addr, "env", cfg.App.Env)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown", "error", err)
	}
}
