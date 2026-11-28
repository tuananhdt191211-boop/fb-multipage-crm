package handlers

// Phân quyền theo khách hàng — phần dùng chung cho các handler.
//
//   - tenantOf / wsID: không gian dữ liệu (workspace) của yêu cầu hiện tại.
//   - Fanpage là nguồn xác định chủ sở hữu: facebook_pages.workspace_id. Hội thoại, chiến dịch, đăng ký nhận tin, cài đặt AI
//     gắn với page_id nên được lọc theo "các Fanpage của không gian này" (wsPageIDs). Đăng bài, đối thủ, hội thoại có cột workspace_id riêng.
//   - MigrateTenancy chuyển dữ liệu cũ (dùng chung, chưa có chủ) sang tài khoản admin và chuẩn hoá vai trò.

import (
	"net/http"
	"strings"

	"fb-multipage-crm/internal/middleware"
	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func tenantOf(c *gin.Context) middleware.Tenant {
	t, _ := middleware.TenantOf(c)
	return t
}

// wsID: id không gian dữ liệu của yêu cầu. Route chưa qua Tenancy (bài kiểm thử cũ) dùng user_id.
func wsID(c *gin.Context) int64 {
	if t, ok := middleware.TenantOf(c); ok {
		return t.WorkspaceID
	}
	return c.GetInt64("user_id")
}

// wsPageIDs: truy vấn con "page_id của các Fanpage thuộc không gian này"
func wsPageIDs(db *gorm.DB, ws int64) *gorm.DB {
	return db.Model(&models.FacebookPage{}).Select("page_id").Where("workspace_id = ?", ws)
}

// ownedPage lấy Fanpage thuộc không gian dữ liệu (không phải của khách khác)
func ownedPage(db *gorm.DB, ws int64, pageID string) (models.FacebookPage, bool) {
	var p models.FacebookPage
	ok := db.Where("page_id = ? AND workspace_id = ?", strings.TrimSpace(pageID), ws).Limit(1).Find(&p).RowsAffected > 0
	return p, ok
}

// ownedConversation lấy hội thoại thuộc không gian dữ liệu
func ownedConversation(db *gorm.DB, ws int64, id any, dest any) bool {
	return db.Where("id = ? AND workspace_id = ?", id, ws).Limit(1).Find(dest).RowsAffected > 0
}

func notFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{"error": msg})
}

func defaultFeatures() []string { return append([]string{}, models.AllFeatures...) }

// cleanFeatures giữ lại các trang hợp lệ (không trùng); allowed != nil thì chỉ giữ các trang nằm trong allowed
func cleanFeatures(in []string, allowed map[string]bool) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, f := range in {
		f = strings.TrimSpace(f)
		if models.IsFeature(f) && !seen[f] && (allowed == nil || allowed[f]) {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// MigrateTenancy: chạy mỗi lần khởi động (idempotent, vài câu UPDATE nhẹ).
//  1. Chuẩn hoá vai trò: manager / user / staff không có chủ → customer (mỗi tài khoản là một khách hàng).
//  2. Tài khoản chưa có danh sách trang (NULL) → được dùng tất cả (giữ nguyên hành vi cũ); danh sách rỗng = không trang nào.
//  3. Dữ liệu Facebook dùng chung trước đây (workspace_id = 0) → giao cho admin đầu tiên.
func MigrateTenancy(db *gorm.DB) {
	db.Exec(`UPDATE users SET system_role = 'customer' WHERE system_role IN ('manager', 'user', '') OR system_role IS NULL`)
	db.Exec(`UPDATE users SET system_role = 'customer' WHERE system_role = 'staff' AND owner_id IS NULL`)
	db.Exec(`UPDATE users SET features = ? WHERE features IS NULL OR features = 'null'::jsonb`, jsonList(models.AllFeatures))

	var adminID int64
	db.Raw(`SELECT COALESCE(MIN(id), 0) FROM users WHERE system_role = 'admin' AND is_active = TRUE`).Scan(&adminID)
	if adminID == 0 {
		return
	}
	for _, table := range []string{"facebook_pages", "scheduled_posts", "competitor_pages", "competitor_posts"} {
		db.Exec("UPDATE "+table+" SET workspace_id = ? WHERE workspace_id = 0 OR workspace_id IS NULL", adminID)
	}
	// Hội thoại theo chủ của Fanpage; hội thoại mồ côi (Fanpage đã gỡ) giao cho admin để không mất
	db.Exec(`UPDATE conversations c SET workspace_id = p.workspace_id FROM facebook_pages p WHERE c.page_id = p.page_id AND (c.workspace_id = 0 OR c.workspace_id IS NULL)`)
	db.Exec(`UPDATE conversations SET workspace_id = ? WHERE workspace_id = 0 OR workspace_id IS NULL`, adminID)
}

func jsonList(items []string) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, s := range items {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`"` + s + `"`)
	}
	sb.WriteString("]")
	return sb.String()
}
