package models

import "time"

type CompetitorPage struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"index;default:0;uniqueIndex:idx_comp_ws_page,priority:1" json:"workspace_id"` // khách hàng sở hữu
	Platform    string    `gorm:"size:30;index;default:'facebook'" json:"platform"`                            // facebook | youtube | tiktok
	PageID      string    `gorm:"size:120;uniqueIndex:idx_comp_ws_page,priority:2;not null" json:"page_id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	PageURL     string    `gorm:"type:text" json:"page_url"`
	AvatarURL   string    `gorm:"type:text" json:"avatar_url"`
	Category    string    `gorm:"size:120" json:"category"`
	Notes       string    `gorm:"type:text" json:"notes"`
	LastSynced  time.Time `json:"last_synced"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CompetitorPost struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID      uint      `gorm:"index;default:0;uniqueIndex:idx_cpost_ws_post,priority:1" json:"workspace_id"` // khách hàng sở hữu
	Platform         string    `gorm:"size:30;index;default:'facebook'" json:"platform"`                             // facebook | youtube | tiktok
	CompetitorPageID string    `gorm:"size:120;index;not null" json:"competitor_page_id"`
	CompetitorName   string    `gorm:"size:255" json:"competitor_name"`
	FBPostID         string    `gorm:"size:180;uniqueIndex:idx_cpost_ws_post,priority:2;not null" json:"fb_post_id"`
	Content          string    `gorm:"type:text" json:"content"`
	PermalinkURL     string    `gorm:"type:text" json:"permalink_url"`
	FullPicture      string    `gorm:"type:text" json:"full_picture"`
	ViewsCount       int       `gorm:"default:0" json:"views_count"`
	LikesCount       int       `gorm:"default:0" json:"likes_count"`
	CommentsCount    int       `gorm:"default:0" json:"comments_count"`
	SharesCount      int       `gorm:"default:0" json:"shares_count"`
	PublishedAt      time.Time `gorm:"index" json:"published_at"`
	AIAnalysis       string    `gorm:"type:text" json:"ai_analysis"`
	AIRemixedPost    string    `gorm:"type:text" json:"ai_remixed_post"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
