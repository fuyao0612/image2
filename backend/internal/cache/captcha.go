package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultCaptchaTTL         = 5 * time.Minute
	defaultMaxCaptchaAttempts = 5
)

var (
	ErrCaptchaInvalid          = errors.New("图形验证码错误或已过期")
	ErrCaptchaAttemptsExceeded = errors.New("图形验证码错误次数过多")
)

type CaptchaPolicy struct {
	TTL         time.Duration
	MaxAttempts int
}

func (p CaptchaPolicy) normalized() CaptchaPolicy {
	if p.TTL <= 0 {
		p.TTL = defaultCaptchaTTL
	}
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = defaultMaxCaptchaAttempts
	}
	return p
}

type CaptchaStore interface {
	Save(ctx context.Context, id string, code string, policy CaptchaPolicy) error
	Verify(ctx context.Context, id string, code string, policy CaptchaPolicy) (bool, error)
}

type memoryCaptchaStore struct {
	mu    sync.Mutex
	items map[string]captchaRecord
}

type redisCaptchaStore struct {
	client *redis.Client
}

type captchaRecord struct {
	Code      string    `json:"code"`
	Attempts  int       `json:"attempts"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

func NewCaptchaStore(client *redis.Client) CaptchaStore {
	if client == nil {
		return &memoryCaptchaStore{items: map[string]captchaRecord{}}
	}
	return &redisCaptchaStore{client: client}
}

func (s *memoryCaptchaStore) Save(ctx context.Context, id string, code string, policy CaptchaPolicy) error {
	_ = ctx
	policy = policy.normalized()
	id = normalizeCaptchaToken(id)
	code = normalizeCaptchaCode(code)
	if id == "" || code == "" {
		return ErrCaptchaInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[captchaKey(id)] = captchaRecord{
		Code:      code,
		Attempts:  0,
		ExpiresAt: time.Now().Add(policy.TTL),
	}
	return nil
}

func (s *memoryCaptchaStore) Verify(ctx context.Context, id string, code string, policy CaptchaPolicy) (bool, error) {
	_ = ctx
	policy = policy.normalized()
	key := captchaKey(normalizeCaptchaToken(id))

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.items[key]
	if !ok || time.Now().After(record.ExpiresAt) {
		delete(s.items, key)
		return false, ErrCaptchaInvalid
	}

	if record.Code != normalizeCaptchaCode(code) {
		record.Attempts++
		if record.Attempts >= policy.MaxAttempts {
			delete(s.items, key)
			return false, ErrCaptchaAttemptsExceeded
		}
		s.items[key] = record
		return false, ErrCaptchaInvalid
	}

	delete(s.items, key)
	return true, nil
}

func (s *redisCaptchaStore) Save(ctx context.Context, id string, code string, policy CaptchaPolicy) error {
	policy = policy.normalized()
	id = normalizeCaptchaToken(id)
	code = normalizeCaptchaCode(code)
	if id == "" || code == "" {
		return ErrCaptchaInvalid
	}

	payload, err := json.Marshal(captchaRecord{Code: code, Attempts: 0})
	if err != nil {
		return err
	}
	return s.client.Set(ctx, captchaKey(id), payload, policy.TTL).Err()
}

func (s *redisCaptchaStore) Verify(ctx context.Context, id string, code string, policy CaptchaPolicy) (bool, error) {
	policy = policy.normalized()
	key := captchaKey(normalizeCaptchaToken(id))

	stored, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, ErrCaptchaInvalid
	}
	if err != nil {
		return false, err
	}

	var record captchaRecord
	if err := json.Unmarshal(stored, &record); err != nil {
		_ = s.client.Del(ctx, key).Err()
		return false, ErrCaptchaInvalid
	}

	if record.Code != normalizeCaptchaCode(code) {
		record.Attempts++
		if record.Attempts >= policy.MaxAttempts {
			_ = s.client.Del(ctx, key).Err()
			return false, ErrCaptchaAttemptsExceeded
		}
		return false, s.saveCaptchaRecordWithRemainingTTL(ctx, key, record)
	}

	_ = s.client.Del(ctx, key).Err()
	return true, nil
}

func (s *redisCaptchaStore) saveCaptchaRecordWithRemainingTTL(ctx context.Context, key string, record captchaRecord) error {
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

func captchaKey(id string) string {
	return fmt.Sprintf("captcha:%s", normalizeCaptchaToken(id))
}

func normalizeCaptchaToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeCaptchaCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
