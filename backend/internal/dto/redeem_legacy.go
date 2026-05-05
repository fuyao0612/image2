package dto

// RedeemPointsRequest 是旧版积分兑换次数入口的兼容请求体；当前业务已关闭该入口。
type RedeemPointsRequest struct {
	PackageCode string `json:"packageCode,omitempty"`
}

// RedeemPointsResponse 是旧版积分兑换次数入口的兼容响应体；当前业务已关闭该入口。
type RedeemPointsResponse struct {
	RedemptionNo        string `json:"redemptionNo"`
	PackageCode         string `json:"packageCode"`
	PackageName         string `json:"packageName"`
	PointsCost          int64  `json:"pointsCost"`
	CreditsAmount       int64  `json:"creditsAmount"`
	PointsBalance       int64  `json:"pointsBalance"`
	FreeCreditsBalance  int64  `json:"freeCreditsBalance"`
	PaidCreditsBalance  int64  `json:"paidCreditsBalance"`
	TotalCreditsBalance int64  `json:"totalCreditsBalance"`
	Status              string `json:"status"`
}
