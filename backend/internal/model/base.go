package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 为大部分业务表提供通用字段。
type BaseModel struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement;comment:主键ID" json:"id"`
	CreatedAt time.Time      `gorm:"comment:创建时间" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"comment:更新时间" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:删除时间" json:"-"`
}
