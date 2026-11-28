package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ProductMedia là một ảnh / video trong bộ sưu tập của sản phẩm (phần tử đầu tiên là ảnh bìa).
type ProductMedia struct {
	Type   string `json:"type"`             // image | video | embed (YouTube / Vimeo)
	URL    string `json:"url"`              // ảnh / file video / địa chỉ nhúng
	Poster string `json:"poster,omitempty"` // ảnh bìa của video
	Alt    string `json:"alt,omitempty"`
}

// ProductOption là một nhóm phân loại của sản phẩm (vd. "Màu" với các giá trị "Đen", "Trắng"). Tối đa 3 nhóm.
type ProductOption struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// ProductVariant là một tổ hợp phân loại cụ thể (vd. "Đen / L") với giá, tồn kho, SKU và ảnh riêng.
// Sản phẩm có biến thể thì giá / tồn kho khi đặt hàng lấy từ biến thể; Product.Price và Product.Stock chỉ là số tổng hợp
// (giá thấp nhất, tổng tồn kho) để hiển thị trong danh sách.
type ProductVariant struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ProductID    uint      `gorm:"index;not null" json:"product_id"`
	StorefrontID uint      `gorm:"index;not null" json:"-"`
	Position     int       `json:"position"`
	Title        string    `gorm:"size:255" json:"title"` // "Đen / L"
	Option1      string    `gorm:"size:100" json:"option1"`
	Option2      string    `gorm:"size:100" json:"option2"`
	Option3      string    `gorm:"size:100" json:"option3"`
	SKU          string    `gorm:"size:100" json:"sku"`
	Barcode      string    `gorm:"size:100" json:"barcode"`
	Price        int64     `json:"price"`         // VND
	ComparePrice int64     `json:"compare_price"` // giá gốc gạch ngang, 0 = không có
	CostPrice    int64     `json:"cost_price"`    // giá nhập — chỉ chủ shop thấy
	Stock        int       `json:"stock"`         // -1 = không giới hạn, 0 = hết hàng
	ImageURL     string    `gorm:"type:text" json:"image_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// OptionValues trả về giá trị của từng nhóm theo đúng thứ tự (chỉ lấy n nhóm đầu).
func (v ProductVariant) OptionValues(n int) []string {
	all := []string{v.Option1, v.Option2, v.Option3}
	if n < 0 {
		n = 0
	}
	if n > 3 {
		n = 3
	}
	return all[:n]
}

type Product struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	StorefrontID uint   `gorm:"index;not null" json:"storefront_id"`
	UserID       uint   `gorm:"index;not null" json:"user_id"`
	Name         string `gorm:"size:255;not null" json:"name"`
	Description  string `gorm:"type:text" json:"description"` // HTML đã lọc (bản cũ có thể là văn bản thuần)
	Price        int64  `json:"price"`                        // VND
	ComparePrice int64  `json:"compare_price"`                // giá gốc gạch ngang, 0 = không có
	ImageURL     string `gorm:"type:text" json:"image_url"`   // ảnh bìa = ảnh đầu tiên của Media
	Stock        int    `json:"stock"`                        // -1 = không giới hạn, 0 = hết hàng
	IsActive     bool   `json:"is_active"`
	SortOrder    int    `json:"sort_order"`

	MediaJSON string         `gorm:"type:text" json:"-"`
	Media     []ProductMedia `gorm:"-" json:"media"`
	SKU       string         `gorm:"size:100" json:"sku"`
	Barcode   string         `gorm:"size:100" json:"barcode"`
	CostPrice int64          `json:"cost_price"` // giá nhập — chỉ chủ shop thấy, không bao giờ công khai
	SourceURL string         `gorm:"type:text" json:"source_url"`
	Tags      string         `gorm:"type:text" json:"tags"` // phân cách bằng dấu phẩy

	OptionsJSON string           `gorm:"type:text" json:"-"`
	Options     []ProductOption  `gorm:"-" json:"options"`  // rỗng = sản phẩm không có biến thể
	Variants    []ProductVariant `gorm:"-" json:"variants"` // chỉ có khi handler nạp riêng (xem loadVariants)

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AfterFind dựng Media từ MediaJSON. Sản phẩm cũ (chưa có bộ sưu tập) dùng ImageURL làm ảnh duy nhất.
func (p *Product) AfterFind(_ *gorm.DB) error {
	p.Media = nil
	if p.MediaJSON != "" {
		_ = json.Unmarshal([]byte(p.MediaJSON), &p.Media)
	}
	if len(p.Media) == 0 && p.ImageURL != "" {
		p.Media = []ProductMedia{{Type: "image", URL: p.ImageURL}}
	}
	if p.Media == nil {
		p.Media = []ProductMedia{}
	}
	p.Options = nil
	if p.OptionsJSON != "" {
		_ = json.Unmarshal([]byte(p.OptionsJSON), &p.Options)
	}
	if p.Options == nil {
		p.Options = []ProductOption{}
	}
	if p.Variants == nil {
		p.Variants = []ProductVariant{}
	}
	return nil
}
