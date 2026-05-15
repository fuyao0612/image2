package dto

import "time"

type ProductCategoryItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SortOrder   int    `json:"sortOrder"`
	Enabled     bool   `json:"enabled"`
}

type ProductItem struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	ShortDescription string                `json:"shortDescription,omitempty"`
	Description      string                `json:"description,omitempty"`
	PreviewURL       string                `json:"previewUrl,omitempty"`
	PreviewThumbURL  string                `json:"previewThumbUrl,omitempty"`
	AmountCents      int64                 `json:"amountCents"`
	ProductURL       string                `json:"productUrl,omitempty"`
	SortOrder        int                   `json:"sortOrder"`
	Enabled          bool                  `json:"enabled"`
	Categories       []ProductCategoryItem `json:"categories"`
	CreatedAt        time.Time             `json:"createdAt"`
	UpdatedAt        time.Time             `json:"updatedAt"`
}

type ListProductsResponse struct {
	Items []ProductItem `json:"items"`
}

type ListProductCategoriesResponse struct {
	Items []ProductCategoryItem `json:"items"`
}

type AdminListProductsResponse struct {
	Items    []ProductItem `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
	Total    int64         `json:"total"`
}

type AdminListProductCategoriesResponse struct {
	Items    []ProductCategoryItem `json:"items"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
	Total    int64                 `json:"total"`
}

type AdminProductPayload struct {
	Name             string   `json:"name" binding:"required,max=120"`
	ShortDescription string   `json:"shortDescription,omitempty" binding:"omitempty,max=500"`
	Description      string   `json:"description,omitempty"`
	PreviewURL       string   `json:"previewUrl,omitempty" binding:"omitempty,max=1024"`
	AmountCents      int64    `json:"amountCents" binding:"required,min=1"`
	ProductURL       string   `json:"productUrl,omitempty" binding:"omitempty,max=1024"`
	CategoryIDs      []string `json:"categoryIds,omitempty"`
	SortOrder        int      `json:"sortOrder"`
	Enabled          *bool    `json:"enabled,omitempty"`
}

type AdminProductCategoryPayload struct {
	Name        string `json:"name" binding:"required,max=64"`
	Description string `json:"description,omitempty" binding:"omitempty,max=255"`
	SortOrder   int    `json:"sortOrder"`
	Enabled     *bool  `json:"enabled,omitempty"`
}
