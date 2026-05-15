package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
)

// AdminListPackagesV2 返回后台套餐列表，带有更严格的配置约束。
func (s *Service) AdminListPackagesV2(ctx context.Context, adminID uint64, packageType string, page int, pageSize int) (dto.AdminListPackagesResponse, error) {
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

// AdminCreatePackageV2 创建套餐并记录审计日志。
func (s *Service) AdminCreatePackageV2(ctx context.Context, adminID uint64, req dto.AdminCreatePackageRequest) (dto.AdminPackageItem, error) {
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

	var created model.BillingPackage
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&pkg).Error; err != nil {
			return err
		}
		if err := ensureEnabledBillingPackageFloor(tx, pkg.PackageType); err != nil {
			return err
		}
		created = pkg
		return recordAdminActionLog(tx, adminID, "billing_package.create", "billing_package", strconv.FormatUint(pkg.ID, 10), nil, pkg, pkg.Code)
	})
	if err != nil {
		return dto.AdminPackageItem{}, err
	}
	return s.billingPackageToAdminDTO(created), nil
}

// AdminUpdatePackageV2 更新套餐并记录审计日志。
func (s *Service) AdminUpdatePackageV2(ctx context.Context, adminID uint64, packageID string, req dto.AdminUpdatePackageRequest) (dto.AdminPackageItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminPackageItem{}, err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(packageID), 10, 64)
	if err != nil || id == 0 {
		return dto.AdminPackageItem{}, ErrPackageInvalid
	}

	var updated model.BillingPackage
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pkg model.BillingPackage
		if err := tx.First(&pkg, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPackageInvalid
			}
			return err
		}
		before := pkg

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
		pkg.Note = strings.TrimSpace(req.Note)
		if err := validateBillingPackage(pkg); err != nil {
			return err
		}
		if err := tx.Save(&pkg).Error; err != nil {
			return err
		}
		if err := ensureEnabledBillingPackageFloor(tx, pkg.PackageType); err != nil {
			return err
		}
		updated = pkg
		return recordAdminActionLog(tx, adminID, "billing_package.update", "billing_package", strconv.FormatUint(pkg.ID, 10), before, pkg, pkg.Code)
	})
	if err != nil {
		return dto.AdminPackageItem{}, err
	}
	return s.billingPackageToAdminDTO(updated), nil
}

// AdminDeletePackageV2 删除后台套餐并记录审计日志。
func (s *Service) AdminDeletePackageV2(ctx context.Context, adminID uint64, packageID string) error {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(packageID), 10, 64)
	if err != nil || id == 0 {
		return ErrPackageInvalid
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pkg model.BillingPackage
		if err := tx.First(&pkg, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPackageInvalid
			}
			return err
		}

		before := pkg
		if err := tx.Delete(&pkg).Error; err != nil {
			return err
		}
		if err := ensureEnabledBillingPackageFloor(tx, pkg.PackageType); err != nil {
			return err
		}
		return recordAdminActionLog(tx, adminID, "billing_package.delete", "billing_package", strconv.FormatUint(pkg.ID, 10), before, nil, pkg.Code)
	})
}

// AdminListActionLogs 返回后台套餐审计日志。
func (s *Service) AdminListActionLogs(ctx context.Context, adminID uint64, targetType, targetID string, action string, page int, pageSize int) (dto.AdminListActionLogsResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)
	if strings.TrimSpace(targetType) == "" && strings.TrimSpace(targetID) == "" && strings.TrimSpace(action) == "" {
		return s.adminListSiteLogs(ctx, page, pageSize)
	}

	query := s.db.WithContext(ctx).Model(&model.AdminActionLog{})
	if targetType = strings.TrimSpace(targetType); targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if targetID = strings.TrimSpace(targetID); targetID != "" {
		query = query.Where("target_id = ?", targetID)
	}
	if action = strings.TrimSpace(action); action != "" {
		query = query.Where("action = ?", action)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}

	var logs []model.AdminActionLog
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}

	items := make([]dto.AdminActionLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, dto.AdminActionLogItem{
			ID:          strconv.FormatUint(log.ID, 10),
			AdminID:     strconv.FormatUint(log.AdminID, 10),
			ActorLabel:  fmt.Sprintf("管理员 #%d", log.AdminID),
			Action:      log.Action,
			RecordType:  "admin_action",
			TargetType:  log.TargetType,
			TargetID:    log.TargetID,
			Title:       log.Action,
			Description: log.Remark,
			BeforeJSON:  log.BeforeJSON,
			AfterJSON:   log.AfterJSON,
			Remark:      log.Remark,
			CreatedAt:   log.CreatedAt,
		})
	}

	return dto.AdminListActionLogsResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

type adminSiteLogRecord struct {
	CreatedAt time.Time
	Item      dto.AdminActionLogItem
}

func (s *Service) adminListSiteLogs(ctx context.Context, page int, pageSize int) (dto.AdminListActionLogsResponse, error) {
	limit := page * pageSize
	if limit < pageSize {
		limit = pageSize
	}

	var adminTotal int64
	if err := s.db.WithContext(ctx).Model(&model.AdminActionLog{}).Count(&adminTotal).Error; err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}
	var adminLogs []model.AdminActionLog
	if err := s.db.WithContext(ctx).Order("created_at desc, id desc").Limit(limit).Find(&adminLogs).Error; err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}

	var pointsTotal int64
	if err := s.db.WithContext(ctx).Model(&model.PointsLog{}).Count(&pointsTotal).Error; err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}
	var pointsLogs []model.PointsLog
	if err := s.db.WithContext(ctx).Order("created_at desc, id desc").Limit(limit).Find(&pointsLogs).Error; err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}

	var creditTotal int64
	if err := s.db.WithContext(ctx).Model(&model.CreditLog{}).Count(&creditTotal).Error; err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}
	var creditLogs []model.CreditLog
	if err := s.db.WithContext(ctx).Order("created_at desc, id desc").Limit(limit).Find(&creditLogs).Error; err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}

	var taskTotal int64
	if err := s.db.WithContext(ctx).Model(&model.ImageTask{}).Count(&taskTotal).Error; err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}
	var tasks []model.ImageTask
	if err := s.db.WithContext(ctx).Order("created_at desc, id desc").Limit(limit).Find(&tasks).Error; err != nil {
		return dto.AdminListActionLogsResponse{}, err
	}

	records := make([]adminSiteLogRecord, 0, len(adminLogs)+len(pointsLogs)+len(creditLogs)+len(tasks))
	for _, log := range adminLogs {
		records = append(records, adminSiteLogRecord{
			CreatedAt: log.CreatedAt,
			Item: dto.AdminActionLogItem{
				ID:          fmt.Sprintf("admin-%d", log.ID),
				AdminID:     strconv.FormatUint(log.AdminID, 10),
				ActorLabel:  fmt.Sprintf("管理员 #%d", log.AdminID),
				Action:      log.Action,
				RecordType:  "admin_action",
				TargetType:  log.TargetType,
				TargetID:    log.TargetID,
				Title:       "后台操作",
				Description: log.Remark,
				BeforeJSON:  log.BeforeJSON,
				AfterJSON:   log.AfterJSON,
				Remark:      log.Remark,
				CreatedAt:   log.CreatedAt,
			},
		})
	}
	for _, log := range pointsLogs {
		remark := normalizeLedgerRemark(log.Remark)
		description := fmt.Sprintf("%+d 积分", log.Delta)
		if remark != "" {
			description = fmt.Sprintf("%s · %s", description, remark)
		}
		records = append(records, adminSiteLogRecord{
			CreatedAt: log.CreatedAt,
			Item: dto.AdminActionLogItem{
				ID:          fmt.Sprintf("points-%d", log.ID),
				UserID:      strconv.FormatUint(log.UserID, 10),
				ActorLabel:  fmt.Sprintf("用户 #%d", log.UserID),
				Action:      "points." + string(log.ChangeType),
				RecordType:  "points_log",
				TargetType:  strings.TrimSpace(log.BizType),
				TargetID:    strings.TrimSpace(log.BizID),
				Title:       pointsChangeTitle(log.ChangeType),
				Description: description,
				Remark:      remark,
				AfterJSON:   mustCompactJSON(map[string]any{"delta": log.Delta, "beforeBalance": log.BeforeBalance, "afterBalance": log.AfterBalance, "bizType": log.BizType, "bizId": log.BizID}),
				CreatedAt:   log.CreatedAt,
			},
		})
	}
	for _, log := range creditLogs {
		remark := normalizeLedgerRemark(log.Remark)
		delta := log.DeltaFree + log.DeltaPaid
		description := fmt.Sprintf("%+d 次数", delta)
		if remark != "" {
			description = fmt.Sprintf("%s · %s", description, remark)
		}
		records = append(records, adminSiteLogRecord{
			CreatedAt: log.CreatedAt,
			Item: dto.AdminActionLogItem{
				ID:          fmt.Sprintf("credit-%d", log.ID),
				UserID:      strconv.FormatUint(log.UserID, 10),
				ActorLabel:  fmt.Sprintf("用户 #%d", log.UserID),
				Action:      "credits." + string(log.ChangeType),
				RecordType:  "credit_log",
				TargetType:  strings.TrimSpace(log.BizType),
				TargetID:    strings.TrimSpace(log.BizID),
				Title:       creditChangeTitle(log.ChangeType),
				Description: description,
				Remark:      remark,
				AfterJSON: mustCompactJSON(map[string]any{
					"deltaFree": log.DeltaFree, "deltaPaid": log.DeltaPaid,
					"beforeFree": log.BeforeFree, "beforePaid": log.BeforePaid,
					"afterFree": log.AfterFree, "afterPaid": log.AfterPaid,
					"bizType": log.BizType, "bizId": log.BizID,
				}),
				CreatedAt: log.CreatedAt,
			},
		})
	}
	for _, task := range tasks {
		records = append(records, adminSiteLogRecord{
			CreatedAt: task.CreatedAt,
			Item: dto.AdminActionLogItem{
				ID:          fmt.Sprintf("task-%d", task.ID),
				UserID:      strconv.FormatUint(task.UserID, 10),
				ActorLabel:  fmt.Sprintf("用户 #%d", task.UserID),
				Action:      "image_task." + string(task.Status),
				RecordType:  "image_task",
				TargetType:  "image_task",
				TargetID:    strconv.FormatUint(task.ID, 10),
				Title:       "生图任务",
				Description: fmt.Sprintf("%s · %s", imageTaskStatusTitle(task.Status), imageTaskCostText(task.CostPoints, task.CostCredits)),
				Status:      string(task.Status),
				Remark:      truncateForLog(task.Prompt, 120),
				AfterJSON: mustCompactJSON(map[string]any{
					"prompt": task.Prompt, "status": task.Status, "requestedCount": task.RequestedCount,
					"costPoints": task.CostPoints, "costCredits": task.CostCredits,
					"provider": task.Provider, "providerTaskId": task.ProviderTaskID,
					"errorMessage": task.ErrorMessage, "requestParams": task.RequestParams,
				}),
				CreatedAt: task.CreatedAt,
			},
		})
	}

	sort.SliceStable(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	start := (page - 1) * pageSize
	if start > len(records) {
		start = len(records)
	}
	end := start + pageSize
	if end > len(records) {
		end = len(records)
	}
	items := make([]dto.AdminActionLogItem, 0, end-start)
	for _, record := range records[start:end] {
		items = append(items, record.Item)
	}
	return dto.AdminListActionLogsResponse{Items: items, Page: page, PageSize: pageSize, Total: adminTotal + pointsTotal + creditTotal + taskTotal}, nil
}

func mustCompactJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func truncateForLog(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}

func ensureEnabledBillingPackageFloor(tx *gorm.DB, packageType model.BillingPackageType) error {
	var enabledCount int64
	if err := tx.Model(&model.BillingPackage{}).
		Where("package_type = ? AND enabled = ?", packageType, true).
		Count(&enabledCount).Error; err != nil {
		return err
	}
	if enabledCount <= 0 {
		return ErrPackageProtected
	}
	return nil
}

func recordAdminActionLog(tx *gorm.DB, adminID uint64, action, targetType, targetID string, before any, after any, remark string) error {
	log := model.AdminActionLog{
		AdminID:    adminID,
		Action:     strings.TrimSpace(action),
		TargetType: strings.TrimSpace(targetType),
		TargetID:   strings.TrimSpace(targetID),
		BeforeJSON: marshalJSON(before),
		AfterJSON:  marshalJSON(after),
		Remark:     strings.TrimSpace(remark),
	}
	return tx.Create(&log).Error
}

func marshalJSON(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}
