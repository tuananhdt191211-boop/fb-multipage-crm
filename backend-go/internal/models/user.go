package models

import "time"

type User struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	FullName     string    `gorm:"size:120;not null" json:"full_name"`
	Email        string    `gorm:"size:150;uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"type:text;default:''" json:"-"`
	AvatarURL    string    `gorm:"type:text;default:''" json:"avatar_url"`
	AuthProvider string    `gorm:"size:20;not null;default:'local'" json:"auth_provider"`
	ProviderID   string    `gorm:"size:128;index;default:''" json:"-"`
	SystemRole   string    `gorm:"size:20;not null;default:'staff'" json:"system_role"` // 'admin' | 'manager' | 'staff'
	ManagerID    *int64    `gorm:"index" json:"manager_id,omitempty"`                   // Quản lý trực tiếp của nhân viên (nếu có)
	Permissions  []string  `gorm:"serializer:json;type:jsonb" json:"permissions"`       // VD: ["inbox.reply", "orders.manage", "posts.schedule"]
	IsActive     bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}
