package dto

// BillingSettingsResponse 表示平台账本相关的可配置参数。
type BillingSettingsResponse struct {
	CustomRechargeUnitPriceCents int64 `json:"customRechargeUnitPriceCents"`
	ImageGeneratePointCost       int64 `json:"imageGeneratePointCost"`
	CheckinPoints                int64 `json:"checkinPoints"`
	CheckinCredits               int64 `json:"checkinCredits"`
}

// UpdateBillingSettingsRequest 用于后台更新账本配置。
type UpdateBillingSettingsRequest struct {
	CustomRechargeUnitPriceCents int64 `json:"customRechargeUnitPriceCents" binding:"required,min=1"`
	ImageGeneratePointCost       int64 `json:"imageGeneratePointCost" binding:"required,min=1"`
	CheckinPoints                int64 `json:"checkinPoints" binding:"required,min=0"`
	CheckinCredits               int64 `json:"checkinCredits" binding:"required,min=0"`
}
