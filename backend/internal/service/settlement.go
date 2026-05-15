package service

import (
	"context"
	"log"
	"time"

	"image2/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ensureUserCreditSettlementTx 当前版本不再做次数转积分，保留接口兼容旧调用点。
func (s *Service) ensureUserCreditSettlementTx(tx *gorm.DB, userID uint64) (model.User, bool, error) {
	user, err := s.lockUser(tx, userID)
	if err != nil {
		return model.User{}, false, err
	}
	return user, false, nil
}

// ensureUserCreditSettlement 当前为兼容空操作，不再执行次数转积分。
func (s *Service) ensureUserCreditSettlement(ctx context.Context, userID uint64) error {
	_ = ctx
	_ = userID
	return nil
}

// ResetAllUsersCredits 将全体用户的福利次数清零。
func (s *Service) ResetAllUsersCredits(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Model(&model.User{}).Updates(map[string]any{
			"free_credits": 0,
			"paid_credits": 0,
		})
		if res.Error != nil {
			return res.Error
		}
		if err := upsertSystemSetting(tx, systemSettingKeyCreditResetLastRunAt, s.now().Format(time.RFC3339), "全体用户福利次数上次清零时间"); err != nil {
			return err
		}
		log.Printf("全体用户福利次数已清零，影响行数: %d", res.RowsAffected)
		return nil
	})
}

func (s *Service) resetAllUsersCreditsIfDue(ctx context.Context, now time.Time, interval time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if interval <= 0 {
		interval = 7 * 24 * time.Hour
	}
	if now.IsZero() {
		now = s.now()
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var setting model.SystemSetting
		err := tx.Where("`key` = ?", systemSettingKeyCreditResetLastRunAt).First(&setting).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == nil {
			lastRunAt, parseErr := time.Parse(time.RFC3339, setting.Value)
			if parseErr == nil && now.Sub(lastRunAt) < interval {
				return nil
			}
		}

		res := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Model(&model.User{}).Updates(map[string]any{
			"free_credits": 0,
			"paid_credits": 0,
		})
		if res.Error != nil {
			return res.Error
		}
		if err := upsertSystemSetting(tx, systemSettingKeyCreditResetLastRunAt, now.Format(time.RFC3339), "全体用户福利次数上次清零时间"); err != nil {
			return err
		}
		log.Printf("全体用户福利次数已按每周二规则清零，影响行数: %d", res.RowsAffected)
		return nil
	})
}

// SettleAllUserCredits 保留旧接口兼容，当前直接执行全体福利次数清零。
func (s *Service) SettleAllUserCredits(ctx context.Context) error {
	return s.ResetAllUsersCredits(ctx)
}

// StartDailyCreditSettlementMaintenance 保留旧入口，当前改为每周二 0 点清零一次全体用户福利次数。
func (s *Service) StartDailyCreditSettlementMaintenance(ctx context.Context) {
	s.StartDailyCreditResetMaintenance(ctx)
}

// StartDailyCreditResetMaintenance 从下一个周二 0 点开始，每周清零一次全体用户福利次数。
func (s *Service) StartDailyCreditResetMaintenance(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	go func() {
		timer := time.NewTimer(timeUntilNextWeekdayMidnight(s.location, time.Tuesday))
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		runReset := func() {
			resetCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			if err := s.resetAllUsersCreditsIfDue(resetCtx, s.now(), 7*24*time.Hour); err != nil {
				log.Printf("周期清零全体福利次数失败: %v", err)
			}
			cancel()
		}

		runReset()

		ticker := time.NewTicker(7 * 24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runReset()
			}
		}
	}()
}

func timeUntilNextWeekdayMidnight(loc *time.Location, weekday time.Weekday) time.Duration {
	now := time.Now()
	if loc != nil {
		now = now.In(loc)
	}
	daysUntil := (int(weekday) - int(now.Weekday()) + 7) % 7
	next := time.Date(now.Year(), now.Month(), now.Day()+daysUntil, 0, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 7)
	}
	return time.Until(next)
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
