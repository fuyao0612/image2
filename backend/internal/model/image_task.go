package model

import "time"

// ImageTaskStatus 表示生图任务状态。
type ImageTaskStatus string

const (
	ImageTaskStatusGenerating ImageTaskStatus = "generating"
	ImageTaskStatusSuccess    ImageTaskStatus = "success"
	ImageTaskStatusFailed     ImageTaskStatus = "failed"
)

const (
	ImageTaskEmailNotifyStatusPending = "pending"
	ImageTaskEmailNotifyStatusSending = "sending"
	ImageTaskEmailNotifyStatusSent    = "sent"
	ImageTaskEmailNotifyStatusFailed  = "failed"
)

// ImageTask 记录一次生图请求与其结果。
type ImageTask struct {
	BaseModel

	UserID                uint64          `gorm:"not null;index;comment:用户ID" json:"userId"`
	Prompt                string          `gorm:"type:text;not null;comment:用户提示词" json:"prompt"`
	RequestedCount        int             `gorm:"not null;default:1;comment:请求生成张数" json:"requestedCount"`
	ImageURL              string          `gorm:"type:varchar(1024);comment:生成的图片URL" json:"imageUrl,omitempty"`
	ImageURLs             string          `gorm:"type:text;comment:生成的图片URL列表JSON" json:"imageUrls,omitempty"`
	ImageElapsedSeconds   string          `gorm:"type:text;comment:每张图片真实完成耗时JSON" json:"imageElapsedSeconds,omitempty"`
	CostPoints            int64           `gorm:"not null;default:0;comment:生图消耗积分数" json:"costPoints"`
	CostCredits           int64           `gorm:"not null;default:0;comment:生图消耗福利次数" json:"costCredits"`
	FreeCreditsCost       int64           `gorm:"not null;default:0;comment:本任务消耗的免费福利次数" json:"freeCreditsCost"`
	PaidCreditsCost       int64           `gorm:"not null;default:0;comment:本任务消耗的付费福利次数" json:"paidCreditsCost"`
	CreditSource          string          `gorm:"type:varchar(20);not null;default:free;index;comment:消耗来源" json:"creditSource"`
	Status                ImageTaskStatus `gorm:"type:varchar(20);not null;default:generating;index;comment:状态" json:"status"`
	Provider              string          `gorm:"type:varchar(64);comment:图片服务提供方" json:"provider,omitempty"`
	ProviderKeyID         uint64          `gorm:"not null;default:0;index;comment:生图服务Key ID" json:"providerKeyId,omitempty"`
	ProviderTaskID        string          `gorm:"type:varchar(128);index;comment:外部任务ID" json:"providerTaskId,omitempty"`
	ProviderResponse      string          `gorm:"type:longtext;comment:第三方原始返回" json:"-"`
	ErrorMessage          string          `gorm:"type:varchar(512);comment:失败原因" json:"errorMessage,omitempty"`
	RequestParams         string          `gorm:"type:text;comment:请求参数快照" json:"requestParams,omitempty"`
	EmailNotifyEnabled    bool            `gorm:"not null;default:false;index" json:"emailNotifyEnabled"`
	EmailNotifyCostPoints int64           `gorm:"not null;default:0" json:"emailNotifyCostPoints"`
	EmailNotifyStatus     string          `gorm:"type:varchar(20);not null;default:'';index" json:"emailNotifyStatus,omitempty"`
	EmailNotifySentAt     *time.Time      `json:"emailNotifySentAt,omitempty"`
	EmailNotifyError      string          `gorm:"type:varchar(512)" json:"emailNotifyError,omitempty"`
}
