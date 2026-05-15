package service

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"image2/backend/internal/config"
	"image2/backend/internal/dto"
	"image2/backend/internal/model"
	"image2/backend/internal/provider"
	imagestorage "image2/backend/internal/storage"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var testUserSeq uint64

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.PointsLog{}, &model.CreditLog{}, &model.ImageTask{}, &model.ImageProviderKey{}, &model.BillingPackage{}, &model.RedeemCode{}, &model.ContentAuditLog{}, &model.AdminActionLog{}, &model.SystemSetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := New(config.Config{
		JWTSecret:                 "test-secret",
		JWTIssuer:                 "test",
		AccessTokenTTL:            time.Hour,
		EmailDelivery:             "debug",
		CheckinPoints:             3,
		ImageCostPoints:           1,
		ImageProviderTimeout:      time.Minute,
		ImageProviderPollInterval: time.Millisecond,
		ImageTaskStaleTTL:         30 * time.Minute,
	}, db, nil)
	return svc, db
}

func createTestUser(t *testing.T, db *gorm.DB, balance int64) model.User {
	t.Helper()
	return createTestUserWithCredits(t, db, balance, 0, 0)
}

func createTestUserWithCredits(t *testing.T, db *gorm.DB, points int64, freeCredits int64, paidCredits int64) model.User {
	t.Helper()

	suffix := strconv.FormatUint(atomic.AddUint64(&testUserSeq, 1), 10)
	user := model.User{
		Username:      "tester_" + suffix,
		Email:         "tester_" + suffix + "@example.com",
		PasswordHash:  "hash",
		PointsBalance: points,
		FreeCredits:   freeCredits,
		PaidCredits:   paidCredits,
		Status:        model.UserStatusNormal,
		Role:          model.UserRoleUser,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func boolPtr(v bool) *bool {
	return &v
}

func validCaptchaForTest(t *testing.T, svc *Service) (string, string) {
	t.Helper()
	id := "captcha-test-" + strconv.FormatUint(atomic.AddUint64(&testUserSeq, 1), 10)
	code := "A7K9"
	if err := svc.captchas.Save(context.Background(), id, code, svc.captchaPolicy()); err != nil {
		t.Fatalf("save captcha: %v", err)
	}
	return id, code
}

func uploadFileHeader(t *testing.T, fileName string, data []byte) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	reader := multipart.NewReader(&body, writer.Boundary())
	form, err := reader.ReadForm(int64(body.Len()))
	if err != nil {
		t.Fatalf("read multipart form: %v", err)
	}
	t.Cleanup(func() {
		_ = form.RemoveAll()
	})
	files := form.File["file"]
	if len(files) == 0 {
		t.Fatalf("multipart file was not created")
	}
	return files[0]
}

func createTestTask(t *testing.T, db *gorm.DB, userID uint64, status model.ImageTaskStatus, cost int64) model.ImageTask {
	t.Helper()

	task := model.ImageTask{
		UserID:        userID,
		Prompt:        "test prompt",
		CostPoints:    cost,
		Status:        status,
		Provider:      "mentalout",
		RequestParams: `{"prompt":"test prompt","count":1}`,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task
}

func createTestCreditTask(t *testing.T, db *gorm.DB, userID uint64, status model.ImageTaskStatus, freeCost int64, paidCost int64) model.ImageTask {
	t.Helper()

	source := "free"
	if freeCost > 0 && paidCost > 0 {
		source = "mixed"
	} else if paidCost > 0 {
		source = "paid"
	}
	task := model.ImageTask{
		UserID:          userID,
		Prompt:          "test prompt",
		CostCredits:     freeCost + paidCost,
		FreeCreditsCost: freeCost,
		PaidCreditsCost: paidCost,
		CreditSource:    source,
		Status:          status,
		Provider:        "mentalout",
		RequestParams:   `{"prompt":"test prompt","count":1}`,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create credit task: %v", err)
	}
	return task
}

func TestAdminStatsExcludesAdminUsersAndSplitsAssetSources(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	admin := createTestUserWithCredits(t, db, 100, 50, 50)
	if err := db.Model(&model.User{}).Where("id = ?", admin.ID).Updates(map[string]any{
		"role":  model.UserRoleAdmin,
		"email": "stats-admin@example.com",
	}).Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	zero := int64(0)
	if _, err := svc.UpdateBillingSettings(ctx, admin.ID, dto.UpdateBillingSettingsRequest{
		ImageGeneratePointCost:  10,
		CheckinPoints:           &zero,
		CheckinCredits:          &zero,
		EmailNotifyPointCost:    &zero,
		InviteRewardPoints:      &zero,
		InviteRewardCredits:     &zero,
		RegistrationGiftCredits: &zero,
	}); err != nil {
		t.Fatalf("update billing settings: %v", err)
	}

	user := createTestUserWithCredits(t, db, 25, 3, 4)
	disabled := createTestUserWithCredits(t, db, 5, 1, 0)
	if err := db.Model(&model.User{}).Where("id = ?", disabled.ID).Update("status", model.UserStatusDisabled).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}

	createTestTask(t, db, user.ID, model.ImageTaskStatusSuccess, 0)
	createTestTask(t, db, user.ID, model.ImageTaskStatusFailed, 0)
	createTestTask(t, db, admin.ID, model.ImageTaskStatusGenerating, 0)

	paidAt := time.Now()

	if err := db.Create(&model.ContentAuditLog{
		UserID:      user.ID,
		BizType:     "image_task",
		ContentType: "prompt",
		Content:     "blocked",
		Status:      model.ContentAuditStatusRejected,
		Action:      "reject",
	}).Error; err != nil {
		t.Fatalf("create user audit: %v", err)
	}
	if err := db.Create(&model.ContentAuditLog{
		UserID:      admin.ID,
		BizType:     "image_task",
		ContentType: "prompt",
		Content:     "admin blocked",
		Status:      model.ContentAuditStatusRejected,
		Action:      "reject",
	}).Error; err != nil {
		t.Fatalf("create admin audit: %v", err)
	}

	if err := db.Create(&[]model.RedeemCode{
		{
			Code:          "STAT-ACTIVE",
			PointsAmount:  10,
			CreditsAmount: 1,
			Status:        model.RedeemCodeStatusActive,
			CreatedBy:     admin.ID,
		},
		{
			Code:          "STAT-USED-USER",
			PointsAmount:  20,
			CreditsAmount: 3,
			Status:        model.RedeemCodeStatusUsed,
			CreatedBy:     admin.ID,
			UsedBy:        user.ID,
			UsedAt:        &paidAt,
		},
		{
			Code:          "STAT-USED-ADMIN",
			PointsAmount:  100,
			CreditsAmount: 100,
			Status:        model.RedeemCodeStatusUsed,
			CreatedBy:     admin.ID,
			UsedBy:        admin.ID,
			UsedAt:        &paidAt,
		},
		{
			Code:          "STAT-DISABLED",
			PointsAmount:  50,
			CreditsAmount: 50,
			Status:        model.RedeemCodeStatusDisabled,
			CreatedBy:     admin.ID,
		},
	}).Error; err != nil {
		t.Fatalf("create redeem codes: %v", err)
	}

	stats, err := svc.AdminStats(ctx, admin.ID)
	if err != nil {
		t.Fatalf("admin stats: %v", err)
	}

	if stats.UsersTotal != 2 || stats.UsersDisabled != 1 {
		t.Fatalf("unexpected user stats: total=%d disabled=%d", stats.UsersTotal, stats.UsersDisabled)
	}
	if stats.TasksTotal != 2 || stats.TasksSuccess != 1 || stats.TasksFailed != 1 || stats.TasksGenerating != 0 {
		t.Fatalf("unexpected task stats: %+v", stats)
	}
	if stats.AuditRejected != 1 {
		t.Fatalf("unexpected audit stats: auditRejected=%d", stats.AuditRejected)
	}
	if stats.PointToCreditRate != 10 || stats.PointsBalanceTotal != 30 || stats.CreditsBalanceTotal != 8 || stats.PointsConvertedCreditsTotal != 3 || stats.ResourceCreditsTotal != 11 {
		t.Fatalf("unexpected resource stats: %+v", stats)
	}
	if stats.RedeemCodesActive != 1 || stats.RedeemCodesUsed != 1 || stats.RedeemCodeCreditsTotal != 4 || stats.RedeemCodePointsTotal != 30 || stats.RedeemCodeConvertedCreditsTotal != 7 {
		t.Fatalf("unexpected redeem code stats: %+v", stats)
	}
}

func createTestUserWithPassword(t *testing.T, db *gorm.DB, username, email, password string) model.User {
	t.Helper()

	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := model.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Status:       model.UserStatusNormal,
		Role:         model.UserRoleUser,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create password user: %v", err)
	}
	return user
}

func TestChangePasswordValidatesAndUpdatesLoginPassword(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUserWithPassword(t, db, "password_user", "password_user@example.com", "old-secret-123")

	if _, err := svc.ChangePassword(context.Background(), user.ID, dto.ChangePasswordRequest{
		CurrentPassword:    "wrong-secret",
		NewPassword:        "new-secret-456",
		ConfirmNewPassword: "new-secret-456",
	}); !errors.Is(err, ErrCurrentPasswordIncorrect) {
		t.Fatalf("expected current password error, got %v", err)
	}

	if _, err := svc.ChangePassword(context.Background(), user.ID, dto.ChangePasswordRequest{
		CurrentPassword:    "old-secret-123",
		NewPassword:        "new-secret-456",
		ConfirmNewPassword: "new-secret-789",
	}); !errors.Is(err, ErrPasswordConfirmationMismatch) {
		t.Fatalf("expected confirmation mismatch, got %v", err)
	}

	resp, err := svc.ChangePassword(context.Background(), user.ID, dto.ChangePasswordRequest{
		CurrentPassword:    "old-secret-123",
		NewPassword:        "new-secret-456",
		ConfirmNewPassword: "new-secret-456",
	})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	if resp.ChangedAt.IsZero() {
		t.Fatalf("expected changedAt to be set")
	}

	if _, err := svc.Login(context.Background(), dto.LoginRequest{
		Account:  user.Username,
		Password: "old-secret-123",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected old password to stop working, got %v", err)
	}

	loginResp, err := svc.Login(context.Background(), dto.LoginRequest{
		Account:  user.Username,
		Password: "new-secret-456",
	})
	if err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if loginResp.User.Username != user.Username {
		t.Fatalf("unexpected login response: %+v", loginResp.User)
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if err := comparePassword(reloaded.PasswordHash, "new-secret-456"); err != nil {
		t.Fatalf("stored hash does not match new password: %v", err)
	}
	if reloaded.TokenVersion != 1 {
		t.Fatalf("expected token version to increment, got %d", reloaded.TokenVersion)
	}

	token, _, err := svc.JWT().Generate(user.ID, user.Username, user.Email, 0)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	userID, tokenVersion, err := svc.JWT().Parse(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if userID != user.ID || tokenVersion != 0 {
		t.Fatalf("unexpected token payload: userID=%d tokenVersion=%d", userID, tokenVersion)
	}
}

func TestRegisterGiftsNewUserFreeCredits(t *testing.T) {
	svc, db := newTestService(t)
	svc.cfg.RegistrationEnabled = true
	svc.cfg.EmailCodeCooldown = time.Millisecond
	captchaID, captchaCode := validCaptchaForTest(t, svc)

	codeResp, err := svc.SendEmailCode(context.Background(), dto.SendEmailCodeRequest{
		Email:       "new-user@example.com",
		Scene:       "register",
		CaptchaID:   captchaID,
		CaptchaCode: captchaCode,
	})
	if err != nil {
		t.Fatalf("send email code: %v", err)
	}
	if codeResp.DebugCode == "" {
		t.Fatalf("expected debug code in non-prod test")
	}

	authResp, err := svc.Register(context.Background(), dto.RegisterRequest{
		Email:    "new-user@example.com",
		Username: "new_user",
		Password: "new-user-password",
		Code:     codeResp.DebugCode,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	const defaultRegistrationGiftCredits int64 = 3
	if authResp.User.FreeCredits != defaultRegistrationGiftCredits || authResp.User.PaidCredits != 0 || authResp.User.PointsBalance != 0 {
		t.Fatalf("unexpected registered user balances: %+v", authResp.User)
	}

	var created model.User
	if err := db.Where("email = ?", "new-user@example.com").First(&created).Error; err != nil {
		t.Fatalf("load registered user: %v", err)
	}
	if created.FreeCredits != defaultRegistrationGiftCredits || created.PaidCredits != 0 || created.PointsBalance != 0 {
		t.Fatalf("unexpected stored balances: points=%d free=%d paid=%d", created.PointsBalance, created.FreeCredits, created.PaidCredits)
	}

	var giftLogs int64
	if err := db.Model(&model.CreditLog{}).
		Where("user_id = ? AND change_type = ? AND delta_free = ?", created.ID, model.CreditChangeTypeRegister, defaultRegistrationGiftCredits).
		Count(&giftLogs).Error; err != nil {
		t.Fatalf("count register gift logs: %v", err)
	}
	if giftLogs != 1 {
		t.Fatalf("expected one register gift log, got %d", giftLogs)
	}
}

func TestSendEmailCodeDoesNotExposeDebugCodeInProd(t *testing.T) {
	svc, _ := newTestService(t)
	svc.cfg.AppEnv = "prod"
	svc.cfg.RegistrationEnabled = true
	svc.cfg.EmailDelivery = "debug"
	captchaID, captchaCode := validCaptchaForTest(t, svc)

	codeResp, err := svc.SendEmailCode(context.Background(), dto.SendEmailCodeRequest{
		Email:       "prod-user@example.com",
		Scene:       "register",
		CaptchaID:   captchaID,
		CaptchaCode: captchaCode,
	})
	if !errors.Is(err, ErrEmailSenderNotConfigured) {
		t.Fatalf("expected sender config error, got resp=%+v err=%v", codeResp, err)
	}
	if codeResp.DebugCode != "" {
		t.Fatalf("prod response must not expose debug code, got %q", codeResp.DebugCode)
	}
}

func TestSendEmailCodeRequiresCaptcha(t *testing.T) {
	svc, _ := newTestService(t)
	svc.cfg.RegistrationEnabled = true

	_, err := svc.SendEmailCode(context.Background(), dto.SendEmailCodeRequest{
		Email:       "captcha-required@example.com",
		Scene:       "register",
		CaptchaID:   "missing",
		CaptchaCode: "0000",
	})
	if !errors.Is(err, ErrInvalidCaptcha) {
		t.Fatalf("expected captcha error, got %v", err)
	}
}

func TestSendEmailCodeChecksRegisteredEmailAfterCaptcha(t *testing.T) {
	svc, db := newTestService(t)
	svc.cfg.RegistrationEnabled = true
	createTestUserWithPassword(t, db, "registered_user", "registered@example.com", "password123")
	captchaID, captchaCode := validCaptchaForTest(t, svc)

	_, err := svc.SendEmailCode(context.Background(), dto.SendEmailCodeRequest{
		Email:       "registered@example.com",
		Scene:       "register",
		CaptchaID:   captchaID,
		CaptchaCode: captchaCode,
	})
	if !errors.Is(err, ErrEmailAlreadyRegistered) {
		t.Fatalf("expected registered email error, got %v", err)
	}
}

func TestEnsureActiveUserRejectsDisabledUser(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUser(t, db, 0)
	if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("status", model.UserStatusDisabled).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}

	if err := svc.EnsureActiveUser(context.Background(), user.ID); !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected disabled user error, got %v", err)
	}
}

func TestUploadReferenceImageRejectsNonImageContent(t *testing.T) {
	svc, db := newTestService(t)
	svc.cfg.ImageStorageDriver = "local"
	svc.cfg.ImageStorageLocalDir = t.TempDir()
	svc.cfg.ImageStorageMaxBytes = 1024 * 1024
	var err error
	svc.imageStore, err = imagestorage.New(imagestorage.Config{
		Driver:        svc.cfg.ImageStorageDriver,
		LocalDir:      svc.cfg.ImageStorageLocalDir,
		PublicPath:    "/uploads",
		Timeout:       time.Second,
		MaxBytes:      svc.cfg.ImageStorageMaxBytes,
		PublicBaseURL: "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("new image store: %v", err)
	}
	user := createTestUser(t, db, 0)

	_, err = svc.UploadReferenceImage(context.Background(), user.ID, uploadFileHeader(t, "evil.png", []byte("<html><script>alert(1)</script></html>")))
	if !errors.Is(err, ErrUploadUnsupportedType) {
		t.Fatalf("expected unsupported type error, got %v", err)
	}

	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	resp, err := svc.UploadReferenceImage(context.Background(), user.ID, uploadFileHeader(t, "ok.png", pngBytes))
	if err != nil {
		t.Fatalf("upload valid png: %v", err)
	}
	if resp.ImageURL == "" || resp.ContentType != "image/png" {
		t.Fatalf("unexpected upload response: %+v", resp)
	}
}

func TestReferenceImageForProviderReadsUploadedLocalImage(t *testing.T) {
	svc, db := newTestService(t)
	svc.cfg.ImageStorageDriver = "local"
	svc.cfg.ImageStorageLocalDir = t.TempDir()
	svc.cfg.ImageStorageMaxBytes = 1024 * 1024
	var err error
	svc.imageStore, err = imagestorage.New(imagestorage.Config{
		Driver:        svc.cfg.ImageStorageDriver,
		LocalDir:      svc.cfg.ImageStorageLocalDir,
		PublicPath:    "/uploads",
		Timeout:       time.Second,
		MaxBytes:      svc.cfg.ImageStorageMaxBytes,
		PublicBaseURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("new image store: %v", err)
	}
	user := createTestUser(t, db, 0)
	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	resp, err := svc.UploadReferenceImage(context.Background(), user.ID, uploadFileHeader(t, "ok.png", pngBytes))
	if err != nil {
		t.Fatalf("upload valid png: %v", err)
	}

	image := svc.referenceImageForProvider(context.Background(), resp.ImageURL)
	if image == nil {
		t.Fatalf("expected uploaded image to be read for provider")
	}
	if image.ContentType != "image/png" {
		t.Fatalf("expected image/png, got %q", image.ContentType)
	}
	if string(image.Data) != string(pngBytes) {
		t.Fatalf("unexpected image bytes: %#v", image.Data)
	}
}

func TestFailImageTaskAndRefundIsIdempotent(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUserWithCredits(t, db, 0, 8, 0)
	task := createTestCreditTask(t, db, user.ID, model.ImageTaskStatusGenerating, 2, 0)

	balance, err := svc.failImageTaskAndRefund(task.ID, user.ID, provider.GenerateResult{}, "provider failed")
	if err != nil {
		t.Fatalf("first refund: %v", err)
	}
	if balance != 10 {
		t.Fatalf("expected credits 10 after first refund, got %d", balance)
	}

	balance, err = svc.failImageTaskAndRefund(task.ID, user.ID, provider.GenerateResult{}, "provider failed again")
	if err != nil {
		t.Fatalf("second refund: %v", err)
	}
	if balance != 10 {
		t.Fatalf("expected idempotent credits 10, got %d", balance)
	}

	var refundLogs int64
	if err := db.Model(&model.CreditLog{}).
		Where("user_id = ? AND change_type = ? AND biz_id = ?", user.ID, model.CreditChangeTypeRefund, "1").
		Count(&refundLogs).Error; err != nil {
		t.Fatalf("count refund logs: %v", err)
	}
	if refundLogs != 1 {
		t.Fatalf("expected one refund log, got %d", refundLogs)
	}

	var reloadedTask model.ImageTask
	if err := db.First(&reloadedTask, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	for _, part := range []string{"provider failed", "已原路退回", "邮件通知管理员", "尽快处理"} {
		if !strings.Contains(reloadedTask.ErrorMessage, part) {
			t.Fatalf("expected failure message to contain %q, got %q", part, reloadedTask.ErrorMessage)
		}
	}
}

func TestFailImageTaskAndRefundRestoresMixedPointsAndCredits(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUserWithCredits(t, db, 0, 0, 0)
	task := createTestCreditTask(t, db, user.ID, model.ImageTaskStatusGenerating, 1, 0)
	if err := db.Model(&model.ImageTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"requested_count": 3,
		"cost_points":     40,
		"credit_source":   "mixed",
	}).Error; err != nil {
		t.Fatalf("set mixed task cost: %v", err)
	}

	if _, err := svc.failImageTaskAndRefund(task.ID, user.ID, provider.GenerateResult{}, "provider failed"); err != nil {
		t.Fatalf("refund mixed task: %v", err)
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.PointsBalance != 40 || reloaded.FreeCredits != 1 || reloaded.PaidCredits != 0 {
		t.Fatalf("unexpected mixed refund balance: points=%d free=%d paid=%d", reloaded.PointsBalance, reloaded.FreeCredits, reloaded.PaidCredits)
	}

	var creditRefundLogs int64
	if err := db.Model(&model.CreditLog{}).
		Where("user_id = ? AND change_type = ? AND biz_id = ?", user.ID, model.CreditChangeTypeRefund, strconv.FormatUint(task.ID, 10)).
		Count(&creditRefundLogs).Error; err != nil {
		t.Fatalf("count credit refund logs: %v", err)
	}
	var pointRefundLogs int64
	if err := db.Model(&model.PointsLog{}).
		Where("user_id = ? AND change_type = ? AND biz_id = ?", user.ID, model.PointsChangeTypeRefund, strconv.FormatUint(task.ID, 10)).
		Count(&pointRefundLogs).Error; err != nil {
		t.Fatalf("count point refund logs: %v", err)
	}
	if creditRefundLogs != 1 || pointRefundLogs != 1 {
		t.Fatalf("expected one credit and one point refund log, got credit=%d point=%d", creditRefundLogs, pointRefundLogs)
	}
}

func TestFailedTaskCannotBeCompletedLater(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUserWithCredits(t, db, 0, 8, 0)
	task := createTestCreditTask(t, db, user.ID, model.ImageTaskStatusGenerating, 2, 0)

	if _, err := svc.failImageTaskAndRefund(task.ID, user.ID, provider.GenerateResult{}, "provider failed"); err != nil {
		t.Fatalf("refund: %v", err)
	}
	balance, err := svc.completeImageTask(task.ID, user.ID, provider.GenerateResult{
		Status:   model.ImageTaskStatusSuccess,
		ImageURL: "https://cdn.example.com/image.png",
	})
	if err != nil {
		t.Fatalf("late completion should be ignored, got error: %v", err)
	}
	if balance != 10 {
		t.Fatalf("expected refunded credits to remain 10, got %d", balance)
	}

	var reloaded model.ImageTask
	if err := db.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if reloaded.Status != model.ImageTaskStatusFailed {
		t.Fatalf("expected task to remain failed, got %s", reloaded.Status)
	}
	if reloaded.ImageURL != "" {
		t.Fatalf("expected failed task image url to remain empty, got %q", reloaded.ImageURL)
	}
}

func TestPersistPartialImageTaskKeepsGeneratingStatus(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUserWithCredits(t, db, 0, 8, 0)
	task := createTestCreditTask(t, db, user.ID, model.ImageTaskStatusGenerating, 2, 0)

	err := svc.persistPartialImageTask(context.Background(), task.ID, user.ID, provider.GenerateResult{
		ProviderTaskID: "batch-partial",
		Status:         model.ImageTaskStatusGenerating,
		ImageURL:       "https://cdn.example.com/results/batch-partial/one.png",
		ImageURLs: []string{
			"https://cdn.example.com/results/batch-partial/one.png",
		},
		RawResponse: `{"status":"running","items":[{"imageUrl":"https://cdn.example.com/results/batch-partial/one.png"}]}`,
	})
	if err != nil {
		t.Fatalf("persist partial image task: %v", err)
	}

	var reloaded model.ImageTask
	if err := db.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if reloaded.Status != model.ImageTaskStatusGenerating {
		t.Fatalf("expected task to remain generating, got %s", reloaded.Status)
	}
	if reloaded.ImageURL != "https://cdn.example.com/results/batch-partial/one.png" {
		t.Fatalf("expected partial image url, got %q", reloaded.ImageURL)
	}
	urls := imageURLsFromTask(reloaded)
	if len(urls) != 1 || urls[0] != "https://cdn.example.com/results/batch-partial/one.png" {
		t.Fatalf("expected one partial image url, got %#v", urls)
	}
}

func TestSuccessfulTaskIsNotRefundedLater(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUserWithCredits(t, db, 0, 8, 0)
	task := createTestCreditTask(t, db, user.ID, model.ImageTaskStatusGenerating, 2, 0)

	balance, err := svc.completeImageTask(task.ID, user.ID, provider.GenerateResult{
		Status:   model.ImageTaskStatusSuccess,
		ImageURL: "https://cdn.example.com/image.png",
	})
	if err != nil {
		t.Fatalf("complete task: %v", err)
	}
	if balance != 8 {
		t.Fatalf("expected credits 8 after completion, got %d", balance)
	}

	balance, err = svc.failImageTaskAndRefund(task.ID, user.ID, provider.GenerateResult{}, "late failure")
	if err != nil {
		t.Fatalf("late refund should be ignored, got error: %v", err)
	}
	if balance != 8 {
		t.Fatalf("expected successful task not to refund, got credits %d", balance)
	}

	var refundLogs int64
	if err := db.Model(&model.CreditLog{}).
		Where("user_id = ? AND change_type = ?", user.ID, model.CreditChangeTypeRefund).
		Count(&refundLogs).Error; err != nil {
		t.Fatalf("count refund logs: %v", err)
	}
	if refundLogs != 0 {
		t.Fatalf("expected no refund logs, got %d", refundLogs)
	}
}

func TestImageURLsFromTaskSkipsAPIEndpoints(t *testing.T) {
	task := model.ImageTask{
		ImageURL:  "http://chunfeng.mentalout.top/v1/images/edits",
		ImageURLs: `["http://chunfeng.mentalout.top/v1/images/edits","https://cdn.example.com/results/batch-abc123/final.png"]`,
	}

	urls := imageURLsFromTask(task)
	if len(urls) != 1 {
		t.Fatalf("expected 1 usable url, got %d: %#v", len(urls), urls)
	}
	if urls[0] != "https://cdn.example.com/results/batch-abc123/final.png" {
		t.Fatalf("expected valid generated image url, got %#v", urls)
	}
}

func TestNormalizeGeneratedImageURLsPrefersOriginalOverThumbnail(t *testing.T) {
	result := provider.GenerateResult{
		ImageURL: "https://cdn.example.com/results/batch-abc123/one.png",
		ImageURLs: []string{
			"https://cdn.example.com/results/batch-abc123/thumbs/one.jpg",
			"https://cdn.example.com/results/batch-abc123/two.png",
		},
	}

	urls := normalizeGeneratedImageURLs(result, 2, "")
	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got %d: %#v", len(urls), urls)
	}
	if urls[0] != "https://cdn.example.com/results/batch-abc123/one.png" {
		t.Fatalf("expected first original url, got %#v", urls)
	}
	if urls[1] != "https://cdn.example.com/results/batch-abc123/two.png" {
		t.Fatalf("expected second original url, got %#v", urls)
	}
}

func TestRecoverPendingImageTasksV2MarksStaleTasksFailed(t *testing.T) {
	svc, db := newTestService(t)
	svc.cfg.ImageTaskStaleTTL = time.Second
	user := createTestUserWithCredits(t, db, 0, 8, 0)
	task := createTestCreditTask(t, db, user.ID, model.ImageTaskStatusGenerating, 2, 0)
	staleTime := time.Now().Add(-time.Hour)
	if err := db.Model(&model.ImageTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"created_at": staleTime,
		"updated_at": staleTime,
	}).Error; err != nil {
		t.Fatalf("make task stale: %v", err)
	}

	if err := svc.RecoverPendingImageTasksV2(context.Background()); err != nil {
		t.Fatalf("recover pending tasks: %v", err)
	}

	var reloaded model.ImageTask
	if err := db.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if reloaded.Status != model.ImageTaskStatusFailed {
		t.Fatalf("expected stale task failed, got %s", reloaded.Status)
	}

	var reloadedUser model.User
	if err := db.First(&reloadedUser, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloadedUser.FreeCredits != 10 {
		t.Fatalf("expected stale task refund free credits 10, got %d", reloadedUser.FreeCredits)
	}
}

func TestRetryImageTaskRejectsGeneratingTask(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUser(t, db, 10)
	task := createTestTask(t, db, user.ID, model.ImageTaskStatusGenerating, 2)

	_, err := svc.RetryImageTask(context.Background(), user.ID, strconv.FormatUint(task.ID, 10))
	if !errors.Is(err, ErrTaskStillGenerating) {
		t.Fatalf("expected ErrTaskStillGenerating, got %v", err)
	}
}

func TestCreateImageTaskRejectsUnsafePromptBeforeCharging(t *testing.T) {
	svc, db := newTestService(t)
	svc.cfg.PromptBlockKeywords = []string{"blocked"}
	user := createTestUser(t, db, 10)

	_, err := svc.CreateImageTask(context.Background(), user.ID, dto.CreateImageTaskRequest{
		Prompt: "this contains blocked content",
		Count:  1,
	})
	if !errors.Is(err, ErrContentRejected) {
		t.Fatalf("expected ErrContentRejected, got %v", err)
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.PointsBalance != 10 {
		t.Fatalf("expected balance unchanged, got %d", reloaded.PointsBalance)
	}
	if reloaded.FreeCredits != 0 || reloaded.PaidCredits != 0 {
		t.Fatalf("expected credits unchanged, got free=%d paid=%d", reloaded.FreeCredits, reloaded.PaidCredits)
	}

	var auditLogs int64
	if err := db.Model(&model.ContentAuditLog{}).Where("user_id = ? AND status = ?", user.ID, model.ContentAuditStatusRejected).Count(&auditLogs).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if auditLogs != 1 {
		t.Fatalf("expected one rejected audit log, got %d", auditLogs)
	}
}

func TestNormalizeSupportedImageSizeValidatesCustomPixelRange(t *testing.T) {
	tests := []struct {
		name    string
		size    string
		wantErr bool
	}{
		{name: "minimum total pixels", size: "640x1024"},
		{name: "maximum total pixels", size: "3840x2160"},
		{name: "preset maximum portrait", size: "2160x3840"},
		{name: "side too wide", size: "3841x2160", wantErr: true},
		{name: "side too tall", size: "2160x3841", wantErr: true},
		{name: "below minimum total pixels", size: "640x1023", wantErr: true},
		{name: "above maximum total pixels", size: "3840x2161", wantErr: true},
		{name: "zero side", size: "0x3840", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeSupportedImageSize(tt.size)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidImageSize) {
					t.Fatalf("expected ErrInvalidImageSize, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected valid size, got %v", err)
			}
		})
	}
}

func TestCreateImageTaskRejectsInvalidSizeBeforeAuditAndHistory(t *testing.T) {
	svc, db := newTestService(t)
	svc.cfg.PromptBlockKeywords = []string{"blocked"}
	user := createTestUser(t, db, 10)

	_, err := svc.CreateImageTask(context.Background(), user.ID, dto.CreateImageTaskRequest{
		Prompt: "this contains blocked content",
		Size:   "3841x2160",
		Count:  1,
	})
	if !errors.Is(err, ErrInvalidImageSize) {
		t.Fatalf("expected ErrInvalidImageSize, got %v", err)
	}

	var tasks int64
	if err := db.Model(&model.ImageTask{}).Where("user_id = ?", user.ID).Count(&tasks).Error; err != nil {
		t.Fatalf("count image tasks: %v", err)
	}
	if tasks != 0 {
		t.Fatalf("expected no image task history, got %d", tasks)
	}

	var auditLogs int64
	if err := db.Model(&model.ContentAuditLog{}).Where("user_id = ?", user.ID).Count(&auditLogs).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if auditLogs != 0 {
		t.Fatalf("expected no audit log for invalid size, got %d", auditLogs)
	}
}

func TestRedeemPointsIsDisabled(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUser(t, db, 11)

	_, err := svc.RedeemPoints(context.Background(), user.ID, dto.RedeemPointsRequest{
		PackageCode: "redeem-11-5",
	})
	if !errors.Is(err, ErrPackageInvalid) {
		t.Fatalf("expected disabled redeem to return ErrPackageInvalid, got %v", err)
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.PointsBalance != 11 || reloaded.FreeCredits != 0 || reloaded.PaidCredits != 0 {
		t.Fatalf("unexpected user balances: points=%d free=%d paid=%d", reloaded.PointsBalance, reloaded.FreeCredits, reloaded.PaidCredits)
	}

	var pointLogs int64
	if err := db.Model(&model.PointsLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.PointsChangeTypeRedeem).Count(&pointLogs).Error; err != nil {
		t.Fatalf("count points logs: %v", err)
	}
	var creditLogs int64
	if err := db.Model(&model.CreditLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.CreditChangeTypeRedeem).Count(&creditLogs).Error; err != nil {
		t.Fatalf("count credit logs: %v", err)
	}
	if pointLogs != 0 || creditLogs != 0 {
		t.Fatalf("expected no redeem logs, got point=%d credit=%d", pointLogs, creditLogs)
	}
}

func TestDailyCreditSettlementIsDisabled(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUserWithCredits(t, db, 5, 2, 3)
	yesterday := svc.now().AddDate(0, 0, -1)
	if err := db.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"created_at": yesterday,
	}).Error; err != nil {
		t.Fatalf("move user creation time: %v", err)
	}

	if err := svc.ensureUserCreditSettlement(context.Background(), user.ID); err != nil {
		t.Fatalf("settle credits: %v", err)
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.PointsBalance != 5 || reloaded.FreeCredits != 2 || reloaded.PaidCredits != 3 {
		t.Fatalf("settlement should be disabled, got points=%d free=%d paid=%d", reloaded.PointsBalance, reloaded.FreeCredits, reloaded.PaidCredits)
	}

	var settlementPoints int64
	if err := db.Model(&model.PointsLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.PointsChangeTypeSettlement).Count(&settlementPoints).Error; err != nil {
		t.Fatalf("count settlement points logs: %v", err)
	}
	var settlementCredits int64
	if err := db.Model(&model.CreditLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.CreditChangeTypeSettlement).Count(&settlementCredits).Error; err != nil {
		t.Fatalf("count settlement credit logs: %v", err)
	}
	if settlementPoints != 0 || settlementCredits != 0 {
		t.Fatalf("expected no settlement logs, got point=%d credit=%d", settlementPoints, settlementCredits)
	}
}

func TestSameDayCreditsAreNotSettledBeforeNextMidnight(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUserWithCredits(t, db, 5, 2, 3)

	if err := svc.ensureUserCreditSettlement(context.Background(), user.ID); err != nil {
		t.Fatalf("settle credits: %v", err)
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.PointsBalance != 5 || reloaded.FreeCredits != 2 || reloaded.PaidCredits != 3 {
		t.Fatalf("same-day credits should remain, got points=%d free=%d paid=%d", reloaded.PointsBalance, reloaded.FreeCredits, reloaded.PaidCredits)
	}
	if reloaded.LastCreditSettlementDate != nil {
		t.Fatalf("settlement is disabled and should not update settlement date, got %v", reloaded.LastCreditSettlementDate)
	}
}

func TestResetAllUsersCreditsClearsEveryUserBalance(t *testing.T) {
	svc, db := newTestService(t)
	userA := createTestUserWithCredits(t, db, 12, 5, 7)
	userB := createTestUserWithCredits(t, db, 3, 1, 2)

	if err := svc.ResetAllUsersCredits(context.Background()); err != nil {
		t.Fatalf("reset all users credits: %v", err)
	}

	var users []model.User
	if err := db.Order("id asc").Find(&users).Error; err != nil {
		t.Fatalf("reload users: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].ID != userA.ID || users[1].ID != userB.ID {
		t.Fatalf("unexpected user order: %+v", users)
	}
	for _, user := range users {
		if user.FreeCredits != 0 || user.PaidCredits != 0 {
			t.Fatalf("user %d credits not cleared: points=%d free=%d paid=%d", user.ID, user.PointsBalance, user.FreeCredits, user.PaidCredits)
		}
	}
}

func TestCheckinAwardsConfiguredCreditsOnly(t *testing.T) {
	svc, db := newTestService(t)
	svc.setSettingsCacheFromValues(map[string]string{
		systemSettingKeyCheckinPoints:  "6",
		systemSettingKeyCheckinCredits: "2",
	})
	user := createTestUser(t, db, 0)

	resp, err := svc.CheckIn(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("checkin: %v", err)
	}
	if !resp.CheckedIn || resp.GainedPoints != 0 || resp.GainedCredits != 2 || resp.PointsBalance != 0 || resp.FreeCredits != 2 {
		t.Fatalf("unexpected checkin response: %+v", resp)
	}

	var pointLogs int64
	if err := db.Model(&model.PointsLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.PointsChangeTypeCheckin).Count(&pointLogs).Error; err != nil {
		t.Fatalf("count checkin point logs: %v", err)
	}
	var creditLogs int64
	if err := db.Model(&model.CreditLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.CreditChangeTypeCheckin).Count(&creditLogs).Error; err != nil {
		t.Fatalf("count checkin credit logs: %v", err)
	}
	if pointLogs != 0 || creditLogs != 1 {
		t.Fatalf("expected only one credit checkin log, got point=%d credit=%d", pointLogs, creditLogs)
	}
}

func TestAdminPackageV2WritesAuditAndProtectsLastEnabledPackage(t *testing.T) {
	svc, db := newTestService(t)
	admin := createTestUser(t, db, 0)
	if err := db.Model(&model.User{}).Where("id = ?", admin.ID).Update("role", model.UserRoleAdmin).Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	created, err := svc.AdminCreatePackageV2(context.Background(), admin.ID, dto.AdminCreatePackageRequest{
		Code:          "recharge-test-audit",
		Name:          "Audit Recharge",
		PackageType:   string(model.BillingPackageTypeRecharge),
		CreditsAmount: 12,
		AmountCents:   1200,
		BonusPoints:   2,
		Enabled:       boolPtr(true),
		SortOrder:     99,
	})
	if err != nil {
		t.Fatalf("create package v2: %v", err)
	}
	if created.Code != "recharge-test-audit" {
		t.Fatalf("unexpected created package: %+v", created)
	}

	var actionLogs int64
	if err := db.Model(&model.AdminActionLog{}).Where("action = ?", "billing_package.create").Count(&actionLogs).Error; err != nil {
		t.Fatalf("count create logs: %v", err)
	}
	if actionLogs != 1 {
		t.Fatalf("expected one package create audit log, got %d", actionLogs)
	}

	if err := svc.ensureBillingPackages(context.Background()); err != nil {
		t.Fatalf("ensure packages: %v", err)
	}
	var rechargePackages []model.BillingPackage
	if err := db.Where("package_type = ?", model.BillingPackageTypeRecharge).Order("id asc").Find(&rechargePackages).Error; err != nil {
		t.Fatalf("list recharge packages: %v", err)
	}
	if len(rechargePackages) < 2 {
		t.Fatalf("expected recharge packages, got %d", len(rechargePackages))
	}

	target := rechargePackages[0]
	if err := db.Model(&model.BillingPackage{}).
		Where("package_type = ? AND id <> ?", model.BillingPackageTypeRecharge, target.ID).
		Update("enabled", false).Error; err != nil {
		t.Fatalf("disable other recharge packages: %v", err)
	}

	disabled := false
	sortOrder := target.SortOrder
	_, err = svc.AdminUpdatePackageV2(context.Background(), admin.ID, strconv.FormatUint(target.ID, 10), dto.AdminUpdatePackageRequest{
		Name:          target.Name,
		CreditsAmount: target.CreditsAmount,
		PointsCost:    0,
		AmountCents:   target.AmountCents,
		BonusPoints:   target.BonusPoints,
		Enabled:       &disabled,
		SortOrder:     &sortOrder,
		Note:          target.Note,
	})
	if !errors.Is(err, ErrPackageProtected) {
		t.Fatalf("expected ErrPackageProtected, got %v", err)
	}
}

func TestAdminUseProviderKeyMarksSingleCurrentKey(t *testing.T) {
	svc, db := newTestService(t)
	admin := createTestUser(t, db, 0)
	if err := db.Model(&model.User{}).Where("id = ?", admin.ID).Update("role", model.UserRoleAdmin).Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	first, err := svc.AdminCreateProviderKey(context.Background(), admin.ID, dto.AdminProviderKeyPayload{
		Name:              "first key",
		Provider:          "mentalout",
		APIKey:            "sk-first",
		Priority:          20,
		AutoSwitchEnabled: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("create first key: %v", err)
	}
	second, err := svc.AdminCreateProviderKey(context.Background(), admin.ID, dto.AdminProviderKeyPayload{
		Name:              "second key",
		Provider:          "mentalout",
		APIKey:            "sk-second",
		Priority:          10,
		AutoSwitchEnabled: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("create second key: %v", err)
	}

	used, err := svc.AdminUseProviderKey(context.Background(), admin.ID, first.ID)
	if err != nil {
		t.Fatalf("use first key: %v", err)
	}
	if !used.IsUsing || used.Status != string(model.ImageProviderKeyStatusActive) {
		t.Fatalf("expected first key using and active, got using=%v status=%s", used.IsUsing, used.Status)
	}

	used, err = svc.AdminUseProviderKey(context.Background(), admin.ID, second.ID)
	if err != nil {
		t.Fatalf("use second key: %v", err)
	}
	if !used.IsUsing {
		t.Fatalf("expected second key using")
	}

	var keys []model.ImageProviderKey
	if err := db.Order("id asc").Find(&keys).Error; err != nil {
		t.Fatalf("list keys: %v", err)
	}
	usingCount := 0
	for _, key := range keys {
		if key.IsUsing {
			usingCount++
			if strconv.FormatUint(key.ID, 10) != second.ID {
				t.Fatalf("expected only second key using, got key %d", key.ID)
			}
		}
	}
	if usingCount != 1 {
		t.Fatalf("expected exactly one using key, got %d", usingCount)
	}

	activeKeys, err := svc.ListActiveProviderKeys(context.Background())
	if err != nil {
		t.Fatalf("list active keys: %v", err)
	}
	if len(activeKeys) == 0 || strconv.FormatUint(activeKeys[0].ID, 10) != second.ID {
		t.Fatalf("expected using key first in provider selection, got %+v", activeKeys)
	}
}

func TestAdminListUsersSearchesByID(t *testing.T) {
	svc, db := newTestService(t)
	admin := createTestUser(t, db, 0)
	user := createTestUser(t, db, 10)
	_ = createTestUser(t, db, 20)
	if err := db.Model(&model.User{}).Where("id = ?", admin.ID).Updates(map[string]any{
		"email": "admin@example.com",
		"role":  model.UserRoleAdmin,
	}).Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	resp, err := svc.AdminListUsers(context.Background(), admin.ID, "#"+strconv.FormatUint(user.ID, 10), "", 1, 10)
	if err != nil {
		t.Fatalf("list users by id: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 || resp.Items[0].ID != strconv.FormatUint(user.ID, 10) {
		t.Fatalf("expected only user %d, got total=%d items=%+v", user.ID, resp.Total, resp.Items)
	}

	if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("status", model.UserStatusDisabled).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}
	disabledResp, err := svc.AdminListUsers(context.Background(), admin.ID, "", "disabled", 1, 10)
	if err != nil {
		t.Fatalf("list disabled users: %v", err)
	}
	if disabledResp.Total != 1 || len(disabledResp.Items) != 1 || disabledResp.Items[0].ID != strconv.FormatUint(user.ID, 10) {
		t.Fatalf("expected only disabled user %d, got total=%d items=%+v", user.ID, disabledResp.Total, disabledResp.Items)
	}
}

func TestAdminUpdateUserStatusCanReenableAndWritesAuditLog(t *testing.T) {
	svc, db := newTestService(t)
	admin := createTestUser(t, db, 0)
	user := createTestUser(t, db, 10)
	if err := db.Model(&model.User{}).Where("id = ?", admin.ID).Updates(map[string]any{
		"email": "admin@example.com",
		"role":  model.UserRoleAdmin,
	}).Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	disabled, err := svc.AdminUpdateUserStatus(context.Background(), admin.ID, strconv.FormatUint(user.ID, 10), dto.AdminUpdateUserStatusRequest{
		Status: int8(model.UserStatusDisabled),
	})
	if err != nil {
		t.Fatalf("disable user: %v", err)
	}
	if disabled.Status != int8(model.UserStatusDisabled) {
		t.Fatalf("expected disabled status, got %d", disabled.Status)
	}

	enabled, err := svc.AdminUpdateUserStatus(context.Background(), admin.ID, strconv.FormatUint(user.ID, 10), dto.AdminUpdateUserStatusRequest{
		Status: int8(model.UserStatusNormal),
	})
	if err != nil {
		t.Fatalf("enable user: %v", err)
	}
	if enabled.Status != int8(model.UserStatusNormal) {
		t.Fatalf("expected normal status, got %d", enabled.Status)
	}

	var logs []model.AdminActionLog
	if err := db.Where("action = ? AND target_type = ? AND target_id = ?", "user.status.update", "user", strconv.FormatUint(user.ID, 10)).
		Order("id asc").
		Find(&logs).Error; err != nil {
		t.Fatalf("list action logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected two status audit logs, got %d", len(logs))
	}
	if logs[0].Remark != "禁用用户" || logs[1].Remark != "启用用户" {
		t.Fatalf("unexpected audit remarks: %+v", logs)
	}

	logResp, err := svc.AdminListActionLogs(context.Background(), admin.ID, "", "", "user.status.update", 1, 10)
	if err != nil {
		t.Fatalf("list action logs by action: %v", err)
	}
	if logResp.Total != 2 {
		t.Fatalf("expected two status action logs by action filter, got %d", logResp.Total)
	}
}

func TestAdminAdjustPointsRequiresAdminAndWritesLog(t *testing.T) {
	svc, db := newTestService(t)
	admin := createTestUser(t, db, 0)
	user := createTestUser(t, db, 10)
	if err := db.Model(&model.User{}).Where("id = ?", admin.ID).Updates(map[string]any{
		"email": "admin@example.com",
		"role":  model.UserRoleAdmin,
	}).Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	resp, err := svc.AdminAdjustPoints(context.Background(), admin.ID, strconv.FormatUint(user.ID, 10), dto.AdminAdjustPointsRequest{
		Delta:  5,
		Remark: "test adjust",
	})
	if err != nil {
		t.Fatalf("admin adjust points: %v", err)
	}
	if resp.AfterBalance != 15 {
		t.Fatalf("expected after balance 15, got %d", resp.AfterBalance)
	}

	var logs int64
	if err := db.Model(&model.PointsLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.PointsChangeTypeAdjust).Count(&logs).Error; err != nil {
		t.Fatalf("count points logs: %v", err)
	}
	if logs != 1 {
		t.Fatalf("expected one adjust log, got %d", logs)
	}

	var actionLogs int64
	if err := db.Model(&model.AdminActionLog{}).
		Where("action = ? AND target_type = ? AND target_id = ?", "user.points.adjust", "user", strconv.FormatUint(user.ID, 10)).
		Count(&actionLogs).Error; err != nil {
		t.Fatalf("count action logs: %v", err)
	}
	if actionLogs != 1 {
		t.Fatalf("expected one points audit log, got %d", actionLogs)
	}
}

func TestAdminListTaskAndRedeemKeywordFilters(t *testing.T) {
	svc, db := newTestService(t)
	admin := createTestUser(t, db, 0)
	user := createTestUser(t, db, 10)
	if err := db.Model(&model.User{}).Where("id = ?", admin.ID).Updates(map[string]any{
		"email": "admin@example.com",
		"role":  model.UserRoleAdmin,
	}).Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	task := createTestTask(t, db, user.ID, model.ImageTaskStatusSuccess, 2)
	if err := db.Model(&model.ImageTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"prompt":   "city skyline admin search",
		"provider": "mentalout",
	}).Error; err != nil {
		t.Fatalf("update task: %v", err)
	}
	taskResp, err := svc.AdminListImageTasks(context.Background(), admin.ID, "", "skyline", 1, 10)
	if err != nil {
		t.Fatalf("list tasks by keyword: %v", err)
	}
	if taskResp.Total != 1 || len(taskResp.Items) != 1 || taskResp.Items[0].ID != strconv.FormatUint(task.ID, 10) {
		t.Fatalf("expected task %d by keyword, got total=%d items=%+v", task.ID, taskResp.Total, taskResp.Items)
	}

	code := model.RedeemCode{
		Code:          "VIPADMINSEARCH001",
		PointsAmount:  10,
		CreditsAmount: 2,
		Status:        model.RedeemCodeStatusActive,
		BatchNo:       "BATCH-ADMIN-SEARCH",
		CreatedBy:     admin.ID,
	}
	if err := db.Create(&code).Error; err != nil {
		t.Fatalf("create redeem code: %v", err)
	}
	codeResp, err := svc.AdminListRedeemCodes(context.Background(), admin.ID, "active", "VIP-ADMIN", 1, 10)
	if err != nil {
		t.Fatalf("list redeem codes by keyword: %v", err)
	}
	if codeResp.Total != 1 || len(codeResp.Items) != 1 || codeResp.Items[0].ID != strconv.FormatUint(code.ID, 10) {
		t.Fatalf("expected redeem code %d by keyword, got total=%d items=%+v", code.ID, codeResp.Total, codeResp.Items)
	}
}

func TestRedeemCodeRejectsExpiredCode(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUser(t, db, 0)
	expiredAt := svc.now().Add(-time.Minute)
	code := model.RedeemCode{
		Code:         "EXPIRED001",
		PointsAmount: 10,
		Status:       model.RedeemCodeStatusActive,
		ExpiresAt:    &expiredAt,
	}
	if err := db.Create(&code).Error; err != nil {
		t.Fatalf("create expired redeem code: %v", err)
	}

	_, err := svc.RedeemCode(context.Background(), user.ID, dto.RedeemCodeRequest{Code: code.Code})
	if !errors.Is(err, ErrRedeemCodeExpired) {
		t.Fatalf("expected ErrRedeemCodeExpired, got %v", err)
	}

	var unchanged model.RedeemCode
	if err := db.First(&unchanged, code.ID).Error; err != nil {
		t.Fatalf("reload redeem code: %v", err)
	}
	if unchanged.Status != model.RedeemCodeStatusActive || unchanged.UsedBy != 0 || unchanged.UsedAt != nil {
		t.Fatalf("expired code should not be consumed: %+v", unchanged)
	}
}

func TestCreateImageTaskRejectsExternalReferenceImageURL(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUser(t, db, 100)
	svc.cfg.BackendPublicURL = "https://example.com"
	svc.cfg.ImageStoragePublicPath = "/uploads"

	_, err := svc.CreateImageTask(context.Background(), user.ID, dto.CreateImageTaskRequest{
		Prompt:            "test prompt",
		Count:             1,
		ReferenceImageURL: "https://169.254.169.254/latest/meta-data",
	})
	if !errors.Is(err, ErrInvalidReferenceImageURL) {
		t.Fatalf("expected ErrInvalidReferenceImageURL, got %v", err)
	}
}

func TestNormalizeReferenceImageURLAllowsOwnUploadsOnly(t *testing.T) {
	svc, _ := newTestService(t)
	svc.cfg.BackendPublicURL = "https://example.com"
	svc.cfg.ImageStoragePublicPath = "/uploads"

	normalized, err := svc.normalizeReferenceImageURL(1, "/uploads/reference-images/user-1/ok.png")
	if err != nil {
		t.Fatalf("expected own upload path to pass, got %v", err)
	}
	if normalized != "https://example.com/uploads/reference-images/user-1/ok.png" {
		t.Fatalf("unexpected normalized url: %s", normalized)
	}

	normalized, err = svc.normalizeReferenceImageURL(1, "/uploads/users/user-1/references/2026/05/13-1.png")
	if err != nil {
		t.Fatalf("expected own new reference path to pass, got %v", err)
	}
	if normalized != "https://example.com/uploads/users/user-1/references/2026/05/13-1.png" {
		t.Fatalf("unexpected normalized new url: %s", normalized)
	}

	normalized, err = svc.normalizeReferenceImageURL(1, "https://example.com/uploads/users/user-2/references/2026/05/13-1.png")
	if err != nil {
		t.Fatalf("expected existing site reference url to pass, got %v", err)
	}
	if normalized != "https://example.com/uploads/users/user-2/references/2026/05/13-1.png" {
		t.Fatalf("unexpected normalized shared reference url: %s", normalized)
	}

	if _, err := svc.normalizeReferenceImageURL(1, "https://example.com/api/v1/admin/stats"); !errors.Is(err, ErrInvalidReferenceImageURL) {
		t.Fatalf("expected own non-upload path to be rejected, got %v", err)
	}
}

func TestValidateBillingPackageAllowsZeroBonusPoints(t *testing.T) {
	err := validateBillingPackage(model.BillingPackage{
		Code:          "recharge-zero-bonus",
		Name:          "Zero bonus",
		PackageType:   model.BillingPackageTypeRecharge,
		CreditsAmount: 100,
		AmountCents:   1999,
		BonusPoints:   0,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("expected zero bonus points to be valid, got %v", err)
	}
}

func TestProviderCooldownErrorIsNotBalanceError(t *testing.T) {
	message := "upstream http 429: All credentials for model gpt-5.4-mini are cooling down; code=model_cooldown; type=upstream_error"

	if isProviderBalanceError(message) {
		t.Fatalf("cooldown should not be treated as low balance")
	}
	if !isProviderCooldownError(message) {
		t.Fatalf("expected cooldown message to be detected")
	}
}

func TestProviderFailureMessageNormalizesCooldown(t *testing.T) {
	message := providerFailureMessage(provider.GenerateResult{
		ErrorMessage: "upstream http 429: All credentials for model gpt-5.4-mini are cooling down; code=model_cooldown; type=upstream_error",
	}, nil)

	if !strings.Contains(message, "gpt-5.4-mini") || !strings.Contains(message, "限流冷却") {
		t.Fatalf("unexpected normalized message: %s", message)
	}
}

func TestProviderFailureMessageNormalizesCommonUpstreamErrors(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		contains []string
	}{
		{
			name:     "model channel missing",
			raw:      "upstream http 503: No available channel for model gpt-image-2 under group gpt (distributor) (request id: 202605110328368060305868268d9d6oMr8kCN1); code=model_not_found; type=new_api_error",
			contains: []string{"gpt-image-2", "没有可用的生图模型通道"},
		},
		{
			name:     "invalid key",
			raw:      "upstream http 401: Invalid token; code=invalid_api_key; type=new_api_error",
			contains: []string{"API Key", "无效"},
		},
		{
			name:     "quota",
			raw:      "upstream http 429: insufficient quota; code=insufficient_quota; type=new_api_error",
			contains: []string{"余额", "额度不足"},
		},
		{
			name:     "temporary upstream",
			raw:      "upstream http 502: bad gateway; type=upstream_error",
			contains: []string{"暂时不可用"},
		},
		{
			name:     "internal server stream error",
			raw:      "upstream http 500: stream error: stream ID 15403; INTERNAL_ERROR; received from peer; code=internal_server_error; type=server_error",
			contains: []string{"暂时不可用", "服务商通道状态"},
		},
		{
			name:     "protocol mismatch",
			raw:      "upstream http 400: 400 The plain HTTP request was sent to HTTPS port",
			contains: []string{"协议配置不匹配", "API Base URL"},
		},
		{
			name:     "reference image",
			raw:      "reference image content type is not supported: text/html",
			contains: []string{"参考图处理失败", "重新上传"},
		},
		{
			name:     "network lookup",
			raw:      "Post \"https://image.mentalout.top/api/batches\": dial tcp: lookup image.mentalout.top: no such host",
			contains: []string{"无法连接上游生图服务", "服务器网络"},
		},
		{
			name:     "generic bad request",
			raw:      "upstream http 400: invalid request",
			contains: []string{"1024x1024", "1 张", "模型和 Key 配置"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := providerFailureMessage(provider.GenerateResult{ErrorMessage: tc.raw}, nil)
			for _, part := range tc.contains {
				if !strings.Contains(message, part) {
					t.Fatalf("expected %q to contain %q", message, part)
				}
			}
		})
	}
}

func TestBuildImageFailureEmailIncludesKeyFields(t *testing.T) {
	subject, body := buildImageFailureEmail(imageTaskFailureNotice{
		TaskID:         123,
		UserID:         456,
		Prompt:         "a tiny robot on a rainy street",
		Message:        "上游模型 gpt-image-2 当前限流冷却中",
		Provider:       "mentalout",
		ProviderTaskID: "batch-abc123",
		RawResponse:    "raw upstream response",
		CreatedAt:      time.Date(2026, 5, 11, 3, 30, 0, 0, time.Local),
	})

	if !strings.Contains(subject, "123") {
		t.Fatalf("unexpected subject: %s", subject)
	}
	for _, part := range []string{"任务ID：123", "用户ID：456", "上游任务ID：batch-abc123", "失败原因：", "raw upstream response"} {
		if !strings.Contains(body, part) {
			t.Fatalf("expected body to contain %q, got %s", part, body)
		}
	}
}

func TestDedupeEmailsNormalizesAndDedupes(t *testing.T) {
	emails := dedupeEmails([]string{"Admin@example.com", " admin@example.com ", "", "ADMIN@example.com", "ops@example.com"})
	if len(emails) != 2 {
		t.Fatalf("expected 2 unique emails, got %v", emails)
	}
	if emails[0] != "admin@example.com" || emails[1] != "ops@example.com" {
		t.Fatalf("unexpected deduped emails: %v", emails)
	}
}
