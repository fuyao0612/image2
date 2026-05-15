package model

// CreditChangeType describes why image credits changed.
type CreditChangeType string

const (
	CreditChangeTypeCheckin    CreditChangeType = "checkin"
	CreditChangeTypeRedeem     CreditChangeType = "redeem"
	CreditChangeTypePurchase   CreditChangeType = "purchase"
	CreditChangeTypeGenerate   CreditChangeType = "generate"
	CreditChangeTypeRefund     CreditChangeType = "refund"
	CreditChangeTypeSettlement CreditChangeType = "settlement"
	CreditChangeTypeAdjust     CreditChangeType = "adjust"
	CreditChangeTypeRegister   CreditChangeType = "register"
	CreditChangeTypeCode       CreditChangeType = "redeem_code"
	CreditChangeTypeInvite     CreditChangeType = "invite"
)

// CreditLog records free and paid credit balance changes.
type CreditLog struct {
	BaseModel

	UserID     uint64           `gorm:"not null;index;comment:用户ID" json:"userId"`
	ChangeType CreditChangeType `gorm:"type:varchar(32);not null;index;comment:变动类型" json:"changeType"`
	DeltaFree  int64            `gorm:"not null;default:0;comment:福利次数变动" json:"deltaFree"`
	DeltaPaid  int64            `gorm:"not null;default:0;comment:福利次数变动" json:"deltaPaid"`
	BeforeFree int64            `gorm:"not null;default:0;comment:变动前福利次数余额" json:"beforeFree"`
	BeforePaid int64            `gorm:"not null;default:0;comment:变动前福利次数余额" json:"beforePaid"`
	AfterFree  int64            `gorm:"not null;default:0;comment:变动后福利次数余额" json:"afterFree"`
	AfterPaid  int64            `gorm:"not null;default:0;comment:变动后福利次数余额" json:"afterPaid"`
	BizType    string           `gorm:"type:varchar(64);index;comment:业务类型" json:"bizType,omitempty"`
	BizID      string           `gorm:"type:varchar(64);index;comment:业务ID" json:"bizId,omitempty"`
	Remark     string           `gorm:"type:varchar(255);comment:备注" json:"remark,omitempty"`
}
