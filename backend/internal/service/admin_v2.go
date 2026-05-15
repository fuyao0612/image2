package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

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
		Code:             strings.TrimSpace(req.Code),
		Name:             strings.TrimSpace(req.Name),
		PackageType:      model.BillingPackageType(strings.TrimSpace(req.PackageType)),
		CreditsAmount:    req.CreditsAmount,
		PointsCost:       req.PointsCost,
		AmountCents:      req.AmountCents,
		BonusPoints:      req.BonusPoints,
		ProductURL:       strings.TrimSpace(req.ProductURL),
		Enabled:          enabled,
		SortOrder:        req.SortOrder,
		Note:             strings.TrimSpace(req.Note),
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
			ID:         strconv.FormatUint(log.ID, 10),
			AdminID:    strconv.FormatUint(log.AdminID, 10),
			Action:     log.Action,
			TargetType: log.TargetType,
			TargetID:   log.TargetID,
			BeforeJSON: log.BeforeJSON,
			AfterJSON:  log.AfterJSON,
			Remark:     log.Remark,
			CreatedAt:  log.CreatedAt,
		})
	}

	return dto.AdminListActionLogsResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
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
