package model

import "time"

// ImageProviderKeyStatus 表示第三方生图 Key 的使用状态。
type ImageProviderKeyStatus string

const (
	ImageProviderKeyStatusActive     ImageProviderKeyStatus = "active"
	ImageProviderKeyStatusDisabled   ImageProviderKeyStatus = "disabled"
	ImageProviderKeyStatusError      ImageProviderKeyStatus = "error"
	ImageProviderKeyStatusLowBalance ImageProviderKeyStatus = "low_balance"
)

// ImageProviderKey 保存后台可切换的第三方生图 Key。
// APIKey 不对外返回；接口只返回脱敏后的 APIKeyMasked。
type ImageProviderKey struct {
	BaseModel

	Name              string                 `gorm:"type:varchar(64);not null;comment:Key名称" json:"name"`
	Provider          string                 `gorm:"type:varchar(32);not null;default:mentalout;index;comment:服务商标识" json:"provider"`
	BatchServiceURL   string                 `gorm:"type:varchar(255);not null;comment:批任务服务地址" json:"batchServiceUrl"`
	APIBaseURL        string                 `gorm:"type:varchar(255);not null;comment:上游API基础地址" json:"apiBaseUrl"`
	APIKey            string                 `gorm:"type:text;not null;comment:原始API Key" json:"-"`
	APIKeyMasked      string                 `gorm:"type:varchar(80);not null;comment:脱敏API Key" json:"apiKeyMasked"`
	Status            ImageProviderKeyStatus `gorm:"type:varchar(32);not null;default:active;index;comment:状态" json:"status"`
	IsUsing           bool                   `gorm:"not null;default:false;index;comment:当前手动指定使用" json:"isUsing"`
	Priority          int                    `gorm:"not null;default:100;index;comment:优先级，数字越小越优先" json:"priority"`
	AutoSwitchEnabled bool                   `gorm:"not null;default:true;comment:失败时是否参与自动切换" json:"autoSwitchEnabled"`
	SuccessCount      int64                  `gorm:"not null;default:0;comment:成功次数" json:"successCount"`
	FailureCount      int64                  `gorm:"not null;default:0;comment:失败次数" json:"failureCount"`
	LastUsedAt        *time.Time             `gorm:"comment:最近使用时间" json:"lastUsedAt,omitempty"`
	LastError         string                 `gorm:"type:varchar(512);comment:最近错误" json:"lastError,omitempty"`
	CreatedBy         uint64                 `gorm:"not null;default:0;comment:创建管理员ID" json:"createdBy"`
}
