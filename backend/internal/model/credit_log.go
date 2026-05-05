package model

// CreditChangeType 表示生图次数变动来源。
type CreditChangeType string

const (
	CreditChangeTypeCheckin    CreditChangeType = "checkin"    // 签到赠送次数
	CreditChangeTypeRedeem     CreditChangeType = "redeem"     // 历史积分兑换次数
	CreditChangeTypePurchase   CreditChangeType = "purchase"   // 充值购买次数
	CreditChangeTypeGenerate   CreditChangeType = "generate"   // 生图消耗次数
	CreditChangeTypeRefund     CreditChangeType = "refund"     // 失败退回次数
	CreditChangeTypeSettlement CreditChangeType = "settlement" // 日终次数转积分
	CreditChangeTypeAdjust     CreditChangeType = "adjust"     // 后台人工调整次数
)

// CreditLog 记录次数流水，区分免费次数和充值次数。
type CreditLog struct {
	BaseModel

	UserID     uint64           `gorm:"not null;index;comment:用户ID" json:"userId"`
	ChangeType CreditChangeType `gorm:"type:varchar(32);not null;index;comment:变动类型" json:"changeType"`
	DeltaFree  int64            `gorm:"not null;default:0;comment:免费次数变动，正数为加，负数为减" json:"deltaFree"`
	DeltaPaid  int64            `gorm:"not null;default:0;comment:充值次数变动，正数为加，负数为减" json:"deltaPaid"`
	BeforeFree int64            `gorm:"not null;default:0;comment:变动前免费次数余额" json:"beforeFree"`
	BeforePaid int64            `gorm:"not null;default:0;comment:变动前充值次数余额" json:"beforePaid"`
	AfterFree  int64            `gorm:"not null;default:0;comment:变动后免费次数余额" json:"afterFree"`
	AfterPaid  int64            `gorm:"not null;default:0;comment:变动后充值次数余额" json:"afterPaid"`
	BizType    string           `gorm:"type:varchar(64);index;comment:业务类型" json:"bizType,omitempty"`
	BizID      string           `gorm:"type:varchar(64);index;comment:业务ID" json:"bizId,omitempty"`
	Remark     string           `gorm:"type:varchar(255);comment:备注" json:"remark,omitempty"`
}
