package dto

import "time"

type RedeemCodeRequest struct {
	Code string `json:"code" binding:"required,min=4,max=64"`
}

type RedeemCodeResponse struct {
	Code                string `json:"code"`
	PointsAmount        int64  `json:"pointsAmount"`
	CreditsAmount       int64  `json:"creditsAmount"`
	PointsBalance       int64  `json:"pointsBalance"`
	FreeCreditsBalance  int64  `json:"freeCreditsBalance"`
	PaidCreditsBalance  int64  `json:"paidCreditsBalance"`
	TotalCreditsBalance int64  `json:"totalCreditsBalance"`
	RedeemedAt          string `json:"redeemedAt"`
}

type AdminCreateRedeemCodesRequest struct {
	Count         int64  `json:"count" binding:"required,min=1,max=500"`
	PointsAmount  int64  `json:"pointsAmount" binding:"omitempty,min=0"`
	CreditsAmount int64  `json:"creditsAmount" binding:"omitempty,min=0"`
	Prefix        string `json:"prefix" binding:"omitempty,max=16"`
	Note          string `json:"note" binding:"omitempty,max=255"`
}

type AdminRedeemCodeItem struct {
	ID            string     `json:"id"`
	Code          string     `json:"code"`
	PointsAmount  int64      `json:"pointsAmount"`
	CreditsAmount int64      `json:"creditsAmount"`
	Status        string     `json:"status"`
	BatchNo       string     `json:"batchNo,omitempty"`
	Note          string     `json:"note,omitempty"`
	CreatedBy     string     `json:"createdBy"`
	UsedBy        string     `json:"usedBy,omitempty"`
	UsedAt        *time.Time `json:"usedAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type AdminCreateRedeemCodesResponse struct {
	BatchNo string                `json:"batchNo"`
	Items   []AdminRedeemCodeItem `json:"items"`
}

type AdminListRedeemCodesResponse struct {
	Items    []AdminRedeemCodeItem `json:"items"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
	Total    int64                 `json:"total"`
}
