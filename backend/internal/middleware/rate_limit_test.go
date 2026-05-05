package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func TestRateLimiterUsesMemoryFallbackWhenRedisFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	redisClient := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
		MaxRetries:   0,
	})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	limiter := NewRateLimiter(true, redisClient)
	engine := gin.New()
	engine.GET("/limited",
		func(c *gin.Context) {
			c.Set(userIDContextKey, uint64(7))
			c.Next()
		},
		limiter.LimitUser("test_image_create", 2, time.Minute),
		func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		},
	)

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		engine.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d expected HTTP 200, got %d body=%s", i+1, recorder.Code, recorder.Body.String())
		}
		if recorder.Header().Get("X-RateLimit-Error") == "" {
			t.Fatalf("request %d expected Redis error header", i+1)
		}
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("third request expected HTTP 429, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
