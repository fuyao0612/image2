package model

import "time"

const (
	PromptPlazaStatusPending  = "pending"
	PromptPlazaStatusApproved = "approved"
	PromptPlazaStatusRejected = "rejected"
)

type PromptPlazaItem struct {
	BaseModel
	Title      string     `gorm:"size:120;not null;comment:标题"`
	Prompt     string     `gorm:"type:text;not null;comment:提示词"`
	PreviewURL string     `gorm:"size:1024;comment:缩略图地址"`
	Tags       string     `gorm:"type:text;comment:标签JSON"`
	UserID     uint64     `gorm:"index;not null;comment:上传用户ID"`
	Author     string     `gorm:"size:64;not null;comment:上传人"`
	Status     string     `gorm:"size:20;index;not null;default:pending;comment:状态"`
	Favorites  int64      `gorm:"not null;default:0;comment:收藏数"`
	ApprovedAt *time.Time `gorm:"index;comment:通过审核时间"`
	ReviewedBy uint64     `gorm:"comment:审核管理员ID"`
	ReviewedAt *time.Time `gorm:"comment:审核时间"`
	RejectNote string     `gorm:"size:255;comment:驳回原因"`
}
