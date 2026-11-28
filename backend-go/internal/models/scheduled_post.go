package models

import "time"

type ScheduledPost struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint      `gorm:"index;default:0" json:"workspace_id"` // khách hàng sở hữu bài đăng
	Content        string    `gorm:"type:text;not null" json:"content"`
	MediaType      string    `gorm:"size:20;default:'none'" json:"media_type"` // none | image | video
	MediaURL       string    `gorm:"type:text" json:"media_url"`
	PageIDsJSON    string    `gorm:"type:text;not null" json:"page_ids_json"` // JSON array: ["page_id_1", "page_id_2"]
	PageNamesJSON  string    `gorm:"type:text" json:"page_names_json"`
	ScheduledAt    time.Time `gorm:"index;not null" json:"scheduled_at"`
	Status         string    `gorm:"size:20;index;default:'pending'" json:"status"` // pending | processing | published | partial | failed
	SuccessCount   int       `gorm:"default:0" json:"success_count"`
	FailedCount    int       `gorm:"default:0" json:"failed_count"`
	PublishResults string    `gorm:"type:text" json:"publish_results"` // JSON log chi tiết từng Page
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
