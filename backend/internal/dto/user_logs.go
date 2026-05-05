package dto

import "time"

// ListPointsLogsResponse 是积分流水分页结果。
type ListPointsLogsResponse struct {
	Items    []PointsLogItem `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	Total    int64           `json:"total"`
}

// ListCreditLogsResponse 是次数流水分页结果。
type ListCreditLogsResponse struct {
	Items    []CreditLogItem `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	Total    int64           `json:"total"`
}

// UserActivityItem 是用户统一记录流的单条数据。
type UserActivityItem struct {
	ID              string    `json:"id"`
	RecordType      string    `json:"recordType"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Status          string    `json:"status,omitempty"`
	ImageURL        string    `json:"imageUrl,omitempty"`
	DeltaPoints     int64     `json:"deltaPoints,omitempty"`
	DeltaFreeCredits int64    `json:"deltaFreeCredits,omitempty"`
	DeltaPaidCredits int64    `json:"deltaPaidCredits,omitempty"`
	CreditsAmount   int64     `json:"creditsAmount,omitempty"`
	PointsAmount    int64     `json:"pointsAmount,omitempty"`
	AmountCents     int64     `json:"amountCents,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// ListUserActivityResponse 是用户统一记录分页结果。
type ListUserActivityResponse struct {
	Items    []UserActivityItem `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int64              `json:"total"`
}
