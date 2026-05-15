package model

import "time"

type PromptPlazaFavorite struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement;comment:主键ID" json:"id"`
	UserID            uint64    `gorm:"not null;uniqueIndex:idx_prompt_plaza_favorite_user_item;index;comment:用户ID" json:"userId"`
	PromptPlazaItemID uint64    `gorm:"not null;uniqueIndex:idx_prompt_plaza_favorite_user_item;index;comment:提示词ID" json:"promptPlazaItemId"`
	CreatedAt         time.Time `gorm:"comment:创建时间" json:"createdAt"`
}
