package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"image2/backend/internal/cache"
	"image2/backend/internal/config"
	"image2/backend/internal/dto"
	"image2/backend/internal/mailer"
	"image2/backend/internal/model"
	"image2/backend/internal/provider"
	"image2/backend/internal/security"
	imagestorage "image2/backend/internal/storage"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type imageTaskGenerationInput struct {
	Prompt             string   `json:"prompt"`
	Size               string   `json:"size,omitempty"`
	Style              string   `json:"style,omitempty"`
	Count              int      `json:"count,omitempty"`
	Quality            string   `json:"quality,omitempty"`
	OutputFormat       string   `json:"outputFormat,omitempty"`
	ReferenceImageURL  string   `json:"referenceImageUrl,omitempty"`
	ReferenceImageURLs []string `json:"referenceImageUrls,omitempty"`
}

// Service 承载平台核心业务。
type Service struct {
	cfg           config.Config
	db            *gorm.DB
	redis         *redis.Client
	emailCodes    cache.EmailCodeStore
	captchas      cache.CaptchaStore
	mailer        *mailer.Client
	jwt           *security.JWTManager
	location      *time.Location
	imageProvider *provider.MentaloutProvider
	imageStore    *imagestorage.Client
	settingsMu    sync.RWMutex
	settings      map[string]string
	captchaPassMu sync.Mutex
	captchaPasses map[string]time.Time
	emailUsageMu  sync.Mutex
	emailUsage    map[string]int
	emailNoticeMu sync.Mutex
	emailNotices  map[string]time.Time
	activeTasksMu sync.RWMutex
	activeTasks   map[uint64]struct{}
}

const imageTaskSubmittingMarker = "__image_task_submitting__"

type imageTaskFailureNotice struct {
	TaskID         uint64
	UserID         uint64
	Prompt         string
	Message        string
	AdminMessage   string
	Provider       string
	ProviderTaskID string
	RawResponse    string
	CreatedAt      time.Time
}

// TokenVersionView 用于鉴权阶段读取账号当前会话版本。
type TokenVersionView struct {
	TokenVersion int64
}

// New 创建业务服务。
func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client) *Service {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}

	store, err := imagestorage.New(imagestorage.Config{
		Driver:            cfg.ImageStorageDriver,
		LocalDir:          cfg.ImageStorageLocalDir,
		PublicBaseURL:     cfg.BackendPublicURL,
		PublicPath:        cfg.ImageStoragePublicPath,
		Timeout:           cfg.ImageStorageTimeout,
		MaxBytes:          cfg.ImageStorageMaxBytes,
		S3Endpoint:        cfg.ImageStorageS3Endpoint,
		S3Region:          cfg.ImageStorageS3Region,
		S3Bucket:          cfg.ImageStorageS3Bucket,
		S3AccessKeyID:     cfg.ImageStorageS3AccessKeyID,
		S3SecretAccessKey: cfg.ImageStorageS3SecretKey,
		S3PublicBaseURL:   cfg.ImageStorageS3PublicURL,
		S3PathPrefix:      cfg.ImageStorageS3PathPrefix,
		S3ForcePathStyle:  cfg.ImageStorageS3PathStyle,
		S3DisableSSL:      cfg.ImageStorageS3DisableSSL,
	})
	if err != nil {
		log.Printf("初始化图片存储失败，图片将保留第三方 URL: %v", err)
	}

	svc := &Service{
		cfg:        cfg,
		db:         db,
		redis:      redisClient,
		emailCodes: cache.NewEmailCodeStore(redisClient),
		captchas:   cache.NewCaptchaStore(redisClient),
		mailer: mailer.New(mailer.Config{
			Delivery:              cfg.EmailDelivery,
			Host:                  cfg.SMTPHost,
			Port:                  cfg.SMTPPort,
			Username:              cfg.SMTPUsername,
			Password:              cfg.SMTPPassword,
			From:                  cfg.SMTPFrom,
			FromName:              cfg.SMTPFromName,
			UseTLS:                cfg.SMTPUseTLS,
			StartTLS:              cfg.SMTPStartTLS,
			Timeout:               cfg.SMTPTimeout,
			ResendKey:             cfg.ResendAPIKey,
			ResendURL:             cfg.ResendAPIURL,
			TencentSecretID:       cfg.TencentCloudSecretID,
			TencentSecretKey:      cfg.TencentCloudSecretKey,
			TencentToken:          cfg.TencentCloudToken,
			TencentRegion:         cfg.TencentSESRegion,
			TencentEndpoint:       cfg.TencentSESEndpoint,
			TencentFrom:           cfg.TencentSESFrom,
			TencentFromName:       cfg.TencentSESFromName,
			TencentTemplateID:     cfg.TencentSESTemplateID,
			TencentTaskTemplateID: cfg.TencentSESTaskTemplateID,
			AppName:               cfg.SMTPFromName,
			AppEnv:                cfg.AppEnv,
		}),
		jwt:           security.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTokenTTL),
		location:      loc,
		imageStore:    store,
		settings:      defaultSystemSettings(cfg),
		captchaPasses: make(map[string]time.Time),
		emailUsage:    make(map[string]int),
		emailNotices:  make(map[string]time.Time),
		activeTasks:   make(map[uint64]struct{}),
		imageProvider: provider.NewMentaloutProvider(provider.MentaloutConfig{
			BatchServiceURL:    cfg.ImageProviderBatchURL,
			ProviderAPIBaseURL: cfg.ImageProviderAPIBaseURL,
			APIKey:             cfg.ImageProviderAPIKey,
			PollInterval:       cfg.ImageProviderPollInterval,
			Timeout:            cfg.ImageProviderTimeout,
		}),
	}
	svc.ensureDefaultProviderKey()
	return svc
}

// JWT 返回令牌管理器。
func (s *Service) JWT() *security.JWTManager {
	return s.jwt
}

// Redis 返回 Redis 客户端，供限流等 HTTP 中间件复用。
func (s *Service) Redis() *redis.Client {
	return s.redis
}

func (s *Service) Config() config.Config {
	return s.cfg
}

func (s *Service) emailCodePolicy() cache.EmailCodePolicy {
	return cache.EmailCodePolicy{
		TTL:               s.cfg.EmailCodeTTL,
		Cooldown:          s.cfg.EmailCodeCooldown,
		SendWindow:        s.cfg.EmailCodeSendWindow,
		MaxSendsPerWindow: s.cfg.EmailCodeMaxPerWindow,
		MaxVerifyAttempts: s.cfg.EmailCodeMaxAttempts,
	}
}

func mapEmailCodeError(err error) error {
	switch {
	case errors.Is(err, cache.ErrEmailCodeCooldown):
		return ErrVerificationCodeTooFrequent
	case errors.Is(err, cache.ErrEmailCodeRateLimited):
		return ErrVerificationCodeRateLimited
	case errors.Is(err, cache.ErrEmailCodeAttemptsExceeded):
		return ErrVerificationAttemptsExceeded
	case errors.Is(err, cache.ErrEmailCodeExpired):
		return ErrVerificationCodeExpired
	default:
		return err
	}
}

func (s *Service) ensureRegisterEmailDailyQuota(ctx context.Context) error {
	if !s.emailDailyLimitEnabled() {
		return nil
	}
	count, err := s.currentEmailDailyUsage(ctx)
	if err != nil {
		log.Printf("读取 Resend 当日发送计数失败，将继续执行: %v", err)
		return nil
	}
	if count >= s.cfg.EmailDailySendLimit {
		return ErrEmailDailyLimitReached
	}
	return nil
}

func (s *Service) recordRegisterEmailSent(ctx context.Context) {
	if !s.emailDailyLimitEnabled() {
		return
	}
	if _, err := s.incrementEmailDailyUsage(ctx); err != nil {
		log.Printf("记录 Resend 当日发送计数失败: %v", err)
	}
}

func (s *Service) emailDailyLimitEnabled() bool {
	if s.cfg.EmailDailySendLimit <= 0 {
		return false
	}
	delivery := strings.ToLower(strings.TrimSpace(s.cfg.EmailDelivery))
	host := strings.ToLower(strings.TrimSpace(s.cfg.SMTPHost))
	return delivery == "resend" || strings.Contains(host, "resend.com")
}

func (s *Service) currentEmailDailyUsage(ctx context.Context) (int, error) {
	key := s.emailDailyUsageKey(time.Now().In(s.location))
	if s.redis != nil {
		value, err := s.redis.Get(ctx, key).Int()
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return value, err
	}
	s.emailUsageMu.Lock()
	defer s.emailUsageMu.Unlock()
	return s.emailUsage[key], nil
}

func (s *Service) incrementEmailDailyUsage(ctx context.Context) (int, error) {
	now := time.Now().In(s.location)
	key := s.emailDailyUsageKey(now)
	if s.redis != nil {
		count, err := s.redis.Incr(ctx, key).Result()
		if err != nil {
			return 0, err
		}
		if count == 1 {
			_ = s.redis.Expire(ctx, key, s.durationUntilNextLocalDay(now)+time.Hour).Err()
		}
		return int(count), nil
	}
	s.emailUsageMu.Lock()
	defer s.emailUsageMu.Unlock()
	s.emailUsage[key]++
	return s.emailUsage[key], nil
}

func (s *Service) emailDailyUsageKey(now time.Time) string {
	return "email:resend:register:daily:" + now.Format("20060102")
}

func (s *Service) durationUntilNextLocalDay(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, s.location)
	return time.Until(next)
}

// EnsureActiveUser 确认当前 Token 对应的账号仍然可用。
func (s *Service) EnsureActiveUser(ctx context.Context, userID uint64) error {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	if user.Status == model.UserStatusDisabled {
		return ErrUserDisabled
	}
	return nil
}

// SendEmailCode 生成并保存验证码。
func (s *Service) SendEmailCode(ctx context.Context, req dto.SendEmailCodeRequest) (dto.SendEmailCodeResponse, error) {
	email := normalizeEmail(req.Email)
	if email == "" || req.Scene == "" {
		return dto.SendEmailCodeResponse{}, fmt.Errorf("请求参数不合法")
	}
	if strings.EqualFold(strings.TrimSpace(req.Scene), "register") && !s.cfg.RegistrationEnabled {
		return dto.SendEmailCodeResponse{}, ErrRegistrationDisabled
	}
	isRegisterScene := strings.EqualFold(strings.TrimSpace(req.Scene), "register")
	isResetPasswordScene := strings.EqualFold(strings.TrimSpace(req.Scene), "reset_password")
	if isRegisterScene {
		if err := s.ensureRegisterEmailDailyQuota(ctx); err != nil {
			return dto.SendEmailCodeResponse{}, err
		}
	}
	if isResetPasswordScene {
		if err := s.ensureResetPasswordEmailAllowed(ctx, email); err != nil {
			return dto.SendEmailCodeResponse{}, err
		}
	}

	if !s.hasCaptchaPass(ctx, email, req.Scene) {
		if strings.TrimSpace(req.CaptchaID) == "" || strings.TrimSpace(req.CaptchaCode) == "" {
			return dto.SendEmailCodeResponse{}, ErrInvalidCaptcha
		}
		if err := s.verifyCaptcha(ctx, req.CaptchaID, req.CaptchaCode); err != nil {
			return dto.SendEmailCodeResponse{}, err
		}
		_ = s.rememberCaptchaPass(ctx, email, req.Scene)
	}
	if isRegisterScene {
		exists, err := s.emailExists(ctx, email)
		if err != nil {
			return dto.SendEmailCodeResponse{}, err
		}
		if exists {
			return dto.SendEmailCodeResponse{}, ErrEmailAlreadyRegistered
		}
	}

	code, err := s.emailCodes.Save(ctx, email, req.Scene, s.emailCodePolicy())
	if err != nil {
		return dto.SendEmailCodeResponse{}, mapEmailCodeError(err)
	}

	isProd := strings.EqualFold(strings.TrimSpace(s.cfg.AppEnv), "prod")
	debugDelivery := strings.EqualFold(strings.TrimSpace(s.cfg.EmailDelivery), "debug")
	if s.mailer != nil && s.mailer.Configured() {
		if err := s.mailer.SendVerificationCode(ctx, email, code, req.Scene, s.cfg.EmailCodeTTL); err != nil {
			_ = s.emailCodes.Delete(ctx, email, req.Scene)
			return dto.SendEmailCodeResponse{}, err
		}
		if isRegisterScene {
			s.recordRegisterEmailSent(ctx)
		}
	} else if debugDelivery && !isProd {
		log.Printf("debug 邮箱验证码: email=%s scene=%s code=%s expires_in=%s", email, req.Scene, code, s.cfg.EmailCodeTTL)
	} else if !isProd {
		log.Printf("debug 邮箱验证码: email=%s scene=%s code=%s expires_in=%s", email, req.Scene, code, s.cfg.EmailCodeTTL)
	} else {
		return dto.SendEmailCodeResponse{}, ErrEmailSenderNotConfigured
	}

	resp := dto.SendEmailCodeResponse{
		Sent:      true,
		ExpiresIn: int(s.cfg.EmailCodeTTL / time.Second),
	}
	if debugDelivery && !isProd {
		resp.DebugCode = code
	}

	return resp, nil
}

// Register 完成注册并直接返回登录态。
// Login 账号密码登录。
func (s *Service) Login(ctx context.Context, req dto.LoginRequest) (dto.AuthResponse, error) {
	_ = ctx
	account := strings.TrimSpace(req.Account)
	if account == "" || strings.TrimSpace(req.Password) == "" {
		return dto.AuthResponse{}, fmt.Errorf("请求参数不合法")
	}

	user, err := s.findUserByAccount(account)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.AuthResponse{}, ErrInvalidCredentials
		}
		return dto.AuthResponse{}, err
	}
	if user.Status == model.UserStatusDisabled {
		return dto.AuthResponse{}, ErrUserDisabled
	}
	if err := comparePassword(user.PasswordHash, req.Password); err != nil {
		return dto.AuthResponse{}, ErrInvalidCredentials
	}

	now := s.now()
	if err := s.db.Model(&model.User{}).Where("id = ?", user.ID).Update("last_login_at", now).Error; err != nil {
		return dto.AuthResponse{}, err
	}
	user.LastLoginAt = &now
	user = s.syncConfiguredAdminRole(user)

	ttl := s.cfg.AccessTokenTTL
	if req.RememberMe {
		ttl = 7 * 24 * time.Hour
	}
	return s.authResponseWithTTL(user, ttl)
}

// CheckIn 完成每日签到，若当天已签到则返回幂等成功。
func (s *Service) CheckIn(ctx context.Context, userID uint64) (dto.CheckinResponse, error) {
	return s.checkInV2(ctx, userID)
}

// GetPoints 获取当前积分和最近流水。
func (s *Service) GetPoints(ctx context.Context, userID uint64) (dto.PointsResponse, error) {
	return s.getPointsV2(ctx, userID)
}

// GetMe 返回当前登录用户信息。
func (s *Service) GetMe(ctx context.Context, userID uint64) (dto.UserInfo, error) {
	return s.getMeV2(ctx, userID)
}

// UploadReferenceImage 上传参考图片并返回可访问地址。
func (s *Service) UploadReferenceImage(ctx context.Context, userID uint64, header *multipart.FileHeader) (dto.UploadImageResponse, error) {
	return s.UploadUserImage(ctx, userID, header, "reference")
}

func (s *Service) UploadUserImage(ctx context.Context, userID uint64, header *multipart.FileHeader, purpose string) (dto.UploadImageResponse, error) {
	return s.uploadImageBytes(ctx, header, userUploadKeyPrefix(userID, purpose, s.now()))
}

func (s *Service) AdminUploadAnnouncementImage(ctx context.Context, adminID uint64, header *multipart.FileHeader) (dto.UploadImageResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.UploadImageResponse{}, err
	}
	return s.uploadImageBytes(ctx, header, adminUploadKeyPrefix(adminID, "announcement", s.now()))
}

func (s *Service) AdminUploadPromptPlazaImage(ctx context.Context, adminID uint64, header *multipart.FileHeader) (dto.UploadImageResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.UploadImageResponse{}, err
	}
	return s.uploadImageBytes(ctx, header, adminUploadKeyPrefix(adminID, "prompt-plaza", s.now()))
}

func (s *Service) AdminUploadProductImage(ctx context.Context, adminID uint64, header *multipart.FileHeader) (dto.UploadImageResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.UploadImageResponse{}, err
	}
	return s.uploadImageBytes(ctx, header, adminUploadKeyPrefix(adminID, "product-image", s.now()))
}

func (s *Service) uploadImageBytes(ctx context.Context, header *multipart.FileHeader, keyPrefix string) (dto.UploadImageResponse, error) {
	if header == nil {
		return dto.UploadImageResponse{}, ErrUploadFileRequired
	}
	if s.imageStore == nil || !s.imageStore.Enabled() {
		return dto.UploadImageResponse{}, ErrUploadStorageNotConfig
	}
	if s.cfg.ImageStorageMaxBytes > 0 && header.Size > s.cfg.ImageStorageMaxBytes {
		return dto.UploadImageResponse{}, ErrUploadFileTooLarge
	}

	file, err := header.Open()
	if err != nil {
		return dto.UploadImageResponse{}, err
	}
	defer file.Close()

	limit := s.cfg.ImageStorageMaxBytes
	if limit <= 0 {
		limit = 25 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return dto.UploadImageResponse{}, err
	}
	if int64(len(data)) > limit {
		return dto.UploadImageResponse{}, ErrUploadFileTooLarge
	}
	if len(data) == 0 {
		return dto.UploadImageResponse{}, ErrUploadFileRequired
	}

	contentType := http.DetectContentType(data)
	if !isAllowedUploadImageType(contentType) {
		return dto.UploadImageResponse{}, ErrUploadUnsupportedType
	}
	imageURL, err := s.imageStore.StoreBytes(ctx, data, keyPrefix, contentType, header.Filename)
	if err != nil {
		return dto.UploadImageResponse{}, err
	}

	return dto.UploadImageResponse{
		ImageURL:    imageURL,
		FileName:    header.Filename,
		ContentType: contentType,
		Size:        int64(len(data)),
	}, nil
}

func userUploadKeyPrefix(userID uint64, purpose string, now time.Time) string {
	category := "references"
	switch strings.ToLower(strings.TrimSpace(purpose)) {
	case "prompt-plaza", "prompt-plaza-preview", "prompt_plaza", "prompt_plaza_preview":
		category = "prompt-plaza-previews"
	case "reference", "reference-image", "image-task-reference", "":
		category = "references"
	}
	return fmt.Sprintf("users/user-%d/%s/%s", userID, category, now.Format("2006/01/02"))
}

func adminUploadKeyPrefix(adminID uint64, purpose string, now time.Time) string {
	category := "announcements"
	switch strings.ToLower(strings.TrimSpace(purpose)) {
	case "prompt-plaza", "prompt-plaza-preview", "prompt_plaza", "prompt_plaza_preview":
		category = "prompt-plaza-previews"
	case "product-image", "product", "products":
		category = "products"
	case "announcement", "announcements", "":
		category = "announcements"
	}
	return fmt.Sprintf("admin/admin-%d/%s/%s", adminID, category, now.Format("2006/01/02"))
}

// CreateImageTask 创建生图任务并立刻返回 mock 结果。
func (s *Service) CreateImageTask(ctx context.Context, userID uint64, req dto.CreateImageTaskRequest) (dto.CreateImageTaskResponse, error) {
	return s.createImageTaskV2(ctx, userID, req)
}

// RetryImageTask 基于历史任务的请求快照重新创建一条生图任务。
func (s *Service) RetryImageTask(ctx context.Context, userID uint64, taskID string) (dto.CreateImageTaskResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	id, err := strconv.ParseUint(strings.TrimSpace(taskID), 10, 64)
	if err != nil || id == 0 {
		return dto.CreateImageTaskResponse{}, ErrTaskNotFound
	}

	var task model.ImageTask
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.CreateImageTaskResponse{}, ErrTaskNotFound
		}
		return dto.CreateImageTaskResponse{}, err
	}
	if task.Status == model.ImageTaskStatusGenerating {
		return dto.CreateImageTaskResponse{}, ErrTaskStillGenerating
	}

	input, err := s.imageTaskInputFromRecord(task)
	if err != nil {
		return dto.CreateImageTaskResponse{}, err
	}

	return s.CreateImageTask(ctx, userID, dto.CreateImageTaskRequest{
		Prompt:            input.Prompt,
		Size:              input.Size,
		Style:             input.Style,
		Count:             input.Count,
		Quality:           input.Quality,
		OutputFormat:      input.OutputFormat,
		ReferenceImageURL: input.ReferenceImageURL,
	})
}

func (s *Service) processImageTask(taskID uint64, userID uint64, input imageTaskGenerationInput) {
	if s == nil {
		return
	}
	s.processImageTaskV2(taskID, userID, input)
}

// GetImageTask 获取用户自己的任务详情。
func (s *Service) GetImageTask(ctx context.Context, userID uint64, taskID string) (dto.ImageTaskDetailResponse, error) {
	return s.getImageTaskV2(ctx, userID, taskID)
}

// DownloadImageTaskImage 下载用户某个生图任务中的指定图片。
func (s *Service) DownloadImageTaskImage(ctx context.Context, userID uint64, taskID string, imageIndex int) ([]byte, string, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if imageIndex < 0 {
		return nil, "", "", ErrImageNotFound
	}

	id, err := strconv.ParseUint(strings.TrimSpace(taskID), 10, 64)
	if err != nil || id == 0 {
		return nil, "", "", ErrTaskNotFound
	}

	var task model.ImageTask
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", ErrTaskNotFound
		}
		return nil, "", "", err
	}

	imageURLs := imageURLsFromTask(task)
	if imageIndex >= len(imageURLs) || strings.TrimSpace(imageURLs[imageIndex]) == "" {
		return nil, "", "", ErrImageNotFound
	}
	if s.imageStore == nil {
		return nil, "", "", ErrImageDownloadFailed
	}

	data, contentType, ext, err := s.imageStore.DownloadStoredImage(ctx, imageURLs[imageIndex])
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: %v", ErrImageDownloadFailed, err)
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	ext = strings.TrimSpace(ext)
	if ext == "" {
		ext = ".png"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	fileName := fmt.Sprintf("image2-task-%d-%d%s", id, imageIndex+1, ext)
	return data, contentType, fileName, nil
}

// ListImageTasks 返回用户任务分页。
func (s *Service) ListImageTasks(ctx context.Context, userID uint64, page int, pageSize int) (dto.ListImageTasksResponse, error) {
	return s.listImageTasksV2(ctx, userID, page, pageSize)
}

// ListPackages 获取套餐列表。
func (s *Service) ListPackages(ctx context.Context, packageType string) dto.ListPackagesResponse {
	items, err := s.listBillingPackages(ctx, true, packageType)
	if err != nil {
		return dto.ListPackagesResponse{Items: nil}
	}
	result := make([]dto.PackageItem, 0, len(items))
	for _, item := range items {
		result = append(result, s.billingPackageToDTO(item))
	}
	return dto.ListPackagesResponse{Items: result}
}

// RedeemPoints 将积分兑换为福利次数。
func (s *Service) RedeemPoints(ctx context.Context, userID uint64, req dto.RedeemPointsRequest) (dto.RedeemPointsResponse, error) {
	return dto.RedeemPointsResponse{}, ErrPackageInvalid
}

func (s *Service) authResponse(user model.User) (dto.AuthResponse, error) {
	return s.authResponseWithTTL(user, s.cfg.AccessTokenTTL)
}

func (s *Service) authResponseWithTTL(user model.User, ttl time.Duration) (dto.AuthResponse, error) {
	token, exp, err := s.jwt.GenerateWithTTL(user.ID, user.Username, user.Email, user.TokenVersion, ttl)
	if err != nil {
		return dto.AuthResponse{}, err
	}

	return dto.AuthResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(exp).Seconds()),
		User:        userInfo(user),
	}, nil
}

func (s *Service) loadUser(userID uint64) (model.User, error) {
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, err
	}
	return user, nil
}

func (s *Service) emailExists(ctx context.Context, email string) (bool, error) {
	var exists int64
	if err := s.db.WithContext(ctx).
		Model(&model.User{}).
		Where("lower(email) = ?", normalizeEmail(email)).
		Count(&exists).Error; err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (s *Service) emailAvailable(ctx context.Context, email string) (bool, error) {
	exists, err := s.emailExists(ctx, email)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

func (s *Service) usernameAvailable(ctx context.Context, username string) (bool, error) {
	var exists int64
	if err := s.db.WithContext(ctx).
		Model(&model.User{}).
		Where("username = ?", normalizeUsername(username)).
		Count(&exists).Error; err != nil {
		return false, err
	}
	return exists == 0, nil
}

// TokenVersionView 读取当前账号会话版本，仅用于鉴权比对。
func (s *Service) TokenVersionView(ctx context.Context, userID uint64) (TokenVersionView, error) {
	user, err := s.loadUser(userID)
	if err != nil {
		return TokenVersionView{}, err
	}
	_ = ctx
	return TokenVersionView{TokenVersion: user.TokenVersion}, nil
}

func (s *Service) findUserByAccount(account string) (model.User, error) {
	var user model.User
	account = strings.TrimSpace(account)
	if strings.Contains(account, "@") {
		email := normalizeEmail(account)
		err := s.db.Where("lower(email) = ?", email).First(&user).Error
		return user, err
	}
	err := s.db.Where("username = ?", account).First(&user).Error
	return user, err
}

func (s *Service) lockUser(tx *gorm.DB, userID uint64) (model.User, error) {
	var user model.User
	query := tx.Where("id = ?", userID)
	if tx.Dialector.Name() == "mysql" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, err
	}
	return user, nil
}

func (s *Service) lockImageTask(tx *gorm.DB, taskID uint64, userID uint64) (model.ImageTask, error) {
	var task model.ImageTask
	query := tx.Where("id = ? AND user_id = ?", taskID, userID)
	if tx.Dialector.Name() == "mysql" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ImageTask{}, ErrTaskNotFound
		}
		return model.ImageTask{}, err
	}
	return task, nil
}

func (s *Service) rememberCheckin(ctx context.Context, userID uint64, at time.Time) error {
	if s.redis == nil {
		return nil
	}
	key := fmt.Sprintf("checkin:%d:%s", userID, at.In(s.location).Format("2006-01-02"))
	return s.redis.Set(ctx, key, "1", 48*time.Hour).Err()
}

func (s *Service) now() time.Time {
	return time.Now().In(s.location)
}

func (s *Service) startOfDay(t time.Time) time.Time {
	y, m, d := t.In(s.location).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, s.location)
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func dayKey(t time.Time) string {
	return t.Format("2006-01-02")
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeUsername(username string) string {
	return strings.TrimSpace(username)
}

func userInfo(user model.User) dto.UserInfo {
	role := user.Role
	if role == "" {
		role = model.UserRoleUser
	}
	return dto.UserInfo{
		ID:            strconv.FormatUint(user.ID, 10),
		Username:      user.Username,
		Email:         user.Email,
		Role:          string(role),
		PointsBalance: user.PointsBalance,
		FreeCredits:   user.FreeCredits,
		PaidCredits:   user.PaidCredits,
		TotalCredits:  user.FreeCredits + user.PaidCredits,
		CheckinStreak: user.CheckinStreak,
		LastCheckinAt: user.LastCheckinAt,
		LastLoginAt:   user.LastLoginAt,
		InviteCode:    user.InviteCode,
	}
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func comparePassword(hash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func generateTaskID(t time.Time) string {
	return fmt.Sprintf("task_%s_%d", t.Format("20060102150405"), time.Now().UnixNano()%10000)
}

func generateOrderNo(t time.Time) string {
	return fmt.Sprintf("ORD%s%d", t.Format("20060102150405"), time.Now().UnixNano()%10000)
}

func buildPlaceholderImageURL() string {
	text := url.QueryEscape("AI Image")
	return fmt.Sprintf("https://placehold.co/1024x1024/111827/8b5cf6/png?text=%s", text)
}

func normalizeImageCount(count int) int {
	if count <= 0 {
		return 1
	}
	if count > 6 {
		return 6
	}
	return count
}

func normalizeSupportedImageSize(size string) (string, error) {
	trimmed := strings.TrimSpace(size)
	switch trimmed {
	case "", "auto", "1024x1024", "1024x1536", "1536x1024", "2048x2048", "2048x1152", "3840x2160", "2160x3840":
		return trimmed, nil
	default:
		if isValidCustomImageSize(trimmed) {
			return trimmed, nil
		}
		return "", ErrInvalidImageSize
	}
}

func isValidCustomImageSize(size string) bool {
	const (
		maxSide   = 3840
		minPixels = 655360
		maxPixels = 8294400
	)

	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return false
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return false
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return false
	}
	if width <= 0 || width > maxSide || height <= 0 || height > maxSide {
		return false
	}
	pixels := width * height
	return pixels >= minPixels && pixels <= maxPixels
}

func isAllowedUploadImageType(contentType string) bool {
	switch strings.TrimSpace(strings.Split(contentType, ";")[0]) {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func (s *Service) findPackage(code string) (dto.PackageItem, bool) {
	for _, item := range s.ListPackages(context.Background(), "").Items {
		if item.Code == code {
			return item, true
		}
	}
	return dto.PackageItem{}, false
}

func imageURLsFromResult(result provider.GenerateResult) []string {
	urls := mergeStringURLs(nil, result.ImageURLs)
	urls = mergeStringURLs(urls, []string{result.ImageURL})
	return urls
}

func normalizeGeneratedImageURLs(result provider.GenerateResult, requestedCount int, referenceImageURL string) []string {
	urls := mergeStringURLs(nil, result.ImageURLs)
	if requestedCount <= 1 || len(urls) == 0 {
		urls = mergeStringURLs(urls, []string{result.ImageURL})
	}
	if len(urls) == 0 {
		return urls
	}

	ordered := make([]string, 0, len(urls))
	fallbackThumbnailURLs := make([]string, 0, len(urls))
	primaryURL := strings.TrimSpace(result.ImageURL)
	primaryInList := false
	primaryKey := canonicalGeneratedImageURLKey(primaryURL)
	if primaryKey != "" {
		for _, rawURL := range result.ImageURLs {
			if canonicalGeneratedImageURLKey(rawURL) == primaryKey {
				primaryInList = true
				break
			}
		}
	}
	primaryAdded := primaryURL != "" && !primaryInList && !isLikelyThumbnailURL(primaryURL)
	if primaryAdded {
		ordered = append(ordered, primaryURL)
	}
	for _, rawURL := range urls {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			continue
		}
		if isLikelyThumbnailURL(trimmed) {
			fallbackThumbnailURLs = append(fallbackThumbnailURLs, trimmed)
			continue
		}
		if primaryAdded && canonicalGeneratedImageURLKey(trimmed) == canonicalGeneratedImageURLKey(primaryURL) {
			continue
		}
		ordered = append(ordered, trimmed)
	}
	if len(ordered) > 0 {
		urls = ordered
	} else {
		urls = fallbackThumbnailURLs
	}

	referenceKey := canonicalGeneratedImageURLKey(referenceImageURL)
	seen := make(map[string]string, len(urls))
	filtered := make([]string, 0, len(urls))
	for _, rawURL := range urls {
		trimmed := strings.TrimSpace(rawURL)
		key := canonicalGeneratedImageURLKey(trimmed)
		if key == "" {
			continue
		}
		if referenceKey != "" && key == referenceKey {
			continue
		}
		if existing, ok := seen[key]; ok {
			if isLikelyThumbnailURL(existing) && !isLikelyThumbnailURL(trimmed) {
				for i := range filtered {
					if filtered[i] == existing {
						filtered[i] = trimmed
						break
					}
				}
				seen[key] = trimmed
			}
			continue
		}
		seen[key] = trimmed
		filtered = append(filtered, trimmed)
	}

	if requestedCount > 0 && len(filtered) > requestedCount {
		filtered = filtered[:requestedCount]
	}
	return filtered
}

func canonicalGeneratedImageURLKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	candidate := trimmed
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
		candidate = parsed.Path
	}
	candidate = strings.ReplaceAll(candidate, "\\", "/")
	candidate = strings.ReplaceAll(candidate, "/thumbs/", "/")
	candidate = path.Clean(candidate)
	if candidate == "." {
		return ""
	}
	return candidate
}

func isLikelyThumbnailURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if parsed, err := url.Parse(trimmed); err == nil {
		return strings.Contains(parsed.Path, "/thumbs/")
	}
	return strings.Contains(trimmed, "/thumbs/")
}

func referenceImageURLFromRequestParams(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var payload struct {
		ReferenceImageURL string `json:"referenceImageUrl"`
	}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.ReferenceImageURL)
}

func imageURLsFromTask(task model.ImageTask) []string {
	var urls []string
	if strings.TrimSpace(task.ImageURLs) != "" {
		_ = json.Unmarshal([]byte(task.ImageURLs), &urls)
	}
	urls = mergeStringURLs(urls, []string{task.ImageURL})
	urls = filterUsableTaskImageURLs(urls)
	return normalizeGeneratedImageURLs(provider.GenerateResult{ImageURLs: urls}, 0, "")
}

func thumbnailURLFromList(urls []string, index int) string {
	if index < 0 || index >= len(urls) {
		return ""
	}
	return urls[index]
}

func mergeStringURLs(base []string, next []string) []string {
	seen := make(map[string]struct{}, len(base)+len(next))
	urls := make([]string, 0, len(base)+len(next))
	for _, value := range append(base, next...) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		urls = append(urls, trimmed)
	}
	return urls
}

func filterUsableTaskImageURLs(urls []string) []string {
	filtered := make([]string, 0, len(urls))
	for _, rawURL := range urls {
		if isUsableTaskImageURL(rawURL) {
			filtered = append(filtered, strings.TrimSpace(rawURL))
		}
	}
	return filtered
}

func isUsableTaskImageURL(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return false
	}

	candidate := trimmed
	if parsed, err := url.Parse(trimmed); err == nil {
		if parsed.Path == "" {
			return false
		}
		candidate = parsed.Path
	}
	candidate = strings.ReplaceAll(candidate, "\\", "/")
	candidate = path.Clean(candidate)
	if candidate == "." {
		return false
	}
	if isKnownTaskImageAPIEndpoint(candidate) {
		return false
	}

	if isLikelyTaskImageFile(candidate) {
		return true
	}

	return strings.HasPrefix(candidate, "/results/") ||
		strings.HasPrefix(candidate, "/images/") ||
		strings.HasPrefix(candidate, "/uploads/") ||
		strings.HasPrefix(candidate, "/outputs/")
}

func isLikelyTaskImageFile(value string) bool {
	switch strings.ToLower(path.Ext(strings.TrimSpace(value))) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".tif", ".tiff", ".avif":
		return true
	default:
		return false
	}
}

func isKnownTaskImageAPIEndpoint(value string) bool {
	candidate := path.Clean("/" + strings.TrimLeft(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")), "/"))
	switch candidate {
	case "/images/edits", "/images/generations", "/images/variations",
		"/v1/images/edits", "/v1/images/generations", "/v1/images/variations":
		return true
	default:
		return false
	}
}

func (s *Service) completeImageTask(taskID uint64, userID uint64, result provider.GenerateResult) (int64, error) {
	var taskMeta model.ImageTask
	_ = s.db.Select("requested_count, request_params").Where("id = ? AND user_id = ?", taskID, userID).First(&taskMeta).Error
	referenceImageURL := referenceImageURLFromRequestParams(taskMeta.RequestParams)
	imageURLs := normalizeGeneratedImageURLs(result, taskMeta.RequestedCount, referenceImageURL)
	imageURLs = s.storeGeneratedImages(taskID, userID, imageURLs)
	requestedCount := taskMeta.RequestedCount
	if requestedCount <= 0 {
		requestedCount = 1
	}
	if len(imageURLs) == 0 {
		message := "生成图片转存到本站失败，请稍后重试"
		failResult := provider.GenerateResult{
			ProviderTaskID: result.ProviderTaskID,
			Status:         model.ImageTaskStatusFailed,
			RawResponse:    result.RawResponse,
			ErrorMessage:   message,
		}
		return s.failImageTaskAndRefund(taskID, userID, failResult, message)
	}

	imageURL := ""
	if imageURL == "" && len(imageURLs) > 0 {
		imageURL = imageURLs[0]
	}
	imageURLsJSON, _ := json.Marshal(imageURLs)
	imageElapsedSeconds := normalizeImageElapsedSeconds(result.ImageElapsedSeconds, len(imageURLs))
	imageElapsedSecondsJSON := marshalImageElapsedSeconds(imageElapsedSeconds)

	updates := map[string]any{
		"status":                model.ImageTaskStatusSuccess,
		"image_url":             imageURL,
		"image_urls":            string(imageURLsJSON),
		"image_elapsed_seconds": imageElapsedSecondsJSON,
		"provider_response":     result.RawResponse,
		"error_message":         "",
	}
	if providerTaskID := strings.TrimSpace(result.ProviderTaskID); providerTaskID != "" {
		updates["provider_task_id"] = providerTaskID
	}

	completed := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var task model.ImageTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND status = ?", taskID, userID, model.ImageTaskStatusGenerating).
			First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		if err := incrementImagesGeneratedTotal(tx, int64(len(imageURLs))); err != nil {
			return err
		}
		if err := tx.Model(&model.ImageTask{}).
			Where("id = ? AND user_id = ? AND status = ?", taskID, userID, model.ImageTaskStatusGenerating).
			Updates(updates).Error; err != nil {
			return err
		}
		completed = true
		return nil
	})
	if err != nil {
		return 0, err
	}
	if !completed {
		var task model.ImageTask
		if err := s.db.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, ErrTaskNotFound
			}
			return 0, err
		}
		user, err := s.loadUser(userID)
		if err != nil {
			return 0, err
		}
		return user.FreeCredits + user.PaidCredits, nil
	}

	user, err := s.loadUser(userID)
	if err != nil {
		return 0, err
	}
	s.notifyImageTaskSuccessAsync(taskID, userID)
	return user.FreeCredits + user.PaidCredits, nil
}

func (s *Service) storeGeneratedImages(taskID uint64, userID uint64, imageURLs []string) []string {
	if s == nil || s.imageStore == nil || !s.imageStore.Enabled() || len(imageURLs) == 0 {
		return imageURLs
	}
	if !s.hasRemoteImageURLs(imageURLs) {
		return imageURLs
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ImageStorageTimeout)
	defer cancel()

	keyPrefix := fmt.Sprintf("images/%s/user-%d/task-%d", s.now().Format("2006/01/02"), userID, taskID)
	storedURLs, err := s.imageStore.StoreRemoteImages(ctx, imageURLs, keyPrefix)
	if err != nil {
		log.Printf("任务 %d 图片转存部分失败，已保留原始 URL：%v", taskID, err)
	}
	finalURLs := make([]string, 0, len(storedURLs))
	fallbackCount := 0
	for _, storedURL := range storedURLs {
		if !s.isStoredImageURL(storedURL) {
			fallbackCount++
		}
		finalURLs = append(finalURLs, storedURL)
	}
	if fallbackCount > 0 {
		log.Printf("任务 %d 图片转存后仍保留 %d 张上游 URL", taskID, fallbackCount)
	}
	return finalURLs
}

func (s *Service) hasRemoteImageURLs(imageURLs []string) bool {
	for _, candidate := range imageURLs {
		if !s.isStoredImageURL(candidate) {
			return true
		}
	}
	return false
}

func (s *Service) isStoredImageURL(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return true
	}

	publicPath := strings.TrimRight(strings.TrimSpace(s.cfg.ImageStoragePublicPath), "/")
	if publicPath == "" {
		publicPath = "/uploads"
	}
	if strings.HasPrefix(trimmed, publicPath+"/") {
		return true
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return false
	}

	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.BackendPublicURL), "/")
	if baseURL == "" {
		return false
	}
	baseParsed, err := url.Parse(baseURL)
	if err != nil || baseParsed.Host == "" {
		return false
	}

	if !strings.EqualFold(parsed.Host, baseParsed.Host) {
		return false
	}
	return strings.HasPrefix(parsed.Path, publicPath+"/")
}

func (s *Service) referenceImageForProvider(ctx context.Context, referenceImageURL string) *provider.ReferenceImage {
	trimmed := strings.TrimSpace(referenceImageURL)
	if trimmed == "" || s == nil || s.imageStore == nil {
		return nil
	}

	data, contentType, ext, err := s.imageStore.DownloadStoredImage(ctx, trimmed)
	if err != nil {
		log.Printf("读取本地参考图失败，将回退为 URL 下载: %v", err)
		return nil
	}
	if len(data) == 0 || !isAllowedUploadImageType(contentType) {
		return nil
	}

	name := "reference-image" + ext
	if parsed, err := url.Parse(trimmed); err == nil {
		if base := path.Base(parsed.Path); base != "." && base != "/" && strings.TrimSpace(base) != "" {
			name = base
		}
	}
	return &provider.ReferenceImage{
		Name:        name,
		ContentType: contentType,
		Data:        data,
	}
}

func (s *Service) referenceImagesForProvider(ctx context.Context, referenceImageURLs []string) []provider.ReferenceImage {
	images := make([]provider.ReferenceImage, 0, len(referenceImageURLs))
	for _, imageURL := range referenceImageURLs {
		image := s.referenceImageForProvider(ctx, imageURL)
		if image != nil {
			images = append(images, *image)
		}
	}
	return images
}

func (s *Service) failImageTaskAndRefund(taskID uint64, userID uint64, result provider.GenerateResult, message string) (int64, error) {
	return s.failImageTaskAndRefundV2(taskID, userID, result, message)
}

func providerFailureMessage(result provider.GenerateResult, err error) string {
	return providerFailureMessageForAudience(result, err, true)
}

func providerFailureAdminMessage(result provider.GenerateResult, err error) string {
	return providerFailureMessageForAudience(result, err, false)
}

func providerFailureMessageForAudience(result provider.GenerateResult, err error, userFacing bool) string {
	message := strings.TrimSpace(result.ErrorMessage)
	if message == "" && errors.Is(err, context.DeadlineExceeded) {
		if userFacing {
			message = "图片生成超时"
		} else {
			message = "upstream timeout"
		}
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	if message == "" {
		if userFacing {
			message = "图片生成失败"
		} else {
			message = "image generation failed"
		}
	}
	message = normalizeProviderFailureMessage(message, userFacing)
	return truncateProviderMessage(message)
}

func normalizeProviderFailureMessage(message string, userFacing bool) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		if userFacing {
			return "图片生成失败"
		}
		return "image generation failed"
	}

	if !userFacing {
		return trimmed
	}

	lower := strings.ToLower(trimmed)
	model := extractProviderModel(trimmed)
	code := extractProviderErrorCode(trimmed)

	switch {
	case isProviderProtocolMismatchError(lower):
		return "生图服务暂时连不上，我们已经记录这个问题，请稍后再试。"
	case isProviderCooldownError(trimmed):
		if model == "" {
			return "现在生成的人有点多，这个模型正在排队，等一会儿再试通常就好了。"
		}
		return fmt.Sprintf("当前模型 %s 有点忙，等一会儿再试试。", model)
	case code == "model_not_found" || strings.Contains(lower, "no available channel") || strings.Contains(lower, "model not found"):
		if model == "" {
			return "这个生图模型暂时不可用，我们已经记录这个问题，请稍后再试。"
		}
		return fmt.Sprintf("模型 %s 暂时不可用，我们已经记录这个问题，请稍后再试。", model)
	case code == "insufficient_user_quota" || code == "insufficient_quota" || code == "billing_not_active" || isProviderBalanceError(trimmed):
		return "当前生图通道额度有点紧张，系统会自动切换，稍后再试一下就好。"
	case code == "invalid_api_key" || code == "invalid_token" || strings.Contains(lower, "invalid token") || strings.Contains(lower, "incorrect api key") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "http 401") || strings.Contains(lower, "http 403"):
		return "当前生图通道暂时不可用，我们已经记录这个问题，请稍后再试。"
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") || strings.Contains(lower, "http 429"):
		return "现在生成的人有点多，稍等一会儿再试试。"
	case isProviderContentRejectedError(lower):
		return "这次描述可能有点敏感，换个更具体、温和一点的说法再试试。"
	case isProviderTimeoutError(lower):
		return "这次生成超时了，不是你的问题，稍后再试一次就行。"
	case isProviderReferenceImageError(lower):
		return "参考图这次没处理成功，换一张更清晰一点的图再试试。"
	case isProviderTemporaryServerError(lower):
		return "生图服务刚刚有点不稳定，等一会儿再试试就好。"
	case isProviderNetworkError(lower):
		return "生图服务暂时连不上，等一会儿再试试。"
	case strings.Contains(lower, "unsupported size") || strings.Contains(lower, "invalid size") || strings.Contains(lower, "size is not supported") || strings.Contains(lower, "invalid image size"):
		return "这个尺寸这次没跑通，试着降低尺寸再生成一次会更稳。"
	case strings.Contains(lower, "unsupported quality") || strings.Contains(lower, "invalid quality"):
		return "这个生成质量这次没跑通，试着降低生成质量再试试。"
	case strings.Contains(lower, "unsupported output") || strings.Contains(lower, "invalid output") || strings.Contains(lower, "output format"):
		return "这个导出格式暂时不支持，先用默认格式试试。"
	case strings.Contains(lower, "count") && (strings.Contains(lower, "invalid") || strings.Contains(lower, "unsupported") || strings.Contains(lower, "too many")):
		return "这次生成张数有点多，先改少一点再试试。"
	case strings.Contains(lower, "http 400") || strings.Contains(lower, "bad request") || strings.Contains(lower, "invalid request"):
		return "这次生成参数没跑通。试着降低尺寸或生成质量再试一次；如果带了参考图，也可以先去掉再试试。"
	}

	return "这次生成失败了，我们已经记录问题。换个简单一点的设置再试试，通常能恢复。"
}

func (s *Service) notifyImageTaskFailureAsync(notice imageTaskFailureNotice) {
	if s == nil || s.mailer == nil || !s.mailer.Configured() {
		return
	}

	if !s.shouldSendImageFailureNotice(context.Background(), notice) {
		return
	}

	recipients := []string{"843553677@qq.com"}
	subject, textBody := buildImageFailureEmail(notice)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		for _, to := range recipients {
			if err := s.mailer.SendPlainText(ctx, to, subject, textBody); err != nil {
				log.Printf("发送生图失败通知邮件失败: to=%s task=%d err=%v", to, notice.TaskID, err)
			}
		}
	}()
}

func (s *Service) shouldSendImageFailureNotice(ctx context.Context, notice imageTaskFailureNotice) bool {
	if isSevereImageFailureNotice(notice) {
		return true
	}
	category := imageFailureNoticeCategory(notice)
	if category == "" {
		category = "general"
	}
	return s.acquireEmailNoticeCooldown(ctx, "image_failure:"+category, time.Hour)
}

func isSevereImageFailureNotice(notice imageTaskFailureNotice) bool {
	text := strings.ToLower(strings.TrimSpace(notice.Message + " " + notice.RawResponse))
	if text == "" {
		return false
	}
	return strings.Contains(text, "api key") ||
		strings.Contains(text, "invalid token") ||
		strings.Contains(text, "invalid_api_key") ||
		strings.Contains(text, "unauthorized") ||
		strings.Contains(text, "http 401") ||
		strings.Contains(text, "http 403") ||
		strings.Contains(text, "余额") ||
		strings.Contains(text, "额度不足") ||
		strings.Contains(text, "insufficient_quota") ||
		strings.Contains(text, "billing_not_active") ||
		strings.Contains(text, "模型通道") ||
		strings.Contains(text, "model_not_found") ||
		strings.Contains(text, "model not found") ||
		strings.Contains(text, "no available channel") ||
		strings.Contains(text, "长时间未完成")
}

func imageFailureNoticeCategory(notice imageTaskFailureNotice) string {
	text := strings.ToLower(strings.TrimSpace(notice.Message + " " + notice.RawResponse))
	switch {
	case strings.Contains(text, "http 500") ||
		strings.Contains(text, "http 502") ||
		strings.Contains(text, "http 503") ||
		strings.Contains(text, "http 504") ||
		strings.Contains(text, "stream disconnected") ||
		strings.Contains(text, "internal_error") ||
		strings.Contains(text, "temporarily unavailable") ||
		strings.Contains(text, "暂时不可用"):
		return "temporary_upstream"
	case strings.Contains(text, "参考图") ||
		strings.Contains(text, "reference image"):
		return "reference_image"
	case strings.Contains(text, "尺寸") ||
		strings.Contains(text, "质量") ||
		strings.Contains(text, "格式") ||
		strings.Contains(text, "张数") ||
		strings.Contains(text, "invalid request") ||
		strings.Contains(text, "bad request") ||
		strings.Contains(text, "http 400"):
		return "user_params"
	case strings.Contains(text, "内容") ||
		strings.Contains(text, "content"):
		return "content_rejected"
	case strings.Contains(text, "图片地址") ||
		strings.Contains(text, "image url"):
		return "missing_image_url"
	default:
		return "general"
	}
}

func (s *Service) acquireEmailNoticeCooldown(ctx context.Context, key string, ttl time.Duration) bool {
	if ttl <= 0 {
		return true
	}
	key = "email_notice_cooldown:" + strings.TrimSpace(key)
	if s.redis != nil {
		ok, err := s.redis.SetNX(ctx, key, "1", ttl).Result()
		if err == nil {
			return ok
		}
		log.Printf("记录邮件通知冷却失败，将使用本地冷却: %v", err)
	}

	now := time.Now().In(s.location)
	s.emailNoticeMu.Lock()
	defer s.emailNoticeMu.Unlock()
	if until, ok := s.emailNotices[key]; ok && until.After(now) {
		return false
	}
	s.emailNotices[key] = now.Add(ttl)
	return true
}

func buildImageFailureEmail(notice imageTaskFailureNotice) (string, string) {
	subject := fmt.Sprintf("image2 生图失败通知：任务 %d", notice.TaskID)
	var b strings.Builder
	b.WriteString("检测到一条生图任务失败。\n\n")
	fmt.Fprintf(&b, "任务ID：%d\n", notice.TaskID)
	fmt.Fprintf(&b, "用户ID：%d\n", notice.UserID)
	if strings.TrimSpace(notice.Provider) != "" {
		fmt.Fprintf(&b, "服务商：%s\n", notice.Provider)
	}
	if strings.TrimSpace(notice.ProviderTaskID) != "" {
		fmt.Fprintf(&b, "上游任务ID：%s\n", notice.ProviderTaskID)
	}
	if !notice.CreatedAt.IsZero() {
		fmt.Fprintf(&b, "创建时间：%s\n", notice.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	if strings.TrimSpace(notice.Prompt) != "" {
		b.WriteString("\n提示词：\n")
		b.WriteString(notice.Prompt)
		b.WriteString("\n")
	}
	adminMessage := strings.TrimSpace(notice.AdminMessage)
	if adminMessage == "" {
		adminMessage = strings.TrimSpace(notice.Message)
	}
	if adminMessage != "" {
		b.WriteString("\n失败原因：\n")
		b.WriteString(adminMessage)
		b.WriteString("\n")
	}
	if strings.TrimSpace(notice.RawResponse) != "" {
		b.WriteString("\n上游原始返回：\n")
		b.WriteString(notice.RawResponse)
		b.WriteString("\n")
	}
	return subject, b.String()
}

func dedupeEmails(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		email := normalizeEmail(item)
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		result = append(result, email)
	}
	return result
}

func extractProviderModel(message string) string {
	lower := strings.ToLower(message)
	start := strings.Index(lower, "model ")
	if start < 0 {
		return ""
	}
	value := strings.TrimSpace(message[start+len("model "):])
	if value == "" {
		return ""
	}
	for _, marker := range []string{" are cooling", " is cooling", ";", ",", "；", "，"} {
		if idx := strings.Index(strings.ToLower(value), marker); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
	}
	return strings.Trim(value, `"'`)
}

func extractProviderErrorCode(message string) string {
	lower := strings.ToLower(message)
	marker := "code="
	start := strings.Index(lower, marker)
	if start < 0 {
		return ""
	}
	value := strings.TrimSpace(lower[start+len(marker):])
	if value == "" {
		return ""
	}
	for _, separator := range []string{";", ",", " ", ")", "；", "，"} {
		if idx := strings.Index(value, separator); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
	}
	return strings.Trim(value, `"'`)
}

func isProviderProtocolMismatchError(lower string) bool {
	return strings.Contains(lower, "plain http request was sent to https port") ||
		strings.Contains(lower, "server gave http response to https client") ||
		strings.Contains(lower, "http response to https client")
}

func isProviderContentRejectedError(lower string) bool {
	return strings.Contains(lower, "content_policy") ||
		strings.Contains(lower, "safety") ||
		strings.Contains(lower, "moderation") ||
		strings.Contains(lower, "refused prompt") ||
		strings.Contains(lower, "policy violation") ||
		strings.Contains(lower, "sensitive content")
}

func isProviderTimeoutError(lower string) bool {
	return strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "deadline exceeded") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "client.timeout exceeded")
}

func isProviderReferenceImageError(lower string) bool {
	return strings.Contains(lower, "reference image") ||
		strings.Contains(lower, "image too large") ||
		strings.Contains(lower, "unsupported image") ||
		strings.Contains(lower, "invalid image") ||
		strings.Contains(lower, "download reference image") ||
		strings.Contains(lower, "read reference image")
}

func isProviderNetworkError(lower string) bool {
	return strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "lookup ") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "unexpected eof") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "network is unreachable")
}

func isProviderTemporaryServerError(lower string) bool {
	return strings.Contains(lower, "http 500") ||
		strings.Contains(lower, "http 502") ||
		strings.Contains(lower, "http 503") ||
		strings.Contains(lower, "http 504") ||
		strings.Contains(lower, "status_code=500") ||
		strings.Contains(lower, "status_code=502") ||
		strings.Contains(lower, "status_code=503") ||
		strings.Contains(lower, "status_code=504") ||
		strings.Contains(lower, "internal_server_error") ||
		strings.Contains(lower, "internal_error") ||
		strings.Contains(lower, "server_error") ||
		strings.Contains(lower, "stream error") ||
		strings.Contains(lower, "stream disconnected") ||
		strings.Contains(lower, "bad gateway") ||
		strings.Contains(lower, "service unavailable")
}

const imageTaskFailureUserNotice = "本次消耗的积分或福利次数已原路退回；失败信息已邮件通知管理员，我们会尽快处理。"
const imageTaskFailureUserNoticeSimple = "这次没生成成功，消耗已经退回，不用担心。我们也记录了这次问题，会继续处理。"

func buildImageTaskFailureUserMessage(message string) string {
	base := strings.TrimSpace(message)
	if base == "" {
		base = "这次图片还没生成成功"
	}
	base = strings.TrimSuffix(base, "。")
	notice := imageTaskFailureUserNoticeSimple
	if strings.Contains(base, notice) {
		return truncateRunes(base, 500)
	}

	maxBaseRunes := 500 - len([]rune(notice)) - 1
	if maxBaseRunes < 0 {
		maxBaseRunes = 0
	}
	base = truncateRunes(base, maxBaseRunes)
	if base == "" {
		return truncateRunes(notice, 500)
	}
	return base + "。" + notice
}

func buildImageTaskPartialSuccessUserMessage(message string, succeeded int, requested int) string {
	if requested <= 0 {
		requested = 1
	}
	if succeeded < 0 {
		succeeded = 0
	}
	if succeeded > requested {
		succeeded = requested
	}
	failed := requested - succeeded
	base := strings.TrimSpace(message)
	if base == "" {
		base = "有几张图片这次还没生成出来"
	}
	base = strings.TrimSuffix(base, "。")
	notice := fmt.Sprintf("已生成 %d/%d 张图片", succeeded, requested)
	if failed > 0 {
		notice += fmt.Sprintf("，还有 %d 张还没完成", failed)
	}
	notice += "；未完成部分的积分或福利次数已原路退回，失败信息已邮件通知管理员。"
	if strings.Contains(base, notice) {
		return truncateRunes(base, 500)
	}

	maxBaseRunes := 500 - len([]rune(notice)) - 1
	if maxBaseRunes < 0 {
		maxBaseRunes = 0
	}
	base = truncateRunes(base, maxBaseRunes)
	if base == "" {
		return truncateRunes(notice, 500)
	}
	return base + "。" + notice
}

func truncateProviderMessage(message string) string {
	return truncateRunes(message, 240)
}

func truncateRunes(message string, max int) string {
	runes := []rune(strings.TrimSpace(message))
	if max <= 0 || len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

// RecoverPendingImageTasks 恢复数据库中仍处于生成中的生图任务。
func (s *Service) RecoverPendingImageTasks(ctx context.Context) error {
	if s == nil {
		return nil
	}
	return s.RecoverPendingImageTasksV2(ctx)
}

func (s *Service) launchImageTask(taskID uint64, userID uint64, input imageTaskGenerationInput) bool {
	if s == nil {
		return false
	}
	return s.launchImageTaskV2(taskID, userID, input)
}

func (s *Service) claimImageTask(taskID uint64) bool {
	s.activeTasksMu.Lock()
	defer s.activeTasksMu.Unlock()

	if _, exists := s.activeTasks[taskID]; exists {
		return false
	}
	s.activeTasks[taskID] = struct{}{}
	return true
}

func (s *Service) releaseImageTask(taskID uint64) {
	s.activeTasksMu.Lock()
	delete(s.activeTasks, taskID)
	s.activeTasksMu.Unlock()
}

func (s *Service) imageTaskInputFromRecord(task model.ImageTask) (imageTaskGenerationInput, error) {
	var input imageTaskGenerationInput
	if strings.TrimSpace(task.RequestParams) != "" {
		if err := json.Unmarshal([]byte(task.RequestParams), &input); err == nil {
			input.Prompt = strings.TrimSpace(input.Prompt)
			input.Size = strings.TrimSpace(input.Size)
			input.Style = strings.TrimSpace(input.Style)
			input.Count = normalizeImageCount(input.Count)
			input.Quality = strings.TrimSpace(input.Quality)
			input.OutputFormat = strings.TrimSpace(input.OutputFormat)
			normalizedReferenceImageURL, err := s.normalizeReferenceImageURL(task.UserID, strings.TrimSpace(input.ReferenceImageURL))
			if err != nil {
				return imageTaskGenerationInput{}, err
			}
			input.ReferenceImageURL = normalizedReferenceImageURL
			if input.Prompt != "" {
				return input, nil
			}
		}
	}

	prompt := strings.TrimSpace(task.Prompt)
	if prompt == "" {
		return imageTaskGenerationInput{}, fmt.Errorf("任务 %d 缺少提示词快照", task.ID)
	}

	count := 1
	if task.CostCredits > 0 {
		count = int(task.CostCredits)
	} else if s.cfg.ImageCostPoints > 0 && task.CostPoints > 0 {
		count = int(task.CostPoints / s.cfg.ImageCostPoints)
		if count <= 0 {
			count = 1
		}
	}

	return imageTaskGenerationInput{
		Prompt: prompt,
		Count:  normalizeImageCount(count),
	}, nil
}

func (s *Service) normalizeReferenceImageURL(userID uint64, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", nil
	}
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", ErrInvalidReferenceImageURL
		}
		base, err := url.Parse(strings.TrimRight(strings.TrimSpace(s.cfg.BackendPublicURL), "/"))
		if err == nil && base.Scheme != "" && base.Host != "" && strings.EqualFold(parsed.Scheme, base.Scheme) && strings.EqualFold(parsed.Host, base.Host) {
			return rawURL, nil
		}
		if isSafeExternalReferenceImageURL(parsed) {
			return rawURL, nil
		}
		return "", ErrInvalidReferenceImageURL
	}
	if strings.HasPrefix(rawURL, "/") {
		if !s.isAllowedReferenceImagePath(userID, rawURL) {
			return "", ErrInvalidReferenceImageURL
		}
		base := strings.TrimRight(strings.TrimSpace(s.cfg.BackendPublicURL), "/")
		if base != "" {
			return base + rawURL, nil
		}
	}
	return "", ErrInvalidReferenceImageURL
}

func isSafeExternalReferenceImageURL(parsed *url.URL) bool {
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return false
	}
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return isPublicReferenceIP(ip)
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !isPublicReferenceIP(ip) {
			return false
		}
	}
	return true
}

func isPublicReferenceIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return !ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast() &&
		!ip.IsUnspecified()
}

func (s *Service) normalizeReferenceImageURLs(userID uint64, primary string, urls []string) ([]string, error) {
	candidates := make([]string, 0, len(urls)+1)
	if strings.TrimSpace(primary) != "" {
		candidates = append(candidates, primary)
	}
	candidates = append(candidates, urls...)
	seen := make(map[string]struct{}, len(candidates))
	normalized := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		url, err := s.normalizeReferenceImageURL(userID, strings.TrimSpace(candidate))
		if err != nil {
			return nil, err
		}
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		normalized = append(normalized, url)
		if len(normalized) > 12 {
			return nil, ErrInvalidReferenceImageURL
		}
	}
	return normalized, nil
}

func (s *Service) isAllowedReferenceImagePath(userID uint64, rawPath string) bool {
	return s.isUserOwnedUploadPath(userID, rawPath, "references")
}

func (s *Service) isAllowedPromptPreviewPath(userID uint64, rawPath string) bool {
	return s.isUserOwnedUploadPath(userID, rawPath, "prompt-plaza-previews", "references")
}

func (s *Service) isUserOwnedUploadPath(userID uint64, rawPath string, categories ...string) bool {
	if userID == 0 {
		return false
	}
	publicPath := strings.TrimRight(strings.TrimSpace(s.cfg.ImageStoragePublicPath), "/")
	if publicPath == "" {
		publicPath = "/uploads"
	}
	normalizedPath := path.Clean("/" + strings.TrimLeft(rawPath, "/"))
	publicRoot := strings.TrimRight(publicPath, "/")
	if normalizedPath == publicRoot || !strings.HasPrefix(normalizedPath, publicRoot+"/") {
		return false
	}
	relative := strings.TrimPrefix(normalizedPath, publicRoot+"/")
	userPart := fmt.Sprintf("user-%d/", userID)
	for _, category := range categories {
		category = strings.Trim(strings.TrimSpace(category), "/")
		if category == "" {
			continue
		}
		if strings.HasPrefix(relative, "users/"+userPart+category+"/") {
			return true
		}
		if category == "references" && strings.HasPrefix(relative, "users/") && strings.Contains(relative, "/references/") {
			return true
		}
		if category == "references" && strings.HasPrefix(relative, "reference-images/"+userPart) {
			return true
		}
		if category == "references" && strings.HasPrefix(relative, "reference-images/") {
			return true
		}
	}
	return false
}

// StartImageTaskMaintenance 启动后台巡检，定期恢复生成中的任务。
func (s *Service) StartImageTaskMaintenance(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweepCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := s.RecoverPendingImageTasks(sweepCtx); err != nil {
					log.Printf("后台恢复生图任务失败: %v", err)
				}
				cancel()
			}
		}
	}()
}

// RecoverPendingImageTasksV2 恢复数据库中仍处于生成中的生图任务。
func (s *Service) RecoverPendingImageTasksV2(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var tasks []model.ImageTask
	if err := s.db.WithContext(ctx).
		Where("status = ?", model.ImageTaskStatusGenerating).
		Order("created_at asc").
		Find(&tasks).Error; err != nil {
		return err
	}

	if len(tasks) == 0 {
		log.Printf("没有需要恢复的生图任务")
		return nil
	}

	log.Printf("发现 %d 个待恢复的生图任务", len(tasks))
	for _, task := range tasks {
		if s.isImageTaskActive(task.ID) {
			log.Printf("任务 %d 当前已在处理中，跳过恢复", task.ID)
			continue
		}

		if s.isImageTaskStale(task) {
			message := "任务长时间未完成，自动标记为失败"
			balance, failErr := s.failImageTaskAndRefund(task.ID, task.UserID, provider.GenerateResult{ProviderTaskID: strings.TrimSpace(task.ProviderTaskID)}, message)
			if failErr != nil {
				log.Printf("任务 %d 无法标记为失败: %v", task.ID, failErr)
				continue
			}
			log.Printf("任务 %d 已超时并退款，当前余额 %d", task.ID, balance)
			continue
		}

		batchID := strings.TrimSpace(task.ProviderTaskID)
		if batchID == "" {
			log.Printf("任务 %d 仍未拿到第三方批次ID，暂不恢复", task.ID)
			continue
		}

		if s.launchRecoveredImageTaskV2(task.ID, task.UserID, batchID) {
			log.Printf("任务 %d 已重新进入轮询", task.ID)
		} else {
			log.Printf("任务 %d 已在处理中，跳过重复恢复", task.ID)
		}
	}

	return nil
}

func (s *Service) launchImageTaskV2(taskID uint64, userID uint64, input imageTaskGenerationInput) bool {
	if !s.claimImageTask(taskID) {
		return false
	}

	go s.processImageTaskV2(taskID, userID, input)
	return true
}

func (s *Service) launchRecoveredImageTaskV2(taskID uint64, userID uint64, batchID string) bool {
	if !s.claimImageTask(taskID) {
		return false
	}

	go s.resumeImageTaskV2(taskID, userID, batchID)
	return true
}

func shouldUseSingleImageProviderBatches(input imageTaskGenerationInput) bool {
	return normalizeImageCount(input.Count) > 1
}

func (s *Service) processImageTaskInSingleImageBatchesV2(taskID uint64, userID uint64, input imageTaskGenerationInput) {
	requestedCount := normalizeImageCount(input.Count)
	referenceCtx, referenceCancel := context.WithTimeout(context.Background(), s.cfg.ImageStorageTimeout)
	referenceImages := s.referenceImagesForProvider(referenceCtx, input.ReferenceImageURLs)
	referenceCancel()

	aggregate := provider.GenerateResult{
		Status: model.ImageTaskStatusGenerating,
	}
	for index := 0; index < requestedCount; index++ {
		imageStartedAt := time.Now()
		submitCtx, submitCancel := context.WithTimeout(context.Background(), s.cfg.ImageProviderTimeout)
		submitted, activeProvider, providerKeyID, submitErr := s.submitImageTaskWithProviderPool(submitCtx, taskID, userID, provider.GenerateRequest{
			Prompt:             input.Prompt,
			Size:               input.Size,
			Style:              input.Style,
			Count:              1,
			Quality:            input.Quality,
			OutputFormat:       input.OutputFormat,
			ReferenceImageURL:  input.ReferenceImageURL,
			ReferenceImageURLs: input.ReferenceImageURLs,
			ReferenceImages:    referenceImages,
		})
		submitCancel()
		currentProviderTaskID := strings.TrimSpace(submitted.ProviderTaskID)
		aggregate = mergeGenerateResult(aggregate, submitted)

		if submitErr != nil {
			message := providerFailureMessage(submitted, submitErr)
			aggregate.Status = model.ImageTaskStatusFailed
			aggregate.ErrorMessage = message
			balance, updateErr := s.failImageTaskAndRefund(taskID, userID, aggregate, message)
			if updateErr != nil {
				log.Printf("多图生图任务 %d 第 %d/%d 张提交失败处理异常: %v", taskID, index+1, requestedCount, updateErr)
				return
			}
			log.Printf("多图生图任务 %d 第 %d/%d 张提交失败，已按完成张数结算，余额 %d，原因: %s", taskID, index+1, requestedCount, balance, message)
			return
		}

		if err := s.recordImageTaskDispatch(taskID, userID, aggregate, providerKeyID); err != nil {
			message := "保存第三方任务信息失败: " + err.Error()
			aggregate.Status = model.ImageTaskStatusFailed
			aggregate.ErrorMessage = message
			balance, updateErr := s.failImageTaskAndRefund(taskID, userID, aggregate, message)
			if updateErr != nil {
				log.Printf("多图生图任务 %d 保存第 %d/%d 张任务信息失败后退款异常: %v", taskID, index+1, requestedCount, updateErr)
				return
			}
			log.Printf("多图生图任务 %d 保存第 %d/%d 张任务信息失败，已按完成张数结算，余额 %d", taskID, index+1, requestedCount, balance)
			return
		}

		if isTemporaryProviderResultFailure(submitted) && strings.TrimSpace(submitted.ProviderTaskID) != "" {
			submitted.Status = model.ImageTaskStatusGenerating
		}

		var result provider.GenerateResult
		switch submitted.Status {
		case model.ImageTaskStatusSuccess:
			if strings.TrimSpace(submitted.ImageURL) == "" {
				message := "第三方服务没有返回可用的图片地址"
				aggregate.Status = model.ImageTaskStatusFailed
				aggregate.ErrorMessage = message
				balance, updateErr := s.failImageTaskAndRefund(taskID, userID, aggregate, message)
				if updateErr != nil {
					log.Printf("多图生图任务 %d 第 %d/%d 张成功态但缺少图片地址处理异常: %v", taskID, index+1, requestedCount, updateErr)
					return
				}
				log.Printf("多图生图任务 %d 第 %d/%d 张未返回图片地址，已按完成张数结算，余额 %d", taskID, index+1, requestedCount, balance)
				return
			}
			result = submitted
		case model.ImageTaskStatusFailed:
			message := providerFailureMessage(submitted, nil)
			aggregate.Status = model.ImageTaskStatusFailed
			aggregate.ErrorMessage = message
			balance, updateErr := s.failImageTaskAndRefund(taskID, userID, aggregate, message)
			if updateErr != nil {
				log.Printf("多图生图任务 %d 第 %d/%d 张失败处理异常: %v", taskID, index+1, requestedCount, updateErr)
				return
			}
			log.Printf("多图生图任务 %d 第 %d/%d 张失败，已按完成张数结算，余额 %d，原因: %s", taskID, index+1, requestedCount, balance, message)
			return
		default:
			batchID := currentProviderTaskID
			if batchID == "" {
				message := "第三方服务没有返回可用于恢复的批次ID"
				aggregate.Status = model.ImageTaskStatusFailed
				aggregate.ErrorMessage = message
				balance, updateErr := s.failImageTaskAndRefund(taskID, userID, aggregate, message)
				if updateErr != nil {
					log.Printf("多图生图任务 %d 第 %d/%d 张缺少批次ID处理异常: %v", taskID, index+1, requestedCount, updateErr)
					return
				}
				log.Printf("多图生图任务 %d 第 %d/%d 张缺少批次ID，已按完成张数结算，余额 %d", taskID, index+1, requestedCount, balance)
				return
			}

			pollCtx, pollCancel := context.WithTimeout(context.Background(), s.cfg.ImageProviderTimeout)
			var pollErr error
			result, pollErr = s.pollImageTaskV2(pollCtx, taskID, userID, 1, activeProvider, batchID, submitted)
			pollCancel()
			if pollErr != nil {
				message := providerFailureMessage(result, pollErr)
				aggregate = mergeGenerateResult(aggregate, result)
				aggregate.Status = model.ImageTaskStatusFailed
				aggregate.ErrorMessage = message
				balance, updateErr := s.failImageTaskAndRefund(taskID, userID, aggregate, message)
				if updateErr != nil {
					log.Printf("多图生图任务 %d 第 %d/%d 张轮询失败处理异常: %v", taskID, index+1, requestedCount, updateErr)
					return
				}
				log.Printf("多图生图任务 %d 第 %d/%d 张轮询失败，已按完成张数结算，余额 %d，原因: %s", taskID, index+1, requestedCount, balance, message)
				return
			}
			if result.Status != model.ImageTaskStatusSuccess || strings.TrimSpace(result.ImageURL) == "" {
				message := providerFailureMessage(result, nil)
				aggregate = mergeGenerateResult(aggregate, result)
				aggregate.Status = model.ImageTaskStatusFailed
				aggregate.ErrorMessage = message
				balance, updateErr := s.failImageTaskAndRefund(taskID, userID, aggregate, message)
				if updateErr != nil {
					log.Printf("多图生图任务 %d 第 %d/%d 张未成功处理异常: %v", taskID, index+1, requestedCount, updateErr)
					return
				}
				log.Printf("多图生图任务 %d 第 %d/%d 张未成功，已按完成张数结算，余额 %d，原因: %s", taskID, index+1, requestedCount, balance, message)
				return
			}
		}

		result.ImageElapsedSeconds = []int64{elapsedSecondsSince(imageStartedAt)}
		aggregate = mergeGenerateResult(aggregate, result)
		aggregate.Status = model.ImageTaskStatusGenerating
		if err := s.persistPartialImageTask(context.Background(), taskID, userID, aggregate); err != nil {
			log.Printf("多图生图任务 %d 保存第 %d/%d 张结果失败: %v", taskID, index+1, requestedCount, err)
		}
		log.Printf("多图生图任务 %d 已完成第 %d/%d 张", taskID, index+1, requestedCount)
	}

	aggregate.Status = model.ImageTaskStatusSuccess
	balance, err := s.completeImageTask(taskID, userID, aggregate)
	if err != nil {
		log.Printf("多图生图任务 %d 完成写库失败: %v", taskID, err)
		return
	}
	log.Printf("多图生图任务 %d 已逐张完成 %d/%d，当前余额 %d", taskID, requestedCount, requestedCount, balance)
}

func (s *Service) processImageTaskV2(taskID uint64, userID uint64, input imageTaskGenerationInput) {
	defer s.releaseImageTask(taskID)

	if shouldUseSingleImageProviderBatches(input) {
		s.processImageTaskInSingleImageBatchesV2(taskID, userID, input)
		return
	}

	imageStartedAt := time.Now()
	if !s.claimImageTaskSubmitInDB(context.Background(), taskID, userID) {
		log.Printf("异步生图任务 %d 已有提交中标记，跳过重复提交", taskID)
		return
	}
	submitCtx, submitCancel := context.WithTimeout(context.Background(), s.cfg.ImageProviderTimeout)
	submitted, activeProvider, providerKeyID, submitErr := s.submitImageTaskWithProviderPool(submitCtx, taskID, userID, provider.GenerateRequest{
		Prompt:             input.Prompt,
		Size:               input.Size,
		Style:              input.Style,
		Count:              input.Count,
		Quality:            input.Quality,
		OutputFormat:       input.OutputFormat,
		ReferenceImageURL:  input.ReferenceImageURL,
		ReferenceImageURLs: input.ReferenceImageURLs,
		ReferenceImages:    s.referenceImagesForProvider(submitCtx, input.ReferenceImageURLs),
	})
	submitCancel()

	if submitErr != nil {
		message := providerFailureMessage(submitted, submitErr)
		balance, updateErr := s.failImageTaskAndRefund(taskID, userID, submitted, message)
		if updateErr != nil {
			log.Printf("异步生图任务 %d 提交失败处理异常: %v", taskID, updateErr)
			return
		}
		log.Printf("异步生图任务 %d 提交失败已退款，余额 %d，原因: %s", taskID, balance, message)
		return
	}

	if err := s.recordImageTaskDispatch(taskID, userID, submitted, providerKeyID); err != nil {
		message := "保存第三方任务信息失败: " + err.Error()
		balance, updateErr := s.failImageTaskAndRefund(taskID, userID, submitted, message)
		if updateErr != nil {
			log.Printf("异步生图任务 %d 保存任务信息失败后退款异常: %v", taskID, updateErr)
			return
		}
		log.Printf("异步生图任务 %d 无法持久化第三方任务信息，已退款，余额 %d", taskID, balance)
		return
	}

	if isTemporaryProviderResultFailure(submitted) && strings.TrimSpace(submitted.ProviderTaskID) != "" {
		submitted.Status = model.ImageTaskStatusGenerating
	}

	switch submitted.Status {
	case model.ImageTaskStatusSuccess:
		if strings.TrimSpace(submitted.ImageURL) == "" {
			message := "第三方服务没有返回可用的图片地址"
			balance, updateErr := s.failImageTaskAndRefund(taskID, userID, submitted, message)
			if updateErr != nil {
				log.Printf("异步生图任务 %d 成功态但缺少图片地址处理异常: %v", taskID, updateErr)
				return
			}
			log.Printf("异步生图任务 %d 未返回图片地址，已退款，余额 %d", taskID, balance)
			return
		}
		if input.Count > 1 && !hasRequestedImageCount(submitted, input.Count) {
			if strings.TrimSpace(submitted.ProviderTaskID) != "" {
				submitted.Status = model.ImageTaskStatusGenerating
				break
			}
			message := "第三方服务只返回了部分图片"
			balance, updateErr := s.failImageTaskAndRefund(taskID, userID, submitted, message)
			if updateErr != nil {
				log.Printf("异步生图任务 %d 部分成功处理异常: %v", taskID, updateErr)
				return
			}
			log.Printf("异步生图任务 %d 只返回部分图片，已按未完成张数退款，余额 %d", taskID, balance)
			return
		}
		submitted.ImageElapsedSeconds = buildCompletedImageElapsedSeconds(submitted, imageStartedAt, input.Count)
		balance, err := s.completeImageTask(taskID, userID, submitted)
		if err != nil {
			log.Printf("异步生图任务 %d 成功写库失败: %v", taskID, err)
			return
		}
		log.Printf("异步生图任务 %d 成功完成，当前余额 %d", taskID, balance)
		return
	case model.ImageTaskStatusFailed:
		message := providerFailureMessage(submitted, nil)
		balance, updateErr := s.failImageTaskAndRefund(taskID, userID, submitted, message)
		if updateErr != nil {
			log.Printf("异步生图任务 %d 失败处理异常: %v", taskID, updateErr)
			return
		}
		log.Printf("异步生图任务 %d 失败已退款，余额 %d，原因: %s", taskID, balance, message)
		return
	}

	batchID := strings.TrimSpace(submitted.ProviderTaskID)
	if batchID == "" {
		message := "第三方服务没有返回可用于恢复的批次ID"
		balance, updateErr := s.failImageTaskAndRefund(taskID, userID, submitted, message)
		if updateErr != nil {
			log.Printf("异步生图任务 %d 缺少批次ID处理异常: %v", taskID, updateErr)
			return
		}
		log.Printf("异步生图任务 %d 缺少批次ID，已退款，余额 %d", taskID, balance)
		return
	}

	pollCtx, pollCancel := context.WithTimeout(context.Background(), s.cfg.ImageProviderTimeout)
	defer pollCancel()

	result, pollErr := s.pollImageTaskV2(pollCtx, taskID, userID, input.Count, activeProvider, batchID, submitted)
	if pollErr != nil {
		if result.Status == model.ImageTaskStatusGenerating {
			log.Printf("image task %d poll round timed out, keep generating for later recovery: %v", taskID, pollErr)
			return
		}
		message := providerFailureMessage(result, pollErr)
		balance, updateErr := s.failImageTaskAndRefund(taskID, userID, result, message)
		if updateErr != nil {
			log.Printf("异步生图任务 %d 轮询失败处理异常: %v", taskID, updateErr)
			return
		}
		log.Printf("异步生图任务 %d 轮询失败已退款，余额 %d，原因: %s", taskID, balance, message)
		return
	}

	switch result.Status {
	case model.ImageTaskStatusSuccess:
		if strings.TrimSpace(result.ImageURL) == "" {
			message := "第三方服务没有返回可用的图片地址"
			balance, updateErr := s.failImageTaskAndRefund(taskID, userID, result, message)
			if updateErr != nil {
				log.Printf("异步生图任务 %d 轮询成功但缺少图片地址处理异常: %v", taskID, updateErr)
				return
			}
			log.Printf("异步生图任务 %d 轮询成功但未返回图片地址，已退款，余额 %d", taskID, balance)
			return
		}
		result.ImageElapsedSeconds = buildCompletedImageElapsedSeconds(result, imageStartedAt, input.Count)
		balance, err := s.completeImageTask(taskID, userID, result)
		if err != nil {
			log.Printf("异步生图任务 %d 轮询完成写库失败: %v", taskID, err)
			return
		}
		log.Printf("异步生图任务 %d 成功完成，当前余额 %d", taskID, balance)
	case model.ImageTaskStatusFailed:
		message := providerFailureMessage(result, nil)
		balance, updateErr := s.failImageTaskAndRefund(taskID, userID, result, message)
		if updateErr != nil {
			log.Printf("异步生图任务 %d 轮询失败处理异常: %v", taskID, updateErr)
			return
		}
		log.Printf("异步生图任务 %d 轮询失败已退款，余额 %d，原因: %s", taskID, balance, message)
	default:
		log.Printf("异步生图任务 %d 本轮轮询未完成，等待后续恢复", taskID)
	}
}

func (s *Service) resumeImageTaskV2(taskID uint64, userID uint64, batchID string) {
	defer s.releaseImageTask(taskID)

	pollCtx, pollCancel := context.WithTimeout(context.Background(), s.cfg.ImageProviderTimeout)
	defer pollCancel()

	var task model.ImageTask
	if err := s.db.WithContext(pollCtx).First(&task, "id = ? AND user_id = ?", taskID, userID).Error; err != nil {
		log.Printf("恢复任务 %d 加载任务失败: %v", taskID, err)
		return
	}
	activeProvider, _, _, err := s.providerForTask(pollCtx, task)
	if err != nil || activeProvider == nil || !activeProvider.Configured() {
		log.Printf("恢复任务 %d 图片服务未配置: %v", taskID, err)
		return
	}

	seed := provider.GenerateResult{
		ProviderTaskID: strings.TrimSpace(batchID),
		Status:         model.ImageTaskStatusGenerating,
	}
	result, pollErr := s.pollImageTaskV2(pollCtx, taskID, userID, task.RequestedCount, activeProvider, batchID, seed)
	if pollErr != nil {
		if result.Status == model.ImageTaskStatusGenerating {
			log.Printf("recovered image task %d poll round timed out, keep generating for later recovery: %v", taskID, pollErr)
			return
		}
		message := providerFailureMessage(result, pollErr)
		balance, updateErr := s.failImageTaskAndRefund(taskID, userID, result, message)
		if updateErr != nil {
			log.Printf("恢复任务 %d 轮询失败处理异常: %v", taskID, updateErr)
			return
		}
		log.Printf("恢复任务 %d 轮询失败已退款，余额 %d，原因: %s", taskID, balance, message)
		return
	}

	switch result.Status {
	case model.ImageTaskStatusSuccess:
		if strings.TrimSpace(result.ImageURL) == "" {
			message := "第三方服务没有返回可用的图片地址"
			balance, updateErr := s.failImageTaskAndRefund(taskID, userID, result, message)
			if updateErr != nil {
				log.Printf("恢复任务 %d 轮询成功但缺少图片地址处理异常: %v", taskID, updateErr)
				return
			}
			log.Printf("恢复任务 %d 轮询成功但未返回图片地址，已退款，余额 %d", taskID, balance)
			return
		}
		balance, err := s.completeImageTask(taskID, userID, result)
		if err != nil {
			log.Printf("恢复任务 %d 轮询完成写库失败: %v", taskID, err)
			return
		}
		log.Printf("恢复任务 %d 成功完成，当前余额 %d", taskID, balance)
	case model.ImageTaskStatusFailed:
		message := providerFailureMessage(result, nil)
		balance, updateErr := s.failImageTaskAndRefund(taskID, userID, result, message)
		if updateErr != nil {
			log.Printf("恢复任务 %d 轮询失败处理异常: %v", taskID, updateErr)
			return
		}
		log.Printf("恢复任务 %d 轮询失败已退款，余额 %d，原因: %s", taskID, balance, message)
	default:
		log.Printf("恢复任务 %d 本轮轮询未完成，等待后续恢复", taskID)
	}
}

func (s *Service) pollImageTaskV2(ctx context.Context, taskID uint64, userID uint64, requestedCount int, activeProvider *provider.MentaloutProvider, batchID string, seed provider.GenerateResult) (provider.GenerateResult, error) {
	result := seed
	if strings.TrimSpace(result.ProviderTaskID) == "" {
		result.ProviderTaskID = strings.TrimSpace(batchID)
	}
	pollInterval := s.cfg.ImageProviderPollInterval
	if pollInterval <= 0 {
		pollInterval = 30 * time.Second
	}
	if pollInterval >= time.Second && pollInterval < 30*time.Second {
		pollInterval = 30 * time.Second
	}
	temporaryFailures := 0
	runningFetches := 0
	const maxTemporaryFailures = 3
	const maxRunningPollDuration = 12 * time.Minute
	maxRunningFetches := int(maxRunningPollDuration / pollInterval)
	if maxRunningFetches < 1 {
		maxRunningFetches = 1
	}
	timeoutMessage := "这次生成等得有点久，我们已经停止继续等待，避免重复扣费。"

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if result.Status == "" {
				result.Status = model.ImageTaskStatusGenerating
			}
			if result.ErrorMessage == "" {
				result.ErrorMessage = ctx.Err().Error()
			}
			return result, ctx.Err()
		case <-ticker.C:
		}

		if activeProvider == nil || !activeProvider.Configured() {
			return result, ErrProviderNotConfigured
		}
		fetched, err := activeProvider.Fetch(ctx, batchID)
		if err != nil {
			if provider.IsTemporaryFetchError(err) {
				temporaryFailures++
				if result.Status == "" {
					result.Status = model.ImageTaskStatusGenerating
				}
				result.ErrorMessage = truncateProviderMessage(err.Error())
				if temporaryFailures >= maxTemporaryFailures {
					result.Status = model.ImageTaskStatusFailed
					result.ErrorMessage = timeoutMessage
					return result, nil
				}
				if err := waitWithContext(ctx, temporaryProviderBackoffDelay(temporaryFailures, pollInterval)); err != nil {
					return result, err
				}
				continue
			}
			return result, err
		}
		if fetched.ProviderTaskID == "" {
			fetched.ProviderTaskID = batchID
		}
		result = mergeGenerateResult(result, fetched)
		temporaryFailed := isTemporaryProviderResultFailure(result)
		if temporaryFailed {
			temporaryFailures++
			result.Status = model.ImageTaskStatusGenerating
		} else {
			temporaryFailures = 0
		}
		if result.Status == model.ImageTaskStatusGenerating {
			runningFetches++
		}
		if err := s.persistPartialImageTask(ctx, taskID, userID, result); err != nil {
			log.Printf("生图任务 %d 保存部分结果失败: %v", taskID, err)
		}

		if hasRequestedImageCount(result, requestedCount) {
			result.Status = model.ImageTaskStatusSuccess
			return result, nil
		}
		if temporaryFailures >= maxTemporaryFailures || runningFetches >= maxRunningFetches {
			result.Status = model.ImageTaskStatusFailed
			result.ErrorMessage = timeoutMessage
			return result, nil
		}

		if temporaryFailed {
			if err := waitWithContext(ctx, temporaryProviderBackoffDelay(temporaryFailures, pollInterval)); err != nil {
				return result, err
			}
			continue
		}

		switch result.Status {
		case model.ImageTaskStatusSuccess:
			if requestedCount <= 1 && strings.TrimSpace(result.ImageURL) != "" {
				return result, nil
			}
		case model.ImageTaskStatusFailed:
			return result, nil
		}
	}
}

func (s *Service) recordImageTaskDispatch(taskID uint64, userID uint64, result provider.GenerateResult, providerKeyID uint64) error {
	updates := map[string]any{
		"provider_task_id":  strings.TrimSpace(result.ProviderTaskID),
		"provider_key_id":   providerKeyID,
		"provider_response": result.RawResponse,
	}
	if strings.TrimSpace(result.ErrorMessage) != "" {
		updates["error_message"] = truncateProviderMessage(result.ErrorMessage)
	}

	return s.db.Model(&model.ImageTask{}).
		Where("id = ? AND user_id = ?", taskID, userID).
		Updates(updates).Error
}

func (s *Service) persistPartialImageTask(ctx context.Context, taskID uint64, userID uint64, result provider.GenerateResult) error {
	urls := normalizeGeneratedImageURLs(result, 0, "")
	if len(urls) == 0 && strings.TrimSpace(result.RawResponse) == "" && strings.TrimSpace(result.ErrorMessage) == "" {
		return nil
	}
	urls = s.storeGeneratedImages(taskID, userID, urls)

	updates := map[string]any{}
	if len(urls) > 0 {
		imageURLsJSON, _ := json.Marshal(urls)
		updates["image_url"] = urls[0]
		updates["image_urls"] = string(imageURLsJSON)
		updates["image_elapsed_seconds"] = marshalImageElapsedSeconds(normalizeImageElapsedSeconds(result.ImageElapsedSeconds, len(urls)))
		if result.Status != model.ImageTaskStatusFailed {
			updates["error_message"] = ""
		}
	}
	if strings.TrimSpace(result.RawResponse) != "" {
		updates["provider_response"] = result.RawResponse
	}
	if providerTaskID := strings.TrimSpace(result.ProviderTaskID); providerTaskID != "" {
		updates["provider_task_id"] = providerTaskID
	}
	if strings.TrimSpace(result.ErrorMessage) != "" && result.Status == model.ImageTaskStatusFailed {
		updates["error_message"] = truncateProviderMessage(result.ErrorMessage)
	}
	if taskLogID, ok := updates["provider_task_id"]; ok || len(urls) > 0 || strings.TrimSpace(result.ErrorMessage) != "" {
		log.Printf("生图任务 %d 持久化部分结果：status=%s providerTaskID=%v urls=%d rawLen=%d errLen=%d", taskID, result.Status, taskLogID, len(urls), len([]rune(result.RawResponse)), len([]rune(result.ErrorMessage)))
	}
	if len(updates) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Model(&model.ImageTask{}).
		Where("id = ? AND user_id = ? AND status = ?", taskID, userID, model.ImageTaskStatusGenerating).
		Updates(updates).Error
}

func isTemporaryProviderResultFailure(result provider.GenerateResult) bool {
	if result.Status != model.ImageTaskStatusFailed {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(result.ErrorMessage + " " + result.RawResponse))
	if message == "" {
		return false
	}
	return isProviderTemporaryServerError(message)
}

func temporaryProviderBackoffDelay(failures int, base time.Duration) time.Duration {
	if failures <= 0 {
		return 0
	}
	if base <= 0 {
		base = 2 * time.Second
	}
	multiplier := failures
	if multiplier > 5 {
		multiplier = 5
	}
	delay := time.Duration(multiplier) * base
	const maxDelay = 15 * time.Second
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func waitWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func hasRequestedImageCount(result provider.GenerateResult, requestedCount int) bool {
	if requestedCount <= 0 {
		return false
	}
	return len(normalizeGeneratedImageURLs(result, requestedCount, "")) >= requestedCount
}

func mergeGenerateResult(base provider.GenerateResult, next provider.GenerateResult) provider.GenerateResult {
	if strings.TrimSpace(next.ProviderTaskID) != "" {
		base.ProviderTaskID = next.ProviderTaskID
	}
	if strings.TrimSpace(next.ImageURL) != "" {
		base.ImageURL = next.ImageURL
	}
	nextURLs := mergeStringURLs(next.ImageURLs, []string{next.ImageURL})
	base.ImageURLs = mergeStringURLs(base.ImageURLs, nextURLs)
	base.ImageElapsedSeconds = mergeImageElapsedSeconds(base.ImageElapsedSeconds, next.ImageElapsedSeconds, len(base.ImageURLs))
	if base.ImageURL == "" && len(base.ImageURLs) > 0 {
		base.ImageURL = base.ImageURLs[0]
	}
	if strings.TrimSpace(next.ErrorMessage) != "" {
		base.ErrorMessage = next.ErrorMessage
	}
	if next.Status != "" {
		base.Status = next.Status
	}
	if strings.TrimSpace(next.RawResponse) != "" {
		base.RawResponse = next.RawResponse
	}
	return base
}

func elapsedSecondsSince(startedAt time.Time) int64 {
	if startedAt.IsZero() {
		return 0
	}
	elapsed := time.Since(startedAt)
	if elapsed <= 0 {
		return 0
	}
	seconds := int64(elapsed / time.Second)
	if seconds == 0 {
		return 1
	}
	return seconds
}

func buildCompletedImageElapsedSeconds(result provider.GenerateResult, startedAt time.Time, requestedCount int) []int64 {
	urlCount := len(normalizeGeneratedImageURLs(result, requestedCount, ""))
	if urlCount <= 0 {
		urlCount = 1
	}
	elapsed := elapsedSecondsSince(startedAt)
	values := make([]int64, urlCount)
	for i := range values {
		values[i] = elapsed
	}
	return values
}

func normalizeImageElapsedSeconds(values []int64, maxCount int) []int64 {
	if maxCount <= 0 || len(values) == 0 {
		return nil
	}
	result := make([]int64, 0, maxCount)
	for _, value := range values {
		if len(result) >= maxCount {
			break
		}
		if value < 0 {
			value = 0
		}
		result = append(result, value)
	}
	return result
}

func mergeImageElapsedSeconds(base []int64, next []int64, maxCount int) []int64 {
	if maxCount <= 0 {
		return nil
	}
	merged := make([]int64, 0, maxCount)
	merged = append(merged, normalizeImageElapsedSeconds(base, maxCount)...)
	remaining := maxCount - len(merged)
	if remaining > 0 {
		merged = append(merged, normalizeImageElapsedSeconds(next, remaining)...)
	}
	return merged
}

func marshalImageElapsedSeconds(values []int64) string {
	if len(values) == 0 {
		return ""
	}
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
}

func imageElapsedSecondsFromTask(task model.ImageTask) []int64 {
	var values []int64
	if err := json.Unmarshal([]byte(strings.TrimSpace(task.ImageElapsedSeconds)), &values); err != nil {
		return nil
	}
	return normalizeImageElapsedSeconds(values, len(imageURLsFromTask(task)))
}

func (s *Service) claimImageTaskSubmitInDB(ctx context.Context, taskID uint64, userID uint64) bool {
	if s == nil || s.db == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	marker := fmt.Sprintf("%s%d", imageTaskSubmittingMarker, time.Now().Unix())
	result := s.db.WithContext(ctx).Model(&model.ImageTask{}).
		Where("id = ? AND user_id = ? AND status = ? AND provider_task_id = ? AND (provider_response = ? OR provider_response IS NULL)",
			taskID, userID, model.ImageTaskStatusGenerating, "", "").
		Update("provider_response", marker)
	if result.Error != nil {
		log.Printf("生图任务 %d 提交锁定失败: %v", taskID, result.Error)
		return false
	}
	return result.RowsAffected == 1
}

func (s *Service) isImageTaskActive(taskID uint64) bool {
	s.activeTasksMu.RLock()
	defer s.activeTasksMu.RUnlock()

	_, exists := s.activeTasks[taskID]
	return exists
}

func (s *Service) isImageTaskStale(task model.ImageTask) bool {
	if s.cfg.ImageTaskStaleTTL <= 0 {
		return false
	}
	return s.now().Sub(task.CreatedAt.In(s.location)) >= s.cfg.ImageTaskStaleTTL
}
