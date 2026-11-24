package models

import "time"

type FacebookPage struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID        uint      `gorm:"index;default:0" json:"workspace_id"`
	PageID             string    `gorm:"size:100;uniqueIndex;not null" json:"page_id"`
	Name               string    `gorm:"size:200;not null" json:"name"`
	Category           string    `gorm:"size:120" json:"category"`
	AvatarURL          string    `gorm:"type:text" json:"avatar_url"`
	EncryptedPageToken string    `gorm:"type:text;not null" json:"-"`
	IsConnected        bool      `gorm:"default:true" json:"is_connected"`
	WebhookSubscribed  bool      `gorm:"default:false" json:"webhook_subscribed"`
	UnreadCount        int       `gorm:"default:0" json:"unread_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
