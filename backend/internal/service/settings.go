package service

import (
	"context"
	"strconv"
	"strings"

	"image2/backend/internal/config"
	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	systemSettingKeyCustomRechargeUnitPriceCents = "billing.custom_recharge_unit_price_cents"
	systemSettingKeyImageGeneratePointCost       = "billing.image_generate_point_cost"
	systemSettingKeyCheckinPoints                = "billing.checkin_points"
	systemSettingKeyCheckinCredits               = "billing.checkin_credits"
)

func defaultSystemSettings(cfg config.Config) map[string]string {
	customPrice := int64(20)
	if customPrice <= 0 {
		customPrice = 20
	}
	imageCost := cfg.ImageCostPoints
	if imageCost <= 0 {
		imageCost = 20
	}
	checkinPoints := cfg.CheckinPoints
	if checkinPoints < 0 {
		checkinPoints = 0
	}

	return map[string]string{
		systemSettingKeyCustomRechargeUnitPriceCents: strconv.FormatInt(customPrice, 10),
		systemSettingKeyImageGeneratePointCost:       strconv.FormatInt(imageCost, 10),
		systemSettingKeyCheckinPoints:                strconv.FormatInt(checkinPoints, 10),
		systemSettingKeyCheckinCredits:               "1",
	}
}

func (s *Service) EnsureSystemSettings(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}

	defaults := defaultSystemSettings(s.cfg)
	descriptions := map[string]string{
		systemSettingKeyCustomRechargeUnitPriceCents: "自定义充值单次价格（分）",
		systemSettingKeyImageGeneratePointCost:       "生图每次消耗积分",
		systemSettingKeyCheckinPoints:                "签到赠送积分",
		systemSettingKeyCheckinCredits:               "签到赠送次数",
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existingKeys []string
		if err := tx.Model(&model.SystemSetting{}).Pluck("key", &existingKeys).Error; err != nil {
			return err
		}
		existing := make(map[string]struct{}, len(existingKeys))
		for _, key := range existingKeys {
			existing[key] = struct{}{}
		}

		for key, value := range defaults {
			if _, ok := existing[key]; ok {
				continue
			}
			if err := tx.Create(&model.SystemSetting{
				Key:         key,
				Value:       value,
				Description: descriptions[key],
			}).Error; err != nil {
				return err
			}
		}

		return s.reloadSystemSettings(tx)
	})
}

func (s *Service) BillingSettings(ctx context.Context) (dto.BillingSettingsResponse, error) {
	if err := s.EnsureSystemSettings(ctx); err != nil {
		return dto.BillingSettingsResponse{}, err
	}

	return dto.BillingSettingsResponse{
		CustomRechargeUnitPriceCents: s.settingInt64(systemSettingKeyCustomRechargeUnitPriceCents, 20),
		ImageGeneratePointCost:       s.settingInt64(systemSettingKeyImageGeneratePointCost, 20),
		CheckinPoints:                s.settingInt64(systemSettingKeyCheckinPoints, 3),
		CheckinCredits:               s.settingInt64(systemSettingKeyCheckinCredits, 1),
	}, nil
}

func (s *Service) UpdateBillingSettings(ctx context.Context, adminID uint64, req dto.UpdateBillingSettingsRequest) (dto.BillingSettingsResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.BillingSettingsResponse{}, err
	}
	if req.CustomRechargeUnitPriceCents <= 0 || req.ImageGeneratePointCost <= 0 || req.CheckinPoints < 0 || req.CheckinCredits < 0 {
		return dto.BillingSettingsResponse{}, ErrSettingsInvalid
	}

	updates := map[string]string{
		systemSettingKeyCustomRechargeUnitPriceCents: strconv.FormatInt(req.CustomRechargeUnitPriceCents, 10),
		systemSettingKeyImageGeneratePointCost:       strconv.FormatInt(req.ImageGeneratePointCost, 10),
		systemSettingKeyCheckinPoints:                strconv.FormatInt(req.CheckinPoints, 10),
		systemSettingKeyCheckinCredits:               strconv.FormatInt(req.CheckinCredits, 10),
	}
	descriptions := map[string]string{
		systemSettingKeyCustomRechargeUnitPriceCents: "自定义充值单次价格（分）",
		systemSettingKeyImageGeneratePointCost:       "生图每次消耗积分",
		systemSettingKeyCheckinPoints:                "签到赠送积分",
		systemSettingKeyCheckinCredits:               "签到赠送次数",
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range updates {
			if err := upsertSystemSetting(tx, key, value, descriptions[key]); err != nil {
				return err
			}
		}
		return s.reloadSystemSettings(tx)
	}); err != nil {
		return dto.BillingSettingsResponse{}, err
	}

	return s.BillingSettings(ctx)
}

func upsertSystemSetting(tx *gorm.DB, key, value, description string) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"value",
			"description",
			"updated_at",
		}),
	}).Create(&model.SystemSetting{
		Key:         key,
		Value:       value,
		Description: description,
	}).Error
}

func (s *Service) reloadSystemSettings(tx *gorm.DB) error {
	var rows []model.SystemSetting
	if err := tx.Order("id asc").Find(&rows).Error; err != nil {
		return err
	}

	defaults := defaultSystemSettings(s.cfg)
	settings := make(map[string]string, len(defaults)+len(rows))
	for key, value := range defaults {
		settings[key] = value
	}
	for _, row := range rows {
		key := strings.TrimSpace(row.Key)
		if key == "" {
			continue
		}
		settings[key] = strings.TrimSpace(row.Value)
	}

	s.settingsMu.Lock()
	s.settings = settings
	s.settingsMu.Unlock()
	return nil
}

func (s *Service) settingInt64(key string, fallback int64) int64 {
	if s == nil {
		return fallback
	}

	s.settingsMu.RLock()
	raw := strings.TrimSpace(s.settings[key])
	s.settingsMu.RUnlock()
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func (s *Service) setSettingsCacheFromValues(values map[string]string) {
	s.settingsMu.Lock()
	if s.settings == nil {
		s.settings = make(map[string]string, len(values))
	}
	for key, value := range values {
		s.settings[key] = value
	}
	s.settingsMu.Unlock()
}
