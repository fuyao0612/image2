package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"image2/backend/internal/config"

	"github.com/redis/go-redis/v9"
)

// OpenRedis initializes the Redis client. Empty REDIS_ADDR disables Redis.
func OpenRedis(cfg config.Config) (*redis.Client, error) {
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return nil, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		PoolSize:     cfg.RedisPoolSize,
		MinIdleConns: cfg.RedisMinIdleConns,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connect failed: %w", err)
	}
	if err := configureRedisMemory(ctx, client, cfg); err != nil {
		return nil, err
	}

	return client, nil
}

func configureRedisMemory(ctx context.Context, client *redis.Client, cfg config.Config) error {
	maxMemory := strings.TrimSpace(cfg.RedisMaxMemory)
	if maxMemory != "" {
		if err := client.ConfigSet(ctx, "maxmemory", maxMemory).Err(); err != nil {
			return fmt.Errorf("redis maxmemory config failed: %w", err)
		}
	}

	policy := strings.TrimSpace(cfg.RedisMaxMemoryPolicy)
	if policy != "" {
		if err := client.ConfigSet(ctx, "maxmemory-policy", policy).Err(); err != nil {
			return fmt.Errorf("redis maxmemory-policy config failed: %w", err)
		}
	}

	return nil
}
