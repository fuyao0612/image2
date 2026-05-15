package dto

import "time"

type PromptPlazaItem struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Prompt          string     `json:"prompt"`
	PreviewURL      string     `json:"previewUrl"`
	PreviewThumbURL string     `json:"previewThumbUrl,omitempty"`
	Tags            []string   `json:"tags"`
	Author          string     `json:"author"`
	Status          string     `json:"status"`
	Favorites       int64      `json:"favorites"`
	IsFavorited     bool       `json:"isFavorited"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	ApprovedAt      *time.Time `json:"approvedAt,omitempty"`
	ReviewedAt      *time.Time `json:"reviewedAt,omitempty"`
	RejectNote      string     `json:"rejectNote,omitempty"`
}

type ListPromptPlazaResponse struct {
	Items []PromptPlazaItem `json:"items"`
}

type SubmitPromptPlazaRequest struct {
	Title      string   `json:"title" binding:"required,max=120"`
	Prompt     string   `json:"prompt" binding:"required,max=8000"`
	PreviewURL string   `json:"previewUrl,omitempty" binding:"omitempty,max=1024"`
	Tags       []string `json:"tags,omitempty"`
}

type DeletePromptPlazaRequest struct {
	Reason string `json:"reason,omitempty" binding:"omitempty,max=255"`
}

type AdminListPromptPlazaResponse struct {
	Items    []PromptPlazaItem `json:"items"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
	Total    int64             `json:"total"`
	Pending  int64             `json:"pending"`
}

type AdminPromptPlazaPayload struct {
	Title      string   `json:"title" binding:"required,max=120"`
	Prompt     string   `json:"prompt" binding:"required,max=8000"`
	PreviewURL string   `json:"previewUrl,omitempty" binding:"omitempty,max=1024"`
	Tags       []string `json:"tags,omitempty"`
	Author     string   `json:"author,omitempty" binding:"omitempty,max=64"`
	Status     string   `json:"status,omitempty" binding:"omitempty,oneof=pending approved rejected"`
}

type AdminReviewPromptPlazaRequest struct {
	Status     string `json:"status" binding:"required,oneof=approved rejected"`
	RejectNote string `json:"rejectNote,omitempty" binding:"omitempty,max=255"`
}
