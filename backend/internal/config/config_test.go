package config

import (
	"strings"
	"testing"
)

func validProdConfigForTest() Config {
	return Config{
		AppEnv:               "prod",
		MySQLDSN:             "user:pass@tcp(127.0.0.1:3306)/image2?parseTime=true",
		RedisAddr:            "127.0.0.1:6379",
		JWTSecret:            "0123456789abcdef0123456789abcdef",
		AdminEmails:          []string{"admin@example.com"},
		ImageProviderAPIKey:  "sk-test",
		CORSAllowOrigins:     []string{"https://example.com"},
		BackendPublicURL:     "https://example.com",
		ImageStorageDriver:   "local",
		ImageStorageLocalDir: "storage/uploads",
	}
}

func TestValidateProdAllowsSMTPUnsetWhenRegistrationDisabled(t *testing.T) {
	cfg := validProdConfigForTest()
	cfg.RegistrationEnabled = false
	cfg.EmailDelivery = "debug"
	cfg.SMTPHost = ""
	cfg.SMTPFrom = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected disabled registration to allow smtp unset, got %v", err)
	}
}

func TestValidateProdRejectsDebugEmailWhenRegistrationEnabled(t *testing.T) {
	cfg := validProdConfigForTest()
	cfg.RegistrationEnabled = true
	cfg.EmailDelivery = "debug"
	cfg.SMTPHost = ""
	cfg.SMTPFrom = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected debug delivery to be rejected in prod")
	}
	if !strings.Contains(err.Error(), "EMAIL_DELIVERY") {
		t.Fatalf("expected email delivery requirement, got %v", err)
	}
}

func TestValidateProdRequiresRealEmailProviderConfigWhenSelected(t *testing.T) {
	cfg := validProdConfigForTest()
	cfg.RegistrationEnabled = true
	cfg.EmailDelivery = "smtp"
	cfg.SMTPHost = ""
	cfg.SMTPFrom = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected smtp provider validation error")
	}
	if !strings.Contains(err.Error(), "SMTP_HOST") {
		t.Fatalf("expected smtp config requirement, got %v", err)
	}
}

func TestValidateProdRequiresResendAPIKeyWhenResendSelected(t *testing.T) {
	cfg := validProdConfigForTest()
	cfg.RegistrationEnabled = true
	cfg.EmailDelivery = "resend"
	cfg.SMTPFrom = "verify@jh612.top"
	cfg.ResendAPIKey = ""
	cfg.SMTPPassword = "legacy-key-should-not-pass-prod-validation"

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected resend api key validation error")
	}
	if !strings.Contains(err.Error(), "RESEND_API_KEY") {
		t.Fatalf("expected resend api key requirement, got %v", err)
	}
}

func TestValidateProdAllowsResendWhenConfigured(t *testing.T) {
	cfg := validProdConfigForTest()
	cfg.RegistrationEnabled = true
	cfg.EmailDelivery = "resend"
	cfg.ResendAPIKey = "re_test_key"
	cfg.SMTPFrom = "verify@jh612.top"
	cfg.SMTPFromName = "jh612"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected configured resend delivery to pass, got %v", err)
	}
}

func TestValidateProdRejectsPlaceholderProviderKeyAndOrigins(t *testing.T) {
	cfg := validProdConfigForTest()
	cfg.ImageProviderAPIKey = "replace-with-real-provider-key"
	cfg.CORSAllowOrigins = []string{"http://localhost:5173"}
	cfg.EmailDelivery = "resend"
	cfg.RegistrationEnabled = false
	cfg.ResendAPIKey = "re_test_key"
	cfg.SMTPFrom = "verify@jh612.top"

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected placeholder provider key and localhost origin to be rejected")
	}
	if !strings.Contains(err.Error(), "IMAGE_PROVIDER_API_KEY") || !strings.Contains(err.Error(), "CORS_ALLOW_ORIGINS") {
		t.Fatalf("expected provider key and cors requirements, got %v", err)
	}
}
