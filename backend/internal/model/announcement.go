package model

import "time"

type AnnouncementKind string

const (
	AnnouncementKindPinned   AnnouncementKind = "pinned"
	AnnouncementKindNormal   AnnouncementKind = "normal"
	AnnouncementKindPersonal AnnouncementKind = "personal"
)

type Announcement struct {
	BaseModel

	Title        string           `gorm:"type:varchar(128);not null;comment:公告标题" json:"title"`
	Content      string           `gorm:"type:longtext;not null;comment:公告内容" json:"content"`
	ImageURL     string           `gorm:"type:varchar(1024);default:'';comment:公告底部图片URL" json:"imageUrl"`
	Kind         AnnouncementKind `gorm:"type:varchar(20);not null;default:normal;index;comment:公告类型" json:"kind"`
	Enabled      bool             `gorm:"not null;default:true;index;comment:是否启用" json:"enabled"`
	SortOrder    int              `gorm:"not null;default:0;index;comment:排序值" json:"sortOrder"`
	TargetUserID uint64           `gorm:"not null;default:0;index;comment:个人公告目标用户ID，0表示全体" json:"targetUserId"`
	CreatedBy    uint64           `gorm:"not null;index;comment:创建管理员ID" json:"createdBy"`
	UpdatedBy    uint64           `gorm:"not null;index;comment:更新管理员ID" json:"updatedBy"`
}

type AnnouncementRead struct {
	BaseModel

	UserID                    uint64    `gorm:"not null;uniqueIndex:uk_announcement_read_user_notice;comment:用户ID" json:"userId"`
	AnnouncementID            uint64    `gorm:"not null;uniqueIndex:uk_announcement_read_user_notice;comment:公告ID" json:"announcementId"`
	AnnouncementUpdatedAtRead time.Time `gorm:"not null;comment:已读时公告更新时间" json:"announcementUpdatedAtRead"`
	ReadAt                    time.Time `gorm:"not null;comment:已读时间" json:"readAt"`
}
