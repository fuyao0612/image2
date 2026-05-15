package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"
)

// ListPointsLogs 返回当前用户的积分流水分页。
func (s *Service) ListPointsLogs(ctx context.Context, userID uint64, page, pageSize int) (dto.ListPointsLogsResponse, error) {
	if err := s.ensureUserCreditSettlement(ctx, userID); err != nil {
		return dto.ListPointsLogsResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)

	logs, total, err := s.loadPointsLogsPage(ctx, userID, page, pageSize)
	if err != nil {
		return dto.ListPointsLogsResponse{}, err
	}

	items := make([]dto.PointsLogItem, 0, len(logs))
	for _, item := range logs {
		items = append(items, pointsLogToDTO(item))
	}

	return dto.ListPointsLogsResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// ListCreditLogs 返回当前用户的福利次数流水分页。
func (s *Service) ListCreditLogs(ctx context.Context, userID uint64, page, pageSize int) (dto.ListCreditLogsResponse, error) {
	if err := s.ensureUserCreditSettlement(ctx, userID); err != nil {
		return dto.ListCreditLogsResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)

	logs, total, err := s.loadCreditLogsPage(ctx, userID, page, pageSize)
	if err != nil {
		return dto.ListCreditLogsResponse{}, err
	}

	items := make([]dto.CreditLogItem, 0, len(logs))
	for _, item := range logs {
		items = append(items, creditLogToDTO(item))
	}

	return dto.ListCreditLogsResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// ListUserActivity 返回当前用户的统一记录流分页。
func (s *Service) ListUserActivity(ctx context.Context, userID uint64, page, pageSize int) (dto.ListUserActivityResponse, error) {
	if err := s.ensureUserCreditSettlement(ctx, userID); err != nil {
		return dto.ListUserActivityResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)

	records, total, err := s.loadUserActivityRecordsPage(ctx, userID, page*pageSize)
	if err != nil {
		return dto.ListUserActivityResponse{}, err
	}

	sort.SliceStable(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].ID > records[j].ID
		}
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})

	start := (page - 1) * pageSize
	if int64(start) >= total {
		return dto.ListUserActivityResponse{
			Items:    []dto.UserActivityItem{},
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		}, nil
	}

	end := start + pageSize
	if end > len(records) {
		end = len(records)
	}

	items := make([]dto.UserActivityItem, 0, end-start)
	for _, record := range records[start:end] {
		items = append(items, record.Item)
	}

	return dto.ListUserActivityResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

type userActivityRecord struct {
	ID        string
	CreatedAt time.Time
	Item      dto.UserActivityItem
}

func (s *Service) loadPointsLogsPage(ctx context.Context, userID uint64, page, pageSize int) ([]model.PointsLog, int64, error) {
	query := s.db.WithContext(ctx).Model(&model.PointsLog{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.PointsLog
	if err := query.Order("created_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (s *Service) loadCreditLogsPage(ctx context.Context, userID uint64, page, pageSize int) ([]model.CreditLog, int64, error) {
	query := s.db.WithContext(ctx).Model(&model.CreditLog{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.CreditLog
	if err := query.Order("created_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (s *Service) loadUserActivityRecordsPage(ctx context.Context, userID uint64, limit int) ([]userActivityRecord, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	var pointsTotal int64
	if err := s.db.WithContext(ctx).Model(&model.PointsLog{}).Where("user_id = ?", userID).Count(&pointsTotal).Error; err != nil {
		return nil, 0, err
	}
	var pointsLogs []model.PointsLog
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Limit(limit).Find(&pointsLogs).Error; err != nil {
		return nil, 0, err
	}

	var creditTotal int64
	if err := s.db.WithContext(ctx).Model(&model.CreditLog{}).Where("user_id = ?", userID).Count(&creditTotal).Error; err != nil {
		return nil, 0, err
	}
	var creditLogs []model.CreditLog
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Limit(limit).Find(&creditLogs).Error; err != nil {
		return nil, 0, err
	}

	var imageTaskTotal int64
	if err := s.db.WithContext(ctx).Model(&model.ImageTask{}).Where("user_id = ?", userID).Count(&imageTaskTotal).Error; err != nil {
		return nil, 0, err
	}
	var imageTasks []model.ImageTask
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Limit(limit).Find(&imageTasks).Error; err != nil {
		return nil, 0, err
	}

	records := make([]userActivityRecord, 0, len(pointsLogs)+len(creditLogs)+len(imageTasks))
	for _, item := range pointsLogs {
		records = append(records, userActivityRecord{
			ID:        fmt.Sprintf("points-%d", item.ID),
			CreatedAt: item.CreatedAt,
			Item:      pointsLogActivityItem(item),
		})
	}
	for _, item := range creditLogs {
		records = append(records, userActivityRecord{
			ID:        fmt.Sprintf("credit-%d", item.ID),
			CreatedAt: item.CreatedAt,
			Item:      creditLogActivityItem(item),
		})
	}
	for _, item := range imageTasks {
		records = append(records, userActivityRecord{
			ID:        fmt.Sprintf("task-%d", item.ID),
			CreatedAt: item.CreatedAt,
			Item:      s.imageTaskActivityItem(ctx, item),
		})
	}

	return records, pointsTotal + creditTotal + imageTaskTotal, nil
}

func pointsLogToDTO(item model.PointsLog) dto.PointsLogItem {
	return dto.PointsLogItem{
		ID:            fmt.Sprintf("%d", item.ID),
		ChangeType:    string(item.ChangeType),
		Delta:         item.Delta,
		BeforeBalance: item.BeforeBalance,
		AfterBalance:  item.AfterBalance,
		Remark:        normalizeLedgerRemark(item.Remark),
		CreatedAt:     item.CreatedAt,
	}
}

func creditLogToDTO(item model.CreditLog) dto.CreditLogItem {
	return dto.CreditLogItem{
		ID:         fmt.Sprintf("%d", item.ID),
		ChangeType: string(item.ChangeType),
		DeltaFree:  item.DeltaFree,
		DeltaPaid:  item.DeltaPaid,
		BeforeFree: item.BeforeFree,
		BeforePaid: item.BeforePaid,
		AfterFree:  item.AfterFree,
		AfterPaid:  item.AfterPaid,
		Remark:     normalizeLedgerRemark(item.Remark),
		CreatedAt:  item.CreatedAt,
	}
}

func pointsLogActivityItem(item model.PointsLog) dto.UserActivityItem {
	description := fmt.Sprintf("%+d 积分", item.Delta)
	if remark := normalizeLedgerRemark(item.Remark); remark != "" {
		description = fmt.Sprintf("%s · %s", description, remark)
	}

	return dto.UserActivityItem{
		ID:          fmt.Sprintf("points-%d", item.ID),
		RecordType:  "points_log",
		Title:       pointsChangeTitle(item.ChangeType),
		Description: description,
		DeltaPoints: item.Delta,
		CreatedAt:   item.CreatedAt,
	}
}

func creditLogActivityItem(item model.CreditLog) dto.UserActivityItem {
	description := fmt.Sprintf("福利次数 %+d", item.DeltaFree+item.DeltaPaid)
	if remark := normalizeLedgerRemark(item.Remark); remark != "" {
		description = fmt.Sprintf("%s · %s", description, remark)
	}

	return dto.UserActivityItem{
		ID:               fmt.Sprintf("credit-%d", item.ID),
		RecordType:       "credit_log",
		Title:            creditChangeTitle(item.ChangeType),
		Description:      description,
		DeltaFreeCredits: item.DeltaFree,
		DeltaPaidCredits: item.DeltaPaid,
		CreatedAt:        item.CreatedAt,
	}
}

func (s *Service) imageTaskActivityItem(ctx context.Context, task model.ImageTask) dto.UserActivityItem {
	description := fmt.Sprintf("%s · 消耗 %s", imageTaskStatusTitle(task.Status), imageTaskCostText(task.CostPoints, task.CostCredits))
	if remark := strings.TrimSpace(task.ErrorMessage); remark != "" && task.Status == model.ImageTaskStatusFailed {
		description = fmt.Sprintf("%s · %s", description, remark)
	}
	thumbURLs := s.thumbnailURLsFromTask(ctx, task)
	imageURLs := imageURLsFromTask(task)

	return dto.UserActivityItem{
		ID:            fmt.Sprintf("task-%d", task.ID),
		RecordType:    "image_task",
		Title:         task.Prompt,
		Description:   description,
		Status:        string(task.Status),
		ImageURL:      thumbnailURLFromList(imageURLs, 0),
		ThumbnailURL:  thumbnailURLFromList(thumbURLs, 0),
		PointsAmount:  task.CostPoints,
		CreditsAmount: task.CostCredits,
		CreatedAt:     task.CreatedAt,
	}
}

func imageTaskCostText(points int64, credits int64) string {
	switch {
	case points > 0 && credits > 0:
		return fmt.Sprintf("%d 积分 + %d 福利次数", points, credits)
	case points > 0:
		return fmt.Sprintf("%d 积分", points)
	case credits > 0:
		return fmt.Sprintf("%d 福利次数", credits)
	default:
		return "-"
	}
}

func normalizeLedgerRemark(value string) string {
	remark := strings.TrimSpace(value)
	switch remark {
	case "福利次数不足时的积分兜底消耗":
		return "消耗积分生图"
	case "生图失败退回积分兜底":
		return "生图失败退回积分"
	default:
		remark = strings.ReplaceAll(remark, "福利次数不足时的积分兜底消耗", "消耗积分生图")
		remark = strings.ReplaceAll(remark, "生图失败退回积分兜底", "生图失败退回积分")
		remark = strings.ReplaceAll(remark, "积分兜底消耗", "消耗积分生图")
		remark = strings.ReplaceAll(remark, "积分兜底", "积分")
		return remark
	}
}

func pointsChangeTitle(changeType model.PointsChangeType) string {
	switch changeType {
	case model.PointsChangeTypeCheckin:
		return "每日签到"
	case model.PointsChangeTypePurchase:
		return "购买入账"
	case model.PointsChangeTypePurchaseBonus:
		return "购买赠送"
	case model.PointsChangeTypeRedeem:
		return "积分兑换"
	case model.PointsChangeTypeSettlement:
		return "日终转换"
	case model.PointsChangeTypeGenerate:
		return "生图消耗"
	case model.PointsChangeTypeRefund:
		return "失败退回"
	case model.PointsChangeTypeAdjust:
		return "后台调整"
	default:
		return "积分变动"
	}
}

func creditChangeTitle(changeType model.CreditChangeType) string {
	switch changeType {
	case model.CreditChangeTypeCheckin:
		return "每日签到福利次数"
	case model.CreditChangeTypeRedeem:
		return "积分兑换福利次数"
	case model.CreditChangeTypePurchase:
		return "福利次数入账"
	case model.CreditChangeTypeSettlement:
		return "日终转换福利次数"
	case model.CreditChangeTypeGenerate:
		return "生图消耗福利次数"
	case model.CreditChangeTypeRefund:
		return "失败退回福利次数"
	case model.CreditChangeTypeAdjust:
		return "后台调整福利次数"
	default:
		return "福利次数变动"
	}
}

func imageTaskStatusTitle(status model.ImageTaskStatus) string {
	switch status {
	case model.ImageTaskStatusGenerating:
		return "生成中"
	case model.ImageTaskStatusSuccess:
		return "生成成功"
	case model.ImageTaskStatusFailed:
		return "生成失败"
	default:
		return string(status)
	}
}
