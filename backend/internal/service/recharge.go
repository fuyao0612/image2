package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListRechargeOrders 返回当前用户的充值订单分页。
func (s *Service) ListRechargeOrders(ctx context.Context, userID uint64, page, pageSize int) (dto.ListRechargeOrdersResponse, error) {
	page, pageSize = normalizePage(page, pageSize)

	query := s.db.WithContext(ctx).Model(&model.RechargeOrder{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.ListRechargeOrdersResponse{}, err
	}

	var orders []model.RechargeOrder
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders).Error; err != nil {
		return dto.ListRechargeOrdersResponse{}, err
	}

	items := make([]dto.RechargeOrderItem, 0, len(orders))
	for _, order := range orders {
		items = append(items, rechargeOrderToItem(order))
	}

	return dto.ListRechargeOrdersResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// GetRechargeOrder 返回当前用户指定订单的详情。
func (s *Service) GetRechargeOrder(ctx context.Context, userID uint64, orderNo string) (dto.RechargeOrderResponse, error) {
	order, err := s.findRechargeOrderByNo(ctx, userID, orderNo)
	if err != nil {
		return dto.RechargeOrderResponse{}, err
	}
	user, err := s.loadUser(order.UserID)
	if err != nil {
		return dto.RechargeOrderResponse{}, err
	}
	return rechargeOrderToResponse(order, user.PointsBalance, creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}), nil
}

// CreateRechargeOrder 仅创建待支付订单，不会立即入账。
func (s *Service) CreateRechargeOrder(ctx context.Context, userID uint64, req dto.CreateRechargeOrderRequest) (dto.RechargeOrderResponse, error) {
	return s.createRechargeOrderV2(ctx, userID, req)

	pkg, err := s.findBillingPackageByCode(ctx, req.PackageCode, true)
	if err != nil {
		return dto.RechargeOrderResponse{}, err
	}
	if pkg.PackageType != model.BillingPackageTypeRecharge {
		return dto.RechargeOrderResponse{}, ErrPackageInvalid
	}
	if pkg.CreditsAmount <= 0 || pkg.AmountCents <= 0 {
		return dto.RechargeOrderResponse{}, ErrPackageInvalid
	}

	now := s.now()
	order := model.RechargeOrder{
		UserID:         userID,
		OrderNo:        generateOrderNo(now),
		PackageCode:    pkg.Code,
		PackageName:    pkg.Name,
		AmountCents:    pkg.AmountCents,
		PointsAmount:   pkg.BonusPoints,
		CreditsAmount:  pkg.CreditsAmount,
		BonusPoints:    pkg.BonusPoints,
		Status:         model.RechargeOrderStatusPending,
		PaymentChannel: "",
	}

	var currentUser model.User
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}
		currentUser = user
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return dto.RechargeOrderResponse{}, err
	}

	return rechargeOrderToResponse(order, currentUser.PointsBalance, creditBalance{Free: currentUser.FreeCredits, Paid: currentUser.PaidCredits}), nil
}

// ConfirmRechargeOrder 将指定订单标记为已支付，并入账次数与赠送积分。
func (s *Service) ConfirmRechargeOrder(ctx context.Context, userID uint64, orderNo string, req dto.ConfirmRechargeOrderRequest) (dto.RechargeOrderResponse, error) {
	return s.confirmRechargeOrderV2(ctx, userID, orderNo, req)
}

// RechargePaymentCallback 用于支付网关回调。
func (s *Service) RechargePaymentCallback(ctx context.Context, req dto.RechargePaymentCallbackRequest) (dto.RechargeOrderResponse, error) {
	return s.confirmRechargeOrderV2(ctx, 0, req.OrderNo, dto.ConfirmRechargeOrderRequest{
		ProviderTradeNo: req.ProviderTradeNo,
		PaymentChannel:  req.PaymentChannel,
		AmountCents:     req.AmountCents,
		PaidAt:          req.PaidAt,
		Remark:          req.Remark,
	})

	return s.settleRechargeOrder(ctx, req.OrderNo, nil, dto.ConfirmRechargeOrderRequest{
		ProviderTradeNo: req.ProviderTradeNo,
		PaymentChannel:  req.PaymentChannel,
		AmountCents:     req.AmountCents,
		PaidAt:          req.PaidAt,
		Remark:          req.Remark,
	})
}

func (s *Service) settleRechargeOrder(ctx context.Context, orderNo string, userID *uint64, req dto.ConfirmRechargeOrderRequest) (dto.RechargeOrderResponse, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return dto.RechargeOrderResponse{}, ErrOrderNotFound
	}

	var settledOrder model.RechargeOrder

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := s.lockRechargeOrder(tx, orderNo)
		if err != nil {
			return err
		}
		if userID != nil && order.UserID != *userID {
			return ErrForbidden
		}
		if req.AmountCents > 0 && req.AmountCents != order.AmountCents {
			return ErrOrderAmountMismatch
		}

		paymentChannel := strings.TrimSpace(req.PaymentChannel)
		if paymentChannel == "" {
			paymentChannel = strings.TrimSpace(order.PaymentChannel)
		}
		if paymentChannel == "" {
			paymentChannel = "mock"
		}
		providerTradeNo := strings.TrimSpace(req.ProviderTradeNo)
		remark := strings.TrimSpace(req.Remark)
		paidAt := s.now()
		if req.PaidAt != nil {
			paidAt = req.PaidAt.In(s.location)
		}

		if order.Status == model.RechargeOrderStatusPaid {
			updates := map[string]any{}
			if order.PaymentChannel == "" && paymentChannel != "" {
				updates["payment_channel"] = paymentChannel
				order.PaymentChannel = paymentChannel
			}
			if order.ProviderTradeNo == "" && providerTradeNo != "" {
				updates["provider_trade_no"] = providerTradeNo
				order.ProviderTradeNo = providerTradeNo
			}
			if order.PaidAt == nil {
				updates["paid_at"] = paidAt
				order.PaidAt = &paidAt
			}
			if len(updates) > 0 {
				if err := tx.Model(&model.RechargeOrder{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
			settledOrder = order
			return nil
		}
		if order.Status != model.RechargeOrderStatusPending {
			return ErrOrderInvalidState
		}

		user, err := s.lockUser(tx, order.UserID)
		if err != nil {
			return err
		}
		beforePoints := user.PointsBalance
		beforeCredits := creditBalance{Free: user.FreeCredits, Paid: user.PaidCredits}
		afterPoints := beforePoints + order.BonusPoints
		afterCredits := creditBalance{Free: beforeCredits.Free, Paid: beforeCredits.Paid + order.CreditsAmount}
		if afterCredits.Free < 0 || afterCredits.Paid < 0 {
			return ErrInsufficientCredits
		}

		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
			"points_balance": afterPoints,
			"free_credits":   afterCredits.Free,
			"paid_credits":   afterCredits.Paid,
		}).Error; err != nil {
			return err
		}

		if order.BonusPoints > 0 {
			remarkText := remark
			if remarkText == "" {
				remarkText = fmt.Sprintf("充值订单 %s 赠送积分", order.OrderNo)
			}
			if err := tx.Create(&model.PointsLog{
				UserID:        user.ID,
				ChangeType:    model.PointsChangeTypePurchaseBonus,
				Delta:         order.BonusPoints,
				BeforeBalance: beforePoints,
				AfterBalance:  afterPoints,
				BizType:       "recharge_order",
				BizID:         order.OrderNo,
				Remark:        remarkText,
			}).Error; err != nil {
				return err
			}
		}

		if err := s.recordCreditLog(tx, user.ID, model.CreditChangeTypePurchase, 0, order.CreditsAmount, beforeCredits, afterCredits, "recharge_order", order.OrderNo, "充值购买次数"); err != nil {
			return err
		}

		if providerTradeNo != "" {
			order.ProviderTradeNo = providerTradeNo
		}
		order.PaymentChannel = paymentChannel
		order.PaidAt = &paidAt
		order.Status = model.RechargeOrderStatusPaid

		if err := tx.Model(&model.RechargeOrder{}).Where("id = ?", order.ID).Updates(map[string]any{
			"status":            order.Status,
			"paid_at":           order.PaidAt,
			"payment_channel":   order.PaymentChannel,
			"provider_trade_no": order.ProviderTradeNo,
		}).Error; err != nil {
			return err
		}

		settledOrder = order
		return nil
	})
	if err != nil {
		return dto.RechargeOrderResponse{}, err
	}

	settledUser, err := s.loadUser(settledOrder.UserID)
	if err != nil {
		return dto.RechargeOrderResponse{}, err
	}

	return rechargeOrderToResponse(settledOrder, settledUser.PointsBalance, creditBalance{Free: settledUser.FreeCredits, Paid: settledUser.PaidCredits}), nil
}

func (s *Service) findRechargeOrderByNo(ctx context.Context, userID uint64, orderNo string) (model.RechargeOrder, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return model.RechargeOrder{}, ErrOrderNotFound
	}

	var order model.RechargeOrder
	query := s.db.WithContext(ctx).Where("order_no = ? AND user_id = ?", orderNo, userID)
	if err := query.First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.RechargeOrder{}, ErrOrderNotFound
		}
		return model.RechargeOrder{}, err
	}
	return order, nil
}

func (s *Service) lockRechargeOrder(tx *gorm.DB, orderNo string) (model.RechargeOrder, error) {
	var order model.RechargeOrder
	query := tx.Where("order_no = ?", strings.TrimSpace(orderNo))
	if tx.Dialector.Name() == "mysql" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.RechargeOrder{}, ErrOrderNotFound
		}
		return model.RechargeOrder{}, err
	}
	return order, nil
}

func rechargeOrderToItem(order model.RechargeOrder) dto.RechargeOrderItem {
	return dto.RechargeOrderItem{
		ID:              strconv.FormatUint(order.ID, 10),
		OrderNo:         order.OrderNo,
		RechargeType:    order.RechargeType,
		PackageCode:     order.PackageCode,
		PackageName:     order.PackageName,
		AmountCents:     order.AmountCents,
		CreditsAmount:   order.CreditsAmount,
		BonusPoints:     order.BonusPoints,
		PointsAmount:    order.PointsAmount,
		Status:          string(order.Status),
		PaymentChannel:  order.PaymentChannel,
		ProviderTradeNo: order.ProviderTradeNo,
		PaidAt:          order.PaidAt,
		CreatedAt:       order.CreatedAt,
	}
}

func rechargeOrderToResponse(order model.RechargeOrder, pointsBalance int64, credits creditBalance) dto.RechargeOrderResponse {
	return dto.RechargeOrderResponse{
		RechargeOrderItem:   rechargeOrderToItem(order),
		PointsBalance:       pointsBalance,
		FreeCreditsBalance:  credits.Free,
		PaidCreditsBalance:  credits.Paid,
		TotalCreditsBalance: credits.Total(),
		Balance:             credits.Total(),
	}
}
