package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"image2/backend/internal/config"

	"github.com/redis/go-redis/v9"
)

// OpenRedis 初始化 Redis 客户端；如果未配置地址，则直接返回 nil。
func OpenRedis(cfg config.Config) (*redis.Client, error) {
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return nil, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis 连接失败: %w", err)
	}

	return client, nil
}
