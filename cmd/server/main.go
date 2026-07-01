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

	ordersvc "payment-gateway/internal/domain/orders/service"
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

	orderService := ordersvc.New(db,
		ordersvc.WithWebhookService(webhookService),
		ordersvc.WithDefaultOrderTTL(cfg.Orders.DefaultTTL),
		ordersvc.WithExpirationEnqueuer(ordersvc.NewRedisExpirationEnqueuer(redisClient)),
	)
	orderWorker := ordersvc.NewWorker(orderService, redisClient, cfg.App.Name)
	go orderWorker.RunExpireScanner(ctx, cfg.Orders.ExpireScanInterval, cfg.Orders.ExpireScanLimit)
	orderWorkerConcurrency := cfg.Orders.ExpireWorkerConcurrency
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
