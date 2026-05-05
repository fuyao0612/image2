package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 保存服务运行所需的基础配置。
type Config struct {
	AppEnv                     string
	HTTPAddr                   string
	MySQLDSN                   string
	SQLitePath                 string
	RedisAddr                  string
	RedisPassword              string
	RedisDB                    int
	JWTSecret                  string
	JWTIssuer                  string
	CORSAllowOrigins           []string
	AdminEmails                []string
	EmailCodeTTL               time.Duration
	AccessTokenTTL             time.Duration
	EmailDelivery              string
	SMTPHost                   string
	SMTPPort                   int
	SMTPUsername               string
	SMTPPassword               string
	SMTPFrom                   string
	SMTPFromName               string
	SMTPUseTLS                 bool
	SMTPStartTLS               bool
	SMTPTimeout                time.Duration
	CheckinPoints              int64
	ImageCostPoints            int64
	ImageProviderBatchURL      string
	ImageProviderAPIBaseURL    string
	ImageProviderAPIKey        string
	ImageProviderTimeout       time.Duration
	ImageProviderPollInterval  time.Duration
	ImageTaskStaleTTL          time.Duration
	BackendPublicURL           string
	ImageStorageDriver         string
	ImageStorageLocalDir       string
	ImageStoragePublicPath     string
	ImageStorageTimeout        time.Duration
	ImageStorageMaxBytes       int64
	ImageStorageS3Endpoint     string
	ImageStorageS3Region       string
	ImageStorageS3Bucket       string
	ImageStorageS3AccessKeyID  string
	ImageStorageS3SecretKey    string
	ImageStorageS3PublicURL    string
	ImageStorageS3PathPrefix   string
	ImageStorageS3PathStyle    bool
	ImageStorageS3DisableSSL   bool
	PromptBlockKeywords        []string
	RateLimitEnabled           bool
	RateLimitRegisterPerHour   int
	RateLimitLoginPerMinute    int
	RateLimitCheckinPerMinute  int
	RateLimitImagePerMinute    int
	RateLimitPurchasePerMinute int
	RegistrationEnabled        bool
}

// Load 从环境变量加载配置，提供一套适合本地开发的默认值。
func Load() Config {
	loadEnvFile(".env")

	return Config{
		AppEnv:                     getEnv("APP_ENV", "dev"),
		HTTPAddr:                   getEnv("HTTP_ADDR", ":8080"),
		MySQLDSN:                   getEnv("MYSQL_DSN", ""),
		SQLitePath:                 getEnv("SQLITE_PATH", "data/app.db"),
		RedisAddr:                  getEnv("REDIS_ADDR", ""),
		RedisPassword:              getEnv("REDIS_PASSWORD", ""),
		RedisDB:                    getEnvInt("REDIS_DB", 0),
		JWTSecret:                  getEnv("JWT_SECRET", "change-me"),
		JWTIssuer:                  getEnv("JWT_ISSUER", "ai-image-saas"),
		CORSAllowOrigins:           splitAndTrim(getEnv("CORS_ALLOW_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
		AdminEmails:                splitAndTrim(getEnv("ADMIN_EMAILS", "")),
		EmailCodeTTL:               5 * time.Minute,
		AccessTokenTTL:             24 * time.Hour,
		EmailDelivery:              getEnv("EMAIL_DELIVERY", "debug"),
		SMTPHost:                   getEnv("SMTP_HOST", ""),
		SMTPPort:                   getEnvInt("SMTP_PORT", 587),
		SMTPUsername:               getEnv("SMTP_USERNAME", ""),
		SMTPPassword:               getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:                   getEnv("SMTP_FROM", ""),
		SMTPFromName:               getEnv("SMTP_FROM_NAME", "AI 生图 SaaS"),
		SMTPUseTLS:                 getEnvBool("SMTP_USE_TLS", false),
		SMTPStartTLS:               getEnvBool("SMTP_STARTTLS", true),
		SMTPTimeout:                getEnvDuration("SMTP_TIMEOUT", 10*time.Second),
		CheckinPoints:              getEnvInt64("CHECKIN_POINTS", 3),
		ImageCostPoints:            getEnvInt64("IMAGE_COST_POINTS", 1),
		ImageProviderBatchURL:      getEnv("IMAGE_PROVIDER_BATCH_URL", "https://image.mentalout.top"),
		ImageProviderAPIBaseURL:    getEnv("IMAGE_PROVIDER_API_BASE_URL", "https://chunfeng.mentalout.top"),
		ImageProviderAPIKey:        getEnv("IMAGE_PROVIDER_API_KEY", ""),
		ImageProviderTimeout:       getEnvDuration("IMAGE_PROVIDER_TIMEOUT", 10*time.Minute),
		ImageProviderPollInterval:  getEnvDuration("IMAGE_PROVIDER_POLL_INTERVAL", 2*time.Second),
		ImageTaskStaleTTL:          getEnvDuration("IMAGE_TASK_STALE_TTL", 30*time.Minute),
		BackendPublicURL:           strings.TrimRight(getEnv("BACKEND_PUBLIC_URL", "http://localhost:8080"), "/"),
		ImageStorageDriver:         getEnv("IMAGE_STORAGE_DRIVER", "local"),
		ImageStorageLocalDir:       getEnv("IMAGE_STORAGE_LOCAL_DIR", "storage/uploads"),
		ImageStoragePublicPath:     getEnv("IMAGE_STORAGE_PUBLIC_PATH", "/uploads"),
		ImageStorageTimeout:        getEnvDuration("IMAGE_STORAGE_TIMEOUT", 30*time.Second),
		ImageStorageMaxBytes:       getEnvInt64("IMAGE_STORAGE_MAX_BYTES", 25*1024*1024),
		ImageStorageS3Endpoint:     getEnv("IMAGE_STORAGE_S3_ENDPOINT", ""),
		ImageStorageS3Region:       getEnv("IMAGE_STORAGE_S3_REGION", ""),
		ImageStorageS3Bucket:       getEnv("IMAGE_STORAGE_S3_BUCKET", ""),
		ImageStorageS3AccessKeyID:  getEnv("IMAGE_STORAGE_S3_ACCESS_KEY_ID", ""),
		ImageStorageS3SecretKey:    getEnv("IMAGE_STORAGE_S3_SECRET_ACCESS_KEY", ""),
		ImageStorageS3PublicURL:    getEnv("IMAGE_STORAGE_S3_PUBLIC_BASE_URL", ""),
		ImageStorageS3PathPrefix:   getEnv("IMAGE_STORAGE_S3_PATH_PREFIX", "generated"),
		ImageStorageS3PathStyle:    getEnvBool("IMAGE_STORAGE_S3_FORCE_PATH_STYLE", true),
		ImageStorageS3DisableSSL:   getEnvBool("IMAGE_STORAGE_S3_DISABLE_SSL", false),
		PromptBlockKeywords:        splitAndTrim(getEnv("PROMPT_BLOCK_KEYWORDS", defaultPromptBlockKeywords())),
		RateLimitEnabled:           getEnvBool("RATE_LIMIT_ENABLED", true),
		RateLimitRegisterPerHour:   getEnvInt("RATE_LIMIT_REGISTER_PER_HOUR", 10),
		RateLimitLoginPerMinute:    getEnvInt("RATE_LIMIT_LOGIN_PER_MINUTE", 10),
		RateLimitCheckinPerMinute:  getEnvInt("RATE_LIMIT_CHECKIN_PER_MINUTE", 5),
		RateLimitImagePerMinute:    getEnvInt("RATE_LIMIT_IMAGE_PER_MINUTE", 3),
		RateLimitPurchasePerMinute: getEnvInt("RATE_LIMIT_PURCHASE_PER_MINUTE", 10),
		RegistrationEnabled:        getEnvBool("REGISTRATION_ENABLED", true),
	}
}

// Validate 检查配置是否满足当前环境的最低运行要求。
func (c Config) Validate() error {
	if !strings.EqualFold(c.AppEnv, "prod") {
		return nil
	}

	var missing []string
	if strings.TrimSpace(c.MySQLDSN) == "" {
		missing = append(missing, "MYSQL_DSN")
	}
	if strings.TrimSpace(c.RedisAddr) == "" {
		missing = append(missing, "REDIS_ADDR")
	}
	if secret := strings.TrimSpace(c.JWTSecret); secret == "" || secret == "change-me" || len(secret) < 32 {
		missing = append(missing, "JWT_SECRET(至少 32 位且不能使用默认值)")
	}
	if len(c.AdminEmails) == 0 {
		missing = append(missing, "ADMIN_EMAILS")
	}
	if key := strings.TrimSpace(c.ImageProviderAPIKey); key == "" || key == "replace-with-your-api-key" {
		missing = append(missing, "IMAGE_PROVIDER_API_KEY")
	}
	if !strings.EqualFold(strings.TrimSpace(c.EmailDelivery), "smtp") {
		missing = append(missing, "EMAIL_DELIVERY=smtp")
	}
	if strings.TrimSpace(c.SMTPHost) == "" {
		missing = append(missing, "SMTP_HOST")
	}
	if strings.TrimSpace(c.SMTPFrom) == "" {
		missing = append(missing, "SMTP_FROM")
	}

	switch strings.ToLower(strings.TrimSpace(c.ImageStorageDriver)) {
	case "local":
		publicURL := strings.TrimSpace(c.BackendPublicURL)
		if publicURL == "" || strings.Contains(publicURL, "localhost") || strings.Contains(publicURL, "127.0.0.1") {
			missing = append(missing, "BACKEND_PUBLIC_URL(生产域名)")
		}
		if strings.TrimSpace(c.ImageStorageLocalDir) == "" {
			missing = append(missing, "IMAGE_STORAGE_LOCAL_DIR")
		}
	case "s3":
		if strings.TrimSpace(c.ImageStorageS3Region) == "" {
			missing = append(missing, "IMAGE_STORAGE_S3_REGION")
		}
		if strings.TrimSpace(c.ImageStorageS3Bucket) == "" {
			missing = append(missing, "IMAGE_STORAGE_S3_BUCKET")
		}
		if strings.TrimSpace(c.ImageStorageS3AccessKeyID) == "" {
			missing = append(missing, "IMAGE_STORAGE_S3_ACCESS_KEY_ID")
		}
		if strings.TrimSpace(c.ImageStorageS3SecretKey) == "" {
			missing = append(missing, "IMAGE_STORAGE_S3_SECRET_ACCESS_KEY")
		}
	default:
		missing = append(missing, "IMAGE_STORAGE_DRIVER(local 或 s3)")
	}

	if len(missing) > 0 {
		return fmt.Errorf("生产环境配置不完整: %s", strings.Join(missing, ", "))
	}
	return nil
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		_ = os.Setenv(key, value)
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitAndTrim(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			origins = append(origins, value)
		}
	}
	return origins
}

func defaultPromptBlockKeywords() string {
	return strings.Join([]string{
		"未成年色情",
		"儿童色情",
		"裸露儿童",
		"萝莉色情",
		"强奸",
		"迷奸",
		"血腥肢解",
		"真实自残",
		"制作毒品",
		"制造炸弹",
		"身份证",
		"银行卡",
		"child sexual",
		"child nude",
		"rape",
		"incest",
		"gore dismemberment",
		"make bomb",
	}, ",")
}
