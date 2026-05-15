package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const passwordResetCooldown = 72 * time.Hour

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
			Updates(map[string]any{
				"password_hash": hash,
				"token_version": gorm.Expr("token_version + 1"),
			}).Error
	})
	if err != nil {
		return dto.ChangePasswordResponse{}, err
	}

	return dto.ChangePasswordResponse{ChangedAt: changedAt}, nil
}

func (s *Service) CheckResetPasswordEmail(ctx context.Context, req dto.CheckResetPasswordEmailRequest) (dto.CheckResetPasswordEmailResponse, error) {
	user, err := s.userByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.CheckResetPasswordEmailResponse{Available: false, Message: ErrEmailNotRegistered.Error()}, nil
		}
		return dto.CheckResetPasswordEmailResponse{}, err
	}
	return s.resetPasswordAvailability(user), nil
}

func (s *Service) ResetPassword(ctx context.Context, req dto.ResetPasswordRequest) (dto.ChangePasswordResponse, error) {
	email := normalizeEmail(req.Email)
	password := strings.TrimSpace(req.Password)
	confirmPassword := strings.TrimSpace(req.ConfirmPassword)
	if email == "" || strings.TrimSpace(req.Code) == "" || password == "" || confirmPassword == "" {
		return dto.ChangePasswordResponse{}, fmt.Errorf("请求参数不合法")
	}
	if password != confirmPassword {
		return dto.ChangePasswordResponse{}, ErrPasswordConfirmationMismatch
	}

	ok, err := s.emailCodes.Verify(ctx, email, "reset_password", strings.TrimSpace(req.Code), s.emailCodePolicy())
	if err != nil {
		return dto.ChangePasswordResponse{}, mapEmailCodeError(err)
	}
	if !ok {
		return dto.ChangePasswordResponse{}, ErrInvalidVerificationCode
	}

	hash, err := hashPassword(password)
	if err != nil {
		return dto.ChangePasswordResponse{}, err
	}

	var changedAt time.Time
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("lower(email) = ?", email).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEmailNotRegistered
			}
			return err
		}
		if user.Status == model.UserStatusDisabled {
			return ErrUserDisabled
		}
		if !s.resetPasswordAvailability(user).Available {
			return ErrPasswordResetTooFrequent
		}

		changedAt = s.now()
		return tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
			"password_hash":          hash,
			"token_version":          gorm.Expr("token_version + 1"),
			"last_password_reset_at": changedAt,
		}).Error
	})
	if err != nil {
		return dto.ChangePasswordResponse{}, err
	}
	_ = s.emailCodes.Delete(ctx, email, "reset_password")
	return dto.ChangePasswordResponse{ChangedAt: changedAt}, nil
}

func (s *Service) ensureResetPasswordEmailAllowed(ctx context.Context, email string) error {
	user, err := s.userByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEmailNotRegistered
		}
		return err
	}
	if user.Status == model.UserStatusDisabled {
		return ErrUserDisabled
	}
	if !s.resetPasswordAvailability(user).Available {
		return ErrPasswordResetTooFrequent
	}
	return nil
}

func (s *Service) userByEmail(ctx context.Context, email string) (model.User, error) {
	var user model.User
	err := s.db.WithContext(ctx).Where("lower(email) = ?", normalizeEmail(email)).First(&user).Error
	return user, err
}

func (s *Service) resetPasswordAvailability(user model.User) dto.CheckResetPasswordEmailResponse {
	if user.LastPasswordResetAt == nil || user.LastPasswordResetAt.IsZero() {
		return dto.CheckResetPasswordEmailResponse{Available: true, Message: "该邮箱可以重置密码。"}
	}
	next := user.LastPasswordResetAt.Add(passwordResetCooldown)
	if s.now().Before(next) {
		return dto.CheckResetPasswordEmailResponse{
			Available:    false,
			Message:      ErrPasswordResetTooFrequent.Error(),
			NextResetAt:  &next,
			ResetAfterAt: &next,
		}
	}
	return dto.CheckResetPasswordEmailResponse{Available: true, Message: "该邮箱可以重置密码。"}
}
