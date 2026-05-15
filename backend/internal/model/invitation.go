package model

// Invitation records one successful invite relationship and the granted reward.
type Invitation struct {
	BaseModel

	InviterUserID uint64 `gorm:"not null;index;comment:邀请人用户ID" json:"inviterUserId"`
	InviteeUserID uint64 `gorm:"not null;uniqueIndex:uk_invitation_invitee;comment:被邀请人用户ID" json:"inviteeUserId"`
	InviteCode    string `gorm:"type:varchar(16);not null;index;comment:注册时使用的邀请码" json:"inviteCode"`
	RewardPoints  int64  `gorm:"not null;default:0;comment:奖励积分" json:"rewardPoints"`
	RewardCredits int64  `gorm:"not null;default:0;comment:奖励福利次数" json:"rewardCredits"`
}
