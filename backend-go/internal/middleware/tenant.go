package middleware

// Phân quyền theo khách hàng (multi-tenant).
//
// Mỗi tài khoản "customer" là một khách hàng với không gian dữ liệu riêng: WorkspaceID chính là id của tài khoản đó.
// Nhân viên (staff) dùng không gian của khách hàng chủ (OwnerID). Admin có không gian của chính mình và có thể
// "xem như khách" (header X-Workspace) — chế độ này CHỈ ĐỌC.
//
// Middleware Tenancy chạy sau JWTAuth: đọc lại tài khoản từ database (có đệm ngắn), kiểm tra còn hoạt động,
// tính không gian dữ liệu + danh sách trang được dùng, rồi đặt vào context để các handler lọc dữ liệu.

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Tenant struct {
	UserID      int64
	Name        string
	Role        string
	WorkspaceID int64 // không gian dữ liệu đang dùng
	OwnerName   string
	IsAdmin     bool // tài khoản đăng nhập là admin
	IsOwner     bool // chủ của không gian dữ liệu (không phải nhân viên)
	Viewing     bool // admin đang xem như khách (chỉ đọc)
	Features    map[string]bool
	MaxPages    int
	MaxSites    int
	MaxStaff    int
}

func (t Tenant) Can(feature string) bool { return t.Features[feature] }

func (t Tenant) FeatureList() []string {
	out := []string{}
	for _, f := range models.AllFeatures {
		if t.Features[f] {
			out = append(out, f)
		}
	}
	return out
}

// ----------------------------------------------------------------- Đệm tài khoản (không đọc database mỗi request)

const userCacheTTL = 20 * time.Second

type cachedUser struct {
	u   models.User
	ok  bool
	exp time.Time
}

var userCache = struct {
	sync.RWMutex
	m map[int64]cachedUser
}{m: map[int64]cachedUser{}}

func loadUser(db *gorm.DB, id int64) (models.User, bool) {
	userCache.RLock()
	c, hit := userCache.m[id]
	userCache.RUnlock()
	if hit && time.Now().Before(c.exp) {
		return c.u, c.ok
	}
	var u models.User
	ok := db.Where("id = ?", id).Limit(1).Find(&u).RowsAffected > 0
	userCache.Lock()
	if len(userCache.m) > 5000 {
		userCache.m = map[int64]cachedUser{}
	}
	userCache.m[id] = cachedUser{u: u, ok: ok, exp: time.Now().Add(userCacheTTL)}
	userCache.Unlock()
	return u, ok
}

// InvalidateUser: gọi sau khi sửa tài khoản (khoá, đổi trang được dùng...) để thay đổi có hiệu lực ngay
func InvalidateUser(ids ...int64) {
	userCache.Lock()
	for _, id := range ids {
		delete(userCache.m, id)
	}
	userCache.Unlock()
}

func InvalidateAllUsers() {
	userCache.Lock()
	userCache.m = map[int64]cachedUser{}
	userCache.Unlock()
}

// effectiveFeatures: chưa đặt (nil — tài khoản cũ) = được dùng tất cả; danh sách rỗng = không trang nào
func effectiveFeatures(u models.User) map[string]bool {
	m := map[string]bool{}
	if u.Features == nil {
		for _, f := range models.AllFeatures {
			m[f] = true
		}
		return m
	}
	for _, f := range u.Features {
		if models.IsFeature(f) {
			m[f] = true
		}
	}
	return m
}

// ResolveTenant tính Tenant cho một tài khoản (viewAs > 0: admin xem không gian của khách đó)
func ResolveTenant(db *gorm.DB, actor models.User, viewAs int64) (Tenant, int, string) {
	t := Tenant{UserID: actor.ID, Name: actor.FullName, Role: actor.SystemRole, WorkspaceID: actor.ID, IsAdmin: actor.SystemRole == models.RoleAdmin}
	owner := actor
	switch actor.SystemRole {
	case models.RoleAdmin:
		t.IsOwner = true
		if viewAs > 0 && viewAs != actor.ID {
			target, ok := loadUser(db, viewAs)
			if !ok {
				return t, http.StatusNotFound, "Không tìm thấy khách hàng cần xem."
			}
			if target.SystemRole == models.RoleStaff && target.OwnerID != nil {
				if o, ok := loadUser(db, *target.OwnerID); ok {
					target = o
				}
			}
			t.WorkspaceID, t.Viewing, t.IsOwner = target.ID, true, false
			owner = target
			t.Features = effectiveFeatures(target)
			t.OwnerName = target.FullName
		} else {
			t.Features = map[string]bool{}
			for _, f := range models.AllFeatures {
				t.Features[f] = true
			}
		}
	case models.RoleStaff:
		if actor.OwnerID == nil {
			return t, http.StatusForbidden, "Tài khoản nhân viên chưa được gắn với khách hàng nào."
		}
		o, ok := loadUser(db, *actor.OwnerID)
		if !ok || !o.IsActive {
			return t, http.StatusForbidden, "Tài khoản chủ của bạn đã bị khoá hoặc không còn tồn tại."
		}
		t.WorkspaceID, t.OwnerName = o.ID, o.FullName
		owner = o
		own, ownerF := effectiveFeatures(actor), effectiveFeatures(o)
		t.Features = map[string]bool{}
		for f := range own {
			t.Features[f] = ownerF[f]
		}
	default: // customer
		t.IsOwner = true
		t.Features = effectiveFeatures(actor)
	}
	t.MaxPages, t.MaxSites, t.MaxStaff = owner.MaxPages, owner.MaxSites, owner.MaxStaff
	return t, 0, ""
}

// Tenancy: chạy sau JWTAuth. allowViewAs=false cho các route quản trị (không dùng header X-Workspace).
func Tenancy(db *gorm.DB, allowViewAs bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.GetInt64("user_id")
		actor, ok := loadUser(db, uid)
		if !ok || !actor.IsActive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Tài khoản không tồn tại hoặc đã bị khoá."})
			return
		}
		var viewAs int64
		if h := strings.TrimSpace(c.GetHeader("X-Workspace")); h != "" && allowViewAs && actor.SystemRole == models.RoleAdmin {
			if v, err := strconv.ParseInt(h, 10, 64); err == nil && v > 0 {
				viewAs = v
			}
		}
		t, status, msg := ResolveTenant(db, actor, viewAs)
		if status != 0 {
			c.AbortWithStatusJSON(status, gin.H{"error": msg})
			return
		}
		if t.Viewing && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Bạn đang xem như khách hàng (chỉ xem). Hãy thoát chế độ này để thao tác.", "code": "view_only"})
			return
		}
		c.Set("tenant", t)
		c.Set("ws_id", t.WorkspaceID)
		c.Set("system_role", actor.SystemRole)
		c.Next()
	}
}

// TenantOf đọc Tenant đã được Tenancy đặt vào context (ok=false nếu route chưa qua Tenancy)
func TenantOf(c *gin.Context) (Tenant, bool) {
	v, ok := c.Get("tenant")
	if !ok {
		return Tenant{}, false
	}
	t, ok := v.(Tenant)
	return t, ok
}

// RequireFeature: tài khoản phải được cấp trang này
func RequireFeature(features ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		t, ok := TenantOf(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Chưa xác định được tài khoản."})
			return
		}
		for _, f := range features {
			if t.Can(f) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Tài khoản của bạn chưa được cấp quyền dùng chức năng này.", "code": "feature_disabled"})
	}
}

// OwnerOnly: chỉ chủ không gian dữ liệu (khách hàng) — nhân viên không được
func OwnerOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		t, ok := TenantOf(c)
		if !ok || !(t.IsOwner || t.Viewing) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Chỉ chủ tài khoản mới được thực hiện thao tác này.", "code": "owner_only"})
			return
		}
		c.Next()
	}
}

// AdminOnly: chỉ admin hệ thống
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		t, ok := TenantOf(c)
		if !ok || !t.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Chỉ admin hệ thống mới có quyền truy cập chức năng này.", "code": "admin_only"})
			return
		}
		c.Next()
	}
}
