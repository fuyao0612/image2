package dto

import "time"

// AdminStatsResponse 是后台首页统计。
type AdminStatsResponse struct {
	UsersTotal                      int64                 `json:"usersTotal"`
	UsersDisabled                   int64                 `json:"usersDisabled"`
	UsersCreatedToday               int64                 `json:"usersCreatedToday"`
	TasksActiveUsersToday           int64                 `json:"tasksActiveUsersToday"`
	TasksTotal                      int64                 `json:"tasksTotal"`
	TasksGenerating                 int64                 `json:"tasksGenerating"`
	TasksSuccess                    int64                 `json:"tasksSuccess"`
	TasksFailed                     int64                 `json:"tasksFailed"`
	TasksPointsConsumedTotal        int64                 `json:"tasksPointsConsumedTotal"`
	TasksCreditsConsumedTotal       int64                 `json:"tasksCreditsConsumedTotal"`
	TasksImagesGeneratedTotal       int64                 `json:"tasksImagesGeneratedTotal"`
	TasksPointsConsumedToday        int64                 `json:"tasksPointsConsumedToday"`
	TasksCreditsConsumedToday       int64                 `json:"tasksCreditsConsumedToday"`
	TasksImagesGeneratedToday       int64                 `json:"tasksImagesGeneratedToday"`
	TasksPointsConsumedYesterday    int64                 `json:"tasksPointsConsumedYesterday"`
	TasksCreditsConsumedYesterday   int64                 `json:"tasksCreditsConsumedYesterday"`
	TasksImagesGeneratedYesterday   int64                 `json:"tasksImagesGeneratedYesterday"`
	AuditRejected                   int64                 `json:"auditRejected"`
	PointsBalanceTotal              int64                 `json:"pointsBalanceTotal"`
	PointsConvertedCreditsTotal     int64                 `json:"pointsConvertedCreditsTotal"`
	FreeCreditsTotal                int64                 `json:"freeCreditsTotal"`
	PaidCreditsTotal                int64                 `json:"paidCreditsTotal"`
	CreditsBalanceTotal             int64                 `json:"creditsBalanceTotal"`
	ResourceCreditsTotal            int64                 `json:"resourceCreditsTotal"`
	PointToCreditRate               int64                 `json:"pointToCreditRate"`
	RedeemCodesActive               int64                 `json:"redeemCodesActive"`
	RedeemCodesUsed                 int64                 `json:"redeemCodesUsed"`
	RedeemCodesDisabled             int64                 `json:"redeemCodesDisabled"`
	RedeemCodeCreditsTotal          int64                 `json:"redeemCodeCreditsTotal"`
	RedeemCodePointsTotal           int64                 `json:"redeemCodePointsTotal"`
	RedeemCodeActivePointsTotal     int64                 `json:"redeemCodeActivePointsTotal"`
	RedeemCodeUsedPointsTotal       int64                 `json:"redeemCodeUsedPointsTotal"`
	RedeemCodeConvertedCreditsTotal int64                 `json:"redeemCodeConvertedCreditsTotal"`
	TaskTrend24h                    []AdminTaskTrendPoint `json:"taskTrend24h"`
	TaskTrend3d                     []AdminTaskTrendPoint `json:"taskTrend3d"`
	TaskTrend7d                     []AdminTaskTrendPoint `json:"taskTrend7d"`
}

// AdminTaskTrendPoint 是后台概览任务趋势点。
type AdminTaskTrendPoint struct {
	Label   string `json:"label"`
	Success int64  `json:"success"`
	Failed  int64  `json:"failed"`
}

// AdminUserItem 是后台用户列表项。
type AdminUserItem struct {
	ID              string     `json:"id"`
	Username        string     `json:"username"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	Status          int8       `json:"status"`
	PointsBalance   int64      `json:"pointsBalance"`
	FreeCredits     int64      `json:"freeCredits"`
	PaidCredits     int64      `json:"paidCredits"`
	TotalCredits    int64      `json:"totalCredits"`
	ConsumedPoints  int64      `json:"consumedPoints"`
	ConsumedCredits int64      `json:"consumedCredits"`
	CheckinStreak   int        `json:"checkinStreak"`
	LastLoginAt     *time.Time `json:"lastLoginAt,omitempty"`
	LastCheckinAt   *time.Time `json:"lastCheckinAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
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
	ID                    string    `json:"id"`
	UserID                string    `json:"userId"`
	Username              string    `json:"username,omitempty"`
	Prompt                string    `json:"prompt"`
	ImageURL              string    `json:"imageUrl,omitempty"`
	ImageURLs             []string  `json:"imageUrls,omitempty"`
	ThumbnailURL          string    `json:"thumbnailUrl,omitempty"`
	ThumbnailURLs         []string  `json:"thumbnailUrls,omitempty"`
	CostPoints            int64     `json:"costPoints"`
	CostCredits           int64     `json:"costCredits"`
	FreeCreditsCost       int64     `json:"freeCreditsCost"`
	PaidCreditsCost       int64     `json:"paidCreditsCost"`
	EmailNotifyEnabled    bool      `json:"emailNotifyEnabled"`
	EmailNotifyCostPoints int64     `json:"emailNotifyCostPoints"`
	CreditSource          string    `json:"creditSource"`
	Status                string    `json:"status"`
	Provider              string    `json:"provider,omitempty"`
	RequestParams         string    `json:"requestParams,omitempty"`
	ErrorMessage          string    `json:"errorMessage,omitempty"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

// AdminListImageTasksResponse 是后台任务分页。
type AdminListImageTasksResponse struct {
	Items    []AdminImageTaskItem `json:"items"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
	Total    int64                `json:"total"`
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
	ID          string    `json:"id"`
	AdminID     string    `json:"adminId"`
	UserID      string    `json:"userId,omitempty"`
	ActorLabel  string    `json:"actorLabel,omitempty"`
	Action      string    `json:"action"`
	RecordType  string    `json:"recordType,omitempty"`
	TargetType  string    `json:"targetType"`
	TargetID    string    `json:"targetId"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status,omitempty"`
	BeforeJSON  string    `json:"beforeJson,omitempty"`
	AfterJSON   string    `json:"afterJson,omitempty"`
	Remark      string    `json:"remark,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// AdminListActionLogsResponse 是后台操作审计日志分页。
type AdminListActionLogsResponse struct {
	Items    []AdminActionLogItem `json:"items"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
	Total    int64                `json:"total"`
}

// DataCleanupRequest 描述管理员要清理的数据范围。
type DataCleanupRequest struct {
	Type      string `json:"type" binding:"required,oneof=failed_image_tasks generated_images user_reference_images temporary_keys action_logs email_failure_notifications successful_image_tasks balance_logs user_activity announcement_reads rejected_prompt_plaza"`
	Mode      string `json:"mode,omitempty" binding:"omitempty,oneof=retention date days_ago"`
	KeepDays  int    `json:"keepDays,omitempty" binding:"omitempty,min=0,max=3650"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	DaysAgo   int    `json:"daysAgo,omitempty" binding:"omitempty,min=1,max=3650"`
}

// DataCleanupTableStat 表示单张表的清理统计。
type DataCleanupTableStat struct {
	Table string `json:"table"`
	Count int64  `json:"count"`
	Bytes int64  `json:"bytes,omitempty"`
	Note  string `json:"note,omitempty"`
}

// DataCleanupResponse 表示数据清理预览或执行结果。
type DataCleanupResponse struct {
	Mode           string                 `json:"mode"`
	Type           string                 `json:"type"`
	TypeLabel      string                 `json:"typeLabel"`
	TargetDate     string                 `json:"targetDate"`
	KeepDays       int                    `json:"keepDays"`
	StartedAt      time.Time              `json:"startedAt"`
	FinishedAt     *time.Time             `json:"finishedAt,omitempty"`
	DryRun         bool                   `json:"dryRun"`
	TableStats     []DataCleanupTableStat `json:"tableStats"`
	FilesRemoved   int64                  `json:"filesRemoved"`
	BytesRemoved   int64                  `json:"bytesRemoved"`
	RecordsRemoved int64                  `json:"recordsRemoved"`
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
	ProductURL    string    `json:"productUrl,omitempty"`
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
	CreditsAmount int64  `json:"creditsAmount" binding:"omitempty,min=0"`
	PointsCost    int64  `json:"pointsCost" binding:"omitempty,min=0"`
	AmountCents   int64  `json:"amountCents" binding:"omitempty,min=0"`
	BonusPoints   int64  `json:"bonusPoints" binding:"min=0"`
	ProductURL    string `json:"productUrl" binding:"omitempty,max=512"`
	Enabled       *bool  `json:"enabled,omitempty"`
	SortOrder     int    `json:"sortOrder"`
	Note          string `json:"note" binding:"omitempty,max=255"`
}

// AdminUpdatePackageRequest 是后台更新套餐请求。
type AdminUpdatePackageRequest struct {
	Name          string `json:"name" binding:"omitempty,min=1,max=128"`
	CreditsAmount int64  `json:"creditsAmount" binding:"omitempty,min=0"`
	PointsCost    int64  `json:"pointsCost" binding:"omitempty,min=0"`
	AmountCents   int64  `json:"amountCents" binding:"omitempty,min=0"`
	BonusPoints   int64  `json:"bonusPoints" binding:"omitempty,min=0"`
	ProductURL    string `json:"productUrl" binding:"omitempty,max=512"`
	Enabled       *bool  `json:"enabled,omitempty"`
	SortOrder     *int   `json:"sortOrder,omitempty"`
	Note          string `json:"note" binding:"omitempty,max=255"`
}

// AdminProviderKeyItem 是后台生图 Key 池列表项。
type AdminProviderKeyItem struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Provider          string     `json:"provider"`
	BatchServiceURL   string     `json:"batchServiceUrl"`
	APIBaseURL        string     `json:"apiBaseUrl"`
	APIKeyMasked      string     `json:"apiKeyMasked"`
	Status            string     `json:"status"`
	IsUsing           bool       `json:"isUsing"`
	Priority          int        `json:"priority"`
	AutoSwitchEnabled bool       `json:"autoSwitchEnabled"`
	SuccessCount      int64      `json:"successCount"`
	FailureCount      int64      `json:"failureCount"`
	LastUsedAt        *time.Time `json:"lastUsedAt,omitempty"`
	LastError         string     `json:"lastError,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// AdminListProviderKeysResponse 是后台生图 Key 池分页响应。
type AdminListProviderKeysResponse struct {
	Items    []AdminProviderKeyItem `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
	Total    int64                  `json:"total"`
}

// AdminProviderKeyPayload 是新增或编辑生图 Key 的请求。
type AdminProviderKeyPayload struct {
	Name              string `json:"name" binding:"required,min=1,max=64"`
	Provider          string `json:"provider" binding:"omitempty,max=32"`
	BatchServiceURL   string `json:"batchServiceUrl" binding:"omitempty,max=255"`
	APIBaseURL        string `json:"apiBaseUrl" binding:"omitempty,max=255"`
	APIKey            string `json:"apiKey" binding:"omitempty,max=512"`
	Status            string `json:"status" binding:"omitempty,oneof=active disabled error low_balance"`
	Priority          int    `json:"priority"`
	AutoSwitchEnabled *bool  `json:"autoSwitchEnabled,omitempty"`
}
