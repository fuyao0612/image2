package service

import (
	"context"
	"fmt"
	"time"

	"image2/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ensureUserCreditSettlementTx 在事务内把某个用户尚未转换的次数结算成积分。
func (s *Service) ensureUserCreditSettlementTx(tx *gorm.DB, userID uint64) (model.User, bool, error) {
	user, err := s.lockUser(tx, userID)
	if err != nil {
		return model.User{}, false, err
	}

	now := s.now()
	today := s.startOfDay(now)

	lastSettledDate := user.LastCreditSettlementDate
	if lastSettledDate == nil && !user.CreatedAt.IsZero() {
		createdDate := s.startOfDay(user.CreatedAt.In(s.location))
		lastSettledDate = &createdDate
	}
	if lastSettledDate == nil || !lastSettledDate.Before(today) {
		return user, false, nil
	}

	freeBefore := user.FreeCredits
	paidBefore := user.PaidCredits
	totalCredits := freeBefore + paidBefore
	settleDate := today

	if totalCredits <= 0 {
		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Update("last_credit_settlement_date", settleDate).Error; err != nil {
			return model.User{}, false, err
		}
		user.LastCreditSettlementDate = &settleDate
		return user, true, nil
	}

	unitPrice := s.settingInt64(systemSettingKeyCustomRechargeUnitPriceCents, 20)
	if unitPrice <= 0 {
		unitPrice = 20
	}
	pointsGain := totalCredits * unitPrice
	beforePoints := user.PointsBalance
	afterPoints := beforePoints + pointsGain

	if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"points_balance":              afterPoints,
		"free_credits":                0,
		"paid_credits":                0,
		"last_credit_settlement_date": settleDate,
	}).Error; err != nil {
		return model.User{}, false, err
	}

	if err := tx.Create(&model.CreditLog{
		UserID:     user.ID,
		ChangeType: model.CreditChangeTypeSettlement,
		DeltaFree:  -freeBefore,
		DeltaPaid:  -paidBefore,
		BeforeFree: freeBefore,
		BeforePaid: paidBefore,
		AfterFree:  0,
		AfterPaid:  0,
		BizType:    "daily_settlement",
		BizID:      dayKey(today),
		Remark:     fmt.Sprintf("日终将 %d 次转换为积分", totalCredits),
	}).Error; err != nil {
		return model.User{}, false, err
	}

	if err := tx.Create(&model.PointsLog{
		UserID:        user.ID,
		ChangeType:    model.PointsChangeTypeSettlement,
		Delta:         pointsGain,
		BeforeBalance: beforePoints,
		AfterBalance:  afterPoints,
		BizType:       "daily_settlement",
		BizID:         dayKey(today),
		Remark:        fmt.Sprintf("日终将 %d 次转换为积分", totalCredits),
	}).Error; err != nil {
		return model.User{}, false, err
	}

	user.PointsBalance = afterPoints
	user.FreeCredits = 0
	user.PaidCredits = 0
	user.LastCreditSettlementDate = &settleDate
	return user, true, nil
}

// ensureUserCreditSettlement 在独立事务里执行单个用户的日终结算。
func (s *Service) ensureUserCreditSettlement(ctx context.Context, userID uint64) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, _, err := s.ensureUserCreditSettlementTx(tx, userID)
		return err
	})
}

// SettleAllUserCredits 扫描所有未结算用户并执行日终转换。
func (s *Service) SettleAllUserCredits(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}

	today := s.startOfDay(s.now())
	var userIDs []uint64
	query := s.db.WithContext(ctx).Model(&model.User{}).
		Where("last_credit_settlement_date IS NULL OR last_credit_settlement_date < ?", today).
		Order("id asc")
	if err := query.Pluck("id", &userIDs).Error; err != nil {
		return err
	}

	for _, userID := range userIDs {
		if err := s.ensureUserCreditSettlement(ctx, userID); err != nil {
			return err
		}
	}
	return nil
}

// StartDailyCreditSettlementMaintenance 启动后台日终结算巡检。
func (s *Service) StartDailyCreditSettlementMaintenance(ctx context.Context) {
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
				sweepCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				if err := s.SettleAllUserCredits(sweepCtx); err != nil {
					fmt.Printf("日终次数转积分失败: %v\n", err)
				}
				cancel()
			}
		}
	}()
}

func (s *Service) lockSystemSetting(tx *gorm.DB, key string) (model.SystemSetting, error) {
	var setting model.SystemSetting
	query := tx.Where("`key` = ?", key)
	if tx.Dialector.Name() == "mysql" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&setting).Error; err != nil {
		return model.SystemSetting{}, err
	}
	return setting, nil
}
