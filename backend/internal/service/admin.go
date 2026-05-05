package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

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

	var resp dto.AdminStatsResponse
	if err := s.db.WithContext(ctx).Model(&model.User{}).Count(&resp.UsersTotal).Error; err != nil {
		return resp, err
	}
	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("status = ?", model.UserStatusDisabled).Count(&resp.UsersDisabled).Error; err != nil {
		return resp, err
	}
	if err := s.db.WithContext(ctx).Model(&model.ImageTask{}).Count(&resp.TasksTotal).Error; err != nil {
		return resp, err
	}
	if err := s.db.WithContext(ctx).Model(&model.ImageTask{}).Where("status = ?", model.ImageTaskStatusGenerating).Count(&resp.TasksGenerating).Error; err != nil {
		return resp, err
	}
	if err := s.db.WithContext(ctx).Model(&model.ImageTask{}).Where("status = ?", model.ImageTaskStatusSuccess).Count(&resp.TasksSuccess).Error; err != nil {
		return resp, err
	}
	if err := s.db.WithContext(ctx).Model(&model.ImageTask{}).Where("status = ?", model.ImageTaskStatusFailed).Count(&resp.TasksFailed).Error; err != nil {
		return resp, err
	}
	if err := s.db.WithContext(ctx).Model(&model.RechargeOrder{}).Where("status = ?", model.RechargeOrderStatusPaid).Count(&resp.OrdersPaid).Error; err != nil {
		return resp, err
	}
	if err := s.db.WithContext(ctx).Model(&model.ContentAuditLog{}).Where("status = ?", model.ContentAuditStatusRejected).Count(&resp.AuditRejected).Error; err != nil {
		return resp, err
	}
	if err := s.db.WithContext(ctx).Model(&model.User{}).Select("COALESCE(SUM(points_balance),0)").Scan(&resp.PointsBalanceTotal).Error; err != nil {
		return resp, err
	}
	if err := s.db.WithContext(ctx).Model(&model.User{}).Select("COALESCE(SUM(free_credits),0)").Scan(&resp.FreeCreditsTotal).Error; err != nil {
		return resp, err
	}
	if err := s.db.WithContext(ctx).Model(&model.User{}).Select("COALESCE(SUM(paid_credits),0)").Scan(&resp.PaidCreditsTotal).Error; err != nil {
		return resp, err
	}
	return resp, nil
}

func (s *Service) AdminListUsers(ctx context.Context, adminID uint64, keyword string, page int, pageSize int) (dto.AdminListUsersResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListUsersResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)

	query := s.db.WithContext(ctx).Model(&model.User{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR email LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListUsersResponse{}, err
	}

	var users []model.User
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		return dto.AdminListUsersResponse{}, err
	}
	items := make([]dto.AdminUserItem, 0, len(users))
	for _, user := range users {
		items = append(items, dto.AdminUserItem{
			ID:            strconv.FormatUint(user.ID, 10),
			Username:      user.Username,
			Email:         user.Email,
			Role:          string(s.effectiveUserRole(user)),
			Status:        int8(user.Status),
			PointsBalance: user.PointsBalance,
			FreeCredits:   user.FreeCredits,
			PaidCredits:   user.PaidCredits,
			TotalCredits:  user.FreeCredits + user.PaidCredits,
			CheckinStreak: user.CheckinStreak,
			LastLoginAt:   user.LastLoginAt,
			LastCheckinAt: user.LastCheckinAt,
			CreatedAt:     user.CreatedAt,
		})
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
	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		return dto.AdminUserItem{}, err
	}
	user, err := s.loadUser(id)
	if err != nil {
		return dto.AdminUserItem{}, err
	}
	return dto.AdminUserItem{
		ID:            strconv.FormatUint(user.ID, 10),
		Username:      user.Username,
		Email:         user.Email,
		Role:          string(s.effectiveUserRole(user)),
		Status:        int8(user.Status),
		PointsBalance: user.PointsBalance,
		FreeCredits:   user.FreeCredits,
		PaidCredits:   user.PaidCredits,
		TotalCredits:  user.FreeCredits + user.PaidCredits,
		CheckinStreak: user.CheckinStreak,
		LastLoginAt:   user.LastLoginAt,
		LastCheckinAt: user.LastCheckinAt,
		CreatedAt:     user.CreatedAt,
	}, nil
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

	page, pageSize = normalizePage(page, pageSize)

	records, err := s.loadUserActivityRecords(ctx, id)
	if err != nil {
		return dto.ListUserActivityResponse{}, err
	}

	sort.SliceStable(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].ID > records[j].ID
		}
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})

	total := len(records)
	start := (page - 1) * pageSize
	if start >= total {
		return dto.ListUserActivityResponse{
			Items:    []dto.UserActivityItem{},
			Page:     page,
			PageSize: pageSize,
			Total:    int64(total),
		}, nil
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	items := make([]dto.UserActivityItem, 0, end-start)
	for _, record := range records[start:end] {
		items = append(items, record.Item)
	}

	return dto.ListUserActivityResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    int64(total),
	}, nil
}

func (s *Service) AdminListImageTasks(ctx context.Context, adminID uint64, status string, page int, pageSize int) (dto.AdminListImageTasksResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListImageTasksResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.ImageTask{})
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListImageTasksResponse{}, err
	}
	var tasks []model.ImageTask
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		return dto.AdminListImageTasksResponse{}, err
	}
	items := make([]dto.AdminImageTaskItem, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, dto.AdminImageTaskItem{
			ID:              strconv.FormatUint(task.ID, 10),
			UserID:          strconv.FormatUint(task.UserID, 10),
			Prompt:          task.Prompt,
			ImageURL:        task.ImageURL,
			ImageURLs:       imageURLsFromTask(task),
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
		})
	}
	return dto.AdminListImageTasksResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) AdminListRechargeOrders(ctx context.Context, adminID uint64, status string, page int, pageSize int) (dto.AdminListRechargeOrdersResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListRechargeOrdersResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.RechargeOrder{})
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListRechargeOrdersResponse{}, err
	}
	var orders []model.RechargeOrder
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders).Error; err != nil {
		return dto.AdminListRechargeOrdersResponse{}, err
	}
	items := make([]dto.AdminRechargeOrderItem, 0, len(orders))
	for _, order := range orders {
		items = append(items, dto.AdminRechargeOrderItem{
			ID:            strconv.FormatUint(order.ID, 10),
			UserID:        strconv.FormatUint(order.UserID, 10),
			OrderNo:       order.OrderNo,
			PackageCode:   order.PackageCode,
			PackageName:   order.PackageName,
			AmountCents:   order.AmountCents,
			CreditsAmount: order.CreditsAmount,
			BonusPoints:   order.BonusPoints,
			PointsAmount:  order.PointsAmount,
			Status:        string(order.Status),
			PaidAt:        order.PaidAt,
			CreatedAt:     order.CreatedAt,
		})
	}
	return dto.AdminListRechargeOrdersResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
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
	if req.CreditsAmount > 0 {
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
		if pkg.CreditsAmount <= 0 || pkg.AmountCents <= 0 {
			return ErrPackageInvalid
		}
		if pkg.PointsCost != 0 {
			return ErrPackageInvalid
		}
	default:
		return ErrPackageInvalid
	}
	return nil
}
