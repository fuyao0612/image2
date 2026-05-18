package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"image2/backend/internal/mailer"
	"image2/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func emailNotifyStatus(enabled bool) string {
	if !enabled {
		return ""
	}
	return model.ImageTaskEmailNotifyStatusPending
}

func (s *Service) notifyImageTaskSuccessAsync(taskID uint64, userID uint64) {
	s.notifyUserImageTaskAsync(taskID, userID, true)
}

func (s *Service) notifyUserImageTaskFailureAsync(taskID uint64, userID uint64) {
	s.notifyUserImageTaskAsync(taskID, userID, false)
}

func (s *Service) notifyUserImageTaskAsync(taskID uint64, userID uint64, success bool) {
	if s == nil || s.db == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var task model.ImageTask
		if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
			return
		}
		if !task.EmailNotifyEnabled || task.EmailNotifyCostPoints <= 0 {
			return
		}
		if task.EmailNotifyStatus != model.ImageTaskEmailNotifyStatusPending {
			return
		}

		if s.mailer == nil || !s.mailer.TaskNotificationConfigured() {
			_ = s.markImageTaskEmailNotifyFailedAndRefund(ctx, task.ID, userID, "腾讯云任务通知邮件未配置", success)
			return
		}

		claimed := s.db.WithContext(ctx).Model(&model.ImageTask{}).
			Where("id = ? AND email_notify_enabled = ? AND email_notify_status = ?", task.ID, true, model.ImageTaskEmailNotifyStatusPending).
			Updates(map[string]any{
				"email_notify_status": model.ImageTaskEmailNotifyStatusSending,
				"email_notify_error":  "",
			})
		if claimed.Error != nil || claimed.RowsAffected == 0 {
			return
		}

		var user model.User
		if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
			return
		}
		to := normalizeEmail(user.Email)
		if to == "" {
			_ = s.markImageTaskEmailNotifyFailedAndRefund(ctx, task.ID, userID, "用户邮箱为空", success)
			return
		}

		subject, data := s.buildUserImageTaskEmail(task, success)
		if err := s.mailer.SendTaskNotification(ctx, to, subject, data); err != nil {
			_ = s.markImageTaskEmailNotifyFailedAndRefund(ctx, task.ID, userID, err.Error(), success)
			log.Printf("发送用户任务邮件通知失败: to=%s task=%d err=%v", to, task.ID, err)
			return
		}
		now := s.now()
		_ = s.db.WithContext(ctx).Model(&model.ImageTask{}).
			Where("id = ? AND email_notify_enabled = ? AND email_notify_status <> ?", task.ID, true, model.ImageTaskEmailNotifyStatusSent).
			Updates(map[string]any{
				"email_notify_status":  model.ImageTaskEmailNotifyStatusSent,
				"email_notify_sent_at": now,
				"email_notify_error":   "",
			}).Error
	}()
}

func (s *Service) markImageTaskEmailNotifyFailedAndRefund(ctx context.Context, taskID uint64, userID uint64, message string, refundCost bool) error {
	message = strings.TrimSpace(message)
	if len(message) > 500 {
		message = message[:500]
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.ImageTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND email_notify_enabled = ? AND email_notify_status <> ?", taskID, userID, true, model.ImageTaskEmailNotifyStatusSent).
			First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		updates := map[string]any{
			"email_notify_status": model.ImageTaskEmailNotifyStatusFailed,
			"email_notify_error":  message,
		}
		if refundCost && task.EmailNotifyCostPoints > 0 {
			user, err := s.lockUser(tx, userID)
			if err != nil {
				return err
			}
			before := user.PointsBalance
			after := before + task.EmailNotifyCostPoints
			if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("points_balance", after).Error; err != nil {
				return err
			}
			if err := s.recordPointsLog(tx, userID, model.PointsChangeTypeRefund, task.EmailNotifyCostPoints, before, after, "image_task", strconv.FormatUint(task.ID, 10), "任务邮件通知发送失败退回积分"); err != nil {
				return err
			}
			updates["email_notify_cost_points"] = int64(0)
		}
		return tx.Model(&model.ImageTask{}).Where("id = ?", task.ID).Updates(updates).Error
	})
}

func (s *Service) buildUserImageTaskEmail(task model.ImageTask, success bool) (string, mailer.TaskNotification) {
	taskID := strconv.FormatUint(task.ID, 10)

	if success {
		var b strings.Builder
		fmt.Fprintf(&b, "任务已完成，完成时间：%s。", s.now().Format("2006-01-02 15:04:05"))
		writePromptSummary(&b, task.Prompt)
		b.WriteString(" 本次任务已生成结果，请登录平台历史任务查看详情和生成图片。")
		if note := strings.TrimSpace(task.ErrorMessage); note != "" {
			fmt.Fprintf(&b, " 任务备注：%s。", note)
		}

		return "image2 生图任务已完成：" + taskID, mailer.TaskNotification{
			TaskID:     taskID,
			Status:     "生成成功",
			Message:    b.String(),
			RefundNote: "",
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "任务生成失败，失败时间：%s。", s.now().Format("2006-01-02 15:04:05"))
	writePromptSummary(&b, task.Prompt)
	if reason := strings.TrimSpace(task.ErrorMessage); reason != "" {
		fmt.Fprintf(&b, " 失败原因：%s。", reason)
	}

	refundNote := ""
	if task.EmailNotifyCostPoints > 0 {
		refundNote = fmt.Sprintf("本次邮件通知额外消耗的 %d 积分已退回账号。", task.EmailNotifyCostPoints)
	}

	return "image2 生图任务失败通知：" + taskID, mailer.TaskNotification{
		TaskID:     taskID,
		Status:     "生成失败",
		Message:    b.String(),
		RefundNote: refundNote,
	}
}
func writePromptSummary(b *strings.Builder, prompt string) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return
	}
	runes := []rune(prompt)
	if len(runes) > 300 {
		prompt = string(runes[:300]) + "..."
	}
	fmt.Fprintf(b, " 提示词摘要：%s。", prompt)
}

func (s *Service) refundEmailNotifyPoints(tx *gorm.DB, task model.ImageTask, userID uint64, currentPoints int64) (int64, error) {
	if !task.EmailNotifyEnabled || task.EmailNotifyCostPoints <= 0 {
		return currentPoints, nil
	}
	afterPoints := currentPoints + task.EmailNotifyCostPoints
	if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("points_balance", afterPoints).Error; err != nil {
		return currentPoints, err
	}
	if err := s.recordPointsLog(tx, userID, model.PointsChangeTypeRefund, task.EmailNotifyCostPoints, currentPoints, afterPoints, "image_task", strconv.FormatUint(task.ID, 10), "任务失败退回邮件通知积分"); err != nil {
		return currentPoints, err
	}
	return afterPoints, nil
}
