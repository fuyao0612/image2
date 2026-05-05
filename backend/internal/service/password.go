package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
)

// ChangePassword 允许已登录用户修改自己的登录密码。
func (s *Service) ChangePassword(ctx context.Context, userID uint64, req dto.ChangePasswordRequest) (dto.ChangePasswordResponse, error) {
	_ = ctx

	currentPassword := req.CurrentPassword
	newPassword := req.NewPassword
	confirmNewPassword := req.ConfirmNewPassword
	if strings.TrimSpace(currentPassword) == "" || strings.TrimSpace(newPassword) == "" || strings.TrimSpace(confirmNewPassword) == "" {
		return dto.ChangePasswordResponse{}, fmt.Errorf("请求参数不合法")
	}
	if newPassword != confirmNewPassword {
		return dto.ChangePasswordResponse{}, ErrPasswordConfirmationMismatch
	}

	var changedAt time.Time
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}
		if user.Status == model.UserStatusDisabled {
			return ErrUserDisabled
		}
		if err := comparePassword(user.PasswordHash, currentPassword); err != nil {
			return ErrCurrentPasswordIncorrect
		}
		if currentPassword == newPassword {
			return ErrPasswordUnchanged
		}

		hash, err := hashPassword(newPassword)
		if err != nil {
			return err
		}

		changedAt = s.now()
		return tx.Model(&model.User{}).
			Where("id = ?", userID).
			Update("password_hash", hash).Error
	})
	if err != nil {
		return dto.ChangePasswordResponse{}, err
	}

	return dto.ChangePasswordResponse{
		ChangedAt: changedAt,
	}, nil
}
