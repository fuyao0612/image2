package model

// AdminActionLog 记录后台管理员对关键配置的操作留痕。
type AdminActionLog struct {
	BaseModel

	AdminID    uint64 `gorm:"not null;index;comment:管理员ID" json:"adminId"`
	Action     string `gorm:"type:varchar(64);not null;index;comment:操作动作" json:"action"`
	TargetType string `gorm:"type:varchar(64);not null;index;comment:目标类型" json:"targetType"`
	TargetID   string `gorm:"type:varchar(64);not null;index;comment:目标ID" json:"targetId"`
	BeforeJSON string `gorm:"type:longtext;comment:变更前内容JSON" json:"beforeJson,omitempty"`
	AfterJSON  string `gorm:"type:longtext;comment:变更后内容JSON" json:"afterJson,omitempty"`
	Remark     string `gorm:"type:varchar(255);comment:备注" json:"remark,omitempty"`
}
