package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func normalizeInviteCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func maxSetting(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

// Register creates a user, applies the configured signup gift, and records invite relationships.
func (s *Service) Register(ctx context.Context, req dto.RegisterRequest) (dto.AuthResponse, error) {
	if !s.cfg.RegistrationEnabled {
		return dto.AuthResponse{}, ErrRegistrationDisabled
	}

	email := normalizeEmail(req.Email)
	username := normalizeUsername(req.Username)
	inviteCode := normalizeInviteCode(req.InviteCode)
	if email == "" || username == "" || strings.TrimSpace(req.Password) == "" {
		return dto.AuthResponse{}, fmt.Errorf("请求参数不合法")
	}

	ok, err := s.emailCodes.Verify(ctx, email, "register", strings.TrimSpace(req.Code), s.emailCodePolicy())
	if err != nil {
		return dto.AuthResponse{}, mapEmailCodeError(err)
	}
	if !ok {
		return dto.AuthResponse{}, ErrInvalidVerificationCode
	}

	registrationGiftCredits := maxSetting(s.settingInt64(systemSettingKeyRegistrationGiftCredits, 3))
	var created model.User
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&model.User{}).
			Where("lower(email) = ? OR username = ?", email, username).
			Count(&exists).Error; err != nil {
			return err
		}
		if exists > 0 {
			return ErrAccountExists
		}

		var inviter *model.User
		if inviteCode != "" {
			var found model.User
			if err := tx.Where("invite_code = ? AND status = ?", inviteCode, model.UserStatusNormal).First(&found).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrInviteCodeInvalid
				}
				return err
			}
			inviter = &found
		}

		hash, err := hashPassword(req.Password)
		if err != nil {
			return err
		}
		userInviteCode, err := s.generateUniqueInviteCode(tx)
		if err != nil {
			return err
		}

		now := s.now()
		settlementDate := s.startOfDay(now)
		user := model.User{
			Username:                 username,
			Email:                    email,
			PasswordHash:             hash,
			PointsBalance:            0,
			FreeCredits:              registrationGiftCredits,
			InviteCode:               userInviteCode,
			CheckinStreak:            0,
			LastCreditSettlementDate: &settlementDate,
			EmailVerifiedAt:          &now,
			LastLoginAt:              &now,
			Status:                   model.UserStatusNormal,
			Role:                     model.UserRoleUser,
		}
		if s.isAdminEmail(email) {
			user.Role = model.UserRoleAdmin
		}
		if inviter != nil {
			user.InviterUserID = &inviter.ID
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		if registrationGiftCredits > 0 {
			if err := tx.Create(&model.CreditLog{
				UserID:     user.ID,
				ChangeType: model.CreditChangeTypeRegister,
				DeltaFree:  registrationGiftCredits,
				DeltaPaid:  0,
				BeforeFree: 0,
				BeforePaid: 0,
				AfterFree:  registrationGiftCredits,
				AfterPaid:  0,
				BizType:    "register",
				BizID:      strconv.FormatUint(user.ID, 10),
				Remark:     "新用户注册赠送",
			}).Error; err != nil {
				return err
			}
		}

		if inviter != nil {
			if err := s.createPendingInvitation(tx, inviter.ID, user.ID, inviteCode); err != nil {
				return err
			}
		}
		created = user
		return nil
	})
	if err != nil {
		return dto.AuthResponse{}, err
	}

	return s.authResponse(created)
}

func (s *Service) createPendingInvitation(tx *gorm.DB, inviterUserID uint64, inviteeUserID uint64, inviteCode string) error {
	return tx.Create(&model.Invitation{
		InviterUserID: inviterUserID,
		InviteeUserID: inviteeUserID,
		InviteCode:    inviteCode,
		RewardPoints:  0,
		RewardCredits: 0,
	}).Error
}

func (s *Service) grantInviteRewardAfterFirstRedeem(tx *gorm.DB, inviteeUserID uint64) error {
	var invitation model.Invitation
	query := tx.Where("invitee_user_id = ?", inviteeUserID)
	if tx.Dialector.Name() == "mysql" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&invitation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if invitation.RewardPoints > 0 || invitation.RewardCredits > 0 {
		return nil
	}

	rewardPoints := maxSetting(s.settingInt64(systemSettingKeyInviteRewardPoints, 0))
	rewardCredits := maxSetting(s.settingInt64(systemSettingKeyInviteRewardCredits, 1))
	if rewardPoints <= 0 && rewardCredits <= 0 {
		return nil
	}

	inviter, err := s.lockUser(tx, invitation.InviterUserID)
	if err != nil {
		return err
	}
	bizID := strconv.FormatUint(invitation.ID, 10)

	if rewardPoints > 0 {
		before := inviter.PointsBalance
		after := before + rewardPoints
		if err := tx.Model(&model.User{}).Where("id = ?", inviter.ID).Update("points_balance", after).Error; err != nil {
			return err
		}
		if err := s.recordPointsLog(tx, inviter.ID, model.PointsChangeTypeInvite, rewardPoints, before, after, "invite", bizID, "邀请好友首次兑换奖励"); err != nil {
			return err
		}
		inviter.PointsBalance = after
	}

	if rewardCredits > 0 {
		before := creditBalance{Free: inviter.FreeCredits, Paid: inviter.PaidCredits}
		after := creditBalance{Free: before.Free + rewardCredits, Paid: before.Paid}
		if err := tx.Model(&model.User{}).Where("id = ?", inviter.ID).Update("free_credits", after.Free).Error; err != nil {
			return err
		}
		if err := s.recordCreditLog(tx, inviter.ID, model.CreditChangeTypeInvite, rewardCredits, 0, before, after, "invite", bizID, "邀请好友首次兑换奖励"); err != nil {
			return err
		}
		inviter.FreeCredits = after.Free
	}

	return tx.Model(&model.Invitation{}).Where("id = ?", invitation.ID).Updates(map[string]any{
		"reward_points":  rewardPoints,
		"reward_credits": rewardCredits,
	}).Error
}

func (s *Service) grantInviteReward(tx *gorm.DB, inviter *model.User, invitee model.User, inviteCode string, rewardPoints, rewardCredits int64) error {
	invitation := model.Invitation{
		InviterUserID: inviter.ID,
		InviteeUserID: invitee.ID,
		InviteCode:    inviteCode,
		RewardPoints:  rewardPoints,
		RewardCredits: rewardCredits,
	}
	if err := tx.Create(&invitation).Error; err != nil {
		return err
	}
	bizID := strconv.FormatUint(invitation.ID, 10)

	if rewardPoints > 0 {
		before := inviter.PointsBalance
		after := before + rewardPoints
		if err := tx.Model(&model.User{}).Where("id = ?", inviter.ID).Update("points_balance", after).Error; err != nil {
			return err
		}
		if err := s.recordPointsLog(tx, inviter.ID, model.PointsChangeTypeInvite, rewardPoints, before, after, "invite", bizID, "邀请好友注册奖励"); err != nil {
			return err
		}
		inviter.PointsBalance = after
	}

	if rewardCredits > 0 {
		before := creditBalance{Free: inviter.FreeCredits, Paid: inviter.PaidCredits}
		after := creditBalance{Free: before.Free + rewardCredits, Paid: before.Paid}
		if err := tx.Model(&model.User{}).Where("id = ?", inviter.ID).Update("free_credits", after.Free).Error; err != nil {
			return err
		}
		if err := s.recordCreditLog(tx, inviter.ID, model.CreditChangeTypeInvite, rewardCredits, 0, before, after, "invite", bizID, "邀请好友注册奖励"); err != nil {
			return err
		}
		inviter.FreeCredits = after.Free
	}
	return nil
}

func (s *Service) GenerateUserInviteCode(ctx context.Context, userID uint64) (string, error) {
	user, err := s.loadUser(userID)
	if err != nil {
		return "", err
	}
	code := normalizeInviteCode(user.InviteCode)
	if code != "" {
		return code, nil
	}

	var generated string
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}
		if code := normalizeInviteCode(user.InviteCode); code != "" {
			generated = code
			return nil
		}
		code, err := s.generateUniqueInviteCode(tx)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("invite_code", code).Error; err != nil {
			return err
		}
		generated = code
		return nil
	})
	if err != nil {
		return "", err
	}
	return generated, nil
}

func (s *Service) InviteInfo(ctx context.Context, userID uint64) (dto.InviteInfoResponse, error) {
	code, err := s.GenerateUserInviteCode(ctx, userID)
	if err != nil {
		return dto.InviteInfoResponse{}, err
	}

	var total struct {
		Count         int64
		RewardPoints  int64
		RewardCredits int64
	}
	if err := s.db.WithContext(ctx).
		Model(&model.Invitation{}).
		Select("count(*) as count, coalesce(sum(reward_points), 0) as reward_points, coalesce(sum(reward_credits), 0) as reward_credits").
		Where("inviter_user_id = ?", userID).
		Scan(&total).Error; err != nil {
		return dto.InviteInfoResponse{}, err
	}

	return dto.InviteInfoResponse{
		InviteCode:              code,
		InvitedCount:            total.Count,
		RewardPointsTotal:       total.RewardPoints,
		RewardCreditsTotal:      total.RewardCredits,
		InviteRewardPoints:      s.settingInt64(systemSettingKeyInviteRewardPoints, 0),
		InviteRewardCredits:     s.settingInt64(systemSettingKeyInviteRewardCredits, 1),
		RegistrationGiftCredits: s.settingInt64(systemSettingKeyRegistrationGiftCredits, 3),
	}, nil
}

func (s *Service) generateUniqueInviteCode(tx *gorm.DB) (string, error) {
	for i := 0; i < 12; i++ {
		code, err := randomInviteCode(8)
		if err != nil {
			return "", err
		}
		var count int64
		if err := tx.Model(&model.User{}).Where("invite_code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", ErrInviteCodeInvalid
}

func randomInviteCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	out := make([]byte, length)
	for i, value := range bytes {
		out[i] = inviteCodeAlphabet[int(value)%len(inviteCodeAlphabet)]
	}
	return string(out), nil
}
