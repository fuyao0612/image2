package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
)

type creditBalance struct {
	Free int64
	Paid int64
}

func (b creditBalance) Total() int64 {
	return b.Free + b.Paid
}

func (b creditBalance) spend(cost int64) (freeCost int64, paidCost int64, next creditBalance) {
	if cost <= 0 {
		return 0, 0, b
	}
	freeCost = min64(b.Free, cost)
	remaining := cost - freeCost
	paidCost = min64(b.Paid, remaining)
	next = creditBalance{
		Free: b.Free - freeCost,
		Paid: b.Paid - paidCost,
	}
	return freeCost, paidCost, next
}

func (s *Service) currentCredits(user model.User) creditBalance {
	return creditBalance{
		Free: user.FreeCredits,
		Paid: user.PaidCredits,
	}
}

func (s *Service) userInfoBalance(user model.User) dto.UserInfo {
	info := userInfo(user)
	info.FreeCredits = user.FreeCredits
	info.PaidCredits = user.PaidCredits
	info.TotalCredits = user.FreeCredits + user.PaidCredits
	return info
}

func (s *Service) pointsResponse(user model.User, pointsLogs []model.PointsLog, creditLogs []model.CreditLog) dto.PointsResponse {
	pointItems := make([]dto.PointsLogItem, 0, len(pointsLogs))
	for _, item := range pointsLogs {
		pointItems = append(pointItems, dto.PointsLogItem{
			ID:            strconv.FormatUint(item.ID, 10),
			ChangeType:    string(item.ChangeType),
			Delta:         item.Delta,
			BeforeBalance: item.BeforeBalance,
			AfterBalance:  item.AfterBalance,
			Remark:        item.Remark,
			CreatedAt:     item.CreatedAt,
		})
	}

	creditItems := make([]dto.CreditLogItem, 0, len(creditLogs))
	for _, item := range creditLogs {
		creditItems = append(creditItems, dto.CreditLogItem{
			ID:         strconv.FormatUint(item.ID, 10),
			ChangeType: string(item.ChangeType),
			DeltaFree:  item.DeltaFree,
			DeltaPaid:  item.DeltaPaid,
			BeforeFree: item.BeforeFree,
			BeforePaid: item.BeforePaid,
			AfterFree:  item.AfterFree,
			AfterPaid:  item.AfterPaid,
			Remark:     item.Remark,
			CreatedAt:  item.CreatedAt,
		})
	}

	return dto.PointsResponse{
		Balance:          user.PointsBalance,
		FreeCredits:      user.FreeCredits,
		PaidCredits:      user.PaidCredits,
		TotalCredits:     user.FreeCredits + user.PaidCredits,
		Streak:           user.CheckinStreak,
		LastCheckinAt:    user.LastCheckinAt,
		RecentLogs:       pointItems,
		RecentCreditLogs: creditItems,
	}
}

func (s *Service) ensureBillingPackages(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.seedDefaultBillingPackages(tx)
	})
}

func (s *Service) seedDefaultBillingPackages(tx *gorm.DB) error {
	packages := defaultBillingPackages()
	var existingCodes []string
	if err := tx.Model(&model.BillingPackage{}).Where("package_type = ?", model.BillingPackageTypeRecharge).Pluck("code", &existingCodes).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(existingCodes))
	for _, code := range existingCodes {
		existing[code] = struct{}{}
	}

	toCreate := make([]model.BillingPackage, 0, len(packages))
	for _, pkg := range packages {
		if _, ok := existing[pkg.Code]; ok {
			continue
		}
		toCreate = append(toCreate, pkg)
	}
	if len(toCreate) == 0 {
		return nil
	}
	return tx.Create(&toCreate).Error
}

func defaultBillingPackages() []model.BillingPackage {
	return []model.BillingPackage{
		{
			Code:          "recharge-100",
			Name:          "入门充值包",
			PackageType:   model.BillingPackageTypeRecharge,
			CreditsAmount: 100,
			AmountCents:   1999,
			BonusPoints:   0,
			Enabled:       true,
			SortOrder:     10,
			Note:          "购买 100 次生图次数",
		},
		{
			Code:          "recharge-300",
			Name:          "标准充值包",
			PackageType:   model.BillingPackageTypeRecharge,
			CreditsAmount: 300,
			AmountCents:   4999,
			BonusPoints:   20,
			Enabled:       true,
			SortOrder:     20,
			Note:          "购买 300 次生图次数，额外赠送 20 积分",
		},
		{
			Code:          "recharge-800",
			Name:          "高阶充值包",
			PackageType:   model.BillingPackageTypeRecharge,
			CreditsAmount: 800,
			AmountCents:   9999,
			BonusPoints:   100,
			Enabled:       true,
			SortOrder:     30,
			Note:          "购买 800 次生图次数，额外赠送 100 积分",
		},
	}
}

func (s *Service) listBillingPackages(ctx context.Context, onlyEnabled bool, packageType string) ([]model.BillingPackage, error) {
	if err := s.ensureBillingPackages(ctx); err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).Model(&model.BillingPackage{}).Where("package_type = ?", model.BillingPackageTypeRecharge)
	if onlyEnabled {
		query = query.Where("enabled = ?", true)
	}
	if packageType = strings.TrimSpace(packageType); packageType != "" && packageType != string(model.BillingPackageTypeRecharge) {
		return []model.BillingPackage{}, nil
	}

	var packages []model.BillingPackage
	if err := query.Order("sort_order asc, created_at asc").Find(&packages).Error; err != nil {
		return nil, err
	}
	return packages, nil
}

func (s *Service) billingPackageToDTO(pkg model.BillingPackage) dto.PackageItem {
	return dto.PackageItem{
		Code:          pkg.Code,
		Name:          pkg.Name,
		PackageType:   string(pkg.PackageType),
		CreditsAmount: pkg.CreditsAmount,
		PointsCost:    pkg.PointsCost,
		AmountCents:   pkg.AmountCents,
		BonusPoints:   pkg.BonusPoints,
		Enabled:       pkg.Enabled,
		SortOrder:     pkg.SortOrder,
		Note:          pkg.Note,
	}
}

func (s *Service) billingPackageToAdminDTO(pkg model.BillingPackage) dto.AdminPackageItem {
	return dto.AdminPackageItem{
		ID:            strconv.FormatUint(pkg.ID, 10),
		Code:          pkg.Code,
		Name:          pkg.Name,
		PackageType:   string(pkg.PackageType),
		CreditsAmount: pkg.CreditsAmount,
		PointsCost:    pkg.PointsCost,
		AmountCents:   pkg.AmountCents,
		BonusPoints:   pkg.BonusPoints,
		Enabled:       pkg.Enabled,
		SortOrder:     pkg.SortOrder,
		Note:          pkg.Note,
		CreatedAt:     pkg.CreatedAt,
		UpdatedAt:     pkg.UpdatedAt,
	}
}

func (s *Service) findBillingPackageByCode(ctx context.Context, code string, onlyEnabled bool) (model.BillingPackage, error) {
	if err := s.ensureBillingPackages(ctx); err != nil {
		return model.BillingPackage{}, err
	}

	var pkg model.BillingPackage
	query := s.db.WithContext(ctx).Where("code = ? AND package_type = ?", strings.TrimSpace(code), model.BillingPackageTypeRecharge)
	if onlyEnabled {
		query = query.Where("enabled = ?", true)
	}
	if err := query.First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.BillingPackage{}, ErrPackageInvalid
		}
		return model.BillingPackage{}, err
	}
	return pkg, nil
}

func (s *Service) awardCredits(tx *gorm.DB, userID uint64, freeDelta, paidDelta int64, changeType model.CreditChangeType, bizType, bizID, remark string) (creditBalance, creditBalance, error) {
	user, err := s.lockUser(tx, userID)
	if err != nil {
		return creditBalance{}, creditBalance{}, err
	}
	before := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
	after := creditBalance{Free: before.Free + freeDelta, Paid: before.Paid + paidDelta}
	if after.Free < 0 || after.Paid < 0 {
		return creditBalance{}, creditBalance{}, ErrInsufficientCredits
	}

	if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
		"free_credits": after.Free,
		"paid_credits": after.Paid,
	}).Error; err != nil {
		return creditBalance{}, creditBalance{}, err
	}

	log := model.CreditLog{
		UserID:     userID,
		ChangeType: changeType,
		DeltaFree:  freeDelta,
		DeltaPaid:  paidDelta,
		BeforeFree: before.Free,
		BeforePaid: before.Paid,
		AfterFree:  after.Free,
		AfterPaid:  after.Paid,
		BizType:    bizType,
		BizID:      bizID,
		Remark:     remark,
	}
	if err := tx.Create(&log).Error; err != nil {
		return creditBalance{}, creditBalance{}, err
	}
	return before, after, nil
}

func (s *Service) applyCreditSpend(tx *gorm.DB, userID uint64, cost int64) (creditBalance, creditBalance, int64, int64, error) {
	user, err := s.lockUser(tx, userID)
	if err != nil {
		return creditBalance{}, creditBalance{}, 0, 0, err
	}
	before := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
	if before.Total() < cost {
		return creditBalance{}, creditBalance{}, 0, 0, ErrInsufficientCredits
	}
	freeCost, paidCost, after := before.spend(cost)
	if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
		"free_credits": after.Free,
		"paid_credits": after.Paid,
	}).Error; err != nil {
		return creditBalance{}, creditBalance{}, 0, 0, err
	}
	return before, after, freeCost, paidCost, nil
}

func (s *Service) recordCreditLog(tx *gorm.DB, userID uint64, changeType model.CreditChangeType, deltaFree, deltaPaid int64, before, after creditBalance, bizType, bizID, remark string) error {
	log := model.CreditLog{
		UserID:     userID,
		ChangeType: changeType,
		DeltaFree:  deltaFree,
		DeltaPaid:  deltaPaid,
		BeforeFree: before.Free,
		BeforePaid: before.Paid,
		AfterFree:  after.Free,
		AfterPaid:  after.Paid,
		BizType:    bizType,
		BizID:      bizID,
		Remark:     remark,
	}
	return tx.Create(&log).Error
}

func (s *Service) recordPointsLog(tx *gorm.DB, userID uint64, changeType model.PointsChangeType, delta int64, before, after int64, bizType, bizID, remark string) error {
	log := model.PointsLog{
		UserID:        userID,
		ChangeType:    changeType,
		Delta:         delta,
		BeforeBalance: before,
		AfterBalance:  after,
		BizType:       bizType,
		BizID:         bizID,
		Remark:        remark,
	}
	return tx.Create(&log).Error
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
