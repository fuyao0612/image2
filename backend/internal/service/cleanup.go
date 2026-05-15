package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const defaultCleanupKeepDays = 30

var dataCleanupTypeLabels = map[string]string{
	"failed_image_tasks":          "失败生图任务记录",
	"generated_images":            "生成图原图和缩略图",
	"user_reference_images":       "用户上传参考图和缩略图",
	"temporary_keys":              "过期验证码 / Redis 临时 Key",
	"action_logs":                 "操作日志",
	"email_failure_notifications": "邮件失败通知记录",
	"successful_image_tasks":      "成功生图任务记录",
	"balance_logs":                "用户积分 / 福利次数流水",
	"user_activity":               "用户活动记录",
	"announcement_reads":          "公告已读记录",
	"rejected_prompt_plaza":       "提示词广场被拒绝记录",
}

func (s *Service) AdminPreviewDataCleanup(ctx context.Context, adminID uint64, req dto.DataCleanupRequest) (dto.DataCleanupResponse, error) {
	return s.runDataCleanup(ctx, adminID, req, true)
}

func (s *Service) AdminExecuteDataCleanup(ctx context.Context, adminID uint64, req dto.DataCleanupRequest) (dto.DataCleanupResponse, error) {
	return s.runDataCleanup(ctx, adminID, req, false)
}

func (s *Service) runDataCleanup(ctx context.Context, adminID uint64, req dto.DataCleanupRequest, dryRun bool) (dto.DataCleanupResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.DataCleanupResponse{}, err
	}

	plan, err := s.buildDataCleanupPlan(ctx, req, dryRun)
	if err != nil {
		return dto.DataCleanupResponse{}, err
	}
	plan.DryRun = dryRun
	if dryRun {
		return plan.toResponse(), nil
	}
	if err := s.executeDataCleanup(ctx, adminID, plan); err != nil {
		return dto.DataCleanupResponse{}, err
	}
	now := s.now()
	plan.FinishedAt = &now
	return plan.toResponse(), nil
}

func (s *Service) resolveDataCleanupRange(req dto.DataCleanupRequest) (time.Time, time.Time, string, error) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" || mode == "retention" {
		keepDays := req.KeepDays
		if keepDays < 0 {
			return time.Time{}, time.Time{}, "", ErrDataCleanupInvalid
		}
		if keepDays == 0 {
			keepDays = defaultCleanupKeepDays
		}
		end := s.now().AddDate(0, 0, -keepDays)
		return time.Time{}, end, fmt.Sprintf("保留最近 %d 天，清理 %s 以前", keepDays, end.Format("2006-01-02 15:04:05")), nil
	}
	switch mode {
	case "date":
		startDate := strings.TrimSpace(req.StartDate)
		endDate := strings.TrimSpace(req.EndDate)
		if startDate == "" || endDate == "" {
			return time.Time{}, time.Time{}, "", ErrDataCleanupInvalid
		}
		startTarget, err := time.ParseInLocation("2006-01-02", startDate, s.location)
		if err != nil {
			return time.Time{}, time.Time{}, "", ErrDataCleanupInvalid
		}
		endTarget, err := time.ParseInLocation("2006-01-02", endDate, s.location)
		if err != nil {
			return time.Time{}, time.Time{}, "", ErrDataCleanupInvalid
		}
		start := s.startOfDay(startTarget)
		endExclusive := s.startOfDay(endTarget).Add(24 * time.Hour)
		if endExclusive.Before(start.Add(24 * time.Hour)) {
			return time.Time{}, time.Time{}, "", ErrDataCleanupInvalid
		}
		return start, endExclusive, fmt.Sprintf("%s ~ %s", start.Format("2006-01-02"), endExclusive.Add(-24*time.Hour).Format("2006-01-02")), nil
	case "days_ago":
		if req.DaysAgo <= 0 {
			return time.Time{}, time.Time{}, "", ErrDataCleanupInvalid
		}
		target := s.startOfDay(s.now().AddDate(0, 0, -req.DaysAgo))
		return target, target.Add(24 * time.Hour), target.Format("2006-01-02"), nil
	default:
		return time.Time{}, time.Time{}, "", ErrDataCleanupInvalid
	}
}

type dataCleanupPlan struct {
	Type           string
	TypeLabel      string
	Mode           string
	TargetDate     string
	KeepDays       int
	StartedAt      time.Time
	FinishedAt     *time.Time
	DryRun         bool
	TableStats     []dto.DataCleanupTableStat
	FilesRemoved   int64
	BytesRemoved   int64
	RecordsRemoved int64
	FileURLs       []string
	start          time.Time
	end            time.Time
}

func (p dataCleanupPlan) toResponse() dto.DataCleanupResponse {
	return dto.DataCleanupResponse{
		Mode:           p.Mode,
		Type:           p.Type,
		TypeLabel:      p.TypeLabel,
		TargetDate:     p.TargetDate,
		KeepDays:       p.KeepDays,
		StartedAt:      p.StartedAt,
		FinishedAt:     p.FinishedAt,
		DryRun:         p.DryRun,
		TableStats:     p.TableStats,
		FilesRemoved:   p.FilesRemoved,
		BytesRemoved:   p.BytesRemoved,
		RecordsRemoved: p.RecordsRemoved,
	}
}

func (s *Service) buildDataCleanupPlan(ctx context.Context, req dto.DataCleanupRequest, dryRun bool) (dataCleanupPlan, error) {
	_ = dryRun
	cleanupType := strings.ToLower(strings.TrimSpace(req.Type))
	typeLabel, ok := dataCleanupTypeLabels[cleanupType]
	if !ok {
		return dataCleanupPlan{}, ErrDataCleanupInvalid
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "retention"
	}
	keepDays := req.KeepDays
	if keepDays == 0 && (mode == "" || mode == "retention") {
		keepDays = defaultCleanupKeepDays
	}
	start, end, targetLabel, err := s.resolveDataCleanupRange(req)
	if err != nil {
		return dataCleanupPlan{}, err
	}

	plan := dataCleanupPlan{
		Type:       cleanupType,
		TypeLabel:  typeLabel,
		Mode:       mode,
		TargetDate: targetLabel,
		KeepDays:   keepDays,
		StartedAt:  s.now(),
		start:      start,
		end:        end,
	}
	addStat := func(table, note string, count int64, bytes int64) {
		plan.TableStats = append(plan.TableStats, dto.DataCleanupTableStat{Table: table, Note: note, Count: count, Bytes: bytes})
		plan.RecordsRemoved += count
	}

	switch cleanupType {
	case "failed_image_tasks":
		tasks, err := s.findCleanupImageTasks(ctx, start, end, model.ImageTaskStatusFailed)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		plan.FileURLs = s.generatedImageCleanupURLs(tasks)
		bytes, err := s.estimateCleanupBytes(ctx, plan.FileURLs)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("image_tasks", "失败生图任务记录；会同步清理这些任务已产生的生成图和缩略图", int64(len(tasks)), bytes)
	case "generated_images":
		tasks, err := s.findCleanupImageTasks(ctx, start, end)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		plan.FileURLs = s.generatedImageCleanupURLs(tasks)
		bytes, err := s.estimateCleanupBytes(ctx, plan.FileURLs)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("image_tasks", "仅清理任务生成图 URL 和文件，保留任务记录", int64(len(tasks)), bytes)
	case "user_reference_images":
		urls, err := s.collectLocalUploadURLsBefore("references", end)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		plan.FileURLs = urls
		bytes, err := s.estimateCleanupBytes(ctx, plan.FileURLs)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("upload_files", "用户上传参考图及其对应缩略图；按文件修改时间清理，不读取任务引用", int64(len(urls)), bytes)
	case "temporary_keys":
		keys, err := s.collectTemporaryRedisKeys(ctx)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("redis_keys", "验证码、验证码通过态、邮箱验证码、限流等临时 Key", int64(len(keys)), 0)
	case "action_logs":
		count, err := s.countRowsBefore(ctx, &model.AdminActionLog{}, start, end)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("admin_action_logs", "后台操作日志", count, 0)
	case "email_failure_notifications":
		count, err := s.countEmailFailureNotifications(ctx, start, end)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("image_tasks", "清空任务上的邮件失败状态和错误信息，不删除任务", count, 0)
	case "successful_image_tasks":
		tasks, err := s.findCleanupImageTasks(ctx, start, end, model.ImageTaskStatusSuccess)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		plan.FileURLs = s.generatedImageCleanupURLs(tasks)
		bytes, err := s.estimateCleanupBytes(ctx, plan.FileURLs)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("image_tasks", "成功生图任务记录；会同步清理生成图和缩略图", int64(len(tasks)), bytes)
	case "balance_logs":
		points, err := s.countRowsBefore(ctx, &model.PointsLog{}, start, end)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		credits, err := s.countRowsBefore(ctx, &model.CreditLog{}, start, end)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("points_logs", "用户积分流水", points, 0)
		addStat("credit_logs", "用户福利次数流水", credits, 0)
	case "user_activity":
		count, err := s.countRowsBefore(ctx, &model.ContentAuditLog{}, start, end)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("content_audit_logs", "用户活动相关的内容审核记录；用户活动列表本身由任务和流水实时汇总，没有独立活动表", count, 0)
	case "announcement_reads":
		count, err := s.countRowsBefore(ctx, &model.AnnouncementRead{}, start, end)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("announcement_reads", "公告已读记录", count, 0)
	case "rejected_prompt_plaza":
		count, err := s.countRejectedPromptPlaza(ctx, start, end)
		if err != nil {
			return dataCleanupPlan{}, err
		}
		addStat("prompt_plaza_items", "提示词广场被拒绝记录", count, 0)
	}
	plan.FileURLs = mergeStringURLs(nil, plan.FileURLs)
	plan.FilesRemoved = int64(len(plan.FileURLs))
	plan.BytesRemoved = sumStatBytes(plan.TableStats)
	return plan, nil
}

func (s *Service) executeDataCleanup(ctx context.Context, adminID uint64, plan dataCleanupPlan) error {
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.executeDataCleanupRows(tx, plan); err != nil {
			return err
		}
		result := map[string]any{
			"type":           plan.Type,
			"typeLabel":      plan.TypeLabel,
			"mode":           plan.Mode,
			"targetDate":     plan.TargetDate,
			"recordsRemoved": plan.RecordsRemoved,
			"filesRemoved":   plan.FilesRemoved,
			"bytesRemoved":   plan.BytesRemoved,
			"tableStats":     plan.TableStats,
		}
		return recordAdminActionLog(tx, adminID, "data_cleanup.execute", "data_cleanup", plan.Type, nil, result, plan.TypeLabel)
	}); err != nil {
		return err
	}
	if plan.Type == "temporary_keys" {
		if _, err := s.deleteTemporaryRedisKeys(ctx); err != nil {
			return err
		}
	}
	if s.imageStore != nil && len(plan.FileURLs) > 0 {
		if err := s.imageStore.DeleteStoredURLs(ctx, plan.FileURLs); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) executeDataCleanupRows(tx *gorm.DB, plan dataCleanupPlan) error {
	switch plan.Type {
	case "failed_image_tasks":
		return tx.Unscoped().Where("status = ? AND created_at >= ? AND created_at < ?", model.ImageTaskStatusFailed, plan.start, plan.end).Delete(&model.ImageTask{}).Error
	case "generated_images":
		return tx.Model(&model.ImageTask{}).Where("created_at >= ? AND created_at < ? AND (image_url <> '' OR image_urls <> '')", plan.start, plan.end).Updates(map[string]any{
			"image_url":             "",
			"image_urls":            "",
			"image_elapsed_seconds": "",
		}).Error
	case "user_reference_images":
		return nil
	case "action_logs":
		return tx.Unscoped().Where("created_at >= ? AND created_at < ?", plan.start, plan.end).Delete(&model.AdminActionLog{}).Error
	case "email_failure_notifications":
		return tx.Model(&model.ImageTask{}).Where("created_at >= ? AND created_at < ? AND (email_notify_status = ? OR email_notify_error <> '')", plan.start, plan.end, model.ImageTaskEmailNotifyStatusFailed).Updates(map[string]any{
			"email_notify_enabled": false,
			"email_notify_status":  "",
			"email_notify_error":   "",
			"email_notify_sent_at": nil,
		}).Error
	case "successful_image_tasks":
		return tx.Unscoped().Where("status = ? AND created_at >= ? AND created_at < ?", model.ImageTaskStatusSuccess, plan.start, plan.end).Delete(&model.ImageTask{}).Error
	case "balance_logs":
		if err := tx.Unscoped().Where("created_at >= ? AND created_at < ?", plan.start, plan.end).Delete(&model.PointsLog{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Where("created_at >= ? AND created_at < ?", plan.start, plan.end).Delete(&model.CreditLog{}).Error
	case "user_activity":
		return tx.Unscoped().Where("created_at >= ? AND created_at < ?", plan.start, plan.end).Delete(&model.ContentAuditLog{}).Error
	case "announcement_reads":
		return tx.Unscoped().Where("created_at >= ? AND created_at < ?", plan.start, plan.end).Delete(&model.AnnouncementRead{}).Error
	case "rejected_prompt_plaza":
		return tx.Unscoped().Where("status = ? AND created_at >= ? AND created_at < ?", model.PromptPlazaStatusRejected, plan.start, plan.end).Delete(&model.PromptPlazaItem{}).Error
	case "temporary_keys":
		return nil
	default:
		return ErrDataCleanupInvalid
	}
}

func (s *Service) countRowsBefore(ctx context.Context, modelValue any, start, end time.Time) (int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Unscoped().Model(modelValue).Where("created_at >= ? AND created_at < ?", start, end).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Service) findCleanupImageTasks(ctx context.Context, start, end time.Time, statuses ...model.ImageTaskStatus) ([]model.ImageTask, error) {
	query := s.db.WithContext(ctx).Unscoped().Where("created_at >= ? AND created_at < ?", start, end)
	if len(statuses) > 0 {
		query = query.Where("status IN ?", statuses)
	}
	if len(statuses) == 0 {
		query = query.Where("image_url <> '' OR image_urls <> ''")
	}
	var tasks []model.ImageTask
	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Service) generatedImageCleanupURLs(tasks []model.ImageTask) []string {
	urls := make([]string, 0, len(tasks)*4)
	for _, task := range tasks {
		for _, rawURL := range imageURLsFromTask(task) {
			trimmed := strings.TrimSpace(rawURL)
			if trimmed == "" {
				continue
			}
			urls = append(urls, trimmed)
			if thumbURL := s.cleanupThumbnailURL(trimmed); thumbURL != "" {
				urls = append(urls, thumbURL)
			}
		}
	}
	return mergeStringURLs(nil, urls)
}

func (s *Service) estimateCleanupBytes(ctx context.Context, urls []string) (int64, error) {
	if s.imageStore == nil || len(urls) == 0 {
		return 0, nil
	}
	return s.imageStore.TotalStoredURLSize(ctx, urls)
}

func (s *Service) collectLocalUploadURLsBefore(category string, before time.Time) ([]string, error) {
	if s == nil || s.imageStore == nil || !s.imageStore.Enabled() {
		return nil, nil
	}
	localDir := strings.TrimSpace(s.imageStore.LocalDir())
	if localDir == "" || before.IsZero() {
		return nil, nil
	}
	publicPath := strings.TrimRight(strings.TrimSpace(s.cfg.ImageStoragePublicPath), "/")
	if publicPath == "" {
		publicPath = "/uploads"
	}

	var roots []string
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "references":
		roots = []string{
			filepath.Join(localDir, "users"),
			filepath.Join(localDir, "reference-images"),
		}
	default:
		return nil, ErrDataCleanupInvalid
	}

	urls := make([]string, 0)
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		err = filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !isCleanupImageExt(filepath.Ext(filePath)) {
				return nil
			}
			relative, err := filepath.Rel(localDir, filePath)
			if err != nil {
				return err
			}
			relativeSlash := filepath.ToSlash(relative)
			if !isReferenceUploadRelativePath(relativeSlash) {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.ModTime().Before(before) {
				return nil
			}
			rawURL := path.Join(publicPath, relativeSlash)
			urls = append(urls, rawURL)
			if thumbURL := s.cleanupThumbnailURL(rawURL); thumbURL != "" {
				urls = append(urls, thumbURL)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return mergeStringURLs(nil, urls), nil
}

func (s *Service) countEmailFailureNotifications(ctx context.Context, start, end time.Time) (int64, error) {
	var total int64
	err := s.db.WithContext(ctx).Unscoped().Model(&model.ImageTask{}).
		Where("created_at >= ? AND created_at < ? AND (email_notify_status = ? OR email_notify_error <> '')", start, end, model.ImageTaskEmailNotifyStatusFailed).
		Count(&total).Error
	return total, err
}

func (s *Service) countRejectedPromptPlaza(ctx context.Context, start, end time.Time) (int64, error) {
	var total int64
	err := s.db.WithContext(ctx).Unscoped().Model(&model.PromptPlazaItem{}).
		Where("status = ? AND created_at >= ? AND created_at < ?", model.PromptPlazaStatusRejected, start, end).
		Count(&total).Error
	return total, err
}

func (s *Service) collectTemporaryRedisKeys(ctx context.Context) ([]string, error) {
	if s.redis == nil {
		return nil, nil
	}
	patterns := []string{"captcha:*", "captcha-pass:*", "email_code:*", "rate_limit:*"}
	keys := make([]string, 0)
	for _, pattern := range patterns {
		var cursor uint64
		for {
			batch, next, err := s.redis.Scan(ctx, cursor, pattern, 200).Result()
			if err != nil {
				if err == redis.Nil {
					break
				}
				return nil, err
			}
			keys = append(keys, batch...)
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	return uniqueStrings(keys), nil
}

func (s *Service) deleteTemporaryRedisKeys(ctx context.Context) (int64, error) {
	keys, err := s.collectTemporaryRedisKeys(ctx)
	if err != nil || len(keys) == 0 || s.redis == nil {
		return 0, err
	}
	return s.redis.Del(ctx, keys...).Result()
}

func (s *Service) cleanupThumbnailURL(rawURL string) string {
	publicPath := strings.TrimRight(strings.TrimSpace(s.cfg.ImageStoragePublicPath), "/")
	if publicPath == "" {
		publicPath = "/uploads"
	}
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsedURL.Path == "" {
		return ""
	}
	normalizedPath := path.Clean("/" + strings.TrimLeft(parsedURL.Path, "/"))
	prefix := strings.TrimRight(publicPath, "/") + "/"
	if normalizedPath != strings.TrimRight(publicPath, "/") && !strings.HasPrefix(normalizedPath, prefix) {
		return ""
	}
	relativePath := strings.TrimPrefix(normalizedPath, strings.TrimRight(publicPath, "/"))
	relativePath = strings.TrimPrefix(relativePath, "/")
	relativePath = filepath.Clean(filepath.FromSlash(relativePath))
	if relativePath == "." || strings.HasPrefix(relativePath, "..") {
		return ""
	}
	thumbRelative := thumbnailRelativePath(relativePath)
	return buildThumbnailURL(rawURL, publicPath, thumbRelative)
}

func sumStatBytes(stats []dto.DataCleanupTableStat) int64 {
	var total int64
	for _, stat := range stats {
		total += stat.Bytes
	}
	return total
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func isReferenceUploadRelativePath(relativePath string) bool {
	normalized := path.Clean("/" + strings.TrimLeft(filepath.ToSlash(relativePath), "/"))
	normalized = strings.TrimPrefix(normalized, "/")
	return strings.HasPrefix(normalized, "reference-images/") ||
		(strings.HasPrefix(normalized, "users/user-") && strings.Contains(normalized, "/references/"))
}

func isCleanupImageExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	default:
		return false
	}
}
