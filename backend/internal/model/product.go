package model

type ProductCategory struct {
	BaseModel
	Name        string `gorm:"size:64;not null;uniqueIndex;comment:分类名称"`
	Description string `gorm:"size:255;comment:分类描述"`
	SortOrder   int    `gorm:"not null;default:0;comment:排序"`
	Enabled     bool   `gorm:"not null;default:true;comment:是否启用"`
}

type ProductItem struct {
	BaseModel
	Name             string            `gorm:"size:120;not null;comment:商品名称"`
	ShortDescription string            `gorm:"size:500;comment:简短说明"`
	Description      string            `gorm:"type:text;comment:详细说明"`
	PreviewURL       string            `gorm:"size:1024;comment:缩略图地址"`
	AmountCents      int64             `gorm:"not null;default:0;comment:商品价格，单位分"`
	ProductURL       string            `gorm:"size:1024;not null;comment:外部购买链接"`
	SortOrder        int               `gorm:"not null;default:0;comment:排序"`
	Enabled          bool              `gorm:"not null;default:true;comment:是否启用"`
	Categories       []ProductCategory `gorm:"many2many:product_item_categories;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type ProductItemCategory struct {
	ProductItemID     uint64 `gorm:"primaryKey;autoIncrement:false;comment:商品ID"`
	ProductCategoryID uint64 `gorm:"primaryKey;autoIncrement:false;comment:分类ID"`
}
