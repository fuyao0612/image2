package dto

// PackageItem 表示可购买的商品套餐。
type PackageItem struct {
	Code             string `json:"code"`
	Name             string `json:"name"`
	PackageType      string `json:"packageType"`
	CreditsAmount    int64  `json:"creditsAmount"`
	PointsCost       int64  `json:"pointsCost"`
	AmountCents      int64  `json:"amountCents"`
	BonusPoints      int64  `json:"bonusPoints"`
	ProductURL       string `json:"productUrl,omitempty"`
	Enabled          bool   `json:"enabled"`
	SortOrder        int    `json:"sortOrder"`
	Note             string `json:"note,omitempty"`
}

// ListPackagesResponse 是套餐列表返回体。
type ListPackagesResponse struct {
	Items []PackageItem `json:"items"`
}
