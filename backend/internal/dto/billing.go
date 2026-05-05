package dto

import "time"

// PackageItem 表示可购买的充值套餐。
type PackageItem struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	PackageType   string `json:"packageType"`
	CreditsAmount int64  `json:"creditsAmount"`
	PointsCost    int64  `json:"pointsCost"`
	AmountCents   int64  `json:"amountCents"`
	BonusPoints   int64  `json:"bonusPoints"`
	Enabled       bool   `json:"enabled"`
	SortOrder     int    `json:"sortOrder"`
	Note          string `json:"note,omitempty"`
}

// ListPackagesResponse 是套餐列表返回体。
type ListPackagesResponse struct {
	Items []PackageItem `json:"items"`
}

// CreateRechargeOrderRequest 用于创建充值订单。
type CreateRechargeOrderRequest struct {
	RechargeType string `json:"rechargeType" binding:"omitempty,oneof=custom package"`
	Count        int64  `json:"count,omitempty" binding:"omitempty,min=1"`
	PackageCode  string `json:"packageCode,omitempty" binding:"omitempty,max=64"`
}

// ConfirmRechargeOrderRequest 用于确认充值订单已支付。
type ConfirmRechargeOrderRequest struct {
	ProviderTradeNo string     `json:"providerTradeNo,omitempty" binding:"omitempty,max=128"`
	PaymentChannel  string     `json:"paymentChannel,omitempty" binding:"omitempty,max=32"`
	AmountCents     int64      `json:"amountCents,omitempty" binding:"omitempty,min=0"`
	PaidAt          *time.Time `json:"paidAt,omitempty"`
	Remark          string     `json:"remark,omitempty" binding:"omitempty,max=255"`
}

// RechargePaymentCallbackRequest 是支付回调请求体。
type RechargePaymentCallbackRequest struct {
	OrderNo         string     `json:"orderNo" binding:"required"`
	ProviderTradeNo string     `json:"providerTradeNo,omitempty" binding:"omitempty,max=128"`
	PaymentChannel  string     `json:"paymentChannel,omitempty" binding:"omitempty,max=32"`
	AmountCents     int64      `json:"amountCents,omitempty" binding:"omitempty,min=0"`
	PaidAt          *time.Time `json:"paidAt,omitempty"`
	Remark          string     `json:"remark,omitempty" binding:"omitempty,max=255"`
}

// RechargeOrderItem 表示一条充值订单。
type RechargeOrderItem struct {
	ID              string     `json:"id"`
	OrderNo         string     `json:"orderNo"`
	RechargeType    string     `json:"rechargeType"`
	PackageCode     string     `json:"packageCode"`
	PackageName     string     `json:"packageName"`
	AmountCents     int64      `json:"amountCents"`
	CreditsAmount   int64      `json:"creditsAmount"`
	BonusPoints     int64      `json:"bonusPoints"`
	PointsAmount    int64      `json:"pointsAmount"`
	Status          string     `json:"status"`
	PaymentChannel  string     `json:"paymentChannel,omitempty"`
	ProviderTradeNo string     `json:"providerTradeNo,omitempty"`
	PaidAt          *time.Time `json:"paidAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// RechargeOrderResponse 表示订单创建或支付确认后的余额返回。
type RechargeOrderResponse struct {
	RechargeOrderItem
	PointsBalance       int64 `json:"pointsBalance"`
	FreeCreditsBalance  int64 `json:"freeCreditsBalance"`
	PaidCreditsBalance  int64 `json:"paidCreditsBalance"`
	TotalCreditsBalance int64 `json:"totalCreditsBalance"`
	Balance             int64 `json:"balance"`
}

// ListRechargeOrdersResponse 是用户充值订单列表返回体。
type ListRechargeOrdersResponse struct {
	Items    []RechargeOrderItem `json:"items"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
	Total    int64               `json:"total"`
}

// MockPurchaseRequest 保留旧的模拟购买入口兼容。
type MockPurchaseRequest = CreateRechargeOrderRequest

// MockPurchaseResponse 保留旧的模拟购买返回体兼容。
type MockPurchaseResponse = RechargeOrderResponse
