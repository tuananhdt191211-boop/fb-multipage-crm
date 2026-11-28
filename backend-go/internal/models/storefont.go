package models

import "time"

type Storefront struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	UserID       uint   `gorm:"index" json:"user_id"`
	Domain       string `gorm:"size:255;index" json:"domain"`
	DomainStatus string `gorm:"size:32" json:"domain_status"` // "" | manual | pending | active | error
	DomainError  string `gorm:"type:text" json:"domain_error"`
	Slug         string `gorm:"size:120;uniqueIndex" json:"slug"`
	SiteTitle    string `gorm:"size:255" json:"site_title"`
	LogoURL      string `gorm:"type:text" json:"logo_url"`
	PrimaryColor string `gorm:"size:32" json:"primary_color"`
	// Trang chủ dạng "ghép sẵn" (phần dùng chung + nội dung trang chủ). Giữ để giao diện / API bản cũ vẫn chạy;
	// nguồn dữ liệu thật là bảng storefront_pages + các cột *_shared_json bên dưới.
	DraftBlocksJSON     string `gorm:"type:text" json:"draft_blocks_json"`
	PublishedBlocksJSON string `gorm:"type:text" json:"published_blocks_json"`
	// Phần dùng chung của mọi trang: dải thông báo, đầu trang (header), chân trang (footer)
	DraftSharedJSON     string `gorm:"type:text" json:"draft_shared_json"`
	PublishedSharedJSON string `gorm:"type:text" json:"published_shared_json"`
	// Menu điều hướng quản lý riêng (đầu trang / chân trang chọn menu theo id) và cài đặt SEO mặc định của cả website
	DraftMenusJSON     string    `gorm:"type:text" json:"draft_menus_json"`
	PublishedMenusJSON string    `gorm:"type:text" json:"published_menus_json"`
	DraftSEOJSON       string    `gorm:"type:text" json:"draft_seo_json"`
	PublishedSEOJSON   string    `gorm:"type:text" json:"published_seo_json"`
	DraftThemeJSON     string    `gorm:"type:text" json:"draft_theme_json"` // kiểu chữ toàn trang (font, cỡ H1..P)
	PublishedThemeJSON string    `gorm:"type:text" json:"published_theme_json"`
	DraftI18nJSON      string    `gorm:"type:text" json:"draft_i18n_json"` // thị trường + bản dịch (nhiều ngôn ngữ)
	PublishedI18nJSON  string    `gorm:"type:text" json:"published_i18n_json"`
	IsPublished        bool      `gorm:"default:false" json:"is_published"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Các loại trang
const (
	PageKindHome   = "home"   // trang chủ (mỗi website đúng một trang, không xoá được)
	PageKindPage   = "page"   // trang thường (giới thiệu, liên hệ...)
	PageKindPolicy = "policy" // trang chính sách (đổi trả, vận chuyển, bảo mật, điều khoản...)
)

// StorefrontPage là một trang của website. Nội dung chỉ gồm các section của riêng trang đó;
// dải thông báo / header / footer nằm ở Storefront.*SharedJSON và hiện trên mọi trang.
type StorefrontPage struct {
	ID                  uint   `gorm:"primaryKey" json:"id"`
	StorefrontID        uint   `gorm:"not null;uniqueIndex:idx_page_site_slug,priority:1;index" json:"storefront_id"`
	Slug                string `gorm:"size:80;not null;uniqueIndex:idx_page_site_slug,priority:2" json:"slug"` // đường dẫn: /pages/<slug>
	Title               string `gorm:"size:160;not null" json:"title"`
	Kind                string `gorm:"size:12;not null;default:'page'" json:"kind"`
	Position            int    `gorm:"not null;default:0" json:"position"`
	IsVisible           bool   `gorm:"not null;default:true" json:"is_visible"` // ẩn = khách không mở được
	DraftBlocksJSON     string `gorm:"type:text" json:"draft_blocks_json"`
	PublishedBlocksJSON string `gorm:"type:text" json:"published_blocks_json"`
	PublishedTitle      string `gorm:"size:160" json:"published_title"` // tên / đường dẫn / trạng thái đã xuất bản (bản nháp có thể đã đổi)
	PublishedSlug       string `gorm:"size:80" json:"published_slug"`
	PublishedVisible    bool   `gorm:"not null;default:false" json:"published_visible"`
	// Cài đặt riêng của trang: SEO (tiêu đề, mô tả, ảnh chia sẻ, noindex) và landing page (ẩn đầu / chân trang) — JSON
	DraftSettingsJSON     string    `gorm:"type:text" json:"draft_settings_json"`
	PublishedSettingsJSON string    `gorm:"type:text" json:"published_settings_json"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
