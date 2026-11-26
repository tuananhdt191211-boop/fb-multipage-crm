package models

import "time"

type Product struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	StorefrontID uint      `gorm:"index;not null" json:"storefront_id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	Name         string    `gorm:"size:255;not null" json:"name"`
	Description  string    `gorm:"type:text" json:"description"`
	Price        int64     `json:"price"`         // VND
	ComparePrice int64     `json:"compare_price"` // giá gốc gạch ngang, 0 = không có
	ImageURL     string    `gorm:"type:text" json:"image_url"`
	Stock        int       `json:"stock"` // -1 = không giới hạn, 0 = hết hàng
	IsActive     bool      `json:"is_active"`
	SortOrder    int       `json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
