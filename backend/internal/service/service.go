package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
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
	Prompt            string `json:"prompt"`
	Size              string `json:"size,omitempty"`
	Style             string `json:"style,omitempty"`
	Count             int    `json:"count,omitempty"`
	Quality           string `json:"quality,omitempty"`
	OutputFormat      string `json:"outputFormat,omitempty"`
	ReferenceImageURL string `json:"referenceImageUrl,omitempty"`
}

// Service 承载平台核心业务。
type Service struct {
	cfg           config.Config
	db            *gorm.DB
	redis         *redis.Client
	emailCodes    cache.EmailCodeStore
	mailer        *mailer.Client
	jwt           *security.JWTManager
	location      *time.Location
	imageProvider *provider.MentaloutProvider
	imageStore    *imagestorage.Client
	settingsMu    sync.RWMutex
	settings      map[string]string
	activeTasksMu sync.RWMutex
	activeTasks   map[uint64]struct{}
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

	return &Service{
		cfg:        cfg,
		db:         db,
		redis:      redisClient,
		emailCodes: cache.NewEmailCodeStore(redisClient),
		mailer: mailer.New(mailer.Config{
			Delivery: cfg.EmailDelivery,
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
			FromName: cfg.SMTPFromName,
			UseTLS:   cfg.SMTPUseTLS,
			StartTLS: cfg.SMTPStartTLS,
			Timeout:  cfg.SMTPTimeout,
			AppName:  "AI 生图 SaaS",
			AppEnv:   cfg.AppEnv,
		}),
		jwt:         security.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTokenTTL),
		location:    loc,
		imageStore:  store,
		settings:    defaultSystemSettings(cfg),
		activeTasks: make(map[uint64]struct{}),
		imageProvider: provider.NewMentaloutProvider(provider.MentaloutConfig{
			BatchServiceURL:    cfg.ImageProviderBatchURL,
			ProviderAPIBaseURL: cfg.ImageProviderAPIBaseURL,
			APIKey:             cfg.ImageProviderAPIKey,
			PollInterval:       cfg.ImageProviderPollInterval,
			Timeout:            cfg.ImageProviderTimeout,
		}),
	}
}

// JWT 返回令牌管理器。
func (s *Service) JWT() *security.JWTManager {
	return s.jwt
}

// Redis 返回 Redis 客户端，供限流等 HTTP 中间件复用。
func (s *Service) Redis() *redis.Client {
	return s.redis
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

	code, err := s.emailCodes.Save(ctx, email, req.Scene, s.cfg.EmailCodeTTL)
	if err != nil {
		return dto.SendEmailCodeResponse{}, err
	}

	if s.mailer != nil && s.mailer.Configured() {
		if err := s.mailer.SendVerificationCode(ctx, email, code, req.Scene, s.cfg.EmailCodeTTL); err != nil {
			return dto.SendEmailCodeResponse{}, err
		}
	} else if strings.EqualFold(s.cfg.AppEnv, "prod") {
		return dto.SendEmailCodeResponse{}, ErrEmailSenderNotConfigured
	} else {
		log.Printf("开发环境邮箱验证码: email=%s scene=%s code=%s expires_in=%s", email, req.Scene, code, s.cfg.EmailCodeTTL)
	}

	resp := dto.SendEmailCodeResponse{
		Sent:      true,
		ExpiresIn: int(s.cfg.EmailCodeTTL / time.Second),
	}
	if !strings.EqualFold(s.cfg.AppEnv, "prod") {
		resp.DebugCode = code
	}

	return resp, nil
}

// Register 完成注册并直接返回登录态。
func (s *Service) Register(ctx context.Context, req dto.RegisterRequest) (dto.AuthResponse, error) {
	if !s.cfg.RegistrationEnabled {
		return dto.AuthResponse{}, ErrRegistrationDisabled
	}

	email := normalizeEmail(req.Email)
	username := normalizeUsername(req.Username)
	if email == "" || username == "" || strings.TrimSpace(req.Password) == "" {
		return dto.AuthResponse{}, fmt.Errorf("请求参数不合法")
	}

	ok, err := s.emailCodes.Verify(ctx, email, "register", req.Code)
	if err != nil {
		return dto.AuthResponse{}, err
	}
	if !ok {
		return dto.AuthResponse{}, ErrInvalidVerificationCode
	}

	var created model.User
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&model.User{}).
			Where("lower(email) = ? OR username = ?", email, username).
			Count(&exists).Error; err != nil {
			return err
		}
		if exists > 0 {
			return ErrAccountExists
		}

		hash, err := hashPassword(req.Password)
		if err != nil {
			return err
		}

		now := s.now()
		settlementDate := s.startOfDay(now)
		user := model.User{
			Username:                 username,
			Email:                    email,
			PasswordHash:             hash,
			PointsBalance:            0,
			CheckinStreak:            0,
			LastCreditSettlementDate: &settlementDate,
			EmailVerifiedAt:          &now,
			LastLoginAt:              &now,
			Status:                   model.UserStatusNormal,
			Role:                     model.UserRoleUser,
		}
		if s.isAdminEmail(email) {
			user.Role = model.UserRoleAdmin
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		created = user
		return nil
	})
	if err != nil {
		return dto.AuthResponse{}, err
	}

	return s.authResponse(created)
}

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

	return s.authResponse(user)
}

// CheckIn 完成每日签到，若当天已签到则返回幂等成功。
func (s *Service) CheckIn(ctx context.Context, userID uint64) (dto.CheckinResponse, error) {
	return s.checkInV2(ctx, userID)

	_ = ctx

	var result dto.CheckinResponse
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}

		now := s.now()
		today := s.startOfDay(now)
		if user.LastCheckinDate != nil && sameDay(*user.LastCheckinDate, today) {
			lastCheckin := now
			if user.LastCheckinAt != nil {
				lastCheckin = *user.LastCheckinAt
			}
			result = dto.CheckinResponse{
				CheckedIn:     false,
				GainedPoints:  0,
				Streak:        user.CheckinStreak,
				PointsBalance: user.PointsBalance,
				FreeCredits:   user.FreeCredits,
				PaidCredits:   user.PaidCredits,
				TotalCredits:  user.FreeCredits + user.PaidCredits,
				LastCheckinAt: lastCheckin,
			}
			return nil
		}

		streak := 1
		if user.LastCheckinDate != nil {
			yesterday := today.AddDate(0, 0, -1)
			if sameDay(*user.LastCheckinDate, yesterday) {
				streak = user.CheckinStreak + 1
			}
		}

		before := user.PointsBalance
		after := before + s.cfg.CheckinPoints
		lastCheckinAt := now
		lastCheckinDate := today

		user.PointsBalance = after
		user.CheckinStreak = streak
		user.LastCheckinAt = &lastCheckinAt
		user.LastCheckinDate = &lastCheckinDate

		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
			"points_balance":    after,
			"checkin_streak":    streak,
			"last_checkin_at":   lastCheckinAt,
			"last_checkin_date": lastCheckinDate,
		}).Error; err != nil {
			return err
		}

		log := model.PointsLog{
			UserID:        user.ID,
			ChangeType:    model.PointsChangeTypeCheckin,
			Delta:         s.cfg.CheckinPoints,
			BeforeBalance: before,
			AfterBalance:  after,
			BizType:       "checkin",
			BizID:         dayKey(today),
			Remark:        "每日签到",
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}

		result = dto.CheckinResponse{
			CheckedIn:     true,
			GainedPoints:  s.cfg.CheckinPoints,
			Streak:        streak,
			PointsBalance: after,
			FreeCredits:   user.FreeCredits,
			PaidCredits:   user.PaidCredits,
			TotalCredits:  user.FreeCredits + user.PaidCredits,
			LastCheckinAt: lastCheckinAt,
		}
		return nil
	})
	if err != nil {
		return dto.CheckinResponse{}, err
	}

	if result.CheckedIn {
		_ = s.rememberCheckin(ctx, userID, result.LastCheckinAt)
	}

	return result, nil
}

// GetPoints 获取当前积分和最近流水。
func (s *Service) GetPoints(ctx context.Context, userID uint64) (dto.PointsResponse, error) {
	return s.getPointsV2(ctx, userID)

	_ = ctx
	user, err := s.loadUser(userID)
	if err != nil {
		return dto.PointsResponse{}, err
	}

	var logs []model.PointsLog
	if err := s.db.Where("user_id = ?", userID).Order("created_at desc").Limit(10).Find(&logs).Error; err != nil {
		return dto.PointsResponse{}, err
	}

	var creditLogs []model.CreditLog
	if err := s.db.Where("user_id = ?", userID).Order("created_at desc").Limit(10).Find(&creditLogs).Error; err != nil {
		return dto.PointsResponse{}, err
	}

	return s.pointsResponse(user, logs, creditLogs), nil
}

// GetMe 返回当前登录用户信息。
func (s *Service) GetMe(ctx context.Context, userID uint64) (dto.UserInfo, error) {
	return s.getMeV2(ctx, userID)

	_ = ctx
	user, err := s.loadUser(userID)
	if err != nil {
		return dto.UserInfo{}, err
	}
	user = s.syncConfiguredAdminRole(user)
	return s.userInfoBalance(user), nil
}

// UploadReferenceImage 上传参考图片并返回可访问地址。
func (s *Service) UploadReferenceImage(ctx context.Context, userID uint64, header *multipart.FileHeader) (dto.UploadImageResponse, error) {
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

	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	keyPrefix := fmt.Sprintf("reference-images/user-%d/%s", userID, s.now().Format("2006/01/02"))
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

// CreateImageTask 创建生图任务并立刻返回 mock 结果。
func (s *Service) CreateImageTask(ctx context.Context, userID uint64, req dto.CreateImageTaskRequest) (dto.CreateImageTaskResponse, error) {
	return s.createImageTaskV2(ctx, userID, req)

	_ = ctx
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return dto.CreateImageTaskResponse{}, ErrPromptRequired
	}
	decision := s.moderatePrompt(prompt)
	if decision.Status == model.ContentAuditStatusRejected {
		_ = s.recordContentAudit(ctx, userID, "image_task", "", "prompt", prompt, decision)
		return dto.CreateImageTaskResponse{}, ErrContentRejected
	}
	if s.imageProvider == nil || !s.imageProvider.Configured() {
		return dto.CreateImageTaskResponse{}, ErrProviderNotConfigured
	}
	count := normalizeImageCount(req.Count)
	size := strings.TrimSpace(req.Size)
	style := strings.TrimSpace(req.Style)
	quality := strings.TrimSpace(req.Quality)
	outputFormat := strings.TrimSpace(req.OutputFormat)
	referenceImageURL := strings.TrimSpace(req.ReferenceImageURL)
	referenceImageURL = s.normalizeReferenceImageURL(referenceImageURL)
	costCredits := int64(count)
	input := imageTaskGenerationInput{
		Prompt:            prompt,
		Size:              size,
		Style:             style,
		Count:             count,
		Quality:           quality,
		OutputFormat:      outputFormat,
		ReferenceImageURL: referenceImageURL,
	}

	var responseData dto.CreateImageTaskResponse
	var task model.ImageTask
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}

		before := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
		if before.Total() < costCredits {
			return ErrInsufficientCredits
		}
		freeCost, paidCost, after := before.spend(costCredits)
		requestSnapshot, _ := json.Marshal(input)
		creditSource := "free"
		if freeCost > 0 && paidCost > 0 {
			creditSource = "mixed"
		} else if paidCost > 0 {
			creditSource = "paid"
		}

		task = model.ImageTask{
			UserID:          userID,
			Prompt:          prompt,
			CostPoints:      0,
			CostCredits:     costCredits,
			FreeCreditsCost: freeCost,
			PaidCreditsCost: paidCost,
			CreditSource:    creditSource,
			Status:          model.ImageTaskStatusGenerating,
			Provider:        s.imageProvider.Name(),
			RequestParams:   string(requestSnapshot),
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
			"free_credits": after.Free,
			"paid_credits": after.Paid,
		}).Error; err != nil {
			return err
		}

		if err := s.recordCreditLog(tx, userID, model.CreditChangeTypeGenerate, -freeCost, -paidCost, before, after, "image_task", strconv.FormatUint(task.ID, 10), "生图消耗次数"); err != nil {
			return err
		}

		responseData = dto.CreateImageTaskResponse{
			TaskID:              strconv.FormatUint(task.ID, 10),
			Status:              string(task.Status),
			Balance:             after.Total(),
			PointsBalance:       user.PointsBalance,
			FreeCreditsBalance:  after.Free,
			PaidCreditsBalance:  after.Paid,
			TotalCreditsBalance: after.Total(),
		}
		return nil
	})

	if err != nil {
		return dto.CreateImageTaskResponse{}, err
	}

	_ = s.recordContentAudit(ctx, userID, "image_task", strconv.FormatUint(task.ID, 10), "prompt", prompt, decision)
	s.launchImageTask(task.ID, userID, input)

	return responseData, nil
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

	_ = ctx
	id, err := strconv.ParseUint(strings.TrimSpace(taskID), 10, 64)
	if err != nil || id == 0 {
		return dto.ImageTaskDetailResponse{}, ErrTaskNotFound
	}

	var task model.ImageTask
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.ImageTaskDetailResponse{}, ErrTaskNotFound
		}
		return dto.ImageTaskDetailResponse{}, err
	}

	return dto.ImageTaskDetailResponse{
		TaskID:          strconv.FormatUint(task.ID, 10),
		Prompt:          task.Prompt,
		ImageURL:        task.ImageURL,
		ImageURLs:       imageURLsFromTask(task),
		CostPoints:      task.CostPoints,
		CostCredits:     task.CostCredits,
		FreeCreditsCost: task.FreeCreditsCost,
		PaidCreditsCost: task.PaidCreditsCost,
		CreditSource:    task.CreditSource,
		Status:          string(task.Status),
		RequestParams:   task.RequestParams,
		ErrorMessage:    task.ErrorMessage,
		CreatedAt:       task.CreatedAt,
	}, nil
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

	_ = ctx
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50
	}

	var total int64
	if err := s.db.Model(&model.ImageTask{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return dto.ListImageTasksResponse{}, err
	}

	var tasks []model.ImageTask
	if err := s.db.Where("user_id = ?", userID).Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		return dto.ListImageTasksResponse{}, err
	}

	items := make([]dto.ImageTaskItem, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, dto.ImageTaskItem{
			ID:              strconv.FormatUint(task.ID, 10),
			Prompt:          task.Prompt,
			ImageURL:        task.ImageURL,
			ImageURLs:       imageURLsFromTask(task),
			CostPoints:      task.CostPoints,
			CostCredits:     task.CostCredits,
			FreeCreditsCost: task.FreeCreditsCost,
			PaidCreditsCost: task.PaidCreditsCost,
			CreditSource:    task.CreditSource,
			Status:          string(task.Status),
			RequestParams:   task.RequestParams,
			ErrorMessage:    task.ErrorMessage,
			CreatedAt:       task.CreatedAt,
		})
	}

	return dto.ListImageTasksResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
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

// RedeemPoints 将积分兑换为免费次数。
func (s *Service) RedeemPoints(ctx context.Context, userID uint64, req dto.RedeemPointsRequest) (dto.RedeemPointsResponse, error) {
	return dto.RedeemPointsResponse{}, ErrPackageInvalid

	pkg, err := s.findBillingPackageByCode(ctx, req.PackageCode, true)
	if err != nil {
		return dto.RedeemPointsResponse{}, err
	}
	if pkg.PackageType != model.BillingPackageTypeRedeem {
		return dto.RedeemPointsResponse{}, ErrPackageInvalid
	}
	if pkg.CreditsAmount <= 0 || pkg.PointsCost <= 0 {
		return dto.RedeemPointsResponse{}, ErrPackageInvalid
	}

	var result dto.RedeemPointsResponse
	err = s.db.Transaction(func(tx *gorm.DB) error {
		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}
		if user.PointsBalance < pkg.PointsCost {
			return ErrInsufficientPoints
		}

		beforeCredits := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
		afterCredits := creditBalance{Free: beforeCredits.Free + pkg.CreditsAmount, Paid: beforeCredits.Paid}
		beforePoints := user.PointsBalance
		afterPoints := beforePoints - pkg.PointsCost

		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
			"points_balance": afterPoints,
			"free_credits":   afterCredits.Free,
			"paid_credits":   afterCredits.Paid,
		}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.PointsLog{
			UserID:        userID,
			ChangeType:    model.PointsChangeTypeRedeem,
			Delta:         -pkg.PointsCost,
			BeforeBalance: beforePoints,
			AfterBalance:  afterPoints,
			BizType:       "billing_package",
			BizID:         pkg.Code,
			Remark:        fmt.Sprintf("兑换 %s", pkg.Name),
		}).Error; err != nil {
			return err
		}

		if err := s.recordCreditLog(tx, userID, model.CreditChangeTypeRedeem, pkg.CreditsAmount, 0, beforeCredits, afterCredits, "billing_package", pkg.Code, fmt.Sprintf("积分兑换 %s", pkg.Name)); err != nil {
			return err
		}

		result = dto.RedeemPointsResponse{
			RedemptionNo:        generateOrderNo(s.now()),
			PackageCode:         pkg.Code,
			PackageName:         pkg.Name,
			PointsCost:          pkg.PointsCost,
			CreditsAmount:       pkg.CreditsAmount,
			PointsBalance:       afterPoints,
			FreeCreditsBalance:  afterCredits.Free,
			PaidCreditsBalance:  afterCredits.Paid,
			TotalCreditsBalance: afterCredits.Total(),
			Status:              "success",
		}
		return nil
	})
	return result, err
}

// MockPurchase 模拟完成一个套餐购买，并直接入账次数。
func (s *Service) MockPurchase(ctx context.Context, userID uint64, req dto.MockPurchaseRequest) (dto.MockPurchaseResponse, error) {
	return s.mockPurchaseV2(ctx, userID, req)

	pkg, err := s.findBillingPackageByCode(ctx, req.PackageCode, true)
	if err != nil {
		return dto.MockPurchaseResponse{}, err
	}
	if pkg.PackageType != model.BillingPackageTypeRecharge {
		return dto.MockPurchaseResponse{}, ErrPackageInvalid
	}
	if pkg.CreditsAmount <= 0 || pkg.AmountCents < 0 {
		return dto.MockPurchaseResponse{}, ErrPackageInvalid
	}

	var result dto.MockPurchaseResponse
	err = s.db.Transaction(func(tx *gorm.DB) error {
		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}

		now := s.now()
		orderNo := generateOrderNo(now)

		order := model.RechargeOrder{
			UserID:         user.ID,
			OrderNo:        orderNo,
			PackageCode:    pkg.Code,
			PackageName:    pkg.Name,
			AmountCents:    pkg.AmountCents,
			PointsAmount:   pkg.BonusPoints,
			CreditsAmount:  pkg.CreditsAmount,
			BonusPoints:    pkg.BonusPoints,
			Status:         model.RechargeOrderStatusPaid,
			PaidAt:         &now,
			PaymentChannel: "mock",
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}

		beforePoints := user.PointsBalance
		afterPoints := beforePoints + pkg.BonusPoints
		beforeCredits := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
		afterCredits := creditBalance{Free: beforeCredits.Free, Paid: beforeCredits.Paid + pkg.CreditsAmount}

		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
			"points_balance": afterPoints,
			"free_credits":   afterCredits.Free,
			"paid_credits":   afterCredits.Paid,
		}).Error; err != nil {
			return err
		}

		if pkg.BonusPoints > 0 {
			if err := tx.Create(&model.PointsLog{
				UserID:        userID,
				ChangeType:    model.PointsChangeTypePurchaseBonus,
				Delta:         pkg.BonusPoints,
				BeforeBalance: beforePoints,
				AfterBalance:  afterPoints,
				BizType:       "recharge_order",
				BizID:         strconv.FormatUint(order.ID, 10),
				Remark:        "充值赠送积分",
			}).Error; err != nil {
				return err
			}
		}

		if err := s.recordCreditLog(tx, userID, model.CreditChangeTypePurchase, 0, pkg.CreditsAmount, beforeCredits, afterCredits, "recharge_order", strconv.FormatUint(order.ID, 10), "充值购买次数"); err != nil {
			return err
		}

		result = dto.MockPurchaseResponse{
			RechargeOrderItem: dto.RechargeOrderItem{
				ID:             strconv.FormatUint(order.ID, 10),
				OrderNo:        orderNo,
				PackageCode:    pkg.Code,
				PackageName:    pkg.Name,
				AmountCents:    pkg.AmountCents,
				CreditsAmount:  pkg.CreditsAmount,
				BonusPoints:    pkg.BonusPoints,
				PointsAmount:   pkg.BonusPoints,
				Status:         string(order.Status),
				PaymentChannel: order.PaymentChannel,
				PaidAt:         order.PaidAt,
				CreatedAt:      order.CreatedAt,
			},
			PointsBalance:       afterPoints,
			FreeCreditsBalance:  afterCredits.Free,
			PaidCreditsBalance:  afterCredits.Paid,
			TotalCreditsBalance: afterCredits.Total(),
			Balance:             afterCredits.Total(),
		}
		return nil
	})
	return result, err
}

func (s *Service) authResponse(user model.User) (dto.AuthResponse, error) {
	token, exp, err := s.jwt.Generate(user.ID, user.Username, user.Email)
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

func imageURLsFromTask(task model.ImageTask) []string {
	var urls []string
	if strings.TrimSpace(task.ImageURLs) != "" {
		_ = json.Unmarshal([]byte(task.ImageURLs), &urls)
	}
	urls = mergeStringURLs(urls, []string{task.ImageURL})
	return urls
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

func (s *Service) completeImageTask(taskID uint64, userID uint64, result provider.GenerateResult) (int64, error) {
	imageURLs := imageURLsFromResult(result)
	imageURLs = s.storeGeneratedImages(taskID, userID, imageURLs)

	imageURL := ""
	if imageURL == "" && len(imageURLs) > 0 {
		imageURL = imageURLs[0]
	}
	imageURLsJSON, _ := json.Marshal(imageURLs)

	updates := map[string]any{
		"status":            model.ImageTaskStatusSuccess,
		"image_url":         imageURL,
		"image_urls":        string(imageURLsJSON),
		"provider_response": result.RawResponse,
		"error_message":     "",
	}
	if providerTaskID := strings.TrimSpace(result.ProviderTaskID); providerTaskID != "" {
		updates["provider_task_id"] = providerTaskID
	}

	res := s.db.Model(&model.ImageTask{}).
		Where("id = ? AND user_id = ? AND status = ?", taskID, userID, model.ImageTaskStatusGenerating).
		Updates(updates)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
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
	return user.FreeCredits + user.PaidCredits, nil
}

func (s *Service) storeGeneratedImages(taskID uint64, userID uint64, imageURLs []string) []string {
	if s == nil || s.imageStore == nil || !s.imageStore.Enabled() || len(imageURLs) == 0 {
		return imageURLs
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ImageStorageTimeout)
	defer cancel()

	keyPrefix := fmt.Sprintf("images/%s/user-%d/task-%d", s.now().Format("2006/01/02"), userID, taskID)
	storedURLs, err := s.imageStore.StoreRemoteImages(ctx, imageURLs, keyPrefix)
	if err != nil {
		log.Printf("任务 %d 图片转存部分失败，已保留可用 URL: %v", taskID, err)
	}
	if len(storedURLs) == 0 {
		return imageURLs
	}
	return storedURLs
}

func (s *Service) failImageTaskAndRefund(taskID uint64, userID uint64, result provider.GenerateResult, message string) (int64, error) {
	message = truncateProviderMessage(message)

	var balance int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		task, err := s.lockImageTask(tx, taskID, userID)
		if err != nil {
			return err
		}
		if task.Status != model.ImageTaskStatusGenerating {
			user, err := s.lockUser(tx, userID)
			if err != nil {
				return err
			}
			balance = user.FreeCredits + user.PaidCredits
			return nil
		}

		updates := map[string]any{
			"status":            model.ImageTaskStatusFailed,
			"provider_response": result.RawResponse,
			"error_message":     message,
		}
		if providerTaskID := strings.TrimSpace(result.ProviderTaskID); providerTaskID != "" {
			updates["provider_task_id"] = providerTaskID
		}
		res := tx.Model(&model.ImageTask{}).
			Where("id = ? AND user_id = ? AND status = ?", taskID, userID, model.ImageTaskStatusGenerating).
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			user, err := s.lockUser(tx, userID)
			if err != nil {
				return err
			}
			balance = user.FreeCredits + user.PaidCredits
			return nil
		}

		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}
		if task.CostCredits > 0 || task.FreeCreditsCost > 0 || task.PaidCreditsCost > 0 {
			before := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
			after := creditBalance{
				Free: before.Free + task.FreeCreditsCost,
				Paid: before.Paid + task.PaidCreditsCost,
			}
			if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
				"free_credits": after.Free,
				"paid_credits": after.Paid,
			}).Error; err != nil {
				return err
			}
			if err := s.recordCreditLog(tx, userID, model.CreditChangeTypeRefund, task.FreeCreditsCost, task.PaidCreditsCost, before, after, "image_task", strconv.FormatUint(task.ID, 10), "生图失败退回次数"); err != nil {
				return err
			}
			balance = after.Total()
			return nil
		}

		beforePoints := user.PointsBalance
		afterPoints := beforePoints + task.CostPoints

		if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("points_balance", afterPoints).Error; err != nil {
			return err
		}

		log := model.PointsLog{
			UserID:        userID,
			ChangeType:    model.PointsChangeTypeRefund,
			Delta:         task.CostPoints,
			BeforeBalance: beforePoints,
			AfterBalance:  afterPoints,
			BizType:       "image_task",
			BizID:         strconv.FormatUint(task.ID, 10),
			Remark:        "生图失败退款",
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}

		balance = afterPoints
		return nil
	})
	return balance, err
}

func providerFailureMessage(result provider.GenerateResult, err error) string {
	message := strings.TrimSpace(result.ErrorMessage)
	if message == "" && errors.Is(err, context.DeadlineExceeded) {
		message = "图片生成超时"
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	if message == "" {
		message = "图片生成失败"
	}
	return truncateProviderMessage(message)
}

func truncateProviderMessage(message string) string {
	runes := []rune(strings.TrimSpace(message))
	if len(runes) <= 240 {
		return string(runes)
	}
	return string(runes[:240])
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
			input.ReferenceImageURL = s.normalizeReferenceImageURL(strings.TrimSpace(input.ReferenceImageURL))
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

func (s *Service) normalizeReferenceImageURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	if strings.HasPrefix(rawURL, "/") {
		base := strings.TrimRight(strings.TrimSpace(s.cfg.BackendPublicURL), "/")
		if base != "" {
			return base + rawURL
		}
	}
	return rawURL
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

func (s *Service) processImageTaskV2(taskID uint64, userID uint64, input imageTaskGenerationInput) {
	defer s.releaseImageTask(taskID)

	if s.imageProvider == nil || !s.imageProvider.Configured() {
		message := ErrProviderNotConfigured.Error()
		balance, err := s.failImageTaskAndRefund(taskID, userID, provider.GenerateResult{}, message)
		if err != nil {
			log.Printf("异步生图任务 %d 失败回滚失败: %v", taskID, err)
			return
		}
		log.Printf("异步生图任务 %d 失败并退款，当前余额 %d", taskID, balance)
		return
	}

	submitCtx, submitCancel := context.WithTimeout(context.Background(), s.cfg.ImageProviderTimeout)
	submitted, submitErr := s.imageProvider.Submit(submitCtx, provider.GenerateRequest{
		Prompt:            input.Prompt,
		Size:              input.Size,
		Style:             input.Style,
		Count:             input.Count,
		Quality:           input.Quality,
		OutputFormat:      input.OutputFormat,
		ReferenceImageURL: input.ReferenceImageURL,
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

	if err := s.recordImageTaskDispatch(taskID, userID, submitted); err != nil {
		message := "保存第三方任务信息失败: " + err.Error()
		balance, updateErr := s.failImageTaskAndRefund(taskID, userID, submitted, message)
		if updateErr != nil {
			log.Printf("异步生图任务 %d 保存任务信息失败后退款异常: %v", taskID, updateErr)
			return
		}
		log.Printf("异步生图任务 %d 无法持久化第三方任务信息，已退款，余额 %d", taskID, balance)
		return
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

	result, pollErr := s.pollImageTaskV2(pollCtx, batchID, submitted)
	if pollErr != nil {
		if result.Status == model.ImageTaskStatusGenerating {
			log.Printf("异步生图任务 %d 本轮轮询未完成，等待后续恢复: %v", taskID, pollErr)
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

	seed := provider.GenerateResult{
		ProviderTaskID: strings.TrimSpace(batchID),
		Status:         model.ImageTaskStatusGenerating,
	}
	result, pollErr := s.pollImageTaskV2(pollCtx, batchID, seed)
	if pollErr != nil {
		if result.Status == model.ImageTaskStatusGenerating {
			log.Printf("恢复任务 %d 本轮轮询未完成，等待后续恢复: %v", taskID, pollErr)
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

func (s *Service) pollImageTaskV2(ctx context.Context, batchID string, seed provider.GenerateResult) (provider.GenerateResult, error) {
	result := seed
	if strings.TrimSpace(result.ProviderTaskID) == "" {
		result.ProviderTaskID = strings.TrimSpace(batchID)
	}
	pollInterval := s.cfg.ImageProviderPollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}

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

		fetched, err := s.imageProvider.Fetch(ctx, batchID)
		if err != nil {
			return result, err
		}
		if fetched.ProviderTaskID == "" {
			fetched.ProviderTaskID = batchID
		}
		result = mergeGenerateResult(result, fetched)

		switch result.Status {
		case model.ImageTaskStatusSuccess:
			if strings.TrimSpace(result.ImageURL) != "" {
				return result, nil
			}
		case model.ImageTaskStatusFailed:
			return result, nil
		}
	}
}

func (s *Service) recordImageTaskDispatch(taskID uint64, userID uint64, result provider.GenerateResult) error {
	updates := map[string]any{
		"provider_task_id":  strings.TrimSpace(result.ProviderTaskID),
		"provider_response": result.RawResponse,
	}
	if strings.TrimSpace(result.ErrorMessage) != "" {
		updates["error_message"] = truncateProviderMessage(result.ErrorMessage)
	}

	return s.db.Model(&model.ImageTask{}).
		Where("id = ? AND user_id = ?", taskID, userID).
		Updates(updates).Error
}

func mergeGenerateResult(base provider.GenerateResult, next provider.GenerateResult) provider.GenerateResult {
	if strings.TrimSpace(next.ProviderTaskID) != "" {
		base.ProviderTaskID = next.ProviderTaskID
	}
	if strings.TrimSpace(next.ImageURL) != "" {
		base.ImageURL = next.ImageURL
	}
	base.ImageURLs = mergeStringURLs(base.ImageURLs, next.ImageURLs)
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
