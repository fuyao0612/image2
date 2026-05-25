package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"image2/backend/internal/config"
	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	systemSettingKeyImageGeneratePointCost        = "billing.image_generate_point_cost"
	systemSettingKeyImageGenerationEnabled        = "image.generation_enabled"
	systemSettingKeyImageFailureNotifyEnabled     = "image.failure_notify_enabled"
	systemSettingKeyEmailNotifyEnabled            = "email_notify.enabled"
	systemSettingKeyEmailNotifyPointCost          = "email_notify.point_cost"
	systemSettingKeyCheckinPoints                 = "billing.checkin_points"
	systemSettingKeyCheckinCredits                = "billing.checkin_credits"
	systemSettingKeyInviteRewardPoints            = "invite.reward_points"
	systemSettingKeyInviteRewardCredits           = "invite.reward_credits"
	systemSettingKeyRegistrationInviteBonusPoints = "registration.invite_bonus_points"
	systemSettingKeyRegistrationGiftCredits       = "registration.gift_credits"
	systemSettingKeyCreditResetLastRunAt          = "credits.reset_last_run_at"
	systemSettingKeyImagesGeneratedTotal          = "stats.images_generated_total"
)

func defaultSystemSettings(cfg config.Config) map[string]string {
	imageCost := cfg.ImageCostPoints
	if imageCost <= 0 {
		imageCost = 20
	}
	checkinPoints := cfg.CheckinPoints
	if checkinPoints < 0 {
		checkinPoints = 0
	}

	return map[string]string{
		systemSettingKeyImageGeneratePointCost:        strconv.FormatInt(imageCost, 10),
		systemSettingKeyImageGenerationEnabled:        "true",
		systemSettingKeyImageFailureNotifyEnabled:     "true",
		systemSettingKeyEmailNotifyEnabled:            "false",
		systemSettingKeyEmailNotifyPointCost:          "1",
		systemSettingKeyCheckinPoints:                 strconv.FormatInt(checkinPoints, 10),
		systemSettingKeyCheckinCredits:                "1",
		systemSettingKeyInviteRewardPoints:            "0",
		systemSettingKeyInviteRewardCredits:           "1",
		systemSettingKeyRegistrationInviteBonusPoints: "0",
		systemSettingKeyRegistrationGiftCredits:       "3",
		systemSettingKeyImagesGeneratedTotal:          "0",
	}
}

func systemSettingDescriptions() map[string]string {
	return map[string]string{
		systemSettingKeyImageGeneratePointCost:  "生图每次消耗积分",
		systemSettingKeyCheckinPoints:           "签到赠送积分",
		systemSettingKeyCheckinCredits:          "签到赠送福利次数",
		systemSettingKeyInviteRewardPoints:      "邀请好友奖励积分",
		systemSettingKeyInviteRewardCredits:     "邀请好友奖励福利次数",
		systemSettingKeyRegistrationGiftCredits: "新用户注册赠送福利次数",
		systemSettingKeyImagesGeneratedTotal:    "历史成功生图张数累计",
	}
}

func (s *Service) EnsureSystemSettings(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}

	defaults := defaultSystemSettings(s.cfg)
	descriptions := systemSettingDescriptions()

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
			if key == systemSettingKeyImagesGeneratedTotal {
				total, err := historicalGeneratedImagesTotal(tx)
				if err != nil {
					return err
				}
				value = strconv.FormatInt(total, 10)
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
		ImageGeneratePointCost:        s.settingInt64(systemSettingKeyImageGeneratePointCost, 20),
		ImageGenerationEnabled:        s.settingBool(systemSettingKeyImageGenerationEnabled, true),
		ImageFailureNotifyEnabled:     s.settingBool(systemSettingKeyImageFailureNotifyEnabled, true),
		EmailNotifyEnabled:            s.settingBool(systemSettingKeyEmailNotifyEnabled, false),
		EmailNotifyPointCost:          s.settingInt64(systemSettingKeyEmailNotifyPointCost, 1),
		CheckinPoints:                 s.settingInt64(systemSettingKeyCheckinPoints, 3),
		CheckinCredits:                s.settingInt64(systemSettingKeyCheckinCredits, 1),
		InviteRewardPoints:            s.settingInt64(systemSettingKeyInviteRewardPoints, 0),
		InviteRewardCredits:           s.settingInt64(systemSettingKeyInviteRewardCredits, 1),
		RegistrationInviteBonusPoints: s.settingInt64(systemSettingKeyRegistrationInviteBonusPoints, 0),
		RegistrationGiftCredits:       s.settingInt64(systemSettingKeyRegistrationGiftCredits, 3),
	}, nil
}

func (s *Service) UpdateBillingSettings(ctx context.Context, adminID uint64, req dto.UpdateBillingSettingsRequest) (dto.BillingSettingsResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.BillingSettingsResponse{}, err
	}
	before, err := s.BillingSettings(ctx)
	if err != nil {
		return dto.BillingSettingsResponse{}, err
	}

	checkinPoints, ok := requiredNonNegativeSetting(req.CheckinPoints)
	if !ok {
		return dto.BillingSettingsResponse{}, ErrSettingsInvalid
	}
	checkinCredits, ok := requiredNonNegativeSetting(req.CheckinCredits)
	if !ok {
		return dto.BillingSettingsResponse{}, ErrSettingsInvalid
	}
	inviteRewardPoints, ok := requiredNonNegativeSetting(req.InviteRewardPoints)
	if !ok {
		return dto.BillingSettingsResponse{}, ErrSettingsInvalid
	}
	inviteRewardCredits, ok := requiredNonNegativeSetting(req.InviteRewardCredits)
	if !ok {
		return dto.BillingSettingsResponse{}, ErrSettingsInvalid
	}
	registrationInviteBonusPoints, ok := requiredNonNegativeSetting(req.RegistrationInviteBonusPoints)
	if !ok {
		return dto.BillingSettingsResponse{}, ErrSettingsInvalid
	}
	registrationGiftCredits, ok := requiredNonNegativeSetting(req.RegistrationGiftCredits)
	if !ok {
		return dto.BillingSettingsResponse{}, ErrSettingsInvalid
	}
	emailNotifyPointCost, ok := requiredNonNegativeSetting(req.EmailNotifyPointCost)
	if !ok {
		return dto.BillingSettingsResponse{}, ErrSettingsInvalid
	}

	if req.ImageGeneratePointCost <= 0 {
		return dto.BillingSettingsResponse{}, ErrSettingsInvalid
	}
	after := dto.BillingSettingsResponse{
		ImageGeneratePointCost:        req.ImageGeneratePointCost,
		ImageGenerationEnabled:        settingBoolValue(req.ImageGenerationEnabled, true),
		ImageFailureNotifyEnabled:     settingBoolValue(req.ImageFailureNotifyEnabled, true),
		EmailNotifyEnabled:            settingBoolValue(req.EmailNotifyEnabled, false),
		EmailNotifyPointCost:          emailNotifyPointCost,
		CheckinPoints:                 checkinPoints,
		CheckinCredits:                checkinCredits,
		InviteRewardPoints:            inviteRewardPoints,
		InviteRewardCredits:           inviteRewardCredits,
		RegistrationInviteBonusPoints: registrationInviteBonusPoints,
		RegistrationGiftCredits:       registrationGiftCredits,
	}

	updates := map[string]string{
		systemSettingKeyImageGeneratePointCost:        strconv.FormatInt(req.ImageGeneratePointCost, 10),
		systemSettingKeyImageGenerationEnabled:        settingBoolString(req.ImageGenerationEnabled, true),
		systemSettingKeyImageFailureNotifyEnabled:     settingBoolString(req.ImageFailureNotifyEnabled, true),
		systemSettingKeyEmailNotifyEnabled:            settingBoolString(req.EmailNotifyEnabled, false),
		systemSettingKeyEmailNotifyPointCost:          strconv.FormatInt(emailNotifyPointCost, 10),
		systemSettingKeyCheckinPoints:                 strconv.FormatInt(checkinPoints, 10),
		systemSettingKeyCheckinCredits:                strconv.FormatInt(checkinCredits, 10),
		systemSettingKeyInviteRewardPoints:            strconv.FormatInt(inviteRewardPoints, 10),
		systemSettingKeyInviteRewardCredits:           strconv.FormatInt(inviteRewardCredits, 10),
		systemSettingKeyRegistrationInviteBonusPoints: strconv.FormatInt(registrationInviteBonusPoints, 10),
		systemSettingKeyRegistrationGiftCredits:       strconv.FormatInt(registrationGiftCredits, 10),
	}
	descriptions := systemSettingDescriptions()

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range updates {
			if err := upsertSystemSetting(tx, key, value, descriptions[key]); err != nil {
				return err
			}
		}
		if err := s.reloadSystemSettings(tx); err != nil {
			return err
		}
		return recordAdminActionLog(tx, adminID, "billing_settings.update", "billing_settings", "global", before, after, "积分与奖励规则")
	}); err != nil {
		return dto.BillingSettingsResponse{}, err
	}

	return s.BillingSettings(ctx)
}

func requiredNonNegativeSetting(value *int64) (int64, bool) {
	if value == nil || *value < 0 {
		return 0, false
	}
	return *value, true
}

func settingBoolString(value *bool, fallback bool) string {
	if value == nil {
		return strconv.FormatBool(fallback)
	}
	return strconv.FormatBool(*value)
}

func settingBoolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
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

func historicalGeneratedImagesTotal(tx *gorm.DB) (int64, error) {
	var total int64
	err := tx.Model(&model.ImageTask{}).
		Joins("JOIN users ON users.id = image_tasks.user_id").
		Where("users.role <> ?", model.UserRoleAdmin).
		Where("image_tasks.status = ?", model.ImageTaskStatusSuccess).
		Select("COALESCE(SUM(image_tasks.requested_count),0)").
		Scan(&total).Error
	return total, err
}

func incrementImagesGeneratedTotal(tx *gorm.DB, delta int64) error {
	if delta <= 0 {
		return nil
	}

	descriptions := systemSettingDescriptions()
	var setting model.SystemSetting
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("`key` = ?", systemSettingKeyImagesGeneratedTotal).
		First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		base, baseErr := historicalGeneratedImagesTotal(tx)
		if baseErr != nil {
			return baseErr
		}
		return upsertSystemSetting(tx, systemSettingKeyImagesGeneratedTotal, strconv.FormatInt(base+delta, 10), descriptions[systemSettingKeyImagesGeneratedTotal])
	}
	if err != nil {
		return err
	}

	current, err := strconv.ParseInt(strings.TrimSpace(setting.Value), 10, 64)
	if err != nil || current < 0 {
		current = 0
	}
	return upsertSystemSetting(tx, systemSettingKeyImagesGeneratedTotal, strconv.FormatInt(current+delta, 10), descriptions[systemSettingKeyImagesGeneratedTotal])
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

func (s *Service) settingBool(key string, fallback bool) bool {
	if s == nil {
		return fallback
	}

	s.settingsMu.RLock()
	raw := strings.TrimSpace(s.settings[key])
	s.settingsMu.RUnlock()
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseBool(raw)
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
