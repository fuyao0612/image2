package model

import "time"

// UserStatus represents whether an account can be used.
type UserStatus int8

// UserRole represents the account permission role.
type UserRole string

const (
	UserStatusNormal   UserStatus = 1
	UserStatusDisabled UserStatus = 2
)

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

// User stores account identity, balances, invite metadata, and sign-in state.
type User struct {
	BaseModel

	Username                 string     `gorm:"type:varchar(32);not null;uniqueIndex:uk_user_username;comment:用户名" json:"username"`
	Email                    string     `gorm:"type:varchar(128);not null;uniqueIndex:uk_user_email;comment:邮箱" json:"email"`
	PasswordHash             string     `gorm:"type:varchar(255);not null;comment:密码哈希" json:"-"`
	TokenVersion             int64      `gorm:"not null;default:0;comment:登录令牌版本" json:"-"`
	InviteCode               string     `gorm:"type:varchar(16);index;comment:邀请码" json:"inviteCode,omitempty"`
	InviterUserID            *uint64    `gorm:"index;comment:邀请人用户ID" json:"inviterUserId,omitempty"`
	PointsBalance            int64      `gorm:"not null;default:0;comment:当前积分余额" json:"pointsBalance"`
	FreeCredits              int64      `gorm:"not null;default:0;comment:生图福利次数余额" json:"freeCredits"`
	PaidCredits              int64      `gorm:"not null;default:0;comment:生图福利次数余额" json:"paidCredits"`
	CheckinStreak            int        `gorm:"not null;default:0;comment:连续签到天数" json:"checkinStreak"`
	LastCheckinAt            *time.Time `gorm:"comment:最后签到时间" json:"lastCheckinAt,omitempty"`
	LastCheckinDate          *time.Time `gorm:"type:date;comment:最后签到日期" json:"-"`
	LastCreditSettlementDate *time.Time `gorm:"type:date;comment:最后一次次数转积分结算日期" json:"-"`
	EmailVerifiedAt          *time.Time `gorm:"comment:邮箱验证时间" json:"emailVerifiedAt,omitempty"`
	LastLoginAt              *time.Time `gorm:"comment:最后登录时间" json:"lastLoginAt,omitempty"`
	LastPasswordResetAt      *time.Time `gorm:"comment:最后重置密码时间" json:"lastPasswordResetAt,omitempty"`
	Status                   UserStatus `gorm:"type:tinyint;not null;default:1;comment:用户状态" json:"status"`
	Role                     UserRole   `gorm:"type:varchar(20);not null;default:user;index;comment:用户角色" json:"role"`
}
