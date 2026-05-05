package model

import "time"

// RechargeOrderStatus 表示充值订单状态。
type RechargeOrderStatus string

const (
	RechargeOrderStatusPending  RechargeOrderStatus = "pending"  // 待支付
	RechargeOrderStatusPaid     RechargeOrderStatus = "paid"     // 已支付
	RechargeOrderStatusCanceled RechargeOrderStatus = "canceled" // 已取消
	RechargeOrderStatusRefunded RechargeOrderStatus = "refunded" // 已退款
)

// RechargeOrder 记录一次充值行为，支持自定义充值和套餐充值。
type RechargeOrder struct {
	BaseModel

	UserID          uint64              `gorm:"not null;index;comment:用户ID" json:"userId"`
	OrderNo         string              `gorm:"type:varchar(64);not null;uniqueIndex;comment:订单号" json:"orderNo"`
	RechargeType    string              `gorm:"type:varchar(20);not null;default:package;index;comment:充值类型：package/custom" json:"rechargeType"`
	PackageCode     string              `gorm:"type:varchar(64);not null;index;comment:套餐编码" json:"packageCode"`
	PackageName     string              `gorm:"type:varchar(128);not null;comment:套餐名称" json:"packageName"`
	AmountCents     int64               `gorm:"not null;comment:订单金额，单位分" json:"amountCents"`
	PointsAmount    int64               `gorm:"not null;default:0;comment:历史兼容字段，保留积分数量" json:"pointsAmount"`
	CreditsAmount   int64               `gorm:"not null;default:0;comment:到账生图次数" json:"creditsAmount"`
	BonusPoints     int64               `gorm:"not null;default:0;comment:额外赠送积分" json:"bonusPoints"`
	Status          RechargeOrderStatus `gorm:"type:varchar(20);not null;default:pending;index;comment:订单状态" json:"status"`
	PaidAt          *time.Time          `gorm:"comment:支付完成时间" json:"paidAt,omitempty"`
	PaymentChannel  string              `gorm:"type:varchar(32);comment:支付渠道" json:"paymentChannel,omitempty"`
	ProviderTradeNo string              `gorm:"type:varchar(128);index;comment:第三方交易号" json:"providerTradeNo,omitempty"`
	Remark          string              `gorm:"type:varchar(255);comment:备注" json:"remark,omitempty"`
}
