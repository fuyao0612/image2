package service

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"image2/backend/internal/config"
	"image2/backend/internal/dto"
	"image2/backend/internal/model"
	"image2/backend/internal/provider"

	"gorm.io/gorm"
)

// ErrProviderKeyInvalid 表示 Key 不存在或参数不合法。
var ErrProviderKeyInvalid = errors.New("Key不存在或参数不合法")

func (s *Service) ensureDefaultProviderKey() {
	if s == nil || s.db == nil || strings.TrimSpace(s.cfg.ImageProviderAPIKey) == "" {
		return
	}
	var count int64
	if err := s.db.Model(&model.ImageProviderKey{}).Count(&count).Error; err != nil {
		log.Printf("检查默认生图 Key 池失败: %v", err)
		return
	}
	if count > 0 {
		return
	}
	key := model.ImageProviderKey{
		Name:              "默认环境 Key",
		Provider:          "mentalout",
		BatchServiceURL:   strings.TrimSpace(s.cfg.ImageProviderBatchURL),
		APIBaseURL:        strings.TrimSpace(s.cfg.ImageProviderAPIBaseURL),
		APIKey:            strings.TrimSpace(s.cfg.ImageProviderAPIKey),
		APIKeyMasked:      maskSecret(s.cfg.ImageProviderAPIKey),
		Status:            model.ImageProviderKeyStatusActive,
		IsUsing:           true,
		Priority:          100,
		AutoSwitchEnabled: true,
	}
	if key.BatchServiceURL == "" {
		key.BatchServiceURL = "https://image.mentalout.top"
	}
	if key.APIBaseURL == "" {
		key.APIBaseURL = "https://chunfeng.mentalout.top"
	}
	if err := s.db.Create(&key).Error; err != nil {
		log.Printf("导入默认生图 Key 到 Key 池失败: %v", err)
		return
	}
	log.Printf("已将环境变量生图 Key 导入 Key 池: %s", key.APIKeyMasked)
}

// AdminListProviderKeys 返回后台生图 Key 池分页列表。
func (s *Service) AdminListProviderKeys(ctx context.Context, adminID uint64, page int, pageSize int) (dto.AdminListProviderKeysResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListProviderKeysResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)

	query := s.db.WithContext(ctx).Model(&model.ImageProviderKey{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListProviderKeysResponse{}, err
	}

	var keys []model.ImageProviderKey
	if err := query.Order("is_using desc, priority asc, created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&keys).Error; err != nil {
		return dto.AdminListProviderKeysResponse{}, err
	}

	items := make([]dto.AdminProviderKeyItem, 0, len(keys))
	for _, key := range keys {
		items = append(items, providerKeyToDTO(key))
	}
	return dto.AdminListProviderKeysResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// AdminCreateProviderKey 新建后台生图 Key。
func (s *Service) AdminCreateProviderKey(ctx context.Context, adminID uint64, req dto.AdminProviderKeyPayload) (dto.AdminProviderKeyItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminProviderKeyItem{}, err
	}
	key, err := buildProviderKeyFromPayload(req, "")
	if err != nil {
		return dto.AdminProviderKeyItem{}, err
	}
	key.CreatedBy = adminID
	if err := s.db.WithContext(ctx).Create(&key).Error; err != nil {
		return dto.AdminProviderKeyItem{}, err
	}
	key.APIKeyMasked = maskSecret(key.APIKey)
	_ = s.db.WithContext(ctx).Model(&model.ImageProviderKey{}).Where("id = ?", key.ID).Update("api_key_masked", key.APIKeyMasked).Error
	return providerKeyToDTO(key), nil
}

// AdminUpdateProviderKey 更新后台生图 Key。
func (s *Service) AdminUpdateProviderKey(ctx context.Context, adminID uint64, providerKeyID string, req dto.AdminProviderKeyPayload) (dto.AdminProviderKeyItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminProviderKeyItem{}, err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(providerKeyID), 10, 64)
	if err != nil || id == 0 {
		return dto.AdminProviderKeyItem{}, ErrProviderKeyInvalid
	}

	var updated model.ImageProviderKey
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var key model.ImageProviderKey
		if err := tx.First(&key, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProviderKeyInvalid
			}
			return err
		}
		before := key

		next, err := buildProviderKeyFromPayload(req, key.APIKey)
		if err != nil {
			return err
		}
		next.ID = key.ID
		next.CreatedAt = key.CreatedAt
		next.CreatedBy = key.CreatedBy
		next.SuccessCount = key.SuccessCount
		next.FailureCount = key.FailureCount
		next.LastUsedAt = key.LastUsedAt
		next.LastError = key.LastError
		next.IsUsing = key.IsUsing
		if strings.TrimSpace(req.APIKey) == "" {
			next.APIKey = key.APIKey
			next.APIKeyMasked = key.APIKeyMasked
		}
		if next.APIKeyMasked == "" {
			next.APIKeyMasked = maskSecret(next.APIKey)
		}
		if err := tx.Save(&next).Error; err != nil {
			return err
		}
		updated = next
		return recordAdminActionLog(tx, adminID, "provider_key.update", "provider_key", strconv.FormatUint(key.ID, 10), before, next, next.Name)
	})
	if err != nil {
		return dto.AdminProviderKeyItem{}, err
	}
	return providerKeyToDTO(updated), nil
}

// AdminDeleteProviderKey 删除后台生图 Key。
func (s *Service) AdminDeleteProviderKey(ctx context.Context, adminID uint64, providerKeyID string) error {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(providerKeyID), 10, 64)
	if err != nil || id == 0 {
		return ErrProviderKeyInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var key model.ImageProviderKey
		if err := tx.First(&key, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProviderKeyInvalid
			}
			return err
		}
		before := key
		if err := tx.Delete(&key).Error; err != nil {
			return err
		}
		return recordAdminActionLog(tx, adminID, "provider_key.delete", "provider_key", strconv.FormatUint(key.ID, 10), before, nil, key.Name)
	})
}

// AdminUseProviderKey 将指定 Key 设置为当前使用中的 Key。
func (s *Service) AdminUseProviderKey(ctx context.Context, adminID uint64, providerKeyID string) (dto.AdminProviderKeyItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminProviderKeyItem{}, err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(providerKeyID), 10, 64)
	if err != nil || id == 0 {
		return dto.AdminProviderKeyItem{}, ErrProviderKeyInvalid
	}

	var updated model.ImageProviderKey
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var key model.ImageProviderKey
		if err := tx.First(&key, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProviderKeyInvalid
			}
			return err
		}
		before := key
		if strings.TrimSpace(key.APIKey) == "" {
			return ErrProviderKeyInvalid
		}

		if err := tx.Model(&model.ImageProviderKey{}).Where("is_using = ?", true).Update("is_using", false).Error; err != nil {
			return err
		}
		key.IsUsing = true
		key.Status = model.ImageProviderKeyStatusActive
		key.AutoSwitchEnabled = true
		if err := tx.Save(&key).Error; err != nil {
			return err
		}
		updated = key
		return recordAdminActionLog(tx, adminID, "provider_key.use", "provider_key", strconv.FormatUint(key.ID, 10), before, key, key.Name)
	})
	if err != nil {
		return dto.AdminProviderKeyItem{}, err
	}
	return providerKeyToDTO(updated), nil
}

// ListActiveProviderKeys 返回可用于自动切换的 Key。
func (s *Service) ListActiveProviderKeys(ctx context.Context) ([]model.ImageProviderKey, error) {
	var keys []model.ImageProviderKey
	if err := s.db.WithContext(ctx).
		Where("provider = ? AND status = ? AND auto_switch_enabled = ?", "mentalout", model.ImageProviderKeyStatusActive, true).
		Order("is_using desc, priority asc, last_used_at asc, id asc").
		Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// providerForTask 返回任务对应的 provider；若任务未绑定 Key 则回退环境变量 provider。
func (s *Service) providerForTask(ctx context.Context, task model.ImageTask) (*provider.MentaloutProvider, model.ImageProviderKey, bool, error) {
	if task.ProviderKeyID > 0 {
		key, ok, err := s.loadProviderKey(ctx, task.ProviderKeyID)
		if err != nil {
			return nil, model.ImageProviderKey{}, false, err
		}
		if ok {
			return buildMentaloutProvider(key, s.cfg), key, true, nil
		}
	}
	if s.imageProvider != nil {
		return s.imageProvider, model.ImageProviderKey{}, false, nil
	}
	return nil, model.ImageProviderKey{}, false, ErrProviderNotConfigured
}

func (s *Service) chooseProviderForSubmit(ctx context.Context) (*provider.MentaloutProvider, model.ImageProviderKey, bool, error) {
	keys, err := s.ListActiveProviderKeys(ctx)
	if err != nil {
		return nil, model.ImageProviderKey{}, false, err
	}
	for _, key := range keys {
		return buildMentaloutProvider(key, s.cfg), key, true, nil
	}
	if s.imageProvider != nil && s.imageProvider.Configured() {
		return s.imageProvider, model.ImageProviderKey{}, false, nil
	}
	return nil, model.ImageProviderKey{}, false, ErrProviderNotConfigured
}

func (s *Service) loadProviderKey(ctx context.Context, id uint64) (model.ImageProviderKey, bool, error) {
	var key model.ImageProviderKey
	if err := s.db.WithContext(ctx).First(&key, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ImageProviderKey{}, false, nil
		}
		return model.ImageProviderKey{}, false, err
	}
	return key, true, nil
}

func (s *Service) markProviderKeySuccess(ctx context.Context, keyID uint64) {
	if keyID == 0 {
		return
	}
	now := time.Now()
	_ = s.db.WithContext(ctx).Model(&model.ImageProviderKey{}).Where("id = ?", keyID).Updates(map[string]any{
		"success_count": gorm.Expr("success_count + ?", 1),
		"last_used_at":  &now,
		"last_error":    "",
		"status":        model.ImageProviderKeyStatusActive,
	}).Error
}

func (s *Service) markProviderKeyFailure(ctx context.Context, keyID uint64, reason string, lowBalance bool) {
	if keyID == 0 {
		return
	}
	updates := map[string]any{
		"failure_count": gorm.Expr("failure_count + ?", 1),
		"last_error":    truncateProviderMessage(reason),
	}
	if lowBalance {
		updates["status"] = model.ImageProviderKeyStatusLowBalance
	}
	_ = s.db.WithContext(ctx).Model(&model.ImageProviderKey{}).Where("id = ?", keyID).Updates(updates).Error
}

func (s *Service) submitImageTaskWithProviderPool(ctx context.Context, taskID uint64, userID uint64, req provider.GenerateRequest) (provider.GenerateResult, *provider.MentaloutProvider, uint64, error) {
	keys, err := s.ListActiveProviderKeys(ctx)
	if err != nil {
		return provider.GenerateResult{}, nil, 0, err
	}

	var lastResult provider.GenerateResult
	var lastErr error
	for _, key := range keys {
		currentProvider := buildMentaloutProvider(key, s.cfg)
		if currentProvider == nil || !currentProvider.Configured() {
			continue
		}
		_ = s.db.WithContext(ctx).Model(&model.ImageTask{}).
			Where("id = ? AND user_id = ?", taskID, userID).
			Updates(map[string]any{"provider": currentProvider.Name(), "provider_key_id": key.ID}).Error
		result, err := currentProvider.Submit(ctx, req)
		if err == nil {
			s.markProviderKeySuccess(ctx, key.ID)
			return result, currentProvider, key.ID, nil
		}
		lastResult = result
		lastErr = err
		message := providerFailureMessage(result, err)
		s.markProviderKeyFailure(ctx, key.ID, message, isProviderBalanceError(message))
		if shouldStopProviderSubmitFailover(result, err) {
			return result, currentProvider, key.ID, err
		}
	}

	if s.imageProvider != nil && s.imageProvider.Configured() {
		_ = s.db.WithContext(ctx).Model(&model.ImageTask{}).
			Where("id = ? AND user_id = ?", taskID, userID).
			Updates(map[string]any{"provider": s.imageProvider.Name(), "provider_key_id": 0}).Error
		result, err := s.imageProvider.Submit(ctx, req)
		return result, s.imageProvider, 0, err
	}

	if lastErr != nil {
		return lastResult, nil, 0, lastErr
	}
	return provider.GenerateResult{}, nil, 0, ErrProviderNotConfigured
}

func shouldStopProviderSubmitFailover(result provider.GenerateResult, err error) bool {
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	raw := strings.ToLower(strings.TrimSpace(result.ErrorMessage + " " + result.RawResponse + " " + errText))
	if raw == "" {
		return false
	}
	return isProviderTemporaryServerError(raw) || isProviderNetworkError(raw) || isProviderTimeoutError(raw)
}

func isProviderBalanceError(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	if isProviderCooldownError(lower) {
		return false
	}
	keywords := []string{"余额", "额度", "配额", "quota", "balance", "insufficient", "not enough", "exhaust", "credits"}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	if strings.Contains(lower, "credit") && !strings.Contains(lower, "credential") {
		return true
	}
	return false
}

func isProviderCooldownError(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(lower, "model_cooldown") ||
		strings.Contains(lower, "cooling down") ||
		strings.Contains(lower, "cooldown")
}

func providerKeyToDTO(key model.ImageProviderKey) dto.AdminProviderKeyItem {
	return dto.AdminProviderKeyItem{
		ID:                strconv.FormatUint(key.ID, 10),
		Name:              key.Name,
		Provider:          key.Provider,
		BatchServiceURL:   key.BatchServiceURL,
		APIBaseURL:        normalizeMentaloutAPIBaseURL(key.APIBaseURL),
		APIKeyMasked:      key.APIKeyMasked,
		Status:            string(key.Status),
		IsUsing:           key.IsUsing,
		Priority:          key.Priority,
		AutoSwitchEnabled: key.AutoSwitchEnabled,
		SuccessCount:      key.SuccessCount,
		FailureCount:      key.FailureCount,
		LastUsedAt:        key.LastUsedAt,
		LastError:         key.LastError,
		CreatedAt:         key.CreatedAt,
		UpdatedAt:         key.UpdatedAt,
	}
}

func buildProviderKeyFromPayload(req dto.AdminProviderKeyPayload, existingKey string) (model.ImageProviderKey, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.ImageProviderKey{}, ErrProviderKeyInvalid
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(existingKey)
	}
	if apiKey == "" {
		return model.ImageProviderKey{}, ErrProviderKeyInvalid
	}

	status := model.ImageProviderKeyStatus(strings.TrimSpace(req.Status))
	if status == "" {
		status = model.ImageProviderKeyStatusActive
	}
	if status != model.ImageProviderKeyStatusActive &&
		status != model.ImageProviderKeyStatusDisabled &&
		status != model.ImageProviderKeyStatusError &&
		status != model.ImageProviderKeyStatusLowBalance {
		return model.ImageProviderKey{}, ErrProviderKeyInvalid
	}

	enabled := true
	if req.AutoSwitchEnabled != nil {
		enabled = *req.AutoSwitchEnabled
	}

	providerName := strings.TrimSpace(req.Provider)
	if providerName == "" {
		providerName = "mentalout"
	}
	batchServiceURL := strings.TrimSpace(req.BatchServiceURL)
	if batchServiceURL == "" {
		batchServiceURL = "https://image.mentalout.top"
	}
	apiBaseURL := strings.TrimSpace(req.APIBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = "https://chunfeng.mentalout.top"
	}
	apiBaseURL = normalizeMentaloutAPIBaseURL(apiBaseURL)

	return model.ImageProviderKey{
		Name:              name,
		Provider:          providerName,
		BatchServiceURL:   batchServiceURL,
		APIBaseURL:        apiBaseURL,
		APIKey:            apiKey,
		APIKeyMasked:      maskSecret(apiKey),
		Status:            status,
		Priority:          req.Priority,
		AutoSwitchEnabled: enabled,
	}, nil
}

func buildMentaloutProvider(key model.ImageProviderKey, cfg config.Config) *provider.MentaloutProvider {
	return provider.NewMentaloutProvider(provider.MentaloutConfig{
		BatchServiceURL:    key.BatchServiceURL,
		ProviderAPIBaseURL: key.APIBaseURL,
		APIKey:             key.APIKey,
		PollInterval:       cfg.ImageProviderPollInterval,
		Timeout:            cfg.ImageProviderTimeout,
	})
}

func maskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return "***"
	}
	return trimmed[:4] + "****" + trimmed[len(trimmed)-4:]
}

func normalizeMentaloutAPIBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}
