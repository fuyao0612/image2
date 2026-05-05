package service

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"image2/backend/internal/config"
	"image2/backend/internal/dto"
	"image2/backend/internal/model"
	"image2/backend/internal/provider"

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
	if err := db.AutoMigrate(&model.User{}, &model.PointsLog{}, &model.CreditLog{}, &model.ImageTask{}, &model.BillingPackage{}, &model.RechargeOrder{}, &model.ContentAuditLog{}, &model.AdminActionLog{}, &model.SystemSetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := New(config.Config{
		JWTSecret:                 "test-secret",
		JWTIssuer:                 "test",
		AccessTokenTTL:            time.Hour,
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

func TestMockPurchaseAddsPaidCreditsAndBonusPoints(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUser(t, db, 0)

	resp, err := svc.MockPurchase(context.Background(), user.ID, dto.MockPurchaseRequest{
		PackageCode: "recharge-300",
	})
	if err != nil {
		t.Fatalf("mock purchase: %v", err)
	}
	if resp.CreditsAmount != 300 || resp.BonusPoints != 20 || resp.PaidCreditsBalance != 300 || resp.PointsBalance != 20 {
		t.Fatalf("unexpected purchase response: %+v", resp)
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.PointsBalance != 20 || reloaded.FreeCredits != 0 || reloaded.PaidCredits != 300 {
		t.Fatalf("unexpected user balances: points=%d free=%d paid=%d", reloaded.PointsBalance, reloaded.FreeCredits, reloaded.PaidCredits)
	}

	var order model.RechargeOrder
	if err := db.Where("user_id = ?", user.ID).First(&order).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if order.CreditsAmount != 300 || order.BonusPoints != 20 || order.PointsAmount != 20 {
		t.Fatalf("unexpected order amounts: credits=%d bonus=%d points=%d", order.CreditsAmount, order.BonusPoints, order.PointsAmount)
	}
}

func TestRechargeOrderFlowIsPendingThenIdempotentlyPaid(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUser(t, db, 0)

	created, err := svc.CreateRechargeOrder(context.Background(), user.ID, dto.CreateRechargeOrderRequest{
		RechargeType: "package",
		PackageCode:  "recharge-300",
	})
	if err != nil {
		t.Fatalf("create recharge order: %v", err)
	}
	if created.Status != string(model.RechargeOrderStatusPending) {
		t.Fatalf("expected pending order, got %s", created.Status)
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.PointsBalance != 0 || reloaded.PaidCredits != 0 {
		t.Fatalf("pending order should not settle balances, points=%d paid=%d", reloaded.PointsBalance, reloaded.PaidCredits)
	}

	paidAt := time.Now()
	paid, err := svc.ConfirmRechargeOrder(context.Background(), user.ID, created.OrderNo, dto.ConfirmRechargeOrderRequest{
		ProviderTradeNo: "trade-1",
		PaymentChannel:  "mock",
		AmountCents:     created.AmountCents,
		PaidAt:          &paidAt,
	})
	if err != nil {
		t.Fatalf("confirm recharge order: %v", err)
	}
	if paid.Status != string(model.RechargeOrderStatusPaid) || paid.PaidCreditsBalance != 300 || paid.PointsBalance != 20 {
		t.Fatalf("unexpected paid response: %+v", paid)
	}

	again, err := svc.ConfirmRechargeOrder(context.Background(), user.ID, created.OrderNo, dto.ConfirmRechargeOrderRequest{
		ProviderTradeNo: "trade-1",
		PaymentChannel:  "mock",
		AmountCents:     created.AmountCents,
	})
	if err != nil {
		t.Fatalf("confirm paid order again: %v", err)
	}
	if again.PaidCreditsBalance != 300 || again.PointsBalance != 20 {
		t.Fatalf("idempotent confirm changed balances: %+v", again)
	}

	var creditLogs int64
	if err := db.Model(&model.CreditLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.CreditChangeTypePurchase).Count(&creditLogs).Error; err != nil {
		t.Fatalf("count credit logs: %v", err)
	}
	var pointsLogs int64
	if err := db.Model(&model.PointsLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.PointsChangeTypePurchaseBonus).Count(&pointsLogs).Error; err != nil {
		t.Fatalf("count points logs: %v", err)
	}
	if creditLogs != 1 || pointsLogs != 1 {
		t.Fatalf("expected one purchase credit log and one bonus points log, got credit=%d points=%d", creditLogs, pointsLogs)
	}
}

func TestCustomRechargeOrderUsesConfiguredUnitPriceWithoutBonusPoints(t *testing.T) {
	svc, db := newTestService(t)
	svc.setSettingsCacheFromValues(map[string]string{
		systemSettingKeyCustomRechargeUnitPriceCents: "30",
	})
	user := createTestUser(t, db, 0)

	created, err := svc.CreateRechargeOrder(context.Background(), user.ID, dto.CreateRechargeOrderRequest{
		RechargeType: "custom",
		Count:        7,
	})
	if err != nil {
		t.Fatalf("create custom recharge order: %v", err)
	}
	if created.RechargeType != "custom" || created.CreditsAmount != 7 || created.AmountCents != 210 || created.BonusPoints != 0 {
		t.Fatalf("unexpected custom order: %+v", created)
	}

	paid, err := svc.ConfirmRechargeOrder(context.Background(), user.ID, created.OrderNo, dto.ConfirmRechargeOrderRequest{
		ProviderTradeNo: "custom-trade-1",
		PaymentChannel:  "mock",
		AmountCents:     created.AmountCents,
	})
	if err != nil {
		t.Fatalf("confirm custom recharge order: %v", err)
	}
	if paid.PaidCreditsBalance != 7 || paid.PointsBalance != 0 {
		t.Fatalf("unexpected custom paid balance: %+v", paid)
	}
}

func TestCreateRechargeOrderInfersLegacyPackagePayload(t *testing.T) {
	svc, db := newTestService(t)
	user := createTestUser(t, db, 0)

	created, err := svc.CreateRechargeOrder(context.Background(), user.ID, dto.CreateRechargeOrderRequest{
		PackageCode: "recharge-300",
	})
	if err != nil {
		t.Fatalf("create legacy package recharge order: %v", err)
	}
	if created.RechargeType != "package" || created.PackageCode != "recharge-300" || created.CreditsAmount != 300 {
		t.Fatalf("unexpected legacy package order: %+v", created)
	}
}

func TestCreateRechargeOrderInfersCustomPayloadFromCount(t *testing.T) {
	svc, db := newTestService(t)
	svc.setSettingsCacheFromValues(map[string]string{
		systemSettingKeyCustomRechargeUnitPriceCents: "30",
	})
	user := createTestUser(t, db, 0)

	created, err := svc.CreateRechargeOrder(context.Background(), user.ID, dto.CreateRechargeOrderRequest{
		Count: 4,
	})
	if err != nil {
		t.Fatalf("create inferred custom recharge order: %v", err)
	}
	if created.RechargeType != "custom" || created.CreditsAmount != 4 || created.AmountCents != 120 {
		t.Fatalf("unexpected inferred custom order: %+v", created)
	}
}

func TestDailyCreditSettlementConvertsExpiredCreditsUsingCustomPrice(t *testing.T) {
	svc, db := newTestService(t)
	svc.setSettingsCacheFromValues(map[string]string{
		systemSettingKeyCustomRechargeUnitPriceCents: "25",
	})
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
	if reloaded.PointsBalance != 130 || reloaded.FreeCredits != 0 || reloaded.PaidCredits != 0 {
		t.Fatalf("unexpected settled balance: points=%d free=%d paid=%d", reloaded.PointsBalance, reloaded.FreeCredits, reloaded.PaidCredits)
	}

	var settlementPoints int64
	if err := db.Model(&model.PointsLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.PointsChangeTypeSettlement).Count(&settlementPoints).Error; err != nil {
		t.Fatalf("count settlement points logs: %v", err)
	}
	var settlementCredits int64
	if err := db.Model(&model.CreditLog{}).Where("user_id = ? AND change_type = ?", user.ID, model.CreditChangeTypeSettlement).Count(&settlementCredits).Error; err != nil {
		t.Fatalf("count settlement credit logs: %v", err)
	}
	if settlementPoints != 1 || settlementCredits != 1 {
		t.Fatalf("expected one settlement point log and credit log, got point=%d credit=%d", settlementPoints, settlementCredits)
	}
}

func TestSameDayCreditsAreNotSettledBeforeNextMidnight(t *testing.T) {
	svc, db := newTestService(t)
	svc.setSettingsCacheFromValues(map[string]string{
		systemSettingKeyCustomRechargeUnitPriceCents: "25",
	})
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
}

func TestCheckinAwardsConfiguredPointsAndCredits(t *testing.T) {
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
	if !resp.CheckedIn || resp.GainedPoints != 6 || resp.GainedCredits != 2 || resp.PointsBalance != 6 || resp.FreeCredits != 2 {
		t.Fatalf("unexpected checkin response: %+v", resp)
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
}
