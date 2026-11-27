package models

import "time"

type Storefront struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	UserID              uint      `gorm:"index" json:"user_id"`
	Domain              string    `gorm:"size:255;index" json:"domain"`
	DomainStatus        string    `gorm:"size:32" json:"domain_status"` // "" | manual | pending | active | error
	DomainError         string    `gorm:"type:text" json:"domain_error"`
	Slug                string    `gorm:"size:120;uniqueIndex" json:"slug"`
	SiteTitle           string    `gorm:"size:255" json:"site_title"`
	LogoURL             string    `gorm:"type:text" json:"logo_url"`
	PrimaryColor        string    `gorm:"size:32" json:"primary_color"`
	DraftBlocksJSON     string    `gorm:"type:text" json:"draft_blocks_json"`
	PublishedBlocksJSON string    `gorm:"type:text" json:"published_blocks_json"`
	IsPublished         bool      `gorm:"default:false" json:"is_published"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
