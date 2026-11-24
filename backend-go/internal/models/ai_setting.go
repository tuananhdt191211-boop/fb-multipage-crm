package models

import "time"

type PageAISetting struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	PageID       string    `gorm:"size:100;uniqueIndex;not null" json:"page_id"`
	Enabled      bool      `gorm:"default:true" json:"enabled"`
	AutoReply    bool      `gorm:"default:false" json:"auto_reply"`
	ProductTag   string    `gorm:"size:80" json:"product_tag"`                 // Tên sản phẩm đặc trưng (VD: Micro Hamax, Hạt Điều...)
	TagColor     string    `gorm:"size:30;default:'emerald'" json:"tag_color"` // emerald, blue, amber, rose, purple, indigo
	QuickReplies string    `gorm:"type:text" json:"quick_replies"`             // JSON mảng các tin nhắn mẫu của riêng Page
	Tone         string    `gorm:"size:255;default:'Thân thiện, nhiệt tình, xưng em - anh/chị'" json:"tone"`
	SystemPrompt string    `gorm:"type:text" json:"system_prompt"`
	UpdatedAt    time.Time `json:"updated_at"`
}
