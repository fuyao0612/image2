package cache

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	emailCodeLength       = 4
	defaultCodeTTL        = 5 * time.Minute
	defaultCooldown       = time.Minute
	defaultSendWindow     = time.Hour
	defaultMaxSends       = 5
	defaultMaxVerifyTries = 5
)

var (
	ErrEmailCodeCooldown         = errors.New("验证码发送过于频繁")
	ErrEmailCodeRateLimited      = errors.New("验证码发送次数过多")
	ErrEmailCodeAttemptsExceeded = errors.New("验证码错误次数过多")
	ErrEmailCodeExpired          = errors.New("验证码已过期")
)

// EmailCodePolicy 定义邮箱验证码的有效期、发送频率和校验次数限制。
type EmailCodePolicy struct {
	TTL               time.Duration
	Cooldown          time.Duration
	SendWindow        time.Duration
	MaxSendsPerWindow int
	MaxVerifyAttempts int
}

func (p EmailCodePolicy) normalized() EmailCodePolicy {
	if p.TTL <= 0 {
		p.TTL = defaultCodeTTL
	}
	if p.Cooldown <= 0 {
		p.Cooldown = defaultCooldown
	}
	if p.SendWindow <= 0 {
		p.SendWindow = defaultSendWindow
	}
	if p.MaxSendsPerWindow <= 0 {
		p.MaxSendsPerWindow = defaultMaxSends
	}
	if p.MaxVerifyAttempts <= 0 {
		p.MaxVerifyAttempts = defaultMaxVerifyTries
	}
	return p
}

// EmailCodeStore 负责保存和校验邮箱验证码。
type EmailCodeStore interface {
	Save(ctx context.Context, email string, scene string, policy EmailCodePolicy) (string, error)
	Verify(ctx context.Context, email string, scene string, code string, policy EmailCodePolicy) (bool, error)
	Delete(ctx context.Context, email string, scene string) error
}

type memoryEmailCodeStore struct {
	mu        sync.Mutex
	codes     map[string]emailCodeRecord
	cooldowns map[string]time.Time
	sends     map[string]memoryCounter
}

type emailCodeRecord struct {
	CodeHash  []byte    `json:"codeHash"`
	Attempts  int       `json:"attempts"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

type memoryCounter struct {
	Count     int
	ExpiresAt time.Time
}

type redisEmailCodeStore struct {
	client *redis.Client
}

// NewEmailCodeStore 根据 Redis 是否可用创建验证码存储。
func NewEmailCodeStore(client *redis.Client) EmailCodeStore {
	if client == nil {
		return &memoryEmailCodeStore{
			codes:     map[string]emailCodeRecord{},
			cooldowns: map[string]time.Time{},
			sends:     map[string]memoryCounter{},
		}
	}
	return &redisEmailCodeStore{client: client}
}

func (s *memoryEmailCodeStore) Save(ctx context.Context, email string, scene string, policy EmailCodePolicy) (string, error) {
	_ = ctx
	policy = policy.normalized()

	code, err := randomCode(emailCodeLength)
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	now := time.Now()
	key := keyFor(email, scene)

	s.mu.Lock()
	defer s.mu.Unlock()

	if until := s.cooldowns[key]; until.After(now) {
		return "", ErrEmailCodeCooldown
	}

	counter := s.sends[key]
	if counter.ExpiresAt.IsZero() || now.After(counter.ExpiresAt) {
		counter = memoryCounter{ExpiresAt: now.Add(policy.SendWindow)}
	}
	if counter.Count >= policy.MaxSendsPerWindow {
		s.sends[key] = counter
		return "", ErrEmailCodeRateLimited
	}
	counter.Count++
	s.sends[key] = counter

	s.cooldowns[key] = now.Add(policy.Cooldown)
	s.codes[key] = emailCodeRecord{
		CodeHash:  hash,
		Attempts:  0,
		ExpiresAt: now.Add(policy.TTL),
	}
	return code, nil
}

func (s *memoryEmailCodeStore) Verify(ctx context.Context, email string, scene string, code string, policy EmailCodePolicy) (bool, error) {
	_ = ctx
	policy = policy.normalized()

	key := keyFor(email, scene)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.codes[key]
	if !ok {
		return false, nil
	}
	if now.After(item.ExpiresAt) {
		delete(s.codes, key)
		return false, ErrEmailCodeExpired
	}
	if bcrypt.CompareHashAndPassword(item.CodeHash, []byte(strings.TrimSpace(code))) != nil {
		item.Attempts++
		if item.Attempts >= policy.MaxVerifyAttempts {
			delete(s.codes, key)
			return false, ErrEmailCodeAttemptsExceeded
		}
		s.codes[key] = item
		return false, nil
	}

	delete(s.codes, key)
	return true, nil
}

func (s *memoryEmailCodeStore) Delete(ctx context.Context, email string, scene string) error {
	_ = ctx
	key := keyFor(email, scene)

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.codes, key)
	delete(s.cooldowns, key)
	return nil
}

func (s *redisEmailCodeStore) Save(ctx context.Context, email string, scene string, policy EmailCodePolicy) (string, error) {
	policy = policy.normalized()
	key := keyFor(email, scene)

	if policy.Cooldown > 0 {
		acquired, err := s.client.SetNX(ctx, cooldownKey(key), "1", policy.Cooldown).Result()
		if err != nil {
			return "", err
		}
		if !acquired {
			return "", ErrEmailCodeCooldown
		}
	}

	if policy.MaxSendsPerWindow > 0 {
		count, err := s.incrementSendCounter(ctx, key, policy.SendWindow)
		if err != nil {
			return "", err
		}
		if count > int64(policy.MaxSendsPerWindow) {
			return "", ErrEmailCodeRateLimited
		}
	}

	code, err := randomCode(emailCodeLength)
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	record := emailCodeRecord{CodeHash: hash, Attempts: 0}
	payload, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	if err := s.client.Set(ctx, key, payload, policy.TTL).Err(); err != nil {
		return "", err
	}
	return code, nil
}

func (s *redisEmailCodeStore) Verify(ctx context.Context, email string, scene string, code string, policy EmailCodePolicy) (bool, error) {
	policy = policy.normalized()
	key := keyFor(email, scene)

	stored, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var record emailCodeRecord
	if err := json.Unmarshal(stored, &record); err != nil {
		_ = s.client.Del(ctx, key).Err()
		return false, nil
	}

	if bcrypt.CompareHashAndPassword(record.CodeHash, []byte(strings.TrimSpace(code))) != nil {
		record.Attempts++
		if record.Attempts >= policy.MaxVerifyAttempts {
			_ = s.client.Del(ctx, key).Err()
			return false, ErrEmailCodeAttemptsExceeded
		}
		return false, s.saveRedisRecordWithRemainingTTL(ctx, key, record)
	}

	_ = s.client.Del(ctx, key).Err()
	return true, nil
}

func (s *redisEmailCodeStore) Delete(ctx context.Context, email string, scene string) error {
	key := keyFor(email, scene)
	return s.client.Del(ctx, key, cooldownKey(key)).Err()
}

func (s *redisEmailCodeStore) incrementSendCounter(ctx context.Context, key string, window time.Duration) (int64, error) {
	counterKey := sendCounterKey(key)
	pipe := s.client.TxPipeline()
	countCmd := pipe.Incr(ctx, counterKey)
	ttlCmd := pipe.TTL(ctx, counterKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}

	count := countCmd.Val()
	if count == 1 || ttlCmd.Val() < 0 {
		if err := s.client.Expire(ctx, counterKey, window).Err(); err != nil {
			return count, err
		}
	}
	return count, nil
}

func (s *redisEmailCodeStore) saveRedisRecordWithRemainingTTL(ctx context.Context, key string, record emailCodeRecord) error {
	ttl, err := s.client.TTL(ctx, key).Result()
	if err != nil {
		return err
	}
	if ttl <= 0 {
		_ = s.client.Del(ctx, key).Err()
		return nil
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, payload, ttl).Err()
}

func keyFor(email string, scene string) string {
	return fmt.Sprintf("email_code:%s:%s", strings.ToLower(strings.TrimSpace(email)), strings.ToLower(strings.TrimSpace(scene)))
}

func cooldownKey(key string) string {
	return key + ":cooldown"
}

func sendCounterKey(key string) string {
	return key + ":send_count"
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
