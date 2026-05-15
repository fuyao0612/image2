package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"
	"image2/backend/internal/provider"

	"gorm.io/gorm"
)

func (s *Service) checkInV2(ctx context.Context, userID uint64) (dto.CheckinResponse, error) {
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

		gainedPoints := int64(0)
		gainedCredits := s.settingInt64(systemSettingKeyCheckinCredits, 1)
		if gainedCredits < 0 {
			gainedCredits = 0
		}

		beforePoints := user.PointsBalance
		afterPoints := beforePoints
		beforeCredits := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
		afterCredits := creditBalance{Free: beforeCredits.Free + gainedCredits, Paid: beforeCredits.Paid}
		lastCheckinAt := now
		lastCheckinDate := today

		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
			"free_credits":      afterCredits.Free,
			"checkin_streak":    streak,
			"last_checkin_at":   lastCheckinAt,
			"last_checkin_date": lastCheckinDate,
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

	response := s.pointsResponse(user, logs, creditLogs)
	checkinPointsTotal, checkinCreditsTotal, err := s.checkinRewardTotals(ctx, userID)
	if err != nil {
		return dto.PointsResponse{}, err
	}
	response.CheckinPointsTotal = checkinPointsTotal
	response.CheckinCreditsTotal = checkinCreditsTotal
	return response, nil
}

func (s *Service) getMeV2(ctx context.Context, userID uint64) (dto.UserInfo, error) {
	user, err := s.loadUser(userID)
	if err != nil {
		return dto.UserInfo{}, err
	}
	user = s.syncConfiguredAdminRole(user)
	return s.userInfoBalance(user), nil
}

func (s *Service) createImageTaskV2(ctx context.Context, userID uint64, req dto.CreateImageTaskRequest) (dto.CreateImageTaskResponse, error) {
	_ = ctx
	if !s.settingBool(systemSettingKeyImageGenerationEnabled, true) {
		return dto.CreateImageTaskResponse{}, ErrImageGenerationDisabled
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return dto.CreateImageTaskResponse{}, ErrPromptRequired
	}

	requestedCount := normalizeImageCount(req.Count)
	size, err := normalizeSupportedImageSize(req.Size)
	if err != nil {
		return dto.CreateImageTaskResponse{}, err
	}

	decision := s.moderatePrompt(prompt)
	if decision.Status == model.ContentAuditStatusRejected {
		_ = s.recordContentAudit(ctx, userID, "image_task", "", "prompt", prompt, decision)
		return dto.CreateImageTaskResponse{}, ErrContentRejected
	}
	referenceImageURL, err := s.normalizeReferenceImageURL(userID, strings.TrimSpace(req.ReferenceImageURL))
	if err != nil {
		return dto.CreateImageTaskResponse{}, err
	}
	activeKeys, keyErr := s.ListActiveProviderKeys(ctx)
	if keyErr != nil {
		return dto.CreateImageTaskResponse{}, keyErr
	}
	if (s.imageProvider == nil || !s.imageProvider.Configured()) && len(activeKeys) == 0 {
		return dto.CreateImageTaskResponse{}, ErrProviderNotConfigured
	}

	style := strings.TrimSpace(req.Style)
	quality := strings.TrimSpace(req.Quality)
	outputFormat := strings.TrimSpace(req.OutputFormat)
	imagePointCost := s.settingInt64(systemSettingKeyImageGeneratePointCost, 20)
	if imagePointCost <= 0 {
		imagePointCost = 20
	}
	emailNotifyCost := int64(0)
	if req.EmailNotifyOnComplete {
		if !s.settingBool(systemSettingKeyEmailNotifyEnabled, false) {
			return dto.CreateImageTaskResponse{}, ErrEmailNotifyDisabled
		}
		emailNotifyCost = s.settingInt64(systemSettingKeyEmailNotifyPointCost, 1)
		if emailNotifyCost <= 0 {
			return dto.CreateImageTaskResponse{}, ErrSettingsInvalid
		}
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
	err = s.db.Transaction(func(tx *gorm.DB) error {
		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}
		if req.EmailNotifyOnComplete && normalizeEmail(user.Email) == "" {
			return ErrEmailNotifyEmailMissing
		}

		creditSpend := min64(user.FreeCredits+user.PaidCredits, int64(requestedCount))
		remainingCount := int64(requestedCount) - creditSpend
		pointSpend := remainingCount * imagePointCost
		totalPointSpend := pointSpend + emailNotifyCost
		beforePoints := user.PointsBalance
		if beforePoints < totalPointSpend {
			return ErrInsufficientPoints
		}

		beforeCredits := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
		freeCost, paidCost, afterCredits := beforeCredits.spend(creditSpend)
		afterImagePoints := beforePoints - pointSpend
		afterPoints := beforePoints - totalPointSpend
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
			UserID:                userID,
			Prompt:                prompt,
			RequestedCount:        requestedCount,
			CostPoints:            pointSpend,
			CostCredits:           creditSpend,
			FreeCreditsCost:       freeCost,
			PaidCreditsCost:       paidCost,
			CreditSource:          creditSource,
			Status:                model.ImageTaskStatusGenerating,
			Provider:              s.imageProvider.Name(),
			RequestParams:         string(requestSnapshot),
			EmailNotifyEnabled:    req.EmailNotifyOnComplete,
			EmailNotifyCostPoints: emailNotifyCost,
			EmailNotifyStatus:     emailNotifyStatus(req.EmailNotifyOnComplete),
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
			if err := s.recordCreditLog(tx, userID, model.CreditChangeTypeGenerate, -freeCost, -paidCost, beforeCredits, afterCredits, "image_task", strconv.FormatUint(task.ID, 10), "生图福利次数消耗"); err != nil {
				return err
			}
		}
		if pointSpend > 0 {
			if err := s.recordPointsLog(tx, userID, model.PointsChangeTypeGenerate, -pointSpend, beforePoints, afterImagePoints, "image_task", strconv.FormatUint(task.ID, 10), "消耗积分生图"); err != nil {
				return err
			}
		}

		if emailNotifyCost > 0 {
			beforeNotifyPoints := beforePoints - pointSpend
			if err := s.recordPointsLog(tx, userID, model.PointsChangeTypeGenerate, -emailNotifyCost, beforeNotifyPoints, afterPoints, "image_task", strconv.FormatUint(task.ID, 10), "任务邮件通知消耗"); err != nil {
				return err
			}
		}

		responseData = dto.CreateImageTaskResponse{
			TaskID:                strconv.FormatUint(task.ID, 10),
			Status:                string(task.Status),
			RequestedCount:        task.RequestedCount,
			CostPoints:            task.CostPoints,
			CostCredits:           task.CostCredits,
			FreeCreditsCost:       task.FreeCreditsCost,
			PaidCreditsCost:       task.PaidCreditsCost,
			CreditSource:          task.CreditSource,
			Balance:               afterCredits.Total(),
			PointsBalance:         afterPoints,
			FreeCreditsBalance:    afterCredits.Free,
			PaidCreditsBalance:    afterCredits.Paid,
			TotalCreditsBalance:   afterCredits.Total(),
			EmailNotifyEnabled:    task.EmailNotifyEnabled,
			EmailNotifyCostPoints: task.EmailNotifyCostPoints,
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

	thumbURLs := s.thumbnailURLsFromTask(ctx, task)
	imageURLs := imageURLsFromTask(task)

	return dto.ImageTaskDetailResponse{
		TaskID:          strconv.FormatUint(task.ID, 10),
		RequestedCount:  task.RequestedCount,
		Prompt:          task.Prompt,
		ImageURL:        thumbnailURLFromList(imageURLs, 0),
		ImageURLs:       imageURLs,
		ThumbnailURL:    thumbnailURLFromList(thumbURLs, 0),
		ThumbnailURLs:   thumbURLs,
		CostPoints:      task.CostPoints,
		CostCredits:     task.CostCredits,
		FreeCreditsCost: task.FreeCreditsCost,
		PaidCreditsCost: task.PaidCreditsCost,
		CreditSource:    task.CreditSource,
		Status:          string(task.Status),
		RequestParams:   task.RequestParams,
		ErrorMessage:    task.ErrorMessage,
		CreatedAt:       task.CreatedAt,
		UpdatedAt:       task.UpdatedAt,
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
		thumbURLs := s.thumbnailURLsFromTask(ctx, task)
		imageURLs := imageURLsFromTask(task)
		items = append(items, dto.ImageTaskItem{
			ID:              strconv.FormatUint(task.ID, 10),
			RequestedCount:  task.RequestedCount,
			Prompt:          task.Prompt,
			ImageURL:        thumbnailURLFromList(imageURLs, 0),
			ImageURLs:       imageURLs,
			ThumbnailURL:    thumbnailURLFromList(thumbURLs, 0),
			ThumbnailURLs:   thumbURLs,
			CostPoints:      task.CostPoints,
			CostCredits:     task.CostCredits,
			FreeCreditsCost: task.FreeCreditsCost,
			PaidCreditsCost: task.PaidCreditsCost,
			CreditSource:    task.CreditSource,
			Status:          string(task.Status),
			RequestParams:   task.RequestParams,
			ErrorMessage:    task.ErrorMessage,
			CreatedAt:       task.CreatedAt,
			UpdatedAt:       task.UpdatedAt,
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
	message = buildImageTaskFailureUserMessage(message)

	var balance int64
	var notice *imageTaskFailureNotice
	var userNotifyTaskID uint64
	var userNotifyUserID uint64
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

		notice = &imageTaskFailureNotice{
			TaskID:         task.ID,
			UserID:         task.UserID,
			Prompt:         task.Prompt,
			Message:        message,
			Provider:       task.Provider,
			ProviderTaskID: strings.TrimSpace(result.ProviderTaskID),
			RawResponse:    truncateProviderMessage(result.RawResponse),
			CreatedAt:      task.CreatedAt,
		}
		if task.EmailNotifyEnabled && task.EmailNotifyCostPoints > 0 {
			userNotifyTaskID = task.ID
			userNotifyUserID = task.UserID
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
			if err := s.recordCreditLog(tx, userID, model.CreditChangeTypeRefund, task.FreeCreditsCost, task.PaidCreditsCost, beforeCredits, afterCredits, "image_task", strconv.FormatUint(task.ID, 10), "生图失败退回福利次数"); err != nil {
				return err
			}
			balance = afterCredits.Total()
		}

		currentPoints := user.PointsBalance
		if task.CostPoints > 0 {
			beforePoints := currentPoints
			afterPoints := beforePoints + task.CostPoints
			if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("points_balance", afterPoints).Error; err != nil {
				return err
			}
			if err := s.recordPointsLog(tx, userID, model.PointsChangeTypeRefund, task.CostPoints, beforePoints, afterPoints, "image_task", strconv.FormatUint(task.ID, 10), "生图失败退回积分"); err != nil {
				return err
			}
			currentPoints = afterPoints
			if task.CostCredits == 0 {
				balance = afterPoints
			}
		}
		refundedPoints, err := s.refundEmailNotifyPoints(tx, task, userID, currentPoints)
		if err != nil {
			return err
		}
		if task.CostCredits == 0 {
			balance = refundedPoints
		}
		return nil
	})
	if err == nil && notice != nil {
		s.notifyImageTaskFailureAsync(*notice)
	}
	if err == nil && userNotifyTaskID > 0 {
		s.notifyUserImageTaskFailureAsync(userNotifyTaskID, userNotifyUserID)
	}
	return balance, err
}
