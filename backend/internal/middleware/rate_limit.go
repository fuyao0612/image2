package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter 提供 Redis 优先、内存兜底的固定窗口限流。
type RateLimiter struct {
	enabled bool
	redis   *redis.Client
	mu      sync.Mutex
	items   map[string]memoryCounter
}

type memoryCounter struct {
	Count     int
	ExpiresAt time.Time
}

// NewRateLimiter 创建限流器。
func NewRateLimiter(enabled bool, redisClient *redis.Client) *RateLimiter {
	return &RateLimiter{
		enabled: enabled,
		redis:   redisClient,
		items:   map[string]memoryCounter{},
	}
}

// LimitIP 按客户端 IP 限流。
func (l *RateLimiter) LimitIP(name string, limit int, window time.Duration) gin.HandlerFunc {
	return l.limit(name, limit, window, func(c *gin.Context) string {
		return c.ClientIP()
	})
}

// LimitUser 按登录用户限流，若没有用户 ID 则退回 IP。
func (l *RateLimiter) LimitUser(name string, limit int, window time.Duration) gin.HandlerFunc {
	return l.limit(name, limit, window, func(c *gin.Context) string {
		if userID, ok := CurrentUserID(c); ok && userID > 0 {
			return fmt.Sprintf("user:%d", userID)
		}
		return "ip:" + c.ClientIP()
	})
}

func (l *RateLimiter) limit(name string, limit int, window time.Duration, identity func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if l == nil || !l.enabled || limit <= 0 || window <= 0 {
			c.Next()
			return
		}

		key := fmt.Sprintf("rate_limit:%s:%s", sanitizeKeyPart(name), sanitizeKeyPart(identity(c)))
		allowed, ttl, err := l.allow(c.Request.Context(), key, limit, window)
		if err != nil {
			// Redis 异常时 allow 会退回进程内存计数；响应头保留错误信息方便排查。
			c.Header("X-RateLimit-Error", err.Error())
		}
		if !allowed {
			if ttl > 0 {
				c.Header("Retry-After", fmt.Sprintf("%.0f", ttl.Seconds()))
			}
			response.JSON(c, http.StatusTooManyRequests, response.CodeTooManyRequests, "请求过于频繁，请稍后再试", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

func (l *RateLimiter) allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	if l.redis != nil {
		allowed, ttl, err := l.allowRedis(ctx, key, limit, window)
		if err == nil {
			return allowed, ttl, nil
		}
		fallbackAllowed, fallbackTTL, _ := l.allowMemory("memory_fallback:"+key, limit, window)
		return fallbackAllowed, fallbackTTL, err
	}
	return l.allowMemory(key, limit, window)
}

func (l *RateLimiter) allowRedis(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	pipe := l.redis.TxPipeline()
	countCmd := pipe.Incr(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, 0, err
	}

	count := int(countCmd.Val())
	ttl := ttlCmd.Val()
	if count == 1 || ttl < 0 {
		if err := l.redis.Expire(ctx, key, window).Err(); err != nil {
			return true, 0, err
		}
		ttl = window
	}
	return count <= limit, ttl, nil
}

func (l *RateLimiter) allowMemory(key string, limit int, window time.Duration) (bool, time.Duration, error) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	item := l.items[key]
	if item.ExpiresAt.IsZero() || now.After(item.ExpiresAt) {
		item = memoryCounter{
			Count:     0,
			ExpiresAt: now.Add(window),
		}
	}
	item.Count++
	l.items[key] = item

	if item.Count > limit {
		return false, time.Until(item.ExpiresAt), nil
	}
	return true, time.Until(item.ExpiresAt), nil
}

func sanitizeKeyPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer(" ", "_", "\n", "_", "\r", "_", "\t", "_", ":", "_")
	return replacer.Replace(value)
}
