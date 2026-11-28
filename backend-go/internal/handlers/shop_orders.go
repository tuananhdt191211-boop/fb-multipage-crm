package handlers

// Giỏ hàng, đặt hàng và thanh toán cho website bán hàng
// ---------------------------------------------------------------------------
// - Khách đặt hàng trên web công khai: giá luôn lấy từ DB (không tin giá từ trình duyệt).
// - Thanh toán: COD hoặc VNPay (mỗi website dùng tài khoản merchant VNPay riêng).
// - VNPay: trang Return chỉ hiển thị kết quả; trạng thái đơn chỉ đổi qua IPN (server-to-server).
// - Tồn kho: COD trừ ngay khi đặt; VNPay trừ khi IPN báo thanh toán thành công.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	vnpSandboxURL    = "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html"
	vnpProductionURL = "https://pay.vnpay.vn/vpcpay.html"
	maxOrderLines    = 30
	maxOrderQty      = 999
)

var (
	phoneRe    = regexp.MustCompile(`^(\+?84|0)[0-9]{8,10}$`)
	tmnCodeRe  = regexp.MustCompile(`^[A-Za-z0-9]{4,20}$`)
	vnLocation = func() *time.Location {
		if l, err := time.LoadLocation("Asia/Ho_Chi_Minh"); err == nil {
			return l
		}
		return time.FixedZone("ICT", 7*3600)
	}()
)

// ----------------------------------------------------------------- Models

type ShopOrderItem struct {
	ProductID    uint   `json:"product_id"`
	VariantID    uint   `json:"variant_id,omitempty"`    // 0 = sản phẩm không có biến thể
	VariantTitle string `json:"variant_title,omitempty"` // "Đen / L" (chụp lại lúc đặt hàng)
	SKU          string `json:"sku,omitempty"`
	Name         string `json:"name"`
	ImageURL     string `json:"image_url"`
	Price        int64  `json:"price"`
	Qty          int    `json:"qty"`
	LineTotal    int64  `json:"line_total"`
}

type ShopOrder struct {
	ID            uint            `gorm:"primaryKey" json:"id"`
	StorefrontID  uint            `gorm:"index;not null" json:"storefront_id"`
	UserID        uint            `gorm:"index;not null" json:"-"`
	CustomerID    *uint           `gorm:"index" json:"-"` // tài khoản khách (nếu đặt khi đã đăng nhập)
	Code          string          `gorm:"size:32;uniqueIndex;not null" json:"code"`
	AccessToken   string          `gorm:"size:64;not null" json:"-"`
	CustomerName  string          `gorm:"size:120" json:"customer_name"`
	Phone         string          `gorm:"size:20" json:"phone"`
	Email         string          `gorm:"size:160" json:"email"`
	Address       string          `gorm:"type:text" json:"address"`
	Note          string          `gorm:"type:text" json:"note"`
	ItemsJSON     string          `gorm:"type:text;not null" json:"-"`
	Items         []ShopOrderItem `gorm:"-" json:"items"`
	Subtotal      int64           `json:"subtotal"`
	ShippingFee   int64           `json:"shipping_fee"`
	Total         int64           `json:"total"`
	PaymentMethod string          `gorm:"size:20" json:"payment_method"`       // cod | vnpay
	PaymentStatus string          `gorm:"size:20;index" json:"payment_status"` // unpaid | pending | paid | failed
	Status        string          `gorm:"size:20;index" json:"status"`         // new | confirmed | shipping | completed | cancelled
	VnpTxnRef     string          `gorm:"size:40;index" json:"-"`              // mã giao dịch gửi VNPay (đổi mỗi lần thanh toán lại)
	VnpTransNo    string          `gorm:"size:40" json:"vnp_transaction_no"`
	VnpBankCode   string          `gorm:"size:40" json:"vnp_bank_code"`
	// Thanh toán ngoại tệ (PayPal / Stripe): số tiền đã quy đổi tại thời điểm đặt
	Currency     string     `gorm:"size:3" json:"currency"`
	ForeignMinor int64      `json:"foreign_minor"` // số nhỏ nhất, vd. cent
	FxRate       float64    `json:"fx_rate"`
	GatewayRef   string     `gorm:"size:120;index" json:"-"` // mã đơn PayPal / mã phiên Stripe
	GatewayTxn   string     `gorm:"size:120" json:"gateway_txn"`
	PaidAt       *time.Time `json:"paid_at"`
	StockApplied bool       `json:"-"`
	CreatedAt    time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (o *ShopOrder) AfterFind(tx *gorm.DB) error {
	_ = json.Unmarshal([]byte(o.ItemsJSON), &o.Items)
	return nil
}

type ShopPaymentSettings struct {
	ID               uint   `gorm:"primaryKey" json:"-"`
	StorefrontID     uint   `gorm:"uniqueIndex;not null" json:"-"`
	CODEnabled       bool   `json:"cod_enabled"`
	VnpEnabled       bool   `json:"vnp_enabled"`
	VnpTmnCode       string `gorm:"size:20" json:"vnp_tmn_code"`
	VnpHashSecretEnc string `gorm:"type:text" json:"-"`
	VnpSandbox       bool   `json:"vnp_sandbox"`
	// Thanh toán quốc tế (ngoại tệ)
	PaypalEnabled          bool      `json:"paypal_enabled"`
	PaypalClientID         string    `gorm:"size:200" json:"paypal_client_id"`
	PaypalSecretEnc        string    `gorm:"type:text" json:"-"`
	PaypalSandbox          bool      `json:"paypal_sandbox"`
	StripeEnabled          bool      `json:"stripe_enabled"`
	StripeSecretEnc        string    `gorm:"type:text" json:"-"`
	StripeWebhookSecretEnc string    `gorm:"type:text" json:"-"`
	ShippingFee            int64     `json:"shipping_fee"`
	FreeShipFrom           int64     `json:"free_ship_from"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type SiteSubscriber struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	StorefrontID uint      `gorm:"not null;uniqueIndex:idx_site_sub_email" json:"-"`
	Email        string    `gorm:"size:160;not null;uniqueIndex:idx_site_sub_email" json:"email"`
	CreatedAt    time.Time `json:"created_at"`
}

func migrateShopTables(db *gorm.DB) {
	_ = db.AutoMigrate(&ShopOrder{}, &ShopPaymentSettings{}, &SiteSubscriber{})
}

// ----------------------------------------------------------------- Tiện ích

func (h *StorefrontHandler) encKey() string {
	if h.Cfg != nil && h.Cfg.JWTSecret != "" {
		return h.Cfg.JWTSecret
	}
	return "default-dev-master-key-change-in-prod"
}

func randomDigits(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		v, _ := rand.Int(rand.Reader, big.NewInt(10))
		b.WriteString(v.String())
	}
	return b.String()
}

func randomToken() string {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func (h *StorefrontHandler) paymentSettings(siteID uint) ShopPaymentSettings {
	var ps ShopPaymentSettings
	if h.DB.Where("storefront_id = ?", siteID).Limit(1).Find(&ps).RowsAffected == 0 {
		ps = ShopPaymentSettings{StorefrontID: siteID, CODEnabled: true, VnpSandbox: true}
	}
	return ps
}

func (h *StorefrontHandler) publishedSiteBySlug(slug string) (*models.Storefront, bool) {
	var sf models.Storefront
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" || h.DB.Where("slug = ? AND is_published = ?", slug, true).First(&sf).Error != nil {
		return nil, false
	}
	return &sf, true
}

func shippingFor(ps ShopPaymentSettings, subtotal int64) int64 {
	if ps.ShippingFee <= 0 {
		return 0
	}
	if ps.FreeShipFrom > 0 && subtotal >= ps.FreeShipFrom {
		return 0
	}
	return ps.ShippingFee
}

// Chống spam đặt hàng: tối đa 8 đơn / 10 phút / IP
var orderLimiter = struct {
	sync.Mutex
	hits map[string][]time.Time
}{hits: map[string][]time.Time{}}

func allowOrder(ip string) bool {
	orderLimiter.Lock()
	defer orderLimiter.Unlock()
	now := time.Now()
	keep := orderLimiter.hits[ip][:0]
	for _, t := range orderLimiter.hits[ip] {
		if now.Sub(t) < 10*time.Minute {
			keep = append(keep, t)
		}
	}
	if len(keep) >= 8 {
		orderLimiter.hits[ip] = keep
		return false
	}
	orderLimiter.hits[ip] = append(keep, now)
	return true
}

// ----------------------------------------------------------------- VNPay

// vnpHashData sắp xếp tham số theo tên, mã hoá URL (khoảng trắng thành '+') và nối bằng '&'
func vnpHashData(params url.Values) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "vnp_SecureHash" || k == "vnp_SecureHashType" || !strings.HasPrefix(k, "vnp_") {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(params.Get(k)))
	}
	return strings.Join(parts, "&")
}

func vnpSign(secret, data string) string {
	m := hmac.New(sha512.New, []byte(secret))
	m.Write([]byte(data))
	return hex.EncodeToString(m.Sum(nil))
}

func vnpVerify(secret string, params url.Values) bool {
	got := strings.ToLower(params.Get("vnp_SecureHash"))
	if got == "" {
		return false
	}
	want := vnpSign(secret, vnpHashData(params))
	return hmac.Equal([]byte(got), []byte(want))
}

func (h *StorefrontHandler) buildVnpURL(ps ShopPaymentSettings, order ShopOrder, clientIP, returnURL string) (string, error) {
	secret, err := security.DecryptSecret(ps.VnpHashSecretEnc, h.encKey())
	if err != nil || secret == "" || ps.VnpTmnCode == "" {
		return "", fmt.Errorf("Website chưa cấu hình VNPay đầy đủ")
	}
	now := time.Now().In(vnLocation)
	if clientIP == "" || clientIP == "::1" {
		clientIP = "127.0.0.1"
	}
	p := url.Values{}
	p.Set("vnp_Version", "2.1.0")
	p.Set("vnp_Command", "pay")
	p.Set("vnp_TmnCode", ps.VnpTmnCode)
	p.Set("vnp_Amount", strconv.FormatInt(order.Total*100, 10))
	p.Set("vnp_CurrCode", "VND")
	txnRef := order.VnpTxnRef
	if txnRef == "" {
		txnRef = order.Code
	}
	p.Set("vnp_TxnRef", txnRef)
	p.Set("vnp_OrderInfo", "Thanh toan don hang "+order.Code)
	p.Set("vnp_OrderType", "other")
	p.Set("vnp_Locale", "vn")
	p.Set("vnp_ReturnUrl", returnURL)
	p.Set("vnp_IpAddr", clientIP)
	p.Set("vnp_CreateDate", now.Format("20060102150405"))
	p.Set("vnp_ExpireDate", now.Add(15*time.Minute).Format("20060102150405"))

	data := vnpHashData(p)
	base := vnpProductionURL
	if ps.VnpSandbox {
		base = vnpSandboxURL
	}
	return base + "?" + data + "&vnp_SecureHash=" + vnpSign(secret, data), nil
}

// ----------------------------------------------------------------- API công khai

// PublicCheckoutConfig GET /api/v1/storefront/public/checkout-config?slug=...
func (h *StorefrontHandler) PublicCheckoutConfig(c *gin.Context) {
	sf, ok := h.publishedSiteBySlug(c.Query("slug"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy cửa hàng."})
		return
	}
	ps := h.paymentSettings(sf.ID)
	c.JSON(http.StatusOK, gin.H{
		"cod":            ps.CODEnabled || (!ps.VnpEnabled && !h.paypalReady(ps) && !h.stripeReady(ps)), // luôn có ít nhất 1 cách
		"vnpay":          ps.VnpEnabled && ps.VnpTmnCode != "" && ps.VnpHashSecretEnc != "",
		"paypal":         h.paypalReady(ps),
		"stripe":         h.stripeReady(ps),
		"shipping_fee":   ps.ShippingFee,
		"free_ship_from": ps.FreeShipFrom,
	})
}

type createOrderReq struct {
	Slug  string `json:"slug"`
	Items []struct {
		ProductID uint `json:"product_id"`
		VariantID uint `json:"variant_id"` // bắt buộc nếu sản phẩm có biến thể
		Qty       int  `json:"qty"`
	} `json:"items"`
	CustomerName  string `json:"customer_name"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Address       string `json:"address"`
	Note          string `json:"note"`
	PaymentMethod string `json:"payment_method"`
	ReturnBase    string `json:"return_base"` // origin của web công khai, để VNPay chuyển khách về
	Currency      string `json:"currency"`    // tiền tệ khách đang xem (PayPal / Stripe)
}

func trimTo(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// allowedReturnBase chỉ chấp nhận origin của frontend hệ thống hoặc tên miền riêng của website
func (h *StorefrontHandler) allowedReturnBase(raw string, sf *models.Storefront) string {
	fallback := ""
	if h.Cfg != nil {
		fallback = strings.TrimRight(strings.TrimSpace(h.Cfg.FrontendURL), "/")
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fallback
	}
	host := normalizeHost(u.Host)
	if fb, err := url.Parse(fallback); err == nil && normalizeHost(fb.Host) == host {
		return u.Scheme + "://" + u.Host
	}
	if sf.Domain != "" && normalizeHost(sf.Domain) == host {
		return u.Scheme + "://" + u.Host
	}
	return fallback
}

// CreatePublicOrder POST /api/v1/storefront/public/orders
func (h *StorefrontHandler) CreatePublicOrder(c *gin.Context) {
	if !allowOrder(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Bạn đặt hàng quá nhanh, vui lòng thử lại sau ít phút."})
		return
	}
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu đặt hàng không hợp lệ."})
		return
	}
	sf, ok := h.publishedSiteBySlug(req.Slug)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy cửa hàng."})
		return
	}

	req.CustomerName = trimTo(req.CustomerName, 120)
	req.Phone = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(req.Phone), " ", ""), ".", "")
	req.Email = trimTo(req.Email, 160)
	req.Address = trimTo(req.Address, 500)
	req.Note = trimTo(req.Note, 1000)
	switch {
	case req.CustomerName == "":
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập họ tên."})
		return
	case !phoneRe.MatchString(req.Phone):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Số điện thoại không hợp lệ."})
		return
	case req.Address == "":
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập địa chỉ nhận hàng."})
		return
	}
	if req.Email != "" {
		if _, err := mail.ParseAddress(req.Email); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email không hợp lệ."})
			return
		}
	}
	if len(req.Items) == 0 || len(req.Items) > maxOrderLines {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Giỏ hàng trống hoặc quá nhiều sản phẩm."})
		return
	}

	ps := h.paymentSettings(sf.ID)
	vnpReady := ps.VnpEnabled && ps.VnpTmnCode != "" && ps.VnpHashSecretEnc != ""
	codReady := ps.CODEnabled || (!vnpReady && !h.paypalReady(ps) && !h.stripeReady(ps))
	switch req.PaymentMethod {
	case "paypal":
		if !h.paypalReady(ps) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cửa hàng chưa bật thanh toán PayPal."})
			return
		}
	case "stripe":
		if !h.stripeReady(ps) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cửa hàng chưa bật thanh toán thẻ quốc tế."})
			return
		}
	case "vnpay":
		if !vnpReady {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cửa hàng chưa bật thanh toán VNPay."})
			return
		}
	default:
		req.PaymentMethod = "cod"
		if !codReady {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cửa hàng không nhận thanh toán khi nhận hàng."})
			return
		}
	}

	// Gộp số lượng theo (sản phẩm, biến thể), lấy giá & tồn kho từ DB
	type lineKey struct{ pid, vid uint }
	qtyByKey := map[lineKey]int{}
	order := []lineKey{}
	pidSet := map[uint]bool{}
	pids := []uint{}
	for _, it := range req.Items {
		if it.ProductID == 0 || it.Qty <= 0 {
			continue
		}
		k := lineKey{it.ProductID, it.VariantID}
		if _, seen := qtyByKey[k]; !seen {
			order = append(order, k)
		}
		qtyByKey[k] += it.Qty
		if !pidSet[it.ProductID] {
			pidSet[it.ProductID] = true
			pids = append(pids, it.ProductID)
		}
	}
	if len(order) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Giỏ hàng trống."})
		return
	}
	var prods []models.Product
	h.DB.Where("storefront_id = ? AND is_active = ? AND id IN ?", sf.ID, true, pids).Find(&prods)
	byID := map[uint]models.Product{}
	for _, p := range prods {
		byID[p.ID] = p
	}
	varsOf := map[uint]map[uint]models.ProductVariant{} // product_id -> variant_id -> biến thể
	if len(byID) > 0 {
		var vs []models.ProductVariant
		h.DB.Where("storefront_id = ? AND product_id IN ?", sf.ID, pids).Find(&vs)
		for _, v := range vs {
			if varsOf[v.ProductID] == nil {
				varsOf[v.ProductID] = map[uint]models.ProductVariant{}
			}
			varsOf[v.ProductID][v.ID] = v
		}
	}

	items := make([]ShopOrderItem, 0, len(order))
	var subtotal int64
	for _, k := range order {
		p, ok := byID[k.pid]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Có sản phẩm trong giỏ không còn bán. Vui lòng tải lại trang."})
			return
		}
		q := qtyByKey[k]
		if q > maxOrderQty {
			q = maxOrderQty
		}
		if len(p.Options) > 0 && len(varsOf[p.ID]) > 0 {
			// Sản phẩm có biến thể: bắt buộc chọn đúng một biến thể của chính sản phẩm này; giá & tồn kho lấy từ biến thể
			if k.vid == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Vui lòng chọn phân loại cho sản phẩm \"%s\".", p.Name)})
				return
			}
			v, ok := varsOf[p.ID][k.vid]
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Phân loại của sản phẩm \"%s\" không còn bán. Vui lòng tải lại trang.", p.Name)})
				return
			}
			if v.Stock == 0 || (v.Stock > 0 && q > v.Stock) {
				c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Sản phẩm \"%s\" (%s) không đủ hàng (còn %d).", p.Name, v.Title, max(v.Stock, 0))})
				return
			}
			img := v.ImageURL
			if img == "" {
				img = p.ImageURL
			}
			line := v.Price * int64(q)
			subtotal += line
			items = append(items, ShopOrderItem{ProductID: p.ID, VariantID: v.ID, VariantTitle: v.Title, SKU: v.SKU,
				Name: p.Name, ImageURL: img, Price: v.Price, Qty: q, LineTotal: line})
			continue
		}
		if k.vid != 0 {
			// Giỏ hàng cũ còn nhớ biến thể mà cửa hàng đã gỡ: để khách xem lại giá thay vì tính nhầm giá
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Phân loại của sản phẩm \"%s\" không còn bán. Vui lòng tải lại trang.", p.Name)})
			return
		}
		if p.Stock == 0 || (p.Stock > 0 && q > p.Stock) {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Sản phẩm \"%s\" không đủ hàng (còn %d).", p.Name, max(p.Stock, 0))})
			return
		}
		line := p.Price * int64(q)
		subtotal += line
		items = append(items, ShopOrderItem{ProductID: p.ID, Name: p.Name, ImageURL: p.ImageURL, Price: p.Price, Qty: q, LineTotal: line})
	}
	ship := shippingFor(ps, subtotal)
	total := subtotal + ship
	if req.PaymentMethod == "vnpay" && total < 5000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Đơn tối thiểu 5.000đ để thanh toán VNPay."})
		return
	}

	itemsJSON, _ := json.Marshal(items)
	o := ShopOrder{
		StorefrontID:  sf.ID,
		UserID:        sf.UserID,
		AccessToken:   randomToken(),
		CustomerName:  req.CustomerName,
		Phone:         req.Phone,
		Email:         req.Email,
		Address:       req.Address,
		Note:          req.Note,
		ItemsJSON:     string(itemsJSON),
		Items:         items,
		Subtotal:      subtotal,
		ShippingFee:   ship,
		Total:         total,
		PaymentMethod: req.PaymentMethod,
		PaymentStatus: "unpaid",
		Status:        "new",
	}
	if o.PaymentMethod == "vnpay" || o.PaymentMethod == "paypal" || o.PaymentMethod == "stripe" {
		o.PaymentStatus = "pending"
	}
	if cu := h.customerFromRequest(c, sf.ID); cu != nil {
		cid := cu.ID
		o.CustomerID = &cid
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < 5; i++ {
			o.Code = "DH" + time.Now().In(vnLocation).Format("060102") + randomDigits(5)
			var n int64
			tx.Model(&ShopOrder{}).Where("code = ?", o.Code).Count(&n)
			if n == 0 {
				break
			}
		}
		if o.PaymentMethod == "cod" {
			if err := applyStock(tx, items, -1); err != nil {
				return err
			}
			o.StockApplied = true
		}
		return tx.Create(&o).Error
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{"code": o.Code, "token": o.AccessToken, "total": o.Total, "payment_method": o.PaymentMethod}
	if o.PaymentMethod == "vnpay" {
		base := h.allowedReturnBase(req.ReturnBase, sf)
		returnURL := h.orderReturnURL(base, sf, o.Code, o.AccessToken)
		payURL, err := h.buildVnpURL(ps, o, c.ClientIP(), returnURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resp["payment_url"] = payURL
	}
	if o.PaymentMethod == "paypal" || o.PaymentMethod == "stripe" {
		base := h.allowedReturnBase(req.ReturnBase, sf)
		payURL, err := h.startIntlPayment(ps, sf, &o, req.Currency, h.orderReturnURL(base, sf, o.Code, o.AccessToken))
		if err != nil {
			// Đơn đã tạo: khách có thể bấm "Thanh toán lại" ở trang đơn hàng
			resp["payment_error"] = err.Error()
		} else {
			resp["payment_url"] = payURL
		}
	}
	c.JSON(http.StatusCreated, resp)
}

// applyStock cộng/trừ tồn kho (dir = -1 trừ, +1 hoàn). Sản phẩm / biến thể stock = -1 (không giới hạn) bỏ qua.
// Dòng có variant_id trừ / hoàn ở biến thể rồi cập nhật lại số tổng hợp của sản phẩm.
func applyStock(tx *gorm.DB, items []ShopOrderItem, dir int) error {
	// Khoá các sản phẩm có biến thể (theo thứ tự id để không deadlock) trước khi đổi tồn kho: hai đơn cùng lúc
	// cho hai biến thể của cùng một sản phẩm sẽ xếp hàng, nên số tổng hợp luôn đúng.
	locked := []uint{}
	seenLock := map[uint]bool{}
	for _, it := range items {
		if it.VariantID != 0 && !seenLock[it.ProductID] {
			seenLock[it.ProductID] = true
			locked = append(locked, it.ProductID)
		}
	}
	if len(locked) > 0 {
		sort.Slice(locked, func(i, j int) bool { return locked[i] < locked[j] })
		var lockRows []models.Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id IN ?", locked).Order("id ASC").Find(&lockRows).Error; err != nil {
			return err
		}
	}
	for _, it := range items {
		if it.VariantID != 0 {
			if dir < 0 {
				res := tx.Model(&models.ProductVariant{}).
					Where("id = ? AND stock >= 0", it.VariantID).
					Where("stock >= ?", it.Qty).
					UpdateColumn("stock", gorm.Expr("stock - ?", it.Qty))
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected == 0 {
					var v models.ProductVariant
					if tx.Select("id, title, stock").First(&v, it.VariantID).Error == nil && v.Stock >= 0 {
						return fmt.Errorf("Sản phẩm \"%s\" (%s) vừa hết hàng.", it.Name, v.Title)
					}
				}
			} else {
				_ = tx.Model(&models.ProductVariant{}).Where("id = ? AND stock >= 0", it.VariantID).
					UpdateColumn("stock", gorm.Expr("stock + ?", it.Qty)).Error
			}
			continue
		}
		// Dòng không có biến thể nhưng sản phẩm nay đã có biến thể (đơn đặt trước khi cửa hàng chuyển sang biến thể):
		// tồn kho tổng hợp của sản phẩm do các biến thể quyết định nên không cộng / trừ gì ở đây.
		if productHasVariants(tx, it.ProductID) {
			continue
		}
		if dir < 0 {
			res := tx.Model(&models.Product{}).
				Where("id = ? AND stock >= 0", it.ProductID).
				Where("stock >= ?", it.Qty).
				UpdateColumn("stock", gorm.Expr("stock - ?", it.Qty))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				// Không trừ được: hoặc không giới hạn (-1) hoặc thiếu hàng
				var p models.Product
				if tx.Select("id, name, stock").First(&p, it.ProductID).Error == nil && p.Stock >= 0 {
					return fmt.Errorf("Sản phẩm \"%s\" vừa hết hàng.", p.Name)
				}
			}
		} else {
			_ = tx.Model(&models.Product{}).Where("id = ? AND stock >= 0", it.ProductID).
				UpdateColumn("stock", gorm.Expr("stock + ?", it.Qty)).Error
		}
	}
	for _, pid := range locked {
		recomputeProductStock(tx, pid)
	}
	return nil
}

// GetPublicOrder GET /api/v1/storefront/public/orders/:code?t=token
func (h *StorefrontHandler) GetPublicOrder(c *gin.Context) {
	var o ShopOrder
	if h.DB.Where("code = ?", c.Param("code")).First(&o).Error != nil ||
		!hmac.Equal([]byte(o.AccessToken), []byte(c.Query("t"))) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy đơn hàng."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": gin.H{
		"code": o.Code, "customer_name": o.CustomerName, "phone": o.Phone, "address": o.Address,
		"items": o.Items, "subtotal": o.Subtotal, "shipping_fee": o.ShippingFee, "total": o.Total,
		"payment_method": o.PaymentMethod, "payment_status": o.PaymentStatus, "status": o.Status,
		"created_at": o.CreatedAt, "currency": o.Currency, "foreign_minor": o.ForeignMinor,
		"foreign_decimals": currencyDecimals(o.Currency, o.PaymentMethod),
	}})
}

// RetryVnpPayment POST /api/v1/storefront/public/orders/:code/pay?t=token — tạo lại link thanh toán
func (h *StorefrontHandler) RetryVnpPayment(c *gin.Context) {
	var o ShopOrder
	if h.DB.Where("code = ?", c.Param("code")).First(&o).Error != nil ||
		!hmac.Equal([]byte(o.AccessToken), []byte(c.Query("t"))) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy đơn hàng."})
		return
	}
	// PayPal / Stripe: tạo phiên thanh toán mới
	if (o.PaymentMethod == "paypal" || o.PaymentMethod == "stripe") && o.PaymentStatus != "paid" && o.Status != "cancelled" {
		var sf models.Storefront
		if h.DB.First(&sf, o.StorefrontID).Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy cửa hàng."})
			return
		}
		ps := h.paymentSettings(sf.ID)
		if (o.PaymentMethod == "paypal" && !h.paypalReady(ps)) || (o.PaymentMethod == "stripe" && !h.stripeReady(ps)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cửa hàng đã tắt cách thanh toán này."})
			return
		}
		cur := c.Query("currency")
		if cur == "" {
			cur = o.Currency
		}
		base := h.allowedReturnBase(c.Query("return_base"), &sf)
		payURL, err := h.startIntlPayment(ps, &sf, &o, cur, h.orderReturnURL(base, &sf, o.Code, o.AccessToken))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"payment_url": payURL})
		return
	}
	if o.PaymentMethod != "vnpay" || o.PaymentStatus == "paid" || o.Status == "cancelled" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Đơn hàng không cần thanh toán lại."})
		return
	}
	var sf models.Storefront
	if h.DB.First(&sf, o.StorefrontID).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy cửa hàng."})
		return
	}
	// VNPay không cho trùng vnp_TxnRef -> mỗi lần thanh toán lại dùng mã giao dịch mới
	o.VnpTxnRef = o.Code + "R" + randomDigits(6)
	base := h.allowedReturnBase(c.Query("return_base"), &sf)
	returnURL := h.orderReturnURL(base, &sf, o.Code, o.AccessToken)
	payURL, err := h.buildVnpURL(h.paymentSettings(sf.ID), o, c.ClientIP(), returnURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_ = h.DB.Model(&o).Updates(map[string]any{"payment_status": "pending", "vnp_txn_ref": o.VnpTxnRef}).Error
	c.JSON(http.StatusOK, gin.H{"payment_url": payURL})
}

// VnpayReturnCheck GET /api/v1/storefront/public/vnpay-return?<tham số VNPay>
// Chỉ kiểm tra chữ ký để hiển thị kết quả cho khách, KHÔNG cập nhật đơn (theo hướng dẫn VNPay).
func (h *StorefrontHandler) VnpayReturnCheck(c *gin.Context) {
	params := c.Request.URL.Query()
	var o ShopOrder
	if h.DB.Where("vnp_txn_ref = ? OR code = ?", params.Get("vnp_TxnRef"), params.Get("vnp_TxnRef")).First(&o).Error != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}
	ps := h.paymentSettings(o.StorefrontID)
	secret, _ := security.DecryptSecret(ps.VnpHashSecretEnc, h.encKey())
	valid := secret != "" && vnpVerify(secret, params)
	success := valid && params.Get("vnp_ResponseCode") == "00" && params.Get("vnp_TransactionStatus") == "00"
	c.JSON(http.StatusOK, gin.H{"valid": valid, "success": success, "response_code": params.Get("vnp_ResponseCode"), "payment_status": o.PaymentStatus})
}

// VnpayIPN GET /api/v1/payments/vnpay/ipn — VNPay gọi server-to-server
func (h *StorefrontHandler) VnpayIPN(c *gin.Context) {
	params := c.Request.URL.Query()
	var o ShopOrder
	if h.DB.Where("vnp_txn_ref = ? OR code = ?", params.Get("vnp_TxnRef"), params.Get("vnp_TxnRef")).First(&o).Error != nil {
		c.JSON(http.StatusOK, gin.H{"RspCode": "01", "Message": "Order not found"})
		return
	}
	ps := h.paymentSettings(o.StorefrontID)
	secret, _ := security.DecryptSecret(ps.VnpHashSecretEnc, h.encKey())
	if secret == "" || !vnpVerify(secret, params) {
		c.JSON(http.StatusOK, gin.H{"RspCode": "97", "Message": "Invalid signature"})
		return
	}
	amount, err := strconv.ParseInt(params.Get("vnp_Amount"), 10, 64)
	if err != nil || amount != o.Total*100 {
		c.JSON(http.StatusOK, gin.H{"RspCode": "04", "Message": "invalid amount"})
		return
	}
	if o.PaymentStatus == "paid" || o.PaymentStatus == "failed" {
		c.JSON(http.StatusOK, gin.H{"RspCode": "02", "Message": "Order already confirmed"})
		return
	}

	success := params.Get("vnp_ResponseCode") == "00" && params.Get("vnp_TransactionStatus") == "00"
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"vnp_trans_no":  params.Get("vnp_TransactionNo"),
			"vnp_bank_code": params.Get("vnp_BankCode"),
		}
		if success {
			now := time.Now()
			updates["payment_status"] = "paid"
			updates["paid_at"] = &now
			if !o.StockApplied {
				// Đã nhận tiền: vẫn trừ kho; nếu thiếu hàng thì chủ shop xử lý thủ công
				_ = applyStock(tx, o.Items, -1)
				updates["stock_applied"] = true
			}
		} else {
			updates["payment_status"] = "failed"
		}
		return tx.Model(&ShopOrder{}).Where("id = ?", o.ID).Updates(updates).Error
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"RspCode": "99", "Message": "Unknow error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"RspCode": "00", "Message": "Confirm Success"})
}

// PublicSubscribe POST /api/v1/storefront/public/subscribe {slug, email}
func (h *StorefrontHandler) PublicSubscribe(c *gin.Context) {
	if !allowOrder("sub:" + c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Thao tác quá nhanh, vui lòng thử lại sau."})
		return
	}
	var req struct {
		Slug  string `json:"slug"`
		Email string `json:"email"`
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
	addr, err := mail.ParseAddress(strings.TrimSpace(req.Email))
	if err != nil || len(addr.Address) > 160 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email không hợp lệ."})
		return
	}
	email := strings.ToLower(addr.Address)
	var n int64
	h.DB.Model(&SiteSubscriber{}).Where("storefront_id = ? AND email = ?", sf.ID, email).Count(&n)
	if n == 0 {
		_ = h.DB.Create(&SiteSubscriber{StorefrontID: sf.ID, Email: email}).Error
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ----------------------------------------------------------------- API chủ shop

// ListOrders GET /api/v1/storefront/sites/:id/orders?status=&q=
func (h *StorefrontHandler) ListOrders(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	q := h.DB.Where("storefront_id = ?", sf.ID)
	if st := c.Query("status"); st != "" && st != "all" {
		q = q.Where("status = ?", st)
	}
	if s := strings.TrimSpace(c.Query("q")); s != "" {
		like := "%" + s + "%"
		q = q.Where("code ILIKE ? OR phone ILIKE ? OR customer_name ILIKE ?", like, like, like)
	}
	var list []ShopOrder
	if err := q.Order("id DESC").Limit(300).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tải được đơn hàng."})
		return
	}
	type stat struct {
		Status string
		N      int64
	}
	var stats []stat
	h.DB.Model(&ShopOrder{}).Select("status, COUNT(*) AS n").Where("storefront_id = ?", sf.ID).Group("status").Scan(&stats)
	counts := gin.H{}
	for _, s := range stats {
		counts[s.Status] = s.N
	}
	c.JSON(http.StatusOK, gin.H{"orders": list, "counts": counts})
}

// UpdateOrderStatus PATCH /api/v1/storefront/sites/:id/orders/:oid {status}
func (h *StorefrontHandler) UpdateOrderStatus(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req struct {
		Status        string `json:"status"`
		PaymentStatus string `json:"payment_status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	valid := map[string]bool{"new": true, "confirmed": true, "shipping": true, "completed": true, "cancelled": true}
	if req.Status != "" && !valid[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trạng thái không hợp lệ."})
		return
	}
	var o ShopOrder
	if h.DB.Where("id = ? AND storefront_id = ?", c.Param("oid"), sf.ID).First(&o).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy đơn hàng."})
		return
	}
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{}
		if req.Status != "" && req.Status != o.Status {
			updates["status"] = req.Status
			// Huỷ đơn -> hoàn kho; mở lại đơn đã huỷ -> trừ kho lại
			if req.Status == "cancelled" && o.StockApplied {
				if err := applyStock(tx, o.Items, +1); err != nil {
					return err
				}
				updates["stock_applied"] = false
			} else if o.Status == "cancelled" && !o.StockApplied && (o.PaymentMethod == "cod" || o.PaymentStatus == "paid") {
				if err := applyStock(tx, o.Items, -1); err != nil {
					return err
				}
				updates["stock_applied"] = true
			}
		}
		// Đơn COD: chủ shop đánh dấu đã thu tiền
		if req.PaymentStatus == "paid" && o.PaymentMethod == "cod" && o.PaymentStatus != "paid" {
			now := time.Now()
			updates["payment_status"] = "paid"
			updates["paid_at"] = &now
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&ShopOrder{}).Where("id = ?", o.ID).Updates(updates).Error
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	h.DB.First(&o, o.ID)
	c.JSON(http.StatusOK, gin.H{"order": o})
}

// GetPaymentSettings GET /api/v1/storefront/sites/:id/payment-settings
func (h *StorefrontHandler) GetPaymentSettings(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	ps := h.paymentSettings(sf.ID)
	ipn := publicBaseURL(c) + "/api/v1/payments/vnpay/ipn"
	c.JSON(http.StatusOK, gin.H{"settings": ps, "vnp_secret_set": ps.VnpHashSecretEnc != "", "vnp_ipn_url": ipn,
		"paypal_secret_set": ps.PaypalSecretEnc != "", "stripe_secret_set": ps.StripeSecretEnc != "",
		"stripe_webhook_set": ps.StripeWebhookSecretEnc != "",
		"stripe_webhook_url": fmt.Sprintf("%s/api/v1/payments/stripe/webhook/%d", publicBaseURL(c), sf.ID)})
}

// SavePaymentSettings PUT /api/v1/storefront/sites/:id/payment-settings
func (h *StorefrontHandler) SavePaymentSettings(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req struct {
		CODEnabled    bool   `json:"cod_enabled"`
		VnpEnabled    bool   `json:"vnp_enabled"`
		VnpTmnCode    string `json:"vnp_tmn_code"`
		VnpHashSecret string `json:"vnp_hash_secret"` // để trống = giữ khoá cũ
		VnpSandbox    bool   `json:"vnp_sandbox"`
		ShippingFee   int64  `json:"shipping_fee"`
		FreeShipFrom  int64  `json:"free_ship_from"`
		// Quốc tế (để trống khoá bí mật = giữ khoá cũ)
		PaypalEnabled       bool   `json:"paypal_enabled"`
		PaypalClientID      string `json:"paypal_client_id"`
		PaypalSecret        string `json:"paypal_secret"`
		PaypalSandbox       bool   `json:"paypal_sandbox"`
		StripeEnabled       bool   `json:"stripe_enabled"`
		StripeSecret        string `json:"stripe_secret"`
		StripeWebhookSecret string `json:"stripe_webhook_secret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	ps := h.paymentSettings(sf.ID)
	ps.StorefrontID = sf.ID
	ps.CODEnabled = req.CODEnabled
	ps.VnpSandbox = req.VnpSandbox
	ps.VnpTmnCode = strings.TrimSpace(req.VnpTmnCode)
	if ps.VnpTmnCode != "" && !tmnCodeRe.MatchString(ps.VnpTmnCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã website VNPay (TmnCode) không hợp lệ."})
		return
	}
	if s := strings.TrimSpace(req.VnpHashSecret); s != "" {
		enc, err := security.EncryptSecret(s, h.encKey())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không mã hoá được khoá bí mật."})
			return
		}
		ps.VnpHashSecretEnc = enc
	}
	if req.VnpEnabled && (ps.VnpTmnCode == "" || ps.VnpHashSecretEnc == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cần nhập TmnCode và Hash Secret để bật VNPay."})
		return
	}
	ps.VnpEnabled = req.VnpEnabled

	// PayPal
	ps.PaypalClientID = strings.TrimSpace(req.PaypalClientID)
	ps.PaypalSandbox = req.PaypalSandbox
	if s := strings.TrimSpace(req.PaypalSecret); s != "" {
		enc, err := security.EncryptSecret(s, h.encKey())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không mã hoá được khoá PayPal."})
			return
		}
		ps.PaypalSecretEnc = enc
	}
	if req.PaypalEnabled && (ps.PaypalClientID == "" || ps.PaypalSecretEnc == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cần nhập Client ID và Secret để bật PayPal."})
		return
	}
	ps.PaypalEnabled = req.PaypalEnabled
	if ps.PaypalEnabled && strings.TrimSpace(req.PaypalSecret) != "" {
		if _, err := h.paypalToken(ps); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Stripe
	if s := strings.TrimSpace(req.StripeSecret); s != "" {
		if !strings.HasPrefix(s, "sk_") && !strings.HasPrefix(s, "rk_") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Khoá Stripe phải là Secret key (bắt đầu bằng sk_live_ hoặc sk_test_)."})
			return
		}
		enc, err := security.EncryptSecret(s, h.encKey())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không mã hoá được khoá Stripe."})
			return
		}
		ps.StripeSecretEnc = enc
	}
	if s := strings.TrimSpace(req.StripeWebhookSecret); s != "" {
		enc, err := security.EncryptSecret(s, h.encKey())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không mã hoá được khoá webhook Stripe."})
			return
		}
		ps.StripeWebhookSecretEnc = enc
	}
	if req.StripeEnabled && ps.StripeSecretEnc == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cần nhập Secret key để bật Stripe."})
		return
	}
	ps.StripeEnabled = req.StripeEnabled

	if !ps.CODEnabled && !ps.VnpEnabled && !ps.PaypalEnabled && !ps.StripeEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cần bật ít nhất một cách thanh toán."})
		return
	}
	if req.ShippingFee < 0 || req.ShippingFee > 10000000 || req.FreeShipFrom < 0 || req.FreeShipFrom > 10000000000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phí vận chuyển không hợp lệ."})
		return
	}
	ps.ShippingFee = req.ShippingFee
	ps.FreeShipFrom = req.FreeShipFrom
	if err := h.DB.Save(&ps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu cài đặt thất bại."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": ps, "vnp_secret_set": ps.VnpHashSecretEnc != "",
		"paypal_secret_set": ps.PaypalSecretEnc != "", "stripe_secret_set": ps.StripeSecretEnc != "",
		"stripe_webhook_set": ps.StripeWebhookSecretEnc != ""})
}

// ListSubscribers GET /api/v1/storefront/sites/:id/subscribers
func (h *StorefrontHandler) ListSiteSubscribers(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var list []SiteSubscriber
	h.DB.Where("storefront_id = ?", sf.ID).Order("id DESC").Limit(2000).Find(&list)
	c.JSON(http.StatusOK, gin.H{"subscribers": list})
}
