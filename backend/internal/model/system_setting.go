package model

// SystemSetting 表示平台可动态调整的系统参数。
type SystemSetting struct {
	BaseModel

	Key         string `gorm:"type:varchar(128);not null;uniqueIndex;comment:配置键" json:"key"`
	Value       string `gorm:"type:longtext;not null;comment:配置值" json:"value"`
	Description string `gorm:"type:varchar(255);comment:配置说明" json:"description,omitempty"`
}
