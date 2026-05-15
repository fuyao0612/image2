package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const redeemCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func (s *Service) RedeemCode(ctx context.Context, userID uint64, req dto.RedeemCodeRequest) (dto.RedeemCodeResponse, error) {
	codeText := normalizeRedeemCode(req.Code)
	if codeText == "" {
		return dto.RedeemCodeResponse{}, ErrRedeemCodeInvalid
	}

	var resp dto.RedeemCodeResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		code, err := s.lockRedeemCode(tx, codeText)
		if err != nil {
			return err
		}
		if code.Status == model.RedeemCodeStatusUsed {
			return ErrRedeemCodeUsed
		}
		if code.Status != model.RedeemCodeStatusActive {
			return ErrRedeemCodeInvalid
		}
		now := s.now()
		if code.ExpiresAt != nil && !code.ExpiresAt.After(now) {
			return ErrRedeemCodeExpired
		}
		if code.PointsAmount <= 0 && code.CreditsAmount <= 0 {
			return ErrRedeemCodeRewardEmpty
		}

		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}

		pointsAfter := user.PointsBalance + code.PointsAmount
		freeAfter := user.FreeCredits
		paidAfter := user.PaidCredits + code.CreditsAmount
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
			"points_balance": pointsAfter,
			"paid_credits":   paidAfter,
		}).Error; err != nil {
			return err
		}

		bizID := strconv.FormatUint(code.ID, 10)
		if code.PointsAmount > 0 {
			if err := s.recordPointsLog(tx, userID, model.PointsChangeTypeCode, code.PointsAmount, user.PointsBalance, pointsAfter, "redeem_code", bizID, "兑换码入账"); err != nil {
				return err
			}
		}
		if code.CreditsAmount > 0 {
			before := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
			after := creditBalance{Free: freeAfter, Paid: paidAfter}
			if err := s.recordCreditLog(tx, userID, model.CreditChangeTypeCode, 0, code.CreditsAmount, before, after, "redeem_code", bizID, "兑换码入账福利次数"); err != nil {
				return err
			}
		}

		if err := tx.Model(&model.RedeemCode{}).Where("id = ?", code.ID).Updates(map[string]any{
			"status":  model.RedeemCodeStatusUsed,
			"used_by": userID,
			"used_at": now,
		}).Error; err != nil {
			return err
		}

		resp = dto.RedeemCodeResponse{
			Code:                code.Code,
			PointsAmount:        code.PointsAmount,
			CreditsAmount:       code.CreditsAmount,
			PointsBalance:       pointsAfter,
			FreeCreditsBalance:  freeAfter,
			PaidCreditsBalance:  paidAfter,
			TotalCreditsBalance: freeAfter + paidAfter,
			RedeemedAt:          now.Format(time.RFC3339),
		}
		return nil
	})
	return resp, err
}

func (s *Service) AdminCreateRedeemCodes(ctx context.Context, adminID uint64, req dto.AdminCreateRedeemCodesRequest) (dto.AdminCreateRedeemCodesResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminCreateRedeemCodesResponse{}, err
	}
	if req.PointsAmount <= 0 && req.CreditsAmount <= 0 {
		return dto.AdminCreateRedeemCodesResponse{}, ErrRedeemCodeRewardEmpty
	}

	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > 500 {
		count = 500
	}

	prefix := normalizeRedeemCode(req.Prefix)
	batchNo := fmt.Sprintf("RC%s%d", s.now().Format("20060102150405"), time.Now().UnixNano()%10000)
	created := make([]model.RedeemCode, 0, count)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for int64(len(created)) < count {
			codeText, err := generateRedeemCode(prefix)
			if err != nil {
				return err
			}
			item := model.RedeemCode{
				Code:          codeText,
				PointsAmount:  req.PointsAmount,
				CreditsAmount: req.CreditsAmount,
				Status:        model.RedeemCodeStatusActive,
				BatchNo:       batchNo,
				Note:          strings.TrimSpace(req.Note),
				CreatedBy:     adminID,
			}
			if err := tx.Create(&item).Error; err != nil {
				if isUniqueConstraintError(err) {
					continue
				}
				return err
			}
			created = append(created, item)
		}
		return recordAdminActionLog(tx, adminID, "redeem_code.create", "redeem_code_batch", batchNo, nil, created, strings.TrimSpace(req.Note))
	})
	if err != nil {
		return dto.AdminCreateRedeemCodesResponse{}, err
	}

	items := make([]dto.AdminRedeemCodeItem, 0, len(created))
	for _, item := range created {
		items = append(items, redeemCodeToAdminDTO(item))
	}
	return dto.AdminCreateRedeemCodesResponse{BatchNo: batchNo, Items: items}, nil
}

func (s *Service) AdminListRedeemCodes(ctx context.Context, adminID uint64, status string, keyword string, page int, pageSize int) (dto.AdminListRedeemCodesResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListRedeemCodesResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)

	query := s.db.WithContext(ctx).Model(&model.RedeemCode{})
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		normalizedCodeLike := "%" + normalizeRedeemCode(keyword) + "%"
		idKeyword := strings.TrimPrefix(keyword, "#")
		if id, err := strconv.ParseUint(idKeyword, 10, 64); err == nil && id > 0 {
			query = query.Where("id = ? OR created_by = ? OR used_by = ? OR code LIKE ? OR batch_no LIKE ? OR note LIKE ?", id, id, id, normalizedCodeLike, like, like)
		} else {
			query = query.Where("code LIKE ? OR batch_no LIKE ? OR note LIKE ?", normalizedCodeLike, like, like)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListRedeemCodesResponse{}, err
	}

	var codes []model.RedeemCode
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&codes).Error; err != nil {
		return dto.AdminListRedeemCodesResponse{}, err
	}

	items := make([]dto.AdminRedeemCodeItem, 0, len(codes))
	for _, code := range codes {
		items = append(items, redeemCodeToAdminDTO(code))
	}
	return dto.AdminListRedeemCodesResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) AdminDeleteRedeemCode(ctx context.Context, adminID uint64, codeID string) error {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(codeID), 10, 64)
	if err != nil || id == 0 {
		return ErrRedeemCodeInvalid
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var code model.RedeemCode
		if err := tx.First(&code, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRedeemCodeInvalid
			}
			return err
		}
		before := code
		if err := tx.Delete(&code).Error; err != nil {
			return err
		}
		return recordAdminActionLog(tx, adminID, "redeem_code.delete", "redeem_code", strconv.FormatUint(code.ID, 10), before, nil, code.Code)
	})
}

func (s *Service) lockRedeemCode(tx *gorm.DB, codeText string) (model.RedeemCode, error) {
	var code model.RedeemCode
	query := tx.Where("code = ?", normalizeRedeemCode(codeText))
	if tx.Dialector.Name() == "mysql" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&code).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.RedeemCode{}, ErrRedeemCodeInvalid
		}
		return model.RedeemCode{}, err
	}
	return code, nil
}

func redeemCodeToAdminDTO(code model.RedeemCode) dto.AdminRedeemCodeItem {
	usedBy := ""
	if code.UsedBy > 0 {
		usedBy = strconv.FormatUint(code.UsedBy, 10)
	}
	return dto.AdminRedeemCodeItem{
		ID:            strconv.FormatUint(code.ID, 10),
		Code:          code.Code,
		PointsAmount:  code.PointsAmount,
		CreditsAmount: code.CreditsAmount,
		Status:        string(code.Status),
		BatchNo:       code.BatchNo,
		Note:          code.Note,
		CreatedBy:     strconv.FormatUint(code.CreatedBy, 10),
		UsedBy:        usedBy,
		UsedAt:        code.UsedAt,
		CreatedAt:     code.CreatedAt,
	}
}

func normalizeRedeemCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	return value
}

func generateRedeemCode(prefix string) (string, error) {
	randomPart, err := randomCodeString(16)
	if err != nil {
		return "", err
	}
	if prefix == "" {
		return randomPart, nil
	}
	return prefix + randomPart, nil
}

func randomCodeString(length int) (string, error) {
	var builder strings.Builder
	builder.Grow(length)
	max := big.NewInt(int64(len(redeemCodeAlphabet)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(redeemCodeAlphabet[n.Int64()])
	}
	return builder.String(), nil
}

func isUniqueConstraintError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") || strings.Contains(message, "unique")
}
