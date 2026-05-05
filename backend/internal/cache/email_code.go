package cache

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// EmailCodeStore 负责保存和校验邮箱验证码。
type EmailCodeStore interface {
	Save(ctx context.Context, email string, scene string, ttl time.Duration) (string, error)
	Verify(ctx context.Context, email string, scene string, code string) (bool, error)
}

type memoryEmailCodeStore struct {
	mu    sync.Mutex
	codes map[string]memoryEmailCode
}

type memoryEmailCode struct {
	Code      string
	ExpiresAt time.Time
}

type redisEmailCodeStore struct {
	client *redis.Client
}

// NewEmailCodeStore 根据 Redis 是否可用创建验证码存储。
func NewEmailCodeStore(client *redis.Client) EmailCodeStore {
	if client == nil {
		return &memoryEmailCodeStore{
			codes: map[string]memoryEmailCode{},
		}
	}
	return &redisEmailCodeStore{client: client}
}

func (s *memoryEmailCodeStore) Save(ctx context.Context, email string, scene string, ttl time.Duration) (string, error) {
	_ = ctx
	code, err := randomCode(6)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[keyFor(email, scene)] = memoryEmailCode{
		Code:      code,
		ExpiresAt: time.Now().Add(ttl),
	}
	return code, nil
}

func (s *memoryEmailCodeStore) Verify(ctx context.Context, email string, scene string, code string) (bool, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	key := keyFor(email, scene)
	item, ok := s.codes[key]
	if !ok {
		return false, nil
	}
	if time.Now().After(item.ExpiresAt) {
		delete(s.codes, key)
		return false, nil
	}
	if strings.TrimSpace(code) != item.Code {
		return false, nil
	}
	delete(s.codes, key)
	return true, nil
}

func (s *redisEmailCodeStore) Save(ctx context.Context, email string, scene string, ttl time.Duration) (string, error) {
	code, err := randomCode(6)
	if err != nil {
		return "", err
	}
	if err := s.client.Set(ctx, keyFor(email, scene), code, ttl).Err(); err != nil {
		return "", err
	}
	return code, nil
}

func (s *redisEmailCodeStore) Verify(ctx context.Context, email string, scene string, code string) (bool, error) {
	key := keyFor(email, scene)
	stored, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if stored != strings.TrimSpace(code) {
		return false, nil
	}
	_ = s.client.Del(ctx, key).Err()
	return true, nil
}

func keyFor(email string, scene string) string {
	return fmt.Sprintf("email_code:%s:%s", strings.ToLower(strings.TrimSpace(email)), strings.ToLower(strings.TrimSpace(scene)))
}

func randomCode(length int) (string, error) {
	max := big.NewInt(10)
	var builder strings.Builder
	builder.Grow(length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + n.Int64()))
	}
	return builder.String(), nil
}
