package model

// BillingPackageType 表示套餐用途。
type BillingPackageType string

const (
	BillingPackageTypeRecharge BillingPackageType = "recharge" // 付费购买生图次数
	BillingPackageTypeRedeem   BillingPackageType = "redeem"   // 积分兑换免费生图次数
)

// BillingPackage 是后台可配置的套餐档位。
type BillingPackage struct {
	BaseModel

	Code          string             `gorm:"type:varchar(64);not null;uniqueIndex;comment:套餐编码" json:"code"`
	Name          string             `gorm:"type:varchar(128);not null;comment:套餐名称" json:"name"`
	PackageType   BillingPackageType `gorm:"type:varchar(20);not null;index;comment:套餐类型" json:"packageType"`
	CreditsAmount int64              `gorm:"not null;default:0;comment:到账生图次数" json:"creditsAmount"`
	PointsCost    int64              `gorm:"not null;default:0;comment:兑换所需积分，仅积分兑换套餐使用" json:"pointsCost"`
	AmountCents   int64              `gorm:"not null;default:0;comment:付款金额，单位分，仅充值套餐使用" json:"amountCents"`
	BonusPoints   int64              `gorm:"not null;default:0;comment:充值额外赠送积分" json:"bonusPoints"`
	Enabled       bool               `gorm:"not null;default:true;index;comment:是否启用" json:"enabled"`
	SortOrder     int                `gorm:"not null;default:0;index;comment:排序值" json:"sortOrder"`
	Note          string             `gorm:"type:varchar(255);comment:备注" json:"note,omitempty"`
}
