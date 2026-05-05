package model

import "time"

// ContentAuditStatus 表示内容审核结果。
type ContentAuditStatus string

const (
	ContentAuditStatusPass     ContentAuditStatus = "pass"     // 通过
	ContentAuditStatusRejected ContentAuditStatus = "rejected" // 拒绝
)

// ContentAuditLog 记录用户提交内容的审核结果，方便后台追踪和人工复盘。
type ContentAuditLog struct {
	BaseModel

	UserID          uint64             `gorm:"not null;index;comment:用户ID" json:"userId"`
	BizType         string             `gorm:"type:varchar(64);not null;index;comment:业务类型" json:"bizType"`
	BizID           string             `gorm:"type:varchar(64);index;comment:业务ID" json:"bizId,omitempty"`
	ContentType     string             `gorm:"type:varchar(32);not null;comment:内容类型" json:"contentType"`
	Content         string             `gorm:"type:text;not null;comment:审核内容" json:"content"`
	Status          ContentAuditStatus `gorm:"type:varchar(20);not null;index;comment:审核状态" json:"status"`
	MatchedKeywords string             `gorm:"type:varchar(512);comment:命中的关键词JSON" json:"matchedKeywords,omitempty"`
	Reason          string             `gorm:"type:varchar(255);comment:审核原因" json:"reason,omitempty"`
	Action          string             `gorm:"type:varchar(32);not null;default:allow;comment:处理动作" json:"action"`
	ReviewerUserID  uint64             `gorm:"index;comment:人工审核人ID" json:"reviewerUserId,omitempty"`
	ReviewedAt      *time.Time         `gorm:"comment:人工审核时间" json:"reviewedAt,omitempty"`
}
