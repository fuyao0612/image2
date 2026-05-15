package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
)

// RequireAdmin 确认当前用户是管理员。
func (s *Service) RequireAdmin(ctx context.Context, userID uint64) error {
	_ = ctx
	user, err := s.loadUser(userID)
	if err != nil {
		return err
	}
	if s.effectiveUserRole(user) != model.UserRoleAdmin {
		return ErrForbidden
	}
	return nil
}

func (s *Service) AdminStats(ctx context.Context, adminID uint64) (dto.AdminStatsResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminStatsResponse{}, err
	}
	if err := s.EnsureSystemSettings(ctx); err != nil {
		return dto.AdminStatsResponse{}, err
	}

	var resp dto.AdminStatsResponse
	pointToCreditRate := s.settingInt64(systemSettingKeyImageGeneratePointCost, 20)
	if pointToCreditRate <= 0 {
		pointToCreditRate = 20
	}
	resp.PointToCreditRate = pointToCreditRate

	userStatsQuery := func() *gorm.DB {
		return s.db.WithContext(ctx).Model(&model.User{}).Where("role <> ?", model.UserRoleAdmin)
	}
	taskStatsQuery := func() *gorm.DB {
		return s.db.WithContext(ctx).
			Model(&model.ImageTask{}).
			Joins("JOIN users ON users.id = image_tasks.user_id").
			Where("users.role <> ?", model.UserRoleAdmin)
	}
	auditStatsQuery := func() *gorm.DB {
		return s.db.WithContext(ctx).
			Model(&model.ContentAuditLog{}).
			Joins("JOIN users ON users.id = content_audit_logs.user_id").
			Where("users.role <> ?", model.UserRoleAdmin)
	}
	redeemCodeStatsQuery := func() *gorm.DB {
		return s.db.WithContext(ctx).
			Model(&model.RedeemCode{}).
			Joins("LEFT JOIN users ON users.id = redeem_codes.used_by").
			Where(
				"redeem_codes.status = ? OR (redeem_codes.status = ? AND (redeem_codes.used_by = 0 OR users.role <> ?))",
				model.RedeemCodeStatusActive,
				model.RedeemCodeStatusUsed,
				model.UserRoleAdmin,
			)
	}
	redeemCodeUsedStatsQuery := func() *gorm.DB {
		return s.db.WithContext(ctx).
			Model(&model.RedeemCode{}).
			Joins("LEFT JOIN users ON users.id = redeem_codes.used_by").
			Where(
				"redeem_codes.status = ? AND (redeem_codes.used_by = 0 OR users.role <> ?)",
				model.RedeemCodeStatusUsed,
				model.UserRoleAdmin,
			)
	}
	todayStart := s.startOfDay(s.now())
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	if err := userStatsQuery().Count(&resp.UsersTotal).Error; err != nil {
		return resp, err
	}
	if err := userStatsQuery().Where("status = ?", model.UserStatusDisabled).Count(&resp.UsersDisabled).Error; err != nil {
		return resp, err
	}
	if err := userStatsQuery().
		Where("created_at >= ? AND created_at < ?", todayStart, tomorrowStart).
		Count(&resp.UsersCreatedToday).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().Count(&resp.TasksTotal).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().Where("image_tasks.status = ?", model.ImageTaskStatusGenerating).Count(&resp.TasksGenerating).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().Where("image_tasks.status = ?", model.ImageTaskStatusSuccess).Count(&resp.TasksSuccess).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().Where("image_tasks.status = ?", model.ImageTaskStatusFailed).Count(&resp.TasksFailed).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().
		Where("image_tasks.status = ?", model.ImageTaskStatusSuccess).
		Select("COALESCE(SUM(image_tasks.cost_points),0)").
		Scan(&resp.TasksPointsConsumedTotal).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().
		Where("image_tasks.status = ?", model.ImageTaskStatusSuccess).
		Select("COALESCE(SUM(image_tasks.cost_credits),0)").
		Scan(&resp.TasksCreditsConsumedTotal).Error; err != nil {
		return resp, err
	}
	resp.TasksImagesGeneratedTotal = s.settingInt64(systemSettingKeyImagesGeneratedTotal, 0)
	if err := taskStatsQuery().
		Where("image_tasks.status = ?", model.ImageTaskStatusSuccess).
		Where("image_tasks.created_at >= ? AND image_tasks.created_at < ?", todayStart, tomorrowStart).
		Select("COALESCE(SUM(image_tasks.cost_points),0)").
		Scan(&resp.TasksPointsConsumedToday).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().
		Where("image_tasks.status = ?", model.ImageTaskStatusSuccess).
		Where("image_tasks.created_at >= ? AND image_tasks.created_at < ?", todayStart, tomorrowStart).
		Select("COALESCE(SUM(image_tasks.cost_credits),0)").
		Scan(&resp.TasksCreditsConsumedToday).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().
		Where("image_tasks.status = ?", model.ImageTaskStatusSuccess).
		Where("image_tasks.created_at >= ? AND image_tasks.created_at < ?", todayStart, tomorrowStart).
		Select("COALESCE(SUM(image_tasks.requested_count),0)").
		Scan(&resp.TasksImagesGeneratedToday).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().
		Where("image_tasks.created_at >= ? AND image_tasks.created_at < ?", todayStart, tomorrowStart).
		Distinct("image_tasks.user_id").
		Count(&resp.TasksActiveUsersToday).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().
		Where("image_tasks.status = ?", model.ImageTaskStatusSuccess).
		Where("image_tasks.created_at >= ? AND image_tasks.created_at < ?", yesterdayStart, todayStart).
		Select("COALESCE(SUM(image_tasks.cost_points),0)").
		Scan(&resp.TasksPointsConsumedYesterday).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().
		Where("image_tasks.status = ?", model.ImageTaskStatusSuccess).
		Where("image_tasks.created_at >= ? AND image_tasks.created_at < ?", yesterdayStart, todayStart).
		Select("COALESCE(SUM(image_tasks.cost_credits),0)").
		Scan(&resp.TasksCreditsConsumedYesterday).Error; err != nil {
		return resp, err
	}
	if err := taskStatsQuery().
		Where("image_tasks.status = ?", model.ImageTaskStatusSuccess).
		Where("image_tasks.created_at >= ? AND image_tasks.created_at < ?", yesterdayStart, todayStart).
		Select("COALESCE(SUM(image_tasks.requested_count),0)").
		Scan(&resp.TasksImagesGeneratedYesterday).Error; err != nil {
		return resp, err
	}
	if err := auditStatsQuery().Where("content_audit_logs.status = ?", model.ContentAuditStatusRejected).Count(&resp.AuditRejected).Error; err != nil {
		return resp, err
	}
	if err := userStatsQuery().Select("COALESCE(SUM(points_balance),0)").Scan(&resp.PointsBalanceTotal).Error; err != nil {
		return resp, err
	}
	if err := userStatsQuery().Select("COALESCE(SUM(free_credits),0)").Scan(&resp.FreeCreditsTotal).Error; err != nil {
		return resp, err
	}
	if err := userStatsQuery().Select("COALESCE(SUM(paid_credits),0)").Scan(&resp.PaidCreditsTotal).Error; err != nil {
		return resp, err
	}
	if err := redeemCodeStatsQuery().
		Where("redeem_codes.status = ?", model.RedeemCodeStatusActive).
		Count(&resp.RedeemCodesActive).Error; err != nil {
		return resp, err
	}
	if err := redeemCodeStatsQuery().
		Where("redeem_codes.status = ?", model.RedeemCodeStatusActive).
		Select("COALESCE(SUM(redeem_codes.points_amount),0)").
		Scan(&resp.RedeemCodeActivePointsTotal).Error; err != nil {
		return resp, err
	}
	if err := redeemCodeUsedStatsQuery().Count(&resp.RedeemCodesUsed).Error; err != nil {
		return resp, err
	}
	if err := redeemCodeStatsQuery().
		Where("redeem_codes.status = ?", model.RedeemCodeStatusDisabled).
		Count(&resp.RedeemCodesDisabled).Error; err != nil {
		return resp, err
	}
	if err := redeemCodeUsedStatsQuery().
		Select("COALESCE(SUM(redeem_codes.points_amount),0)").
		Scan(&resp.RedeemCodeUsedPointsTotal).Error; err != nil {
		return resp, err
	}
	if err := redeemCodeStatsQuery().
		Select("COALESCE(SUM(redeem_codes.credits_amount),0)").
		Scan(&resp.RedeemCodeCreditsTotal).Error; err != nil {
		return resp, err
	}
	if err := redeemCodeStatsQuery().
		Select("COALESCE(SUM(redeem_codes.points_amount),0)").
		Scan(&resp.RedeemCodePointsTotal).Error; err != nil {
		return resp, err
	}
	resp.CreditsBalanceTotal = resp.FreeCreditsTotal + resp.PaidCreditsTotal
	resp.PointsConvertedCreditsTotal = convertPointsToCredits(resp.PointsBalanceTotal, pointToCreditRate)
	resp.ResourceCreditsTotal = resp.CreditsBalanceTotal + resp.PointsConvertedCreditsTotal
	resp.RedeemCodeConvertedCreditsTotal = resp.RedeemCodeCreditsTotal + convertPointsToCredits(resp.RedeemCodePointsTotal, pointToCreditRate)
	var err error
	resp.TaskTrend24h, resp.TaskTrend3d, resp.TaskTrend7d, err = s.adminTaskTrends(ctx)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

type adminTaskTrendRow struct {
	Status    model.ImageTaskStatus
	CreatedAt time.Time
}

func (s *Service) adminTaskTrends(ctx context.Context) ([]dto.AdminTaskTrendPoint, []dto.AdminTaskTrendPoint, []dto.AdminTaskTrendPoint, error) {
	now := time.Now()
	since := now.AddDate(0, 0, -7)
	var tasks []adminTaskTrendRow
	err := s.db.WithContext(ctx).
		Model(&model.ImageTask{}).
		Select("image_tasks.status, image_tasks.created_at").
		Joins("JOIN users ON users.id = image_tasks.user_id").
		Where("users.role <> ?", model.UserRoleAdmin).
		Where("image_tasks.status IN ?", []model.ImageTaskStatus{model.ImageTaskStatusSuccess, model.ImageTaskStatusFailed}).
		Where("image_tasks.created_at >= ?", since).
		Find(&tasks).Error
	if err != nil {
		return nil, nil, nil, err
	}

	return buildTaskTrend(tasks, now, 24, time.Hour, "15:04"),
		buildTaskTrend(tasks, now, 6, 12*time.Hour, "01/02 15:04"),
		buildTaskTrend(tasks, now, 7, 24*time.Hour, "01/02"),
		nil
}

func buildTaskTrend(tasks []adminTaskTrendRow, now time.Time, buckets int, bucketSize time.Duration, labelFormat string) []dto.AdminTaskTrendPoint {
	points := make([]dto.AdminTaskTrendPoint, buckets)
	start := now.Add(-time.Duration(buckets-1) * bucketSize).Truncate(bucketSize)
	for i := range points {
		points[i].Label = start.Add(time.Duration(i) * bucketSize).Format(labelFormat)
	}

	for _, task := range tasks {
		index := int(task.CreatedAt.Truncate(bucketSize).Sub(start) / bucketSize)
		if index < 0 || index >= buckets {
			continue
		}
		switch task.Status {
		case model.ImageTaskStatusSuccess:
			points[index].Success++
		case model.ImageTaskStatusFailed:
			points[index].Failed++
		}
	}

	return points
}

func convertPointsToCredits(points int64, pointToCreditRate int64) int64 {
	if points <= 0 || pointToCreditRate <= 0 {
		return 0
	}
	return points / pointToCreditRate
}

func (s *Service) AdminListUsers(ctx context.Context, adminID uint64, keyword string, statusFilter string, page int, pageSize int) (dto.AdminListUsersResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListUsersResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)

	query := s.db.WithContext(ctx).Model(&model.User{})
	if status, ok := parseAdminUserStatusFilter(statusFilter); ok {
		query = query.Where("status = ?", status)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		idKeyword := strings.TrimPrefix(keyword, "#")
		if id, err := strconv.ParseUint(idKeyword, 10, 64); err == nil && id > 0 {
			query = query.Where("id = ? OR username LIKE ? OR email LIKE ?", id, like, like)
		} else {
			query = query.Where("username LIKE ? OR email LIKE ?", like, like)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListUsersResponse{}, err
	}

	var users []model.User
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		return dto.AdminListUsersResponse{}, err
	}
	consumptionByUser, err := s.adminUserConsumptionMap(ctx, users)
	if err != nil {
		return dto.AdminListUsersResponse{}, err
	}
	items := make([]dto.AdminUserItem, 0, len(users))
	for _, user := range users {
		items = append(items, s.adminUserToItem(user, consumptionByUser[user.ID]))
	}
	return dto.AdminListUsersResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) AdminUpdateUserStatus(ctx context.Context, adminID uint64, targetID string, req dto.AdminUpdateUserStatusRequest) (dto.AdminUserItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminUserItem{}, err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(targetID), 10, 64)
	if err != nil || id == 0 {
		return dto.AdminUserItem{}, ErrUserNotFound
	}
	if id == adminID && model.UserStatus(req.Status) == model.UserStatusDisabled {
		return dto.AdminUserItem{}, ErrForbidden
	}
	status := model.UserStatus(req.Status)
	if status != model.UserStatusNormal && status != model.UserStatusDisabled {
		return dto.AdminUserItem{}, ErrUserNotFound
	}

	var updated model.User
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		before := user
		user.Status = status
		if before.Status == status {
			updated = user
			return nil
		}

		if err := tx.Model(&model.User{}).Where("id = ?", id).Update("status", status).Error; err != nil {
			return err
		}
		remark := "启用用户"
		if status == model.UserStatusDisabled {
			remark = "禁用用户"
		}
		if err := recordAdminActionLog(tx, adminID, "user.status.update", "user", strconv.FormatUint(id, 10), before, user, remark); err != nil {
			return err
		}
		updated = user
		return nil
	})
	if err != nil {
		return dto.AdminUserItem{}, err
	}
	consumptionByUser, err := s.adminUserConsumptionMap(ctx, []model.User{updated})
	if err != nil {
		return dto.AdminUserItem{}, err
	}
	return s.adminUserToItem(updated, consumptionByUser[updated.ID]), nil
}

func (s *Service) AdminAdjustPoints(ctx context.Context, adminID uint64, targetID string, req dto.AdminAdjustPointsRequest) (dto.AdminAdjustPointsResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminAdjustPointsResponse{}, err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(targetID), 10, 64)
	if err != nil || id == 0 {
		return dto.AdminAdjustPointsResponse{}, ErrUserNotFound
	}
	if req.Delta == 0 {
		return dto.AdminAdjustPointsResponse{}, ErrInsufficientPoints
	}

	var result dto.AdminAdjustPointsResponse
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := s.lockUser(tx, id)
		if err != nil {
			return err
		}
		before := user.PointsBalance
		after := before + req.Delta
		if after < 0 {
			return ErrInsufficientPoints
		}
		if err := tx.Model(&model.User{}).Where("id = ?", id).Update("points_balance", after).Error; err != nil {
			return err
		}
		remark := strings.TrimSpace(req.Remark)
		if remark == "" {
			remark = "后台人工调整"
		}
		log := model.PointsLog{
			UserID:        id,
			ChangeType:    model.PointsChangeTypeAdjust,
			Delta:         req.Delta,
			BeforeBalance: before,
			AfterBalance:  after,
			BizType:       "admin_adjust",
			BizID:         strconv.FormatUint(adminID, 10),
			Remark:        remark,
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		afterUser := user
		afterUser.PointsBalance = after
		if err := recordAdminActionLog(tx, adminID, "user.points.adjust", "user", strconv.FormatUint(id, 10), user, afterUser, remark); err != nil {
			return err
		}
		result = dto.AdminAdjustPointsResponse{
			UserID:        strconv.FormatUint(id, 10),
			Delta:         req.Delta,
			BeforeBalance: before,
			AfterBalance:  after,
		}
		return nil
	})
	return result, err
}

// AdminListUserActivity 返回指定用户的业务日志。
func (s *Service) AdminListUserActivity(ctx context.Context, adminID uint64, targetID string, page, pageSize int) (dto.ListUserActivityResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.ListUserActivityResponse{}, err
	}

	id, err := strconv.ParseUint(strings.TrimSpace(targetID), 10, 64)
	if err != nil || id == 0 {
		return dto.ListUserActivityResponse{}, ErrUserNotFound
	}
	if _, err := s.loadUser(id); err != nil {
		return dto.ListUserActivityResponse{}, err
	}
	if err := s.ensureUserCreditSettlement(ctx, id); err != nil {
		return dto.ListUserActivityResponse{}, err
	}

	page, pageSize = normalizePage(page, pageSize)

	records, total, err := s.loadUserActivityRecordsPage(ctx, id, page*pageSize)
	if err != nil {
		return dto.ListUserActivityResponse{}, err
	}

	sort.SliceStable(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].ID > records[j].ID
		}
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})

	start := (page - 1) * pageSize
	if int64(start) >= total {
		return dto.ListUserActivityResponse{
			Items:    []dto.UserActivityItem{},
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		}, nil
	}

	end := start + pageSize
	if end > len(records) {
		end = len(records)
	}

	items := make([]dto.UserActivityItem, 0, end-start)
	for _, record := range records[start:end] {
		items = append(items, record.Item)
	}

	return dto.ListUserActivityResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *Service) AdminListImageTasks(ctx context.Context, adminID uint64, status string, keyword string, page int, pageSize int) (dto.AdminListImageTasksResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListImageTasksResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.ImageTask{}).Joins("LEFT JOIN users ON users.id = image_tasks.user_id")
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("image_tasks.status = ?", status)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		idKeyword := strings.TrimPrefix(keyword, "#")
		if id, err := strconv.ParseUint(idKeyword, 10, 64); err == nil && id > 0 {
			query = query.Where("image_tasks.id = ? OR image_tasks.user_id = ? OR image_tasks.prompt LIKE ? OR image_tasks.provider LIKE ? OR image_tasks.error_message LIKE ? OR users.username LIKE ? OR users.email LIKE ?", id, id, like, like, like, like, like)
		} else {
			query = query.Where("image_tasks.prompt LIKE ? OR image_tasks.provider LIKE ? OR image_tasks.error_message LIKE ? OR users.username LIKE ? OR users.email LIKE ?", like, like, like, like, like)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListImageTasksResponse{}, err
	}
	var tasks []model.ImageTask
	if err := query.Order("image_tasks.created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		return dto.AdminListImageTasksResponse{}, err
	}
	userIDs := make([]uint64, 0, len(tasks))
	seenUserIDs := make(map[uint64]struct{}, len(tasks))
	for _, task := range tasks {
		if task.UserID == 0 {
			continue
		}
		if _, ok := seenUserIDs[task.UserID]; ok {
			continue
		}
		seenUserIDs[task.UserID] = struct{}{}
		userIDs = append(userIDs, task.UserID)
	}
	usernames := make(map[uint64]string, len(userIDs))
	if len(userIDs) > 0 {
		var users []model.User
		if err := s.db.WithContext(ctx).Select("id", "username").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return dto.AdminListImageTasksResponse{}, err
		}
		for _, user := range users {
			usernames[user.ID] = user.Username
		}
	}
	items := make([]dto.AdminImageTaskItem, 0, len(tasks))
	for _, task := range tasks {
		thumbURLs := s.thumbnailURLsFromTask(ctx, task)
		imageURLs := imageURLsFromTask(task)
		items = append(items, dto.AdminImageTaskItem{
			ID:              strconv.FormatUint(task.ID, 10),
			UserID:          strconv.FormatUint(task.UserID, 10),
			Username:        usernames[task.UserID],
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
			Provider:        task.Provider,
			RequestParams:   task.RequestParams,
			ErrorMessage:    task.ErrorMessage,
			CreatedAt:       task.CreatedAt,
			UpdatedAt:       task.UpdatedAt,
		})
	}
	return dto.AdminListImageTasksResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) AdminListContentAudits(ctx context.Context, adminID uint64, status string, page int, pageSize int) (dto.AdminListContentAuditsResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListContentAuditsResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.ContentAuditLog{})
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListContentAuditsResponse{}, err
	}
	var logs []model.ContentAuditLog
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		return dto.AdminListContentAuditsResponse{}, err
	}
	items := make([]dto.AdminContentAuditItem, 0, len(logs))
	for _, item := range logs {
		items = append(items, contentAuditToAdminDTO(item))
	}
	return dto.AdminListContentAuditsResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) AdminReviewContentAudit(ctx context.Context, adminID uint64, auditID string, req dto.AdminReviewContentAuditRequest) (dto.AdminContentAuditItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminContentAuditItem{}, err
	}

	id, err := strconv.ParseUint(strings.TrimSpace(auditID), 10, 64)
	if err != nil || id == 0 {
		return dto.AdminContentAuditItem{}, ErrAuditNotFound
	}

	var updated model.ContentAuditLog
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var audit model.ContentAuditLog
		if err := tx.First(&audit, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAuditNotFound
			}
			return err
		}

		before := audit
		status := model.ContentAuditStatus(strings.TrimSpace(req.Status))
		action := strings.TrimSpace(req.Action)
		if action == "" {
			if status == model.ContentAuditStatusPass {
				action = "allow"
			} else {
				action = "reject"
			}
		}
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			if status == model.ContentAuditStatusPass {
				reason = "管理员人工通过"
			} else {
				reason = "管理员人工驳回"
			}
		}

		now := s.now()
		audit.Status = status
		audit.Action = action
		audit.Reason = reason
		audit.ReviewerUserID = adminID
		audit.ReviewedAt = &now

		if err := tx.Save(&audit).Error; err != nil {
			return err
		}
		if err := recordAdminActionLog(tx, adminID, "content_audit.review", "content_audit", strconv.FormatUint(audit.ID, 10), before, audit, reason); err != nil {
			return err
		}

		updated = audit
		return nil
	})
	if err != nil {
		return dto.AdminContentAuditItem{}, err
	}

	return contentAuditToAdminDTO(updated), nil
}

type adminUserConsumption struct {
	UserID          uint64
	ConsumedPoints  int64
	ConsumedCredits int64
}

func (s *Service) adminUserConsumptionMap(ctx context.Context, users []model.User) (map[uint64]adminUserConsumption, error) {
	result := make(map[uint64]adminUserConsumption, len(users))
	if len(users) == 0 {
		return result, nil
	}
	userIDs := make([]uint64, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
		result[user.ID] = adminUserConsumption{UserID: user.ID}
	}

	var rows []adminUserConsumption
	if err := s.db.WithContext(ctx).
		Model(&model.ImageTask{}).
		Select("user_id, COALESCE(SUM(cost_points),0) AS consumed_points, COALESCE(SUM(cost_credits),0) AS consumed_credits").
		Where("user_id IN ?", userIDs).
		Group("user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = row
	}
	return result, nil
}

func (s *Service) adminUserToItem(user model.User, consumption adminUserConsumption) dto.AdminUserItem {
	return dto.AdminUserItem{
		ID:              strconv.FormatUint(user.ID, 10),
		Username:        user.Username,
		Email:           user.Email,
		Role:            string(s.effectiveUserRole(user)),
		Status:          int8(user.Status),
		PointsBalance:   user.PointsBalance,
		FreeCredits:     user.FreeCredits,
		PaidCredits:     user.PaidCredits,
		TotalCredits:    user.FreeCredits + user.PaidCredits,
		ConsumedPoints:  consumption.ConsumedPoints,
		ConsumedCredits: consumption.ConsumedCredits,
		CheckinStreak:   user.CheckinStreak,
		LastLoginAt:     user.LastLoginAt,
		LastCheckinAt:   user.LastCheckinAt,
		CreatedAt:       user.CreatedAt,
	}
}

func parseAdminUserStatusFilter(value string) (model.UserStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "normal", "enabled", "active":
		return model.UserStatusNormal, true
	case "2", "disabled":
		return model.UserStatusDisabled, true
	default:
		return 0, false
	}
}

func (s *Service) effectiveUserRole(user model.User) model.UserRole {
	if s.isAdminEmail(user.Email) {
		return model.UserRoleAdmin
	}
	if user.Role == model.UserRoleAdmin {
		return model.UserRoleAdmin
	}
	return model.UserRoleUser
}

func (s *Service) isAdminEmail(email string) bool {
	email = normalizeEmail(email)
	if email == "" {
		return false
	}
	for _, item := range s.cfg.AdminEmails {
		if normalizeEmail(item) == email {
			return true
		}
	}
	return false
}

func (s *Service) syncConfiguredAdminRole(user model.User) model.User {
	role := s.effectiveUserRole(user)
	if role == user.Role || user.ID == 0 {
		return user
	}
	if err := s.db.Model(&model.User{}).Where("id = ?", user.ID).Update("role", role).Error; err == nil {
		user.Role = role
	}
	return user
}

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func decodeKeywordList(raw string) []string {
	var values []string
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}

func contentAuditToAdminDTO(item model.ContentAuditLog) dto.AdminContentAuditItem {
	reviewerID := ""
	if item.ReviewerUserID > 0 {
		reviewerID = strconv.FormatUint(item.ReviewerUserID, 10)
	}

	return dto.AdminContentAuditItem{
		ID:              strconv.FormatUint(item.ID, 10),
		UserID:          strconv.FormatUint(item.UserID, 10),
		BizType:         item.BizType,
		BizID:           item.BizID,
		ContentType:     item.ContentType,
		Content:         item.Content,
		Status:          string(item.Status),
		MatchedKeywords: decodeKeywordList(item.MatchedKeywords),
		Reason:          item.Reason,
		Action:          item.Action,
		ReviewerUserID:  reviewerID,
		ReviewedAt:      item.ReviewedAt,
		CreatedAt:       item.CreatedAt,
	}
}

func (s *Service) AdminListPackages(ctx context.Context, adminID uint64, packageType string, page int, pageSize int) (dto.AdminListPackagesResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListPackagesResponse{}, err
	}
	if err := s.ensureBillingPackages(ctx); err != nil {
		return dto.AdminListPackagesResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.BillingPackage{}).Where("package_type = ?", model.BillingPackageTypeRecharge)
	if packageType = strings.TrimSpace(packageType); packageType != "" && packageType != string(model.BillingPackageTypeRecharge) {
		return dto.AdminListPackagesResponse{Items: []dto.AdminPackageItem{}, Page: page, PageSize: pageSize, Total: 0}, nil
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListPackagesResponse{}, err
	}
	var packages []model.BillingPackage
	if err := query.Order("sort_order asc, created_at asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&packages).Error; err != nil {
		return dto.AdminListPackagesResponse{}, err
	}
	items := make([]dto.AdminPackageItem, 0, len(packages))
	for _, pkg := range packages {
		items = append(items, s.billingPackageToAdminDTO(pkg))
	}
	return dto.AdminListPackagesResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) AdminCreatePackage(ctx context.Context, adminID uint64, req dto.AdminCreatePackageRequest) (dto.AdminPackageItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminPackageItem{}, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	pkg := model.BillingPackage{
		Code:          strings.TrimSpace(req.Code),
		Name:          strings.TrimSpace(req.Name),
		PackageType:   model.BillingPackageType(strings.TrimSpace(req.PackageType)),
		CreditsAmount: req.CreditsAmount,
		PointsCost:    req.PointsCost,
		AmountCents:   req.AmountCents,
		BonusPoints:   req.BonusPoints,
		ProductURL:    strings.TrimSpace(req.ProductURL),
		Enabled:       enabled,
		SortOrder:     req.SortOrder,
		Note:          strings.TrimSpace(req.Note),
	}
	if pkg.PackageType == "" {
		pkg.PackageType = model.BillingPackageTypeRecharge
	}
	if err := validateBillingPackage(pkg); err != nil {
		return dto.AdminPackageItem{}, err
	}
	if err := s.db.WithContext(ctx).Create(&pkg).Error; err != nil {
		return dto.AdminPackageItem{}, err
	}
	return s.billingPackageToAdminDTO(pkg), nil
}

func (s *Service) AdminUpdatePackage(ctx context.Context, adminID uint64, packageID string, req dto.AdminUpdatePackageRequest) (dto.AdminPackageItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminPackageItem{}, err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(packageID), 10, 64)
	if err != nil || id == 0 {
		return dto.AdminPackageItem{}, ErrPackageInvalid
	}

	var pkg model.BillingPackage
	if err := s.db.WithContext(ctx).First(&pkg, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.AdminPackageItem{}, ErrPackageInvalid
		}
		return dto.AdminPackageItem{}, err
	}

	if name := strings.TrimSpace(req.Name); name != "" {
		pkg.Name = name
	}
	if req.CreditsAmount >= 0 {
		pkg.CreditsAmount = req.CreditsAmount
	}
	if req.PointsCost >= 0 {
		pkg.PointsCost = req.PointsCost
	}
	if req.AmountCents >= 0 {
		pkg.AmountCents = req.AmountCents
	}
	if req.BonusPoints >= 0 {
		pkg.BonusPoints = req.BonusPoints
	}
	pkg.ProductURL = strings.TrimSpace(req.ProductURL)
	if req.Enabled != nil {
		pkg.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		pkg.SortOrder = *req.SortOrder
	}
	if note := strings.TrimSpace(req.Note); note != "" {
		pkg.Note = note
	}
	if err := validateBillingPackage(pkg); err != nil {
		return dto.AdminPackageItem{}, err
	}
	if err := s.db.WithContext(ctx).Save(&pkg).Error; err != nil {
		return dto.AdminPackageItem{}, err
	}
	return s.billingPackageToAdminDTO(pkg), nil
}

func validateBillingPackage(pkg model.BillingPackage) error {
	switch pkg.PackageType {
	case model.BillingPackageTypeRecharge:
		if pkg.CreditsAmount < 0 || pkg.AmountCents <= 0 || pkg.BonusPoints < 0 {
			return ErrPackageInvalid
		}
		if pkg.CreditsAmount == 0 && pkg.BonusPoints == 0 {
			return ErrPackageInvalid
		}
		if pkg.PointsCost != 0 {
			return ErrPackageInvalid
		}
	default:
		return ErrPackageInvalid
	}
	if value := strings.TrimSpace(pkg.ProductURL); value != "" {
		parsed, err := url.ParseRequestURI(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return ErrPackageInvalid
		}
	}
	return nil
}
