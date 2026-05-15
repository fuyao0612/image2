package model

import "time"

type RedeemCodeStatus string

const (
	RedeemCodeStatusActive   RedeemCodeStatus = "active"
	RedeemCodeStatusUsed     RedeemCodeStatus = "used"
	RedeemCodeStatusDisabled RedeemCodeStatus = "disabled"
)

type RedeemCode struct {
	BaseModel

	Code          string           `gorm:"type:varchar(64);not null;uniqueIndex:uk_redeem_code;comment:兑换码" json:"code"`
	PointsAmount  int64            `gorm:"not null;default:0;comment:包含积分" json:"pointsAmount"`
	CreditsAmount int64            `gorm:"not null;default:0;comment:包含福利次数" json:"creditsAmount"`
	Status        RedeemCodeStatus `gorm:"type:varchar(20);not null;default:active;index;comment:状态" json:"status"`
	BatchNo       string           `gorm:"type:varchar(64);index;comment:生成批次" json:"batchNo,omitempty"`
	Note          string           `gorm:"type:varchar(255);comment:备注" json:"note,omitempty"`
	CreatedBy     uint64           `gorm:"not null;default:0;index;comment:创建管理员ID" json:"createdBy"`
	UsedBy        uint64           `gorm:"index;comment:兑换用户ID" json:"usedBy,omitempty"`
	UsedAt        *time.Time       `gorm:"comment:兑换时间" json:"usedAt,omitempty"`
	ExpiresAt     *time.Time       `gorm:"comment:过期时间" json:"expiresAt,omitempty"`
}
