package models

import "time"

// Vai trò hệ thống:
//   - admin:    quản trị hệ thống, theo dõi toàn bộ khách hàng (có thể "xem như khách")
//   - customer: một khách hàng — chủ của một không gian dữ liệu riêng (Fanpage, hộp thư, bài đăng, đối thủ, website...)
//   - staff:    nhân viên của một khách hàng (OwnerID) — làm việc trên dữ liệu của khách hàng đó, theo các trang được cấp
const (
	RoleAdmin    = "admin"
	RoleCustomer = "customer"
	RoleStaff    = "staff"
)

// Các trang / nhóm chức năng có thể bật hoặc tắt cho từng tài khoản
const (
	FeaturePages       = "pages"       // Fanpage: kết nối, quản lý
	FeatureInbox       = "inbox"       // Hộp thư đa Page
	FeatureAI          = "ai"          // CSKH tự động
	FeaturePosts       = "posts"       // Lên lịch đăng bài
	FeatureCompetitors = "competitors" // Phân tích đối thủ
	FeatureBroadcasts  = "broadcasts"  // Chăm sóc khách cũ
	FeatureSites       = "sites"       // Website bán hàng
)

var AllFeatures = []string{FeaturePages, FeatureInbox, FeatureAI, FeaturePosts, FeatureCompetitors, FeatureBroadcasts, FeatureSites}

func IsFeature(f string) bool {
	for _, x := range AllFeatures {
		if x == f {
			return true
		}
	}
	return false
}

type User struct {
	ID           int64    `gorm:"primaryKey" json:"id"`
	FullName     string   `gorm:"size:120;not null" json:"full_name"`
	Email        string   `gorm:"size:150;uniqueIndex;not null" json:"email"`
	PasswordHash string   `gorm:"type:text;default:''" json:"-"`
	AvatarURL    string   `gorm:"type:text;default:''" json:"avatar_url"`
	AuthProvider string   `gorm:"size:20;not null;default:'local'" json:"auth_provider"`
	ProviderID   string   `gorm:"size:128;index;default:''" json:"-"`
	SystemRole   string   `gorm:"size:20;not null;default:'customer'" json:"system_role"` // 'admin' | 'customer' | 'staff'
	ManagerID    *int64   `gorm:"index" json:"manager_id,omitempty"`                      // (cũ, không còn dùng)
	Permissions  []string `gorm:"serializer:json;type:jsonb" json:"permissions"`          // (cũ, không còn dùng)
	IsActive     bool     `gorm:"not null;default:true" json:"is_active"`

	// Phân quyền theo khách hàng
	OwnerID     *int64     `gorm:"index" json:"owner_id,omitempty"`            // nhân viên: id tài khoản khách hàng chủ
	Features    []string   `gorm:"serializer:json;type:jsonb" json:"features"` // các trang được dùng
	MaxPages    int        `gorm:"not null;default:0" json:"max_pages"`        // giới hạn số Fanpage (0 = không giới hạn)
	MaxSites    int        `gorm:"not null;default:0" json:"max_sites"`        // giới hạn số website (0 = không giới hạn)
	MaxStaff    int        `gorm:"not null;default:0" json:"max_staff"`        // giới hạn số nhân viên (0 = không giới hạn)
	LastLoginAt *time.Time `json:"last_login_at"`                              // lần đăng nhập gần nhất
	CreatedAt   time.Time  `json:"created_at"`
}
