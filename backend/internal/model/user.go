package model

import "time"

// UserStatus 表示用户状态。
type UserStatus int8

// UserRole 表示用户角色。
type UserRole string

const (
	UserStatusNormal   UserStatus = 1 // 正常
	UserStatusDisabled UserStatus = 2 // 已禁用
)

const (
	UserRoleUser  UserRole = "user"  // 普通用户
	UserRoleAdmin UserRole = "admin" // 管理员
)

// User 是平台用户表，保存账号身份、积分、次数和签到状态。
type User struct {
	BaseModel

	Username                 string     `gorm:"type:varchar(32);not null;uniqueIndex:uk_user_username;comment:用户名" json:"username"`
	Email                    string     `gorm:"type:varchar(128);not null;uniqueIndex:uk_user_email;comment:邮箱" json:"email"`
	PasswordHash             string     `gorm:"type:varchar(255);not null;comment:密码哈希" json:"-"`
	PointsBalance            int64      `gorm:"not null;default:0;comment:当前总积分" json:"pointsBalance"`
	FreeCredits              int64      `gorm:"not null;default:0;comment:免费生图次数余额" json:"freeCredits"`
	PaidCredits              int64      `gorm:"not null;default:0;comment:充值生图次数余额" json:"paidCredits"`
	CheckinStreak            int        `gorm:"not null;default:0;comment:连续签到天数" json:"checkinStreak"`
	LastCheckinAt            *time.Time `gorm:"comment:最后签到时间" json:"lastCheckinAt,omitempty"`
	LastCheckinDate          *time.Time `gorm:"type:date;comment:最后签到日期" json:"-"`
	LastCreditSettlementDate *time.Time `gorm:"type:date;comment:最后一次次数转积分结算日期" json:"-"`
	EmailVerifiedAt          *time.Time `gorm:"comment:邮箱验证时间" json:"emailVerifiedAt,omitempty"`
	LastLoginAt              *time.Time `gorm:"comment:最后登录时间" json:"lastLoginAt,omitempty"`
	Status                   UserStatus  `gorm:"type:tinyint;not null;default:1;comment:用户状态" json:"status"`
	Role                     UserRole    `gorm:"type:varchar(20);not null;default:user;index;comment:用户角色" json:"role"`
}
