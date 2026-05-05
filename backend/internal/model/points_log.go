package model

// PointsChangeType 表示积分变动来源。
type PointsChangeType string

const (
	PointsChangeTypeCheckin    PointsChangeType = "checkin"    // 签到
	PointsChangeTypePurchase   PointsChangeType = "purchase"   // 充值入账
	PointsChangeTypePurchaseBonus PointsChangeType = "purchase_bonus" // 充值赠送
	PointsChangeTypeRedeem     PointsChangeType = "redeem"     // 历史兑换
	PointsChangeTypeSettlement PointsChangeType = "settlement" // 日终次数转积分
	PointsChangeTypeGenerate   PointsChangeType = "generate"   // 生图消耗
	PointsChangeTypeRefund     PointsChangeType = "refund"     // 失败退回
	PointsChangeTypeAdjust     PointsChangeType = "adjust"     // 人工调整
)

// PointsLog 记录积分流水。
type PointsLog struct {
	BaseModel

	UserID        uint64           `gorm:"not null;index;comment:用户ID" json:"userId"`
	ChangeType    PointsChangeType `gorm:"type:varchar(32);not null;index;comment:变动类型" json:"changeType"`
	Delta         int64            `gorm:"not null;comment:变动数量，正数为加，负数为减" json:"delta"`
	BeforeBalance int64            `gorm:"not null;comment:变动前余额" json:"beforeBalance"`
	AfterBalance  int64            `gorm:"not null;comment:变动后余额" json:"afterBalance"`
	BizType       string           `gorm:"type:varchar(64);index;comment:业务类型" json:"bizType,omitempty"`
	BizID         string           `gorm:"type:varchar(64);index;comment:业务ID" json:"bizId,omitempty"`
	Remark        string           `gorm:"type:varchar(255);comment:备注" json:"remark,omitempty"`
}
