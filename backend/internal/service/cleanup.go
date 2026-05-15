package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
)

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

	start, end, targetLabel, err := s.resolveDataCleanupRange(req)
	if err != nil {
		return dto.DataCleanupResponse{}, err
	}

	plan, err := s.buildDataCleanupPlan(ctx, req.Mode, start, end, targetLabel, dryRun)
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
	switch strings.ToLower(strings.TrimSpace(req.Mode)) {
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
	Mode           string
	TargetDate     string
	StartedAt      time.Time
	FinishedAt     *time.Time
	DryRun         bool
	TableStats     []dto.DataCleanupTableStat
	FilesRemoved   int64
	BytesRemoved   int64
	RecordsRemoved int64
	start          time.Time
	end            time.Time
}

func (p dataCleanupPlan) toResponse() dto.DataCleanupResponse {
	return dto.DataCleanupResponse{
		Mode:           p.Mode,
		TargetDate:     p.TargetDate,
		StartedAt:      p.StartedAt,
		FinishedAt:     p.FinishedAt,
		DryRun:         p.DryRun,
		TableStats:     p.TableStats,
		FilesRemoved:   p.FilesRemoved,
		BytesRemoved:   p.BytesRemoved,
		RecordsRemoved: p.RecordsRemoved,
	}
}

func (s *Service) buildDataCleanupPlan(ctx context.Context, mode string, start, end time.Time, targetLabel string, dryRun bool) (dataCleanupPlan, error) {
	_ = dryRun
	stats := make([]dto.DataCleanupTableStat, 0, 5)
	var recordsRemoved int64

	appendStat := func(table, note string, count int64, bytes int64) {
		stats = append(stats, dto.DataCleanupTableStat{Table: table, Note: note, Count: count, Bytes: bytes})
		recordsRemoved += count
	}

	imageTaskCount, err := s.countDataCleanupRows(ctx, &model.ImageTask{}, start, end)
	if err != nil {
		return dataCleanupPlan{}, err
	}
	appendStat("image_tasks", "生图任务", imageTaskCount, 0)

	pointsLogCount, err := s.countDataCleanupRows(ctx, &model.PointsLog{}, start, end)
	if err != nil {
		return dataCleanupPlan{}, err
	}
	appendStat("points_logs", "积分流水", pointsLogCount, 0)

	creditLogCount, err := s.countDataCleanupRows(ctx, &model.CreditLog{}, start, end)
	if err != nil {
		return dataCleanupPlan{}, err
	}
	appendStat("credit_logs", "福利次数流水", creditLogCount, 0)

	contentAuditCount, err := s.countDataCleanupRows(ctx, &model.ContentAuditLog{}, start, end)
	if err != nil {
		return dataCleanupPlan{}, err
	}
	appendStat("content_audit_logs", "内容审核记录", contentAuditCount, 0)

	announcementReadCount, err := s.countDataCleanupRows(ctx, &model.AnnouncementRead{}, start, end)
	if err != nil {
		return dataCleanupPlan{}, err
	}
	appendStat("announcement_reads", "公告已读记录", announcementReadCount, 0)

	filesRemoved, err := s.collectCleanupFiles(ctx, start, end)
	if err != nil {
		return dataCleanupPlan{}, err
	}
	bytesRemoved, err := s.estimateCleanupBytes(ctx, filesRemoved)
	if err != nil {
		return dataCleanupPlan{}, err
	}
	if len(stats) > 0 {
		stats[0].Bytes = bytesRemoved
	}

	return dataCleanupPlan{
		Mode:           mode,
		TargetDate:     targetLabel,
		StartedAt:      s.now(),
		TableStats:     stats,
		FilesRemoved:   int64(len(filesRemoved)),
		BytesRemoved:   bytesRemoved,
		RecordsRemoved: recordsRemoved,
		start:          start,
		end:            end,
	}, nil
}

func (s *Service) executeDataCleanup(ctx context.Context, adminID uint64, plan dataCleanupPlan) error {
	fileURLs, err := s.collectCleanupFiles(ctx, plan.start, plan.end)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("created_at >= ? AND created_at < ?", plan.start, plan.end).Delete(&model.AnnouncementRead{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("created_at >= ? AND created_at < ?", plan.start, plan.end).Delete(&model.ContentAuditLog{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("created_at >= ? AND created_at < ?", plan.start, plan.end).Delete(&model.CreditLog{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("created_at >= ? AND created_at < ?", plan.start, plan.end).Delete(&model.PointsLog{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("created_at >= ? AND created_at < ?", plan.start, plan.end).Delete(&model.ImageTask{}).Error; err != nil {
			return err
		}

		result := map[string]any{
			"mode":           plan.Mode,
			"targetDate":     plan.TargetDate,
			"recordsRemoved": plan.RecordsRemoved,
			"filesRemoved":   len(fileURLs),
			"bytesRemoved":   plan.BytesRemoved,
			"tableStats":     plan.TableStats,
		}
		return recordAdminActionLog(tx, adminID, "data_cleanup.execute", "data_cleanup", plan.TargetDate, nil, result, "数据清理")
	}); err != nil {
		return err
	}

	if s.imageStore != nil {
		if err := s.imageStore.DeleteStoredURLs(ctx, fileURLs); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) countDataCleanupRows(ctx context.Context, modelValue any, start, end time.Time) (int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Unscoped().Model(modelValue).Where("created_at >= ? AND created_at < ?", start, end).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Service) collectCleanupFiles(ctx context.Context, start, end time.Time) ([]string, error) {
	var tasks []model.ImageTask
	if err := s.db.WithContext(ctx).
		Unscoped().
		Where("created_at >= ? AND created_at < ?", start, end).
		Find(&tasks).Error; err != nil {
		return nil, err
	}

	urls := make([]string, 0, len(tasks)*4)
	for _, task := range tasks {
		urls = append(urls, imageURLsFromTask(task)...)
		urls = append(urls, s.collectTaskReferenceURLs(task)...)
	}

	derived := make([]string, 0, len(urls)*2)
	for _, rawURL := range urls {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			continue
		}
		derived = append(derived, trimmed)
		if thumbURL := s.cleanupThumbnailURL(trimmed); thumbURL != "" {
			derived = append(derived, thumbURL)
		}
	}

	return mergeStringURLs(nil, derived), nil
}

func (s *Service) estimateCleanupBytes(ctx context.Context, urls []string) (int64, error) {
	if s.imageStore == nil || len(urls) == 0 {
		return 0, nil
	}
	return s.imageStore.TotalStoredURLSize(ctx, urls)
}

func (s *Service) collectTaskReferenceURLs(task model.ImageTask) []string {
	if strings.TrimSpace(task.RequestParams) == "" {
		return nil
	}
	var input imageTaskGenerationInput
	if err := json.Unmarshal([]byte(task.RequestParams), &input); err != nil {
		return nil
	}
	if trimmed := strings.TrimSpace(input.ReferenceImageURL); trimmed != "" {
		return []string{trimmed}
	}
	return nil
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
