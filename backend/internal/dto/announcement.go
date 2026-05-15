package dto

import "time"

type AnnouncementItem struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Content      string     `json:"content"`
	ImageURL     string     `json:"imageUrl,omitempty"`
	Kind         string     `json:"kind"`
	Enabled      bool       `json:"enabled"`
	SortOrder    int        `json:"sortOrder"`
	TargetUserID string     `json:"targetUserId,omitempty"`
	IsRead       bool       `json:"isRead"`
	ReadAt       *time.Time `json:"readAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type ListAnnouncementsResponse struct {
	Items []AnnouncementItem `json:"items"`
}

type MarkAnnouncementReadResponse struct {
	ID     string    `json:"id"`
	IsRead bool      `json:"isRead"`
	ReadAt time.Time `json:"readAt"`
}

type AdminListAnnouncementsResponse struct {
	Items    []AnnouncementItem `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int64              `json:"total"`
}

type AdminCreateAnnouncementRequest struct {
	Title        string `json:"title" binding:"required,min=1,max=128"`
	Content      string `json:"content" binding:"required,min=1,max=10000"`
	ImageURL     string `json:"imageUrl,omitempty" binding:"omitempty,max=1024"`
	Kind         string `json:"kind" binding:"required,oneof=pinned normal personal"`
	TargetUserID string `json:"targetUserId,omitempty" binding:"omitempty"`
	Enabled      *bool  `json:"enabled,omitempty"`
	SortOrder    int    `json:"sortOrder"`
}

type AdminUpdateAnnouncementRequest struct {
	Title        string `json:"title" binding:"omitempty,min=1,max=128"`
	Content      string `json:"content" binding:"omitempty,min=1,max=10000"`
	ImageURL     string `json:"imageUrl,omitempty" binding:"omitempty,max=1024"`
	Kind         string `json:"kind" binding:"omitempty,oneof=pinned normal personal"`
	TargetUserID string `json:"targetUserId,omitempty" binding:"omitempty"`
	Enabled      *bool  `json:"enabled,omitempty"`
	SortOrder    *int   `json:"sortOrder,omitempty"`
}
