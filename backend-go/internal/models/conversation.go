package models

import "time"

type Conversation struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID     uint       `gorm:"index;default:0" json:"workspace_id"`
	PageID          string     `gorm:"size:100;index;not null" json:"page_id"`
	PageName        string     `gorm:"size:200" json:"page_name"`
	ThreadID        string     `gorm:"size:120;uniqueIndex;not null" json:"thread_id"`
	CustomerPSID    string     `gorm:"size:120;index;not null" json:"customer_psid"`
	CustomerName    string     `gorm:"size:200;not null" json:"customer_name"`
	CustomerAvatar  string     `gorm:"type:text" json:"customer_avatar"`
	LastMessage     string     `gorm:"type:text" json:"last_message"`
	LastMessageTime time.Time  `gorm:"index" json:"last_message_time"`
	UnreadCount     int        `gorm:"default:0" json:"unread_count"`
	AssignedUserID  *uint      `gorm:"index" json:"assigned_user_id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LeadStatus      string     `gorm:"type:varchar(30);default:'new'" json:"lead_status"` // new | hesitant | closed | follow_up
	CustomerPhone   string     `gorm:"type:varchar(30)" json:"customer_phone"`
	CustomerAddress string     `gorm:"type:text" json:"customer_address"`
	CareNote        string     `gorm:"type:text" json:"care_note"`
	NextFollowUpAt  *time.Time `json:"next_follow_up_at"`
}

type Message struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ConversationID uint      `gorm:"index;not null" json:"conversation_id"`
	FBMessageID    string    `gorm:"size:150;uniqueIndex" json:"fb_message_id"`
	SenderType     string    `gorm:"size:20;not null" json:"sender_type"` // customer | page
	SenderName     string    `gorm:"size:200" json:"sender_name"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	AttachmentURL  string    `gorm:"type:text" json:"attachment_url"`
	AttachmentType string    `gorm:"size:30" json:"attachment_type"` // image | file
	SentAt         time.Time `gorm:"index" json:"sent_at"`
	CreatedAt      time.Time `json:"created_at"`
}
