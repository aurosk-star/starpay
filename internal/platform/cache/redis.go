package cache

import (
	"github.com/redis/go-redis/v9"

	"payment-gateway/internal/platform/config"
)

func New(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}
