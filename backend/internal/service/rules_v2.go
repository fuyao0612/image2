package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"
	"image2/backend/internal/provider"

	"gorm.io/gorm"
)

func (s *Service) checkInV2(ctx context.Context, userID uint64) (dto.CheckinResponse, error) {
	_ = ctx

	var result dto.CheckinResponse
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, _, err := s.ensureUserCreditSettlementTx(tx, userID)
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
				GainedCredits: 0,
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

		gainedPoints := s.settingInt64(systemSettingKeyCheckinPoints, 3)
		if gainedPoints < 0 {
			gainedPoints = 0
		}
		gainedCredits := s.settingInt64(systemSettingKeyCheckinCredits, 1)
		if gainedCredits < 0 {
			gainedCredits = 0
		}

		beforePoints := user.PointsBalance
		afterPoints := beforePoints + gainedPoints
		beforeCredits := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
		afterCredits := creditBalance{Free: beforeCredits.Free + gainedCredits, Paid: beforeCredits.Paid}
		lastCheckinAt := now
		lastCheckinDate := today

		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
			"points_balance":    afterPoints,
			"free_credits":      afterCredits.Free,
			"checkin_streak":    streak,
			"last_checkin_at":   lastCheckinAt,
			"last_checkin_date": lastCheckinDate,
		}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.PointsLog{
			UserID:        user.ID,
			ChangeType:    model.PointsChangeTypeCheckin,
			Delta:         gainedPoints,
			BeforeBalance: beforePoints,
			AfterBalance:  afterPoints,
			BizType:       "checkin",
			BizID:         dayKey(today),
			Remark:        "每日签到",
		}).Error; err != nil {
			return err
		}

		if gainedCredits > 0 {
			if err := tx.Create(&model.CreditLog{
				UserID:     user.ID,
				ChangeType: model.CreditChangeTypeCheckin,
				DeltaFree:  gainedCredits,
				DeltaPaid:  0,
				BeforeFree: beforeCredits.Free,
				BeforePaid: beforeCredits.Paid,
				AfterFree:  afterCredits.Free,
				AfterPaid:  afterCredits.Paid,
				BizType:    "checkin",
				BizID:      dayKey(today),
				Remark:     "每日签到",
			}).Error; err != nil {
				return err
			}
		}

		result = dto.CheckinResponse{
			CheckedIn:     true,
			GainedPoints:  gainedPoints,
			GainedCredits: gainedCredits,
			Streak:        streak,
			PointsBalance: afterPoints,
			FreeCredits:   afterCredits.Free,
			PaidCredits:   afterCredits.Paid,
			TotalCredits:  afterCredits.Total(),
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

func (s *Service) getPointsV2(ctx context.Context, userID uint64) (dto.PointsResponse, error) {
	if err := s.ensureUserCreditSettlement(ctx, userID); err != nil {
		return dto.PointsResponse{}, err
	}

	user, err := s.loadUser(userID)
	if err != nil {
		return dto.PointsResponse{}, err
	}

	var logs []model.PointsLog
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Limit(10).Find(&logs).Error; err != nil {
		return dto.PointsResponse{}, err
	}

	var creditLogs []model.CreditLog
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Limit(10).Find(&creditLogs).Error; err != nil {
		return dto.PointsResponse{}, err
	}

	return s.pointsResponse(user, logs, creditLogs), nil
}

func (s *Service) getMeV2(ctx context.Context, userID uint64) (dto.UserInfo, error) {
	if err := s.ensureUserCreditSettlement(ctx, userID); err != nil {
		return dto.UserInfo{}, err
	}

	user, err := s.loadUser(userID)
	if err != nil {
		return dto.UserInfo{}, err
	}
	user = s.syncConfiguredAdminRole(user)
	return s.userInfoBalance(user), nil
}

func (s *Service) createImageTaskV2(ctx context.Context, userID uint64, req dto.CreateImageTaskRequest) (dto.CreateImageTaskResponse, error) {
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

	requestedCount := normalizeImageCount(req.Count)
	size := strings.TrimSpace(req.Size)
	style := strings.TrimSpace(req.Style)
	quality := strings.TrimSpace(req.Quality)
	outputFormat := strings.TrimSpace(req.OutputFormat)
	referenceImageURL := strings.TrimSpace(req.ReferenceImageURL)
	referenceImageURL = s.normalizeReferenceImageURL(referenceImageURL)
	imagePointCost := s.settingInt64(systemSettingKeyImageGeneratePointCost, 20)
	if imagePointCost <= 0 {
		imagePointCost = 20
	}

	input := imageTaskGenerationInput{
		Prompt:            prompt,
		Size:              size,
		Style:             style,
		Count:             requestedCount,
		Quality:           quality,
		OutputFormat:      outputFormat,
		ReferenceImageURL: referenceImageURL,
	}

	var responseData dto.CreateImageTaskResponse
	var task model.ImageTask
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, _, err := s.ensureUserCreditSettlementTx(tx, userID)
		if err != nil {
			return err
		}

		creditSpend := min64(user.FreeCredits+user.PaidCredits, int64(requestedCount))
		remainingCount := int64(requestedCount) - creditSpend
		pointSpend := remainingCount * imagePointCost
		beforePoints := user.PointsBalance
		if beforePoints < pointSpend {
			return ErrInsufficientPoints
		}

		beforeCredits := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
		freeCost, paidCost, afterCredits := beforeCredits.spend(creditSpend)
		afterPoints := beforePoints - pointSpend
		requestSnapshot, _ := json.Marshal(input)

		creditSource := "free"
		switch {
		case creditSpend > 0 && pointSpend > 0:
			creditSource = "mixed"
		case creditSpend > 0 && freeCost > 0 && paidCost > 0:
			creditSource = "mixed"
		case creditSpend > 0 && paidCost > 0:
			creditSource = "paid"
		case creditSpend > 0:
			creditSource = "free"
		case pointSpend > 0:
			creditSource = "points"
		}

		task = model.ImageTask{
			UserID:          userID,
			Prompt:          prompt,
			RequestedCount:  requestedCount,
			CostPoints:      pointSpend,
			CostCredits:     creditSpend,
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
			"free_credits":   afterCredits.Free,
			"paid_credits":   afterCredits.Paid,
			"points_balance": afterPoints,
		}).Error; err != nil {
			return err
		}

		if creditSpend > 0 {
			if err := s.recordCreditLog(tx, userID, model.CreditChangeTypeGenerate, -freeCost, -paidCost, beforeCredits, afterCredits, "image_task", strconv.FormatUint(task.ID, 10), "生图次数消耗"); err != nil {
				return err
			}
		}
		if pointSpend > 0 {
			if err := s.recordPointsLog(tx, userID, model.PointsChangeTypeGenerate, -pointSpend, beforePoints, afterPoints, "image_task", strconv.FormatUint(task.ID, 10), "次数不足时的积分兜底消耗"); err != nil {
				return err
			}
		}

		responseData = dto.CreateImageTaskResponse{
			TaskID:              strconv.FormatUint(task.ID, 10),
			Status:              string(task.Status),
			Balance:             afterCredits.Total(),
			PointsBalance:       afterPoints,
			FreeCreditsBalance:  afterCredits.Free,
			PaidCreditsBalance:  afterCredits.Paid,
			TotalCreditsBalance: afterCredits.Total(),
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

func (s *Service) getImageTaskV2(ctx context.Context, userID uint64, taskID string) (dto.ImageTaskDetailResponse, error) {
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
		RequestedCount:  task.RequestedCount,
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

func (s *Service) listImageTasksV2(ctx context.Context, userID uint64, page, pageSize int) (dto.ListImageTasksResponse, error) {
	page, pageSize = normalizePage(page, pageSize)

	query := s.db.WithContext(ctx).Model(&model.ImageTask{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.ListImageTasksResponse{}, err
	}

	var tasks []model.ImageTask
	if err := query.Order("created_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		return dto.ListImageTasksResponse{}, err
	}

	items := make([]dto.ImageTaskItem, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, dto.ImageTaskItem{
			ID:              strconv.FormatUint(task.ID, 10),
			RequestedCount:  task.RequestedCount,
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

func (s *Service) failImageTaskAndRefundV2(taskID uint64, userID uint64, result provider.GenerateResult, message string) (int64, error) {
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

		balance = user.FreeCredits + user.PaidCredits
		if task.CostCredits > 0 {
			beforeCredits := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
			afterCredits := creditBalance{
				Free: beforeCredits.Free + task.FreeCreditsCost,
				Paid: beforeCredits.Paid + task.PaidCreditsCost,
			}
			if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
				"free_credits": afterCredits.Free,
				"paid_credits": afterCredits.Paid,
			}).Error; err != nil {
				return err
			}
			if err := s.recordCreditLog(tx, userID, model.CreditChangeTypeRefund, task.FreeCreditsCost, task.PaidCreditsCost, beforeCredits, afterCredits, "image_task", strconv.FormatUint(task.ID, 10), "生图失败退回次数"); err != nil {
				return err
			}
			balance = afterCredits.Total()
		}

		if task.CostPoints > 0 {
			beforePoints := user.PointsBalance
			afterPoints := beforePoints + task.CostPoints
			if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("points_balance", afterPoints).Error; err != nil {
				return err
			}
			if err := s.recordPointsLog(tx, userID, model.PointsChangeTypeRefund, task.CostPoints, beforePoints, afterPoints, "image_task", strconv.FormatUint(task.ID, 10), "生图失败退回积分兜底"); err != nil {
				return err
			}
			if task.CostCredits == 0 {
				balance = afterPoints
			}
		}
		return nil
	})
	return balance, err
}

func (s *Service) createRechargeOrderV2(ctx context.Context, userID uint64, req dto.CreateRechargeOrderRequest) (dto.RechargeOrderResponse, error) {
	if err := s.ensureUserCreditSettlement(ctx, userID); err != nil {
		return dto.RechargeOrderResponse{}, err
	}

	var order model.RechargeOrder
	now := s.now()
	rechargeType := strings.TrimSpace(req.RechargeType)
	if rechargeType == "" {
		switch {
		case strings.TrimSpace(req.PackageCode) != "":
			rechargeType = "package"
		case req.Count > 0:
			rechargeType = "custom"
		}
	}
	switch rechargeType {
	case "custom":
		count := req.Count
		if count <= 0 {
			return dto.RechargeOrderResponse{}, ErrPackageInvalid
		}
		unitPrice := s.settingInt64(systemSettingKeyCustomRechargeUnitPriceCents, 20)
		if unitPrice <= 0 {
			unitPrice = 20
		}
		order = model.RechargeOrder{
			UserID:        userID,
			OrderNo:       generateOrderNo(now),
			RechargeType:  "custom",
			PackageCode:   "custom",
			PackageName:   "自定义充值",
			AmountCents:   count * unitPrice,
			PointsAmount:  0,
			CreditsAmount: count,
			BonusPoints:   0,
			Status:        model.RechargeOrderStatusPending,
		}
	case "package":
		pkg, err := s.findBillingPackageByCode(ctx, req.PackageCode, true)
		if err != nil {
			return dto.RechargeOrderResponse{}, err
		}
		if pkg.PackageType != model.BillingPackageTypeRecharge {
			return dto.RechargeOrderResponse{}, ErrPackageInvalid
		}
		order = model.RechargeOrder{
			UserID:        userID,
			OrderNo:       generateOrderNo(now),
			RechargeType:  "package",
			PackageCode:   pkg.Code,
			PackageName:   pkg.Name,
			AmountCents:   pkg.AmountCents,
			PointsAmount:  pkg.BonusPoints,
			CreditsAmount: pkg.CreditsAmount,
			BonusPoints:   pkg.BonusPoints,
			Status:        model.RechargeOrderStatusPending,
		}
	default:
		return dto.RechargeOrderResponse{}, ErrPackageInvalid
	}

	var currentUser model.User
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}
		currentUser = user
		return tx.Create(&order).Error
	}); err != nil {
		return dto.RechargeOrderResponse{}, err
	}

	return rechargeOrderToResponseV2(order, currentUser), nil
}

func (s *Service) confirmRechargeOrderV2(ctx context.Context, userID uint64, orderNo string, req dto.ConfirmRechargeOrderRequest) (dto.RechargeOrderResponse, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return dto.RechargeOrderResponse{}, ErrOrderNotFound
	}

	var settledOrder model.RechargeOrder
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := s.lockRechargeOrder(tx, orderNo)
		if err != nil {
			return err
		}
		if userID != 0 && order.UserID != userID {
			return ErrForbidden
		}
		if req.AmountCents > 0 && req.AmountCents != order.AmountCents {
			return ErrOrderAmountMismatch
		}

		paymentChannel := strings.TrimSpace(req.PaymentChannel)
		if paymentChannel == "" {
			paymentChannel = strings.TrimSpace(order.PaymentChannel)
		}
		if paymentChannel == "" {
			paymentChannel = "mock"
		}
		providerTradeNo := strings.TrimSpace(req.ProviderTradeNo)
		remark := strings.TrimSpace(req.Remark)
		paidAt := s.now()
		if req.PaidAt != nil {
			paidAt = req.PaidAt.In(s.location)
		}

		if order.Status == model.RechargeOrderStatusPaid {
			updates := map[string]any{}
			if order.PaymentChannel == "" && paymentChannel != "" {
				updates["payment_channel"] = paymentChannel
				order.PaymentChannel = paymentChannel
			}
			if order.ProviderTradeNo == "" && providerTradeNo != "" {
				updates["provider_trade_no"] = providerTradeNo
				order.ProviderTradeNo = providerTradeNo
			}
			if order.PaidAt == nil {
				updates["paid_at"] = paidAt
				order.PaidAt = &paidAt
			}
			if len(updates) > 0 {
				if err := tx.Model(&model.RechargeOrder{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
			settledOrder = order
			return nil
		}
		if order.Status != model.RechargeOrderStatusPending {
			return ErrOrderInvalidState
		}

		if _, _, err := s.ensureUserCreditSettlementTx(tx, order.UserID); err != nil {
			return err
		}

		user, err := s.lockUser(tx, order.UserID)
		if err != nil {
			return err
		}
		beforePoints := user.PointsBalance
		beforeCredits := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
		afterPoints := beforePoints + order.BonusPoints
		afterCredits := creditBalance{Free: beforeCredits.Free, Paid: beforeCredits.Paid + order.CreditsAmount}
		if afterCredits.Free < 0 || afterCredits.Paid < 0 {
			return ErrInsufficientCredits
		}

		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
			"points_balance": afterPoints,
			"free_credits":   afterCredits.Free,
			"paid_credits":   afterCredits.Paid,
		}).Error; err != nil {
			return err
		}

		if order.BonusPoints > 0 {
			remarkText := remark
			if remarkText == "" {
				remarkText = fmt.Sprintf("充值订单 %s 赠送积分", order.OrderNo)
			}
			if err := tx.Create(&model.PointsLog{
				UserID:        user.ID,
				ChangeType:    model.PointsChangeTypePurchaseBonus,
				Delta:         order.BonusPoints,
				BeforeBalance: beforePoints,
				AfterBalance:  afterPoints,
				BizType:       "recharge_order",
				BizID:         order.OrderNo,
				Remark:        remarkText,
			}).Error; err != nil {
				return err
			}
		}

		if err := s.recordCreditLog(tx, user.ID, model.CreditChangeTypePurchase, 0, order.CreditsAmount, beforeCredits, afterCredits, "recharge_order", order.OrderNo, "充值购买次数"); err != nil {
			return err
		}

		if providerTradeNo != "" {
			order.ProviderTradeNo = providerTradeNo
		}
		order.PaymentChannel = paymentChannel
		order.PaidAt = &paidAt
		order.Status = model.RechargeOrderStatusPaid

		if err := tx.Model(&model.RechargeOrder{}).Where("id = ?", order.ID).Updates(map[string]any{
			"status":            order.Status,
			"paid_at":           order.PaidAt,
			"payment_channel":   order.PaymentChannel,
			"provider_trade_no": order.ProviderTradeNo,
		}).Error; err != nil {
			return err
		}

		settledOrder = order
		return nil
	})
	if err != nil {
		return dto.RechargeOrderResponse{}, err
	}

	settledUser, err := s.loadUser(settledOrder.UserID)
	if err != nil {
		return dto.RechargeOrderResponse{}, err
	}

	return rechargeOrderToResponseV2(settledOrder, settledUser), nil
}

func (s *Service) mockPurchaseV2(ctx context.Context, userID uint64, req dto.MockPurchaseRequest) (dto.MockPurchaseResponse, error) {
	order, err := s.createRechargeOrderV2(ctx, userID, dto.CreateRechargeOrderRequest{
		RechargeType: "package",
		PackageCode:  req.PackageCode,
	})
	if err != nil {
		return dto.MockPurchaseResponse{}, err
	}
	return s.confirmRechargeOrderV2(ctx, userID, order.OrderNo, dto.ConfirmRechargeOrderRequest{
		ProviderTradeNo: "mock-" + order.OrderNo,
		PaymentChannel:  "mock",
		AmountCents:     order.AmountCents,
		PaidAt:          func() *time.Time { t := time.Now().In(s.location); return &t }(),
		Remark:          "兼容旧版模拟支付",
	})
}

func rechargeOrderToResponseV2(order model.RechargeOrder, user model.User) dto.RechargeOrderResponse {
	item := dto.RechargeOrderItem{
		ID:              strconv.FormatUint(order.ID, 10),
		OrderNo:         order.OrderNo,
		RechargeType:    order.RechargeType,
		PackageCode:     order.PackageCode,
		PackageName:     order.PackageName,
		AmountCents:     order.AmountCents,
		CreditsAmount:   order.CreditsAmount,
		BonusPoints:     order.BonusPoints,
		PointsAmount:    order.PointsAmount,
		Status:          string(order.Status),
		PaymentChannel:  order.PaymentChannel,
		ProviderTradeNo: order.ProviderTradeNo,
		PaidAt:          order.PaidAt,
		CreatedAt:       order.CreatedAt,
	}
	return dto.RechargeOrderResponse{
		RechargeOrderItem:   item,
		PointsBalance:       user.PointsBalance,
		FreeCreditsBalance:  user.FreeCredits,
		PaidCreditsBalance:  user.PaidCredits,
		TotalCreditsBalance: user.FreeCredits + user.PaidCredits,
		Balance:             user.FreeCredits + user.PaidCredits,
	}
}
