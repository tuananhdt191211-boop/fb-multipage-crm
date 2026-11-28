package handlers

// Quản lý khách hàng (admin) và nhân viên (khách hàng).
//
//   Admin  GET   /admin/overview                    số liệu toàn hệ thống
//          GET   /admin/customers?q=                danh sách khách hàng kèm số liệu
//          POST  /admin/customers                   tạo tài khoản khách hàng
//          GET   /admin/customers/:id               chi tiết (Fanpage, website, nhân viên)
//          PATCH /admin/customers/:id               khoá / mở, trang được dùng, giới hạn
//          POST  /admin/customers/:id/password      đặt lại mật khẩu
//   Khách  GET/POST /team, PATCH/DELETE /team/:id, POST /team/:id/password   nhân viên của mình
//
// "Xem như khách": admin gửi header X-Workspace = id khách hàng trên các API thường (chỉ đọc) — xem middleware.Tenancy.

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"fb-multipage-crm/internal/middleware"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CustomerHandler struct {
	DB *gorm.DB
}

func NewCustomerHandler(db *gorm.DB) *CustomerHandler { return &CustomerHandler{DB: db} }

type customerStats struct {
	Pages         int64 `json:"pages"`
	Conversations int64 `json:"conversations"`
	Unread        int64 `json:"unread"`
	Sites         int64 `json:"sites"`
	PublishedSite int64 `json:"published_sites"`
	Orders        int64 `json:"orders"`
	Revenue       int64 `json:"revenue"`
	Posts         int64 `json:"posts"`
	Staff         int64 `json:"staff"`
}

type customerRow struct {
	models.User
	Stats customerStats `json:"stats"`
}

// statsByWorkspace gom số liệu của mọi khách hàng bằng vài truy vấn GROUP BY (không truy vấn từng khách)
func (h *CustomerHandler) statsByWorkspace() map[int64]*customerStats {
	out := map[int64]*customerStats{}
	get := func(id int64) *customerStats {
		s := out[id]
		if s == nil {
			s = &customerStats{}
			out[id] = s
		}
		return s
	}
	var rows []struct {
		WorkspaceID int64
		N           int64
		M           int64
	}
	h.DB.Raw(`SELECT workspace_id, COUNT(*) AS n FROM facebook_pages GROUP BY workspace_id`).Scan(&rows)
	for _, r := range rows {
		get(r.WorkspaceID).Pages = r.N
	}
	rows = nil
	h.DB.Raw(`SELECT workspace_id, COUNT(*) AS n, COALESCE(SUM(unread_count), 0) AS m FROM conversations GROUP BY workspace_id`).Scan(&rows)
	for _, r := range rows {
		s := get(r.WorkspaceID)
		s.Conversations, s.Unread = r.N, r.M
	}
	rows = nil
	h.DB.Raw(`SELECT workspace_id, COUNT(*) AS n FROM scheduled_posts GROUP BY workspace_id`).Scan(&rows)
	for _, r := range rows {
		get(r.WorkspaceID).Posts = r.N
	}
	var sites []struct {
		UserID    int64
		N         int64
		Published int64
	}
	h.DB.Raw(`SELECT user_id, COUNT(*) AS n, COUNT(*) FILTER (WHERE is_published) AS published FROM storefronts GROUP BY user_id`).Scan(&sites)
	for _, r := range sites {
		s := get(r.UserID)
		s.Sites, s.PublishedSite = r.N, r.Published
	}
	var orders []struct {
		UserID  int64
		N       int64
		Revenue int64
	}
	h.DB.Raw(`SELECT user_id, COUNT(*) AS n, COALESCE(SUM(total), 0) AS revenue FROM shop_orders WHERE status <> 'cancelled' GROUP BY user_id`).Scan(&orders)
	for _, r := range orders {
		s := get(r.UserID)
		s.Orders, s.Revenue = r.N, r.Revenue
	}
	var staff []struct {
		OwnerID int64
		N       int64
	}
	h.DB.Raw(`SELECT owner_id, COUNT(*) AS n FROM users WHERE owner_id IS NOT NULL GROUP BY owner_id`).Scan(&staff)
	for _, r := range staff {
		get(r.OwnerID).Staff = r.N
	}
	return out
}

// AdminOverview GET /api/v1/admin/overview
func (h *CustomerHandler) AdminOverview(c *gin.Context) {
	var customers, active, newWeek, staff int64
	h.DB.Model(&models.User{}).Where("system_role = ?", models.RoleCustomer).Count(&customers)
	h.DB.Model(&models.User{}).Where("system_role = ? AND is_active = TRUE", models.RoleCustomer).Count(&active)
	h.DB.Model(&models.User{}).Where("system_role = ? AND created_at >= ?", models.RoleCustomer, time.Now().AddDate(0, 0, -7)).Count(&newWeek)
	h.DB.Model(&models.User{}).Where("system_role = ?", models.RoleStaff).Count(&staff)
	var total customerStats
	for _, s := range h.statsByWorkspace() {
		total.Pages += s.Pages
		total.Conversations += s.Conversations
		total.Unread += s.Unread
		total.Sites += s.Sites
		total.PublishedSite += s.PublishedSite
		total.Orders += s.Orders
		total.Revenue += s.Revenue
		total.Posts += s.Posts
	}
	var activeWeek int64
	h.DB.Model(&models.User{}).Where("system_role = ? AND last_login_at >= ?", models.RoleCustomer, time.Now().AddDate(0, 0, -7)).Count(&activeWeek)
	c.JSON(http.StatusOK, gin.H{
		"customers": customers, "active_customers": active, "new_customers_7d": newWeek, "active_logins_7d": activeWeek,
		"staff": staff, "totals": total,
	})
}

// ListCustomers GET /api/v1/admin/customers?q=
func (h *CustomerHandler) ListCustomers(c *gin.Context) {
	q := h.DB.Where("system_role = ?", models.RoleCustomer)
	if s := strings.TrimSpace(c.Query("q")); s != "" {
		like := "%" + s + "%"
		q = q.Where("full_name ILIKE ? OR email ILIKE ?", like, like)
	}
	var users []models.User
	q.Order("id DESC").Limit(500).Find(&users)
	stats := h.statsByWorkspace()
	rows := make([]customerRow, 0, len(users))
	for _, u := range users {
		row := customerRow{User: u}
		if s := stats[u.ID]; s != nil {
			row.Stats = *s
		}
		rows = append(rows, row)
	}
	c.JSON(http.StatusOK, gin.H{"customers": rows, "all_features": models.AllFeatures})
}

func (h *CustomerHandler) targetUser(c *gin.Context) (models.User, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var u models.User
	if err != nil || id <= 0 || h.DB.Where("id = ?", id).Limit(1).Find(&u).RowsAffected == 0 {
		notFound(c, "Không tìm thấy khách hàng.")
		return u, false
	}
	if u.SystemRole == models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Không thể thay đổi tài khoản admin ở đây."})
		return u, false
	}
	return u, true
}

type pageBrief struct {
	ID          uint   `json:"id"`
	PageID      string `json:"page_id"`
	Name        string `json:"name"`
	IsConnected bool   `json:"is_connected"`
}

type siteBrief struct {
	ID          uint   `json:"id"`
	Slug        string `json:"slug"`
	SiteTitle   string `json:"site_title"`
	IsPublished bool   `json:"is_published"`
}

// GetCustomer GET /api/v1/admin/customers/:id
func (h *CustomerHandler) GetCustomer(c *gin.Context) {
	u, ok := h.targetUser(c)
	if !ok {
		return
	}
	ws := u.ID
	if u.SystemRole == models.RoleStaff && u.OwnerID != nil {
		ws = *u.OwnerID
	}
	var owner models.User
	h.DB.Where("id = ?", ws).Limit(1).Find(&owner)
	// Luôn trả mảng (không phải null) để giao diện không lỗi khi danh sách trống
	staff := []models.User{}
	h.DB.Where("owner_id = ?", ws).Order("id ASC").Find(&staff)
	pages := []pageBrief{}
	h.DB.Model(&models.FacebookPage{}).Select("id, page_id, name, is_connected").Where("workspace_id = ?", ws).Order("name ASC").Scan(&pages)
	sites := []siteBrief{}
	h.DB.Model(&models.Storefront{}).Select("id, slug, site_title, is_published").Where("user_id = ?", ws).Order("id ASC").Scan(&sites)
	row := customerRow{User: owner}
	if s := h.statsByWorkspace()[ws]; s != nil {
		row.Stats = *s
	}
	c.JSON(http.StatusOK, gin.H{"customer": row, "staff": staff, "pages": pages, "sites": sites, "all_features": models.AllFeatures})
}

type updateCustomerReq struct {
	FullName *string   `json:"full_name"`
	IsActive *bool     `json:"is_active"`
	Features *[]string `json:"features"`
	MaxPages *int      `json:"max_pages"`
	MaxSites *int      `json:"max_sites"`
	MaxStaff *int      `json:"max_staff"`
}

func clampLimit(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100000 {
		return 100000
	}
	return v
}

// UpdateCustomer PATCH /api/v1/admin/customers/:id — khoá / mở tài khoản, bật tắt trang, đặt giới hạn
func (h *CustomerHandler) UpdateCustomer(c *gin.Context) {
	u, ok := h.targetUser(c)
	if !ok {
		return
	}
	var req updateCustomerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	updates := map[string]any{}
	if req.FullName != nil {
		name := strings.TrimSpace(*req.FullName)
		if name == "" || len([]rune(name)) > 120 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tên không hợp lệ."})
			return
		}
		updates["full_name"] = name
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.Features != nil {
		updates["features"] = jsonList(cleanFeatures(*req.Features, nil))
	}
	if u.SystemRole == models.RoleCustomer { // giới hạn chỉ áp dụng cho khách hàng (nhân viên dùng giới hạn của khách hàng chủ)
		if req.MaxPages != nil {
			updates["max_pages"] = clampLimit(*req.MaxPages)
		}
		if req.MaxSites != nil {
			updates["max_sites"] = clampLimit(*req.MaxSites)
		}
		if req.MaxStaff != nil {
			updates["max_staff"] = clampLimit(*req.MaxStaff)
		}
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không có gì để cập nhật."})
		return
	}
	if err := h.DB.Model(&models.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không cập nhật được tài khoản."})
		return
	}
	middleware.InvalidateAllUsers() // khách hàng đổi quyền thì nhân viên của họ cũng đổi theo
	var fresh models.User
	h.DB.Where("id = ?", u.ID).Limit(1).Find(&fresh)
	c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật tài khoản.", "customer": fresh})
}

type createCustomerReq struct {
	FullName string   `json:"full_name"`
	Email    string   `json:"email"`
	Password string   `json:"password"`
	Features []string `json:"features"`
	MaxPages int      `json:"max_pages"`
	MaxSites int      `json:"max_sites"`
	MaxStaff int      `json:"max_staff"`
}

// CreateCustomer POST /api/v1/admin/customers — admin tạo sẵn tài khoản khách hàng
func (h *CustomerHandler) CreateCustomer(c *gin.Context) {
	var req createCustomerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	u, status, msg := buildUser(req.FullName, req.Email, req.Password, models.RoleCustomer, nil, req.Features, nil)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	u.MaxPages, u.MaxSites, u.MaxStaff = clampLimit(req.MaxPages), clampLimit(req.MaxSites), clampLimit(req.MaxStaff)
	if err := h.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email này đã được sử dụng."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Đã tạo tài khoản khách hàng.", "customer": u})
}

// buildUser kiểm tra dữ liệu và dựng tài khoản mới (khách hàng hoặc nhân viên)
func buildUser(fullName, email, password, role string, ownerID *int64, features []string, allowed map[string]bool) (models.User, int, string) {
	name := strings.TrimSpace(fullName)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" || len([]rune(name)) > 120 {
		return models.User{}, http.StatusBadRequest, "Vui lòng nhập họ tên (tối đa 120 ký tự)."
	}
	if !strings.Contains(email, "@") || len(email) > 150 || strings.ContainsAny(email, " \t") {
		return models.User{}, http.StatusBadRequest, "Email không hợp lệ."
	}
	if len(password) < 6 {
		return models.User{}, http.StatusBadRequest, "Mật khẩu tối thiểu 6 ký tự."
	}
	hash, err := utils.HashPassword(password)
	if err != nil {
		return models.User{}, http.StatusInternalServerError, "Lỗi mã hóa mật khẩu."
	}
	feats := cleanFeatures(features, allowed)
	if features == nil {
		feats = cleanFeatures(defaultFeatures(), allowed)
	}
	return models.User{
		FullName: name, Email: email, PasswordHash: hash, AuthProvider: "local",
		SystemRole: role, OwnerID: ownerID, Features: feats, IsActive: true,
	}, 0, ""
}

type passwordReq struct {
	Password string `json:"password"`
}

// SetCustomerPassword POST /api/v1/admin/customers/:id/password
func (h *CustomerHandler) SetCustomerPassword(c *gin.Context) {
	u, ok := h.targetUser(c)
	if !ok {
		return
	}
	setPassword(c, h.DB, u.ID)
}

func setPassword(c *gin.Context, db *gorm.DB, id int64) {
	var req passwordReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mật khẩu tối thiểu 6 ký tự."})
		return
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi mã hóa mật khẩu."})
		return
	}
	db.Model(&models.User{}).Where("id = ?", id).Updates(map[string]any{"password_hash": hash, "auth_provider": "local"})
	c.JSON(http.StatusOK, gin.H{"message": "Đã đặt lại mật khẩu."})
}

// ----------------------------------------------------------------- Nhân viên của khách hàng

// ownerStaff: nhân viên thuộc đúng không gian dữ liệu của yêu cầu
func (h *CustomerHandler) ownerStaff(c *gin.Context) (models.User, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var u models.User
	ws := wsID(c)
	if err != nil || id <= 0 || h.DB.Where("id = ? AND owner_id = ? AND system_role = ?", id, ws, models.RoleStaff).Limit(1).Find(&u).RowsAffected == 0 {
		notFound(c, "Không tìm thấy nhân viên.")
		return u, false
	}
	return u, true
}

// ListTeam GET /api/v1/team
func (h *CustomerHandler) ListTeam(c *gin.Context) {
	t := tenantOf(c)
	staff := []models.User{}
	h.DB.Where("owner_id = ? AND system_role = ?", t.WorkspaceID, models.RoleStaff).Order("id ASC").Find(&staff)
	c.JSON(http.StatusOK, gin.H{
		"staff": staff, "features": t.FeatureList(), "max_staff": t.MaxStaff,
	})
}

type teamReq struct {
	FullName string    `json:"full_name"`
	Email    string    `json:"email"`
	Password string    `json:"password"`
	Features *[]string `json:"features"`
	IsActive *bool     `json:"is_active"`
}

// CreateStaff POST /api/v1/team
func (h *CustomerHandler) CreateStaff(c *gin.Context) {
	t := tenantOf(c)
	var req teamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	if t.MaxStaff > 0 {
		var n int64
		h.DB.Model(&models.User{}).Where("owner_id = ?", t.WorkspaceID).Count(&n)
		if n >= int64(t.MaxStaff) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Tài khoản của bạn chỉ được tạo tối đa " + strconv.Itoa(t.MaxStaff) + " nhân viên. Hãy liên hệ admin để nâng giới hạn.", "code": "limit_reached"})
			return
		}
	}
	owner := t.WorkspaceID
	var feats []string
	if req.Features != nil {
		feats = *req.Features
	}
	u, status, msg := buildUser(req.FullName, req.Email, req.Password, models.RoleStaff, &owner, nilIfEmptyPtr(req.Features, feats), t.Features)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	if err := h.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email này đã được sử dụng."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Đã tạo tài khoản nhân viên.", "staff": u})
}

// nilIfEmptyPtr: không gửi features = mặc định tất cả trang của khách hàng; gửi (kể cả rỗng) = đúng danh sách đó
func nilIfEmptyPtr(p *[]string, v []string) []string {
	if p == nil {
		return nil
	}
	if v == nil {
		return []string{}
	}
	return v
}

// UpdateStaff PATCH /api/v1/team/:id
func (h *CustomerHandler) UpdateStaff(c *gin.Context) {
	t := tenantOf(c)
	u, ok := h.ownerStaff(c)
	if !ok {
		return
	}
	var req teamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	updates := map[string]any{}
	if n := strings.TrimSpace(req.FullName); n != "" {
		if len([]rune(n)) > 120 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tên quá dài."})
			return
		}
		updates["full_name"] = n
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.Features != nil {
		updates["features"] = jsonList(cleanFeatures(*req.Features, t.Features))
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không có gì để cập nhật."})
		return
	}
	h.DB.Model(&models.User{}).Where("id = ?", u.ID).Updates(updates)
	middleware.InvalidateUser(u.ID)
	var fresh models.User
	h.DB.Where("id = ?", u.ID).Limit(1).Find(&fresh)
	c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật nhân viên.", "staff": fresh})
}

// SetStaffPassword POST /api/v1/team/:id/password
func (h *CustomerHandler) SetStaffPassword(c *gin.Context) {
	u, ok := h.ownerStaff(c)
	if !ok {
		return
	}
	setPassword(c, h.DB, u.ID)
}

// DeleteStaff DELETE /api/v1/team/:id
func (h *CustomerHandler) DeleteStaff(c *gin.Context) {
	u, ok := h.ownerStaff(c)
	if !ok {
		return
	}
	h.DB.Delete(&models.User{}, u.ID)
	middleware.InvalidateUser(u.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Đã xoá nhân viên."})
}
