package handlers

// Tài khoản khách hàng của từng website bán hàng (không liên quan tài khoản quản trị).
// - Đăng ký bằng họ tên + số điện thoại (+ email tuỳ chọn) + mật khẩu; đăng nhập bằng SĐT hoặc email.
// - Token gửi qua header X-Customer-Token, ký bằng khoá RIÊNG (dẫn xuất từ JWT_SECRET)
//   nên không bao giờ dùng được để vào trang quản trị.
// - Đơn hàng đặt khi đã đăng nhập được gắn customer_id -> xem lại ở "Đơn hàng của tôi".

import (
	"crypto/sha256"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type SiteCustomer struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	StorefrontID uint      `gorm:"not null;uniqueIndex:idx_site_cust_phone" json:"-"`
	Phone        string    `gorm:"size:20;not null;uniqueIndex:idx_site_cust_phone" json:"phone"`
	Email        string    `gorm:"size:160;index" json:"email"`
	Name         string    `gorm:"size:120" json:"name"`
	Address      string    `gorm:"type:text" json:"address"`
	PasswordHash string    `gorm:"size:100;not null" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func migrateSiteCustomers(db *gorm.DB) {
	_ = db.AutoMigrate(&SiteCustomer{})
}

// ----------------------------------------------------------------- Token

type customerClaims struct {
	CustomerID   uint   `json:"cid"`
	StorefrontID uint   `json:"sid"`
	Typ          string `json:"typ"`
	jwt.RegisteredClaims
}

func (h *StorefrontHandler) customerKey() []byte {
	secret := ""
	if h.Cfg != nil {
		secret = h.Cfg.JWTSecret
	}
	sum := sha256.Sum256([]byte(secret + "|shop-customer-v1"))
	return sum[:]
}

func (h *StorefrontHandler) issueCustomerToken(c *SiteCustomer) (string, error) {
	claims := customerClaims{
		CustomerID:   c.ID,
		StorefrontID: c.StorefrontID,
		Typ:          "shop_customer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.customerKey())
}

// customerFromRequest trả về khách đang đăng nhập (đúng website sid), hoặc nil
func (h *StorefrontHandler) customerFromRequest(c *gin.Context, sid uint) *SiteCustomer {
	raw := strings.TrimSpace(c.GetHeader("X-Customer-Token"))
	if raw == "" {
		return nil
	}
	var cl customerClaims
	tok, err := jwt.ParseWithClaims(raw, &cl, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("sai thuật toán")
		}
		return h.customerKey(), nil
	})
	if err != nil || !tok.Valid || cl.Typ != "shop_customer" || cl.StorefrontID != sid || cl.CustomerID == 0 {
		return nil
	}
	var cu SiteCustomer
	if h.DB.Where("id = ? AND storefront_id = ?", cl.CustomerID, sid).First(&cu).Error != nil {
		return nil
	}
	return &cu
}

// ----------------------------------------------------------------- Chống dò mật khẩu

var authLimiter = struct {
	sync.Mutex
	hits map[string][]time.Time
}{hits: map[string][]time.Time{}}

func allowAuth(key string) bool {
	authLimiter.Lock()
	defer authLimiter.Unlock()
	now := time.Now()
	keep := authLimiter.hits[key][:0]
	for _, t := range authLimiter.hits[key] {
		if now.Sub(t) < 10*time.Minute {
			keep = append(keep, t)
		}
	}
	if len(keep) >= 12 {
		authLimiter.hits[key] = keep
		return false
	}
	authLimiter.hits[key] = append(keep, now)
	return true
}

func normPhone(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), " ", ""), ".", ""), "-", "")
}

func customerJSON(cu *SiteCustomer) gin.H {
	return gin.H{"id": cu.ID, "name": cu.Name, "phone": cu.Phone, "email": cu.Email, "address": cu.Address}
}

// bcrypt của chuỗi bất kỳ, dùng khi không tìm thấy tài khoản để thời gian phản hồi như nhau
var dummyHash, _ = utils.HashPassword("pp-dummy-password-for-timing")

// ----------------------------------------------------------------- Handlers

// CustomerRegister POST /api/v1/storefront/public/customers/register
func (h *StorefrontHandler) CustomerRegister(c *gin.Context) {
	if !allowAuth("reg:" + c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Bạn thao tác quá nhanh, vui lòng thử lại sau ít phút."})
		return
	}
	var req struct {
		Slug     string `json:"slug"`
		Name     string `json:"name"`
		Phone    string `json:"phone"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	sf, ok := h.publishedSiteBySlug(req.Slug)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy cửa hàng."})
		return
	}
	name := trimTo(req.Name, 120)
	phone := normPhone(req.Phone)
	email := strings.ToLower(trimTo(req.Email, 160))
	switch {
	case name == "":
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập họ tên."})
		return
	case !phoneRe.MatchString(phone):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Số điện thoại không hợp lệ."})
		return
	case len(req.Password) < 6 || len(req.Password) > 72:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mật khẩu cần từ 6 đến 72 ký tự."})
		return
	}
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email không hợp lệ."})
			return
		}
	}
	var n int64
	h.DB.Model(&SiteCustomer{}).Where("storefront_id = ? AND phone = ?", sf.ID, phone).Count(&n)
	if n > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Số điện thoại này đã có tài khoản. Hãy đăng nhập."})
		return
	}
	if email != "" {
		h.DB.Model(&SiteCustomer{}).Where("storefront_id = ? AND email = ?", sf.ID, email).Count(&n)
		if n > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Email này đã có tài khoản. Hãy đăng nhập."})
			return
		}
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tạo được tài khoản."})
		return
	}
	cu := SiteCustomer{StorefrontID: sf.ID, Name: name, Phone: phone, Email: email, PasswordHash: hash}
	if err := h.DB.Create(&cu).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Số điện thoại này đã có tài khoản. Hãy đăng nhập."})
		return
	}
	tok, err := h.issueCustomerToken(&cu)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tạo được phiên đăng nhập."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": tok, "customer": customerJSON(&cu)})
}

// CustomerLogin POST /api/v1/storefront/public/customers/login {slug, login (SĐT hoặc email), password}
func (h *StorefrontHandler) CustomerLogin(c *gin.Context) {
	var req struct {
		Slug     string `json:"slug"`
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	login := strings.ToLower(strings.TrimSpace(req.Login))
	if !allowAuth("login:"+c.ClientIP()) || !allowAuth("login:"+req.Slug+":"+login) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Đăng nhập sai quá nhiều lần, vui lòng thử lại sau ít phút."})
		return
	}
	sf, ok := h.publishedSiteBySlug(req.Slug)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy cửa hàng."})
		return
	}
	var cu SiteCustomer
	q := h.DB.Where("storefront_id = ?", sf.ID)
	if strings.Contains(login, "@") {
		q = q.Where("email = ?", login)
	} else {
		q = q.Where("phone = ?", normPhone(login))
	}
	found := login != "" && q.First(&cu).Error == nil
	if !found {
		utils.CheckPassword(dummyHash, req.Password)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sai số điện thoại / email hoặc mật khẩu."})
		return
	}
	if !utils.CheckPassword(cu.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sai số điện thoại / email hoặc mật khẩu."})
		return
	}
	tok, err := h.issueCustomerToken(&cu)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tạo được phiên đăng nhập."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tok, "customer": customerJSON(&cu)})
}

func (h *StorefrontHandler) currentCustomer(c *gin.Context) (*models.Storefront, *SiteCustomer, bool) {
	sf, ok := h.publishedSiteBySlug(c.Query("slug"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy cửa hàng."})
		return nil, nil, false
	}
	cu := h.customerFromRequest(c, sf.ID)
	if cu == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập đã hết hạn, vui lòng đăng nhập lại."})
		return nil, nil, false
	}
	return sf, cu, true
}

// CustomerMe GET /api/v1/storefront/public/customers/me?slug=
func (h *StorefrontHandler) CustomerMe(c *gin.Context) {
	if _, cu, ok := h.currentCustomer(c); ok {
		c.JSON(http.StatusOK, gin.H{"customer": customerJSON(cu)})
	}
}

// CustomerUpdate PUT /api/v1/storefront/public/customers/me?slug=  {name, email, address}
func (h *StorefrontHandler) CustomerUpdate(c *gin.Context) {
	sf, cu, ok := h.currentCustomer(c)
	if !ok {
		return
	}
	var req struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Address string `json:"address"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	name := trimTo(req.Name, 120)
	email := strings.ToLower(trimTo(req.Email, 160))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập họ tên."})
		return
	}
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email không hợp lệ."})
			return
		}
		var n int64
		h.DB.Model(&SiteCustomer{}).Where("storefront_id = ? AND email = ? AND id <> ?", sf.ID, email, cu.ID).Count(&n)
		if n > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Email này đã được dùng cho tài khoản khác."})
			return
		}
	}
	cu.Name, cu.Email, cu.Address = name, email, trimTo(req.Address, 500)
	if err := h.DB.Model(cu).Updates(map[string]any{"name": cu.Name, "email": cu.Email, "address": cu.Address}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không lưu được thông tin."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"customer": customerJSON(cu)})
}

// CustomerOrders GET /api/v1/storefront/public/customers/orders?slug=
func (h *StorefrontHandler) CustomerOrders(c *gin.Context) {
	sf, cu, ok := h.currentCustomer(c)
	if !ok {
		return
	}
	var orders []ShopOrder
	h.DB.Where("storefront_id = ? AND customer_id = ?", sf.ID, cu.ID).Order("id DESC").Limit(50).Find(&orders)
	out := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		qty := 0
		for _, it := range o.Items {
			qty += it.Qty
		}
		out = append(out, gin.H{
			"code": o.Code, "token": o.AccessToken, "total": o.Total, "status": o.Status,
			"payment_method": o.PaymentMethod, "payment_status": o.PaymentStatus,
			"items": len(o.Items), "qty": qty, "created_at": o.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"orders": out})
}
