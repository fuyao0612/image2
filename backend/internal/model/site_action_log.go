package model

// SiteActionLog records important user and system actions for the admin log center.
type SiteActionLog struct {
	BaseModel

	ActorType   string `gorm:"type:varchar(20);not null;index;comment:操作者类型" json:"actorType"`
	ActorID     uint64 `gorm:"not null;default:0;index;comment:操作者ID" json:"actorId"`
	Action      string `gorm:"type:varchar(64);not null;index;comment:操作动作" json:"action"`
	TargetType  string `gorm:"type:varchar(64);not null;index;comment:目标类型" json:"targetType"`
	TargetID    string `gorm:"type:varchar(64);not null;default:'';index;comment:目标ID" json:"targetId"`
	Title       string `gorm:"type:varchar(128);not null;default:'';comment:中文标题" json:"title"`
	Description string `gorm:"type:varchar(255);not null;default:'';comment:简要描述" json:"description"`
	Status      string `gorm:"type:varchar(32);not null;default:'';comment:状态" json:"status"`
	DetailJSON  string `gorm:"type:longtext;comment:详情JSON" json:"detailJson,omitempty"`
}
