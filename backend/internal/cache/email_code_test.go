package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testEmailCodePolicy() EmailCodePolicy {
	return EmailCodePolicy{
		TTL:               time.Minute,
		Cooldown:          time.Hour,
		SendWindow:        time.Hour,
		MaxSendsPerWindow: 5,
		MaxVerifyAttempts: 2,
	}
}

func TestMemoryEmailCodeStoreGeneratesFourDigitCodeAndVerifiesOnce(t *testing.T) {
	store := NewEmailCodeStore(nil)

	code, err := store.Save(context.Background(), "USER@example.com", "register", testEmailCodePolicy())
	if err != nil {
		t.Fatalf("save code: %v", err)
	}
	if len(code) != 4 {
		t.Fatalf("expected 4-digit code, got %q", code)
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			t.Fatalf("expected numeric code, got %q", code)
		}
	}

	ok, err := store.Verify(context.Background(), "user@example.com", "register", code, testEmailCodePolicy())
	if err != nil || !ok {
		t.Fatalf("verify code: ok=%v err=%v", ok, err)
	}

	ok, err = store.Verify(context.Background(), "user@example.com", "register", code, testEmailCodePolicy())
	if err != nil || ok {
		t.Fatalf("verified code must be one-time use, ok=%v err=%v", ok, err)
	}
}

func TestMemoryEmailCodeStoreEnforcesCooldown(t *testing.T) {
	store := NewEmailCodeStore(nil)

	if _, err := store.Save(context.Background(), "cooldown@example.com", "register", testEmailCodePolicy()); err != nil {
		t.Fatalf("first save: %v", err)
	}

	_, err := store.Save(context.Background(), "cooldown@example.com", "register", testEmailCodePolicy())
	if !errors.Is(err, ErrEmailCodeCooldown) {
		t.Fatalf("expected cooldown error, got %v", err)
	}
}

func TestMemoryEmailCodeStoreAllowsResendAfterDelete(t *testing.T) {
	store := NewEmailCodeStore(nil)

	if _, err := store.Save(context.Background(), "cleanup@example.com", "register", testEmailCodePolicy()); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := store.Delete(context.Background(), "cleanup@example.com", "register"); err != nil {
		t.Fatalf("delete code: %v", err)
	}
	if _, err := store.Save(context.Background(), "cleanup@example.com", "register", testEmailCodePolicy()); err != nil {
		t.Fatalf("save after delete should bypass stale cooldown: %v", err)
	}
}

func TestMemoryEmailCodeStoreReportsExpiredCode(t *testing.T) {
	store := NewEmailCodeStore(nil)
	policy := testEmailCodePolicy()
	policy.TTL = time.Nanosecond
	policy.Cooldown = time.Nanosecond

	code, err := store.Save(context.Background(), "expired@example.com", "register", policy)
	if err != nil {
		t.Fatalf("save code: %v", err)
	}
	time.Sleep(time.Millisecond)

	ok, err := store.Verify(context.Background(), "expired@example.com", "register", code, policy)
	if !errors.Is(err, ErrEmailCodeExpired) || ok {
		t.Fatalf("expected expired code, ok=%v err=%v", ok, err)
	}
}

func TestMemoryEmailCodeStoreLimitsWrongAttempts(t *testing.T) {
	store := NewEmailCodeStore(nil)

	code, err := store.Save(context.Background(), "attempts@example.com", "register", testEmailCodePolicy())
	if err != nil {
		t.Fatalf("save code: %v", err)
	}

	ok, err := store.Verify(context.Background(), "attempts@example.com", "register", wrongEmailCode(code, "0000"), testEmailCodePolicy())
	if err != nil || ok {
		t.Fatalf("first wrong code should only fail verification, ok=%v err=%v", ok, err)
	}

	ok, err = store.Verify(context.Background(), "attempts@example.com", "register", wrongEmailCode(code, "1111"), testEmailCodePolicy())
	if !errors.Is(err, ErrEmailCodeAttemptsExceeded) || ok {
		t.Fatalf("expected attempts exceeded, ok=%v err=%v", ok, err)
	}

	ok, err = store.Verify(context.Background(), "attempts@example.com", "register", code, testEmailCodePolicy())
	if err != nil || ok {
		t.Fatalf("code should be deleted after attempts are exceeded, ok=%v err=%v", ok, err)
	}
}

func wrongEmailCode(actual string, preferred string) string {
	if actual != preferred {
		return preferred
	}
	return "9999"
}
