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

// ListCreditLogs 返回当前用户的次数流水分页。
func (s *Service) ListCreditLogs(ctx context.Context, userID uint64, page, pageSize int) (dto.ListCreditLogsResponse, error) {
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
	page, pageSize = normalizePage(page, pageSize)

	records, err := s.loadUserActivityRecords(ctx, userID)
	if err != nil {
		return dto.ListUserActivityResponse{}, err
	}

	sort.SliceStable(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].ID > records[j].ID
		}
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})

	total := len(records)
	start := (page - 1) * pageSize
	if start >= total {
		return dto.ListUserActivityResponse{
			Items:    []dto.UserActivityItem{},
			Page:     page,
			PageSize: pageSize,
			Total:    int64(total),
		}, nil
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	items := make([]dto.UserActivityItem, 0, end-start)
	for _, record := range records[start:end] {
		items = append(items, record.Item)
	}

	return dto.ListUserActivityResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    int64(total),
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

func (s *Service) loadUserActivityRecords(ctx context.Context, userID uint64) ([]userActivityRecord, error) {
	var pointsLogs []model.PointsLog
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Find(&pointsLogs).Error; err != nil {
		return nil, err
	}

	var creditLogs []model.CreditLog
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Find(&creditLogs).Error; err != nil {
		return nil, err
	}

	var imageTasks []model.ImageTask
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Find(&imageTasks).Error; err != nil {
		return nil, err
	}

	var rechargeOrders []model.RechargeOrder
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Find(&rechargeOrders).Error; err != nil {
		return nil, err
	}

	records := make([]userActivityRecord, 0, len(pointsLogs)+len(creditLogs)+len(imageTasks)+len(rechargeOrders))
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
			Item:      imageTaskActivityItem(item),
		})
	}
	for _, item := range rechargeOrders {
		records = append(records, userActivityRecord{
			ID:        fmt.Sprintf("order-%d", item.ID),
			CreatedAt: item.CreatedAt,
			Item:      rechargeOrderActivityItem(item),
		})
	}

	return records, nil
}

func pointsLogToDTO(item model.PointsLog) dto.PointsLogItem {
	return dto.PointsLogItem{
		ID:            fmt.Sprintf("%d", item.ID),
		ChangeType:    string(item.ChangeType),
		Delta:         item.Delta,
		BeforeBalance: item.BeforeBalance,
		AfterBalance:  item.AfterBalance,
		Remark:        item.Remark,
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
		Remark:     item.Remark,
		CreatedAt:  item.CreatedAt,
	}
}

func pointsLogActivityItem(item model.PointsLog) dto.UserActivityItem {
	description := fmt.Sprintf("%+d 积分", item.Delta)
	if remark := strings.TrimSpace(item.Remark); remark != "" {
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
	description := fmt.Sprintf("免费 %+d / 充值 %+d", item.DeltaFree, item.DeltaPaid)
	if remark := strings.TrimSpace(item.Remark); remark != "" {
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

func imageTaskActivityItem(task model.ImageTask) dto.UserActivityItem {
	description := fmt.Sprintf("%s · 消耗 %d 次", imageTaskStatusTitle(task.Status), task.CostCredits)
	if source := strings.TrimSpace(task.CreditSource); source != "" {
		description = fmt.Sprintf("%s · 来源 %s", description, source)
	}
	if remark := strings.TrimSpace(task.ErrorMessage); remark != "" && task.Status == model.ImageTaskStatusFailed {
		description = fmt.Sprintf("%s · %s", description, remark)
	}

	return dto.UserActivityItem{
		ID:          fmt.Sprintf("task-%d", task.ID),
		RecordType:  "image_task",
		Title:       task.Prompt,
		Description: description,
		Status:      string(task.Status),
		ImageURL:    task.ImageURL,
		CreditsAmount: task.CostCredits,
		CreatedAt:   task.CreatedAt,
	}
}

func rechargeOrderActivityItem(order model.RechargeOrder) dto.UserActivityItem {
	description := fmt.Sprintf("%s · %.2f 元 · %d 次", rechargeOrderStatusTitle(order.Status), float64(order.AmountCents)/100, order.CreditsAmount)
	if order.BonusPoints > 0 {
		description = fmt.Sprintf("%s · 赠送 %d 积分", description, order.BonusPoints)
	}
	if paidAt := order.PaidAt; paidAt != nil {
		description = fmt.Sprintf("%s · 支付于 %s", description, paidAt.Format("2006-01-02 15:04"))
	}

	return dto.UserActivityItem{
		ID:            fmt.Sprintf("order-%d", order.ID),
		RecordType:    "recharge_order",
		Title:         order.PackageName,
		Description:   description,
		Status:        string(order.Status),
		CreditsAmount: order.CreditsAmount,
		PointsAmount:  order.BonusPoints,
		AmountCents:   order.AmountCents,
		CreatedAt:     order.CreatedAt,
	}
}

func rechargeOrderStatusTitle(status model.RechargeOrderStatus) string {
	switch status {
	case model.RechargeOrderStatusPending:
		return "待支付"
	case model.RechargeOrderStatusPaid:
		return "已支付"
	case model.RechargeOrderStatusCanceled:
		return "已取消"
	case model.RechargeOrderStatusRefunded:
		return "已退款"
	default:
		return string(status)
	}
}

func pointsChangeTitle(changeType model.PointsChangeType) string {
	switch changeType {
	case model.PointsChangeTypeCheckin:
		return "每日签到"
	case model.PointsChangeTypePurchase:
		return "充值入账"
	case model.PointsChangeTypePurchaseBonus:
		return "充值赠送"
	case model.PointsChangeTypeRedeem:
		return "积分兑换"
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
	case model.CreditChangeTypeRedeem:
		return "积分兑换次数"
	case model.CreditChangeTypePurchase:
		return "充值入账次数"
	case model.CreditChangeTypeGenerate:
		return "生图消耗次数"
	case model.CreditChangeTypeRefund:
		return "失败退回次数"
	case model.CreditChangeTypeAdjust:
		return "后台调整次数"
	default:
		return "次数变动"
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
