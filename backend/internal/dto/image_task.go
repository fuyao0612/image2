package dto

import "time"

// CreateImageTaskRequest 是创建生图任务的请求体。
type CreateImageTaskRequest struct {
	Prompt                string   `json:"prompt" binding:"required,min=1,max=2000"`
	Size                  string   `json:"size,omitempty" binding:"omitempty,max=32"`
	Style                 string   `json:"style,omitempty" binding:"omitempty,max=64"`
	Count                 int      `json:"count,omitempty" binding:"omitempty,min=1,max=6"`
	Quality               string   `json:"quality,omitempty" binding:"omitempty,oneof=auto low medium high"`
	OutputFormat          string   `json:"outputFormat,omitempty" binding:"omitempty,oneof=default jpeg webp png"`
	ReferenceImageURL     string   `json:"referenceImageUrl,omitempty" binding:"omitempty,max=1024"`
	ReferenceImageURLs    []string `json:"referenceImageUrls,omitempty" binding:"omitempty,max=12,dive,max=1024"`
	EmailNotifyOnComplete bool     `json:"emailNotifyOnComplete,omitempty"`
}

// CreateImageTaskResponse 是提交任务后的简化返回体。
type CreateImageTaskResponse struct {
	TaskID                string   `json:"taskId"`
	Status                string   `json:"status"`
	ImageURL              string   `json:"imageUrl,omitempty"`
	ImageURLs             []string `json:"imageUrls,omitempty"`
	ImageElapsedSeconds   []int64  `json:"imageElapsedSeconds,omitempty"`
	RequestedCount        int      `json:"requestedCount"`
	CostPoints            int64    `json:"costPoints"`
	CostCredits           int64    `json:"costCredits"`
	FreeCreditsCost       int64    `json:"freeCreditsCost"`
	PaidCreditsCost       int64    `json:"paidCreditsCost"`
	CreditSource          string   `json:"creditSource,omitempty"`
	Balance               int64    `json:"balance,omitempty"`
	PointsBalance         int64    `json:"pointsBalance,omitempty"`
	FreeCreditsBalance    int64    `json:"freeCreditsBalance,omitempty"`
	PaidCreditsBalance    int64    `json:"paidCreditsBalance,omitempty"`
	TotalCreditsBalance   int64    `json:"totalCreditsBalance,omitempty"`
	EmailNotifyEnabled    bool     `json:"emailNotifyEnabled"`
	EmailNotifyCostPoints int64    `json:"emailNotifyCostPoints"`
	ErrorMessage          string   `json:"errorMessage,omitempty"`
}

// ImageTaskItem 是任务列表项。
type ImageTaskItem struct {
	ID                    string    `json:"id"`
	RequestedCount        int       `json:"requestedCount"`
	Prompt                string    `json:"prompt"`
	ImageURL              string    `json:"imageUrl,omitempty"`
	ImageURLs             []string  `json:"imageUrls,omitempty"`
	ImageElapsedSeconds   []int64   `json:"imageElapsedSeconds,omitempty"`
	ThumbnailURL          string    `json:"thumbnailUrl,omitempty"`
	ThumbnailURLs         []string  `json:"thumbnailUrls,omitempty"`
	CostPoints            int64     `json:"costPoints"`
	CostCredits           int64     `json:"costCredits"`
	FreeCreditsCost       int64     `json:"freeCreditsCost"`
	PaidCreditsCost       int64     `json:"paidCreditsCost"`
	CreditSource          string    `json:"creditSource"`
	EmailNotifyEnabled    bool      `json:"emailNotifyEnabled"`
	EmailNotifyCostPoints int64     `json:"emailNotifyCostPoints"`
	Status                string    `json:"status"`
	RequestParams         string    `json:"requestParams,omitempty"`
	ErrorMessage          string    `json:"errorMessage,omitempty"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

// ImageTaskDetailResponse 是单个任务详情。
type ImageTaskDetailResponse struct {
	TaskID                string    `json:"taskId"`
	RequestedCount        int       `json:"requestedCount"`
	Prompt                string    `json:"prompt"`
	ImageURL              string    `json:"imageUrl,omitempty"`
	ImageURLs             []string  `json:"imageUrls,omitempty"`
	ImageElapsedSeconds   []int64   `json:"imageElapsedSeconds,omitempty"`
	ThumbnailURL          string    `json:"thumbnailUrl,omitempty"`
	ThumbnailURLs         []string  `json:"thumbnailUrls,omitempty"`
	CostPoints            int64     `json:"costPoints"`
	CostCredits           int64     `json:"costCredits"`
	FreeCreditsCost       int64     `json:"freeCreditsCost"`
	PaidCreditsCost       int64     `json:"paidCreditsCost"`
	CreditSource          string    `json:"creditSource"`
	EmailNotifyEnabled    bool      `json:"emailNotifyEnabled"`
	EmailNotifyCostPoints int64     `json:"emailNotifyCostPoints"`
	Status                string    `json:"status"`
	RequestParams         string    `json:"requestParams,omitempty"`
	ErrorMessage          string    `json:"errorMessage,omitempty"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

// ListImageTasksResponse 是任务列表分页返回体。
type ListImageTasksResponse struct {
	Items    []ImageTaskItem `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	Total    int64           `json:"total"`
}
