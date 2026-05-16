package model

// PointsChangeType describes why points changed.
type PointsChangeType string

const (
	PointsChangeTypeCheckin       PointsChangeType = "checkin"
	PointsChangeTypePurchase      PointsChangeType = "purchase"
	PointsChangeTypePurchaseBonus PointsChangeType = "purchase_bonus"
	PointsChangeTypeRedeem        PointsChangeType = "redeem"
	PointsChangeTypeSettlement    PointsChangeType = "settlement"
	PointsChangeTypeGenerate      PointsChangeType = "generate"
	PointsChangeTypeRefund        PointsChangeType = "refund"
	PointsChangeTypeAdjust        PointsChangeType = "adjust"
	PointsChangeTypeRegister      PointsChangeType = "register"
	PointsChangeTypeCode          PointsChangeType = "redeem_code"
	PointsChangeTypeInvite        PointsChangeType = "invite"
)

// PointsLog records point balance changes.
type PointsLog struct {
	BaseModel

	UserID        uint64           `gorm:"not null;index;comment:用户ID" json:"userId"`
	ChangeType    PointsChangeType `gorm:"type:varchar(32);not null;index;comment:变动类型" json:"changeType"`
	Delta         int64            `gorm:"not null;comment:变动数量" json:"delta"`
	BeforeBalance int64            `gorm:"not null;comment:变动前余额" json:"beforeBalance"`
	AfterBalance  int64            `gorm:"not null;comment:变动后余额" json:"afterBalance"`
	BizType       string           `gorm:"type:varchar(64);index;comment:业务类型" json:"bizType,omitempty"`
	BizID         string           `gorm:"type:varchar(64);index;comment:业务ID" json:"bizId,omitempty"`
	Remark        string           `gorm:"type:varchar(255);comment:备注" json:"remark,omitempty"`
}
