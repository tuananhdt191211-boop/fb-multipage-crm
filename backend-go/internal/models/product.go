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
	return nil
}
