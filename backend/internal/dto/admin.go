package dto

import "time"

// AdminStatsResponse 是后台首页统计。
type AdminStatsResponse struct {
	UsersTotal         int64 `json:"usersTotal"`
	UsersDisabled      int64 `json:"usersDisabled"`
	TasksTotal         int64 `json:"tasksTotal"`
	TasksGenerating    int64 `json:"tasksGenerating"`
	TasksSuccess       int64 `json:"tasksSuccess"`
	TasksFailed        int64 `json:"tasksFailed"`
	OrdersPaid         int64 `json:"ordersPaid"`
	AuditRejected      int64 `json:"auditRejected"`
	PointsBalanceTotal int64 `json:"pointsBalanceTotal"`
	FreeCreditsTotal   int64 `json:"freeCreditsTotal"`
	PaidCreditsTotal   int64 `json:"paidCreditsTotal"`
}

// AdminUserItem 是后台用户列表项。
type AdminUserItem struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	Status        int8       `json:"status"`
	PointsBalance int64      `json:"pointsBalance"`
	FreeCredits   int64      `json:"freeCredits"`
	PaidCredits   int64      `json:"paidCredits"`
	TotalCredits  int64      `json:"totalCredits"`
	CheckinStreak int        `json:"checkinStreak"`
	LastLoginAt   *time.Time `json:"lastLoginAt,omitempty"`
	LastCheckinAt *time.Time `json:"lastCheckinAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// AdminListUsersResponse 是后台用户分页。
type AdminListUsersResponse struct {
	Items    []AdminUserItem `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	Total    int64           `json:"total"`
}

// AdminUpdateUserStatusRequest 是后台更新用户状态请求。
type AdminUpdateUserStatusRequest struct {
	Status int8 `json:"status" binding:"required,oneof=1 2"`
}

// AdminAdjustPointsRequest 是后台人工调整积分请求。
type AdminAdjustPointsRequest struct {
	Delta  int64  `json:"delta" binding:"required"`
	Remark string `json:"remark" binding:"omitempty,max=255"`
}

// AdminAdjustPointsResponse 是后台调积分结果。
type AdminAdjustPointsResponse struct {
	UserID        string `json:"userId"`
	Delta         int64  `json:"delta"`
	BeforeBalance int64  `json:"beforeBalance"`
	AfterBalance  int64  `json:"afterBalance"`
}

// AdminImageTaskItem 是后台任务列表项。
type AdminImageTaskItem struct {
	ID              string    `json:"id"`
	UserID          string    `json:"userId"`
	Prompt          string    `json:"prompt"`
	ImageURL        string    `json:"imageUrl,omitempty"`
	ImageURLs       []string  `json:"imageUrls,omitempty"`
	CostPoints      int64     `json:"costPoints"`
	CostCredits     int64     `json:"costCredits"`
	FreeCreditsCost int64     `json:"freeCreditsCost"`
	PaidCreditsCost int64     `json:"paidCreditsCost"`
	CreditSource    string    `json:"creditSource"`
	Status          string    `json:"status"`
	Provider        string    `json:"provider,omitempty"`
	RequestParams   string    `json:"requestParams,omitempty"`
	ErrorMessage    string    `json:"errorMessage,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// AdminListImageTasksResponse 是后台任务分页。
type AdminListImageTasksResponse struct {
	Items    []AdminImageTaskItem `json:"items"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
	Total    int64                `json:"total"`
}

// AdminRechargeOrderItem 是后台订单列表项。
type AdminRechargeOrderItem struct {
	ID              string     `json:"id"`
	UserID          string     `json:"userId"`
	OrderNo         string     `json:"orderNo"`
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

// AdminListRechargeOrdersResponse 是后台订单分页。
type AdminListRechargeOrdersResponse struct {
	Items    []AdminRechargeOrderItem `json:"items"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
	Total    int64                    `json:"total"`
}

// AdminContentAuditItem 是后台内容审核日志列表项。
type AdminContentAuditItem struct {
	ID              string     `json:"id"`
	UserID          string     `json:"userId"`
	BizType         string     `json:"bizType"`
	BizID           string     `json:"bizId,omitempty"`
	ContentType     string     `json:"contentType"`
	Content         string     `json:"content"`
	Status          string     `json:"status"`
	MatchedKeywords []string   `json:"matchedKeywords,omitempty"`
	Reason          string     `json:"reason,omitempty"`
	Action          string     `json:"action"`
	ReviewerUserID  string     `json:"reviewerUserId,omitempty"`
	ReviewedAt      *time.Time `json:"reviewedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// AdminListContentAuditsResponse 是后台内容审核日志分页。
type AdminListContentAuditsResponse struct {
	Items    []AdminContentAuditItem `json:"items"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"pageSize"`
	Total    int64                   `json:"total"`
}

// AdminReviewContentAuditRequest 是内容审核人工复审请求。
type AdminReviewContentAuditRequest struct {
	Status string `json:"status" binding:"required,oneof=pass rejected"`
	Action string `json:"action" binding:"omitempty,oneof=allow reject"`
	Reason string `json:"reason" binding:"omitempty,max=255"`
}

// AdminActionLogItem 是后台操作审计日志项。
type AdminActionLogItem struct {
	ID         string    `json:"id"`
	AdminID    string    `json:"adminId"`
	Action     string    `json:"action"`
	TargetType string    `json:"targetType"`
	TargetID   string    `json:"targetId"`
	BeforeJSON string    `json:"beforeJson,omitempty"`
	AfterJSON  string    `json:"afterJson,omitempty"`
	Remark     string    `json:"remark,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// AdminListActionLogsResponse 是后台操作审计日志分页。
type AdminListActionLogsResponse struct {
	Items    []AdminActionLogItem `json:"items"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
	Total    int64                `json:"total"`
}

// AdminPackageItem 是后台套餐配置项。
type AdminPackageItem struct {
	ID            string    `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	PackageType   string    `json:"packageType"`
	CreditsAmount int64     `json:"creditsAmount"`
	PointsCost    int64     `json:"pointsCost"`
	AmountCents   int64     `json:"amountCents"`
	BonusPoints   int64     `json:"bonusPoints"`
	Enabled       bool      `json:"enabled"`
	SortOrder     int       `json:"sortOrder"`
	Note          string    `json:"note,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// AdminListPackagesResponse 是后台套餐分页。
type AdminListPackagesResponse struct {
	Items    []AdminPackageItem `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int64              `json:"total"`
}

// AdminCreatePackageRequest 是后台创建套餐请求。
type AdminCreatePackageRequest struct {
	Code          string `json:"code" binding:"required,min=2,max=64"`
	Name          string `json:"name" binding:"required,min=1,max=128"`
	PackageType   string `json:"packageType" binding:"omitempty,oneof=recharge"`
	CreditsAmount int64  `json:"creditsAmount" binding:"required,min=1"`
	PointsCost    int64  `json:"pointsCost" binding:"omitempty,min=0"`
	AmountCents   int64  `json:"amountCents" binding:"omitempty,min=0"`
	BonusPoints   int64  `json:"bonusPoints" binding:"omitempty,min=0"`
	Enabled       *bool  `json:"enabled,omitempty"`
	SortOrder     int    `json:"sortOrder"`
	Note          string `json:"note" binding:"omitempty,max=255"`
}

// AdminUpdatePackageRequest 是后台更新套餐请求。
type AdminUpdatePackageRequest struct {
	Name          string `json:"name" binding:"omitempty,min=1,max=128"`
	CreditsAmount int64  `json:"creditsAmount" binding:"omitempty,min=1"`
	PointsCost    int64  `json:"pointsCost" binding:"omitempty,min=0"`
	AmountCents   int64  `json:"amountCents" binding:"omitempty,min=0"`
	BonusPoints   int64  `json:"bonusPoints" binding:"omitempty,min=0"`
	Enabled       *bool  `json:"enabled,omitempty"`
	SortOrder     *int   `json:"sortOrder,omitempty"`
	Note          string `json:"note" binding:"omitempty,max=255"`
}
