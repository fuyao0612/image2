package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryCaptchaStoreVerifiesOnce(t *testing.T) {
	store := NewCaptchaStore(nil)
	policy := CaptchaPolicy{TTL: time.Minute, MaxAttempts: 3}

	if err := store.Save(context.Background(), "captcha-1", "A7K9", policy); err != nil {
		t.Fatalf("save captcha: %v", err)
	}

	ok, err := store.Verify(context.Background(), "CAPTCHA-1", "a7k9", policy)
	if err != nil || !ok {
		t.Fatalf("verify captcha: ok=%v err=%v", ok, err)
	}

	ok, err = store.Verify(context.Background(), "captcha-1", "A7K9", policy)
	if !errors.Is(err, ErrCaptchaInvalid) || ok {
		t.Fatalf("captcha should be one-time use, ok=%v err=%v", ok, err)
	}
}

func TestMemoryCaptchaStoreLimitsAttempts(t *testing.T) {
	store := NewCaptchaStore(nil)
	policy := CaptchaPolicy{TTL: time.Minute, MaxAttempts: 2}

	if err := store.Save(context.Background(), "captcha-2", "B8LM", policy); err != nil {
		t.Fatalf("save captcha: %v", err)
	}

	ok, err := store.Verify(context.Background(), "captcha-2", "WRONG", policy)
	if !errors.Is(err, ErrCaptchaInvalid) || ok {
		t.Fatalf("expected first wrong attempt to be invalid, ok=%v err=%v", ok, err)
	}

	ok, err = store.Verify(context.Background(), "captcha-2", "NOPE", policy)
	if !errors.Is(err, ErrCaptchaAttemptsExceeded) || ok {
		t.Fatalf("expected attempts exceeded, ok=%v err=%v", ok, err)
	}
}

func TestMemoryCaptchaStoreReportsExpired(t *testing.T) {
	store := NewCaptchaStore(nil)
	policy := CaptchaPolicy{TTL: time.Nanosecond, MaxAttempts: 3}

	if err := store.Save(context.Background(), "captcha-3", "C9NP", policy); err != nil {
		t.Fatalf("save captcha: %v", err)
	}
	time.Sleep(time.Millisecond)

	ok, err := store.Verify(context.Background(), "captcha-3", "C9NP", policy)
	if !errors.Is(err, ErrCaptchaInvalid) || ok {
		t.Fatalf("expected expired captcha, ok=%v err=%v", ok, err)
	}
}
