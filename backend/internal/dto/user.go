package dto

import "time"

// CheckinResponse 表示签到接口的返回结果。
type CheckinResponse struct {
	CheckedIn     bool      `json:"checkedIn"`
	GainedPoints  int64     `json:"gainedPoints"`
	GainedCredits int64     `json:"gainedCredits"`
	Streak        int       `json:"streak"`
	PointsBalance int64     `json:"pointsBalance"`
	FreeCredits   int64     `json:"freeCredits"`
	PaidCredits   int64     `json:"paidCredits"`
	TotalCredits  int64     `json:"totalCredits"`
	LastCheckinAt time.Time `json:"lastCheckinAt"`
}

// PointsLogItem 是前端展示用的积分流水。
type PointsLogItem struct {
	ID            string    `json:"id"`
	ChangeType    string    `json:"changeType"`
	Delta         int64     `json:"delta"`
	BeforeBalance int64     `json:"beforeBalance"`
	AfterBalance  int64     `json:"afterBalance"`
	Remark        string    `json:"remark,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// CreditLogItem 是前端展示用的福利次数流水。
type CreditLogItem struct {
	ID         string    `json:"id"`
	ChangeType string    `json:"changeType"`
	DeltaFree  int64     `json:"deltaFree"`
	DeltaPaid  int64     `json:"deltaPaid"`
	BeforeFree int64     `json:"beforeFree"`
	BeforePaid int64     `json:"beforePaid"`
	AfterFree  int64     `json:"afterFree"`
	AfterPaid  int64     `json:"afterPaid"`
	Remark     string    `json:"remark,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// PointsResponse 是用户积分与福利次数总览。
type PointsResponse struct {
	Balance             int64           `json:"balance"`
	FreeCredits         int64           `json:"freeCredits"`
	PaidCredits         int64           `json:"paidCredits"`
	TotalCredits        int64           `json:"totalCredits"`
	Streak              int             `json:"streak"`
	LastCheckinAt       *time.Time      `json:"lastCheckinAt,omitempty"`
	CheckinPointsTotal  int64           `json:"checkinPointsTotal"`
	CheckinCreditsTotal int64           `json:"checkinCreditsTotal"`
	RecentLogs          []PointsLogItem `json:"recentLogs"`
	RecentCreditLogs    []CreditLogItem `json:"recentCreditLogs,omitempty"`
}
