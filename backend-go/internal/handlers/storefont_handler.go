package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StorefrontHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewStorefrontHandler(db *gorm.DB, cfg *config.Config) *StorefrontHandler {
	_ = db.AutoMigrate(&models.Storefront{}, &models.Product{})
	migrateSiteMedia(db)
	migrateShopTables(db)
	return &StorefrontHandler{
		DB:  db,
		Cfg: cfg,
	}
}

type StorefrontBlock struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"` // header | hero | products | features | testimonials | cta
	Enabled bool                   `json:"enabled"`
	Data    map[string]interface{} `json:"data"`
}

func defaultDomainFromConfig(cfg *config.Config, fallbackHost string) string {
	if cfg != nil && strings.TrimSpace(cfg.FrontendURL) != "" {
		raw := strings.TrimSpace(cfg.FrontendURL)
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			return u.Host
		}
		clean := strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
		clean = strings.Split(clean, "/")[0]
		if clean != "" {
			return clean
		}
	}
	return normalizeHost(fallbackHost)
}

func normalizeHost(rawHost string) string {
	h := strings.ToLower(strings.TrimSpace(rawHost))
	if idx := strings.Index(h, ":"); idx != -1 {
		h = h[:idx]
	}
	return h
}

func defaultBlocksJSON() string {
	blocks := []StorefrontBlock{
		{
			ID:      "blk_header",
			Type:    "header",
			Enabled: true,
			Data: map[string]interface{}{
				"brand_name": "Cửa hàng của tôi",
				"nav_items":  []string{"Trang chủ", "Sản phẩm", "Ưu đãi", "Liên hệ"},
				"cta_label":  "Đặt mua ngay",
				"cta_link":   "#products",
			},
		},
		{
			ID:      "blk_hero",
			Type:    "hero",
			Enabled: true,
			Data: map[string]interface{}{
				"badge":     "Hàng chính hãng • Giao nhanh",
				"title":     "Nâng tầm trải nghiệm mua sắm trực tiếp trên tên miền của bạn",
				"subtitle":  "Tùy chỉnh từng khối nội dung, hình ảnh, giá bán và xuất bản ngay lập tức.",
				"image_url": "",
				"cta_label": "Khám phá sản phẩm",
				"cta_link":  "#products",
			},
		},
		{
			ID:      "blk_products",
			Type:    "products",
			Enabled: true,
			Data: map[string]interface{}{
				"heading": "Sản phẩm nổi bật",
				"source":  "catalog",
				"items": []map[string]interface{}{
					{
						"id":        "p1",
						"name":      "Combo Sản Phẩm Mẫu #1",
						"price":     "490.000đ",
						"image_url": "",
						"desc":      "Mô tả ngắn gọn điểm nổi bật và lợi ích sản phẩm.",
					},
					{
						"id":        "p2",
						"name":      "Combo Sản Phẩm Mẫu #2",
						"price":     "790.000đ",
						"image_url": "",
						"desc":      "Phiên bản cao cấp phù hợp tặng quà hoặc sử dụng lâu dài.",
					},
				},
			},
		},
		{
			ID:      "blk_features",
			Type:    "features",
			Enabled: true,
			Data: map[string]interface{}{
				"heading": "Cam kết dịch vụ",
				"items": []string{
					"Kiểm tra hàng trước khi thanh toán",
					"Đổi trả linh hoạt trong 7 ngày",
					"Hỗ trợ tư vấn trực tiếp nhanh chóng",
				},
			},
		},
		{
			ID:      "blk_cta",
			Type:    "cta",
			Enabled: true,
			Data: map[string]interface{}{
				"title":       "Sẵn sàng ra mắt trang bán hàng của bạn?",
				"subtitle":    "Chỉnh sửa trực tiếp trên trình soạn thảo và bấm Xuất bản để cập nhật ngay.",
				"button_text": "Liên hệ tư vấn",
				"button_link": "#contact",
				"footer_note": "© Bản quyền thuộc tên miền của bạn.",
			},
		},
	}
	b, _ := json.Marshal(blocks)
	return string(b)
}

func (h *StorefrontHandler) getOrCreateForUser(userID uint, requestHost string) (*models.Storefront, error) {
	var sf models.Storefront
	err := h.DB.Where("user_id = ?", userID).Order("id ASC").First(&sf).Error
	if err == nil {
		return &sf, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	defDomain := defaultDomainFromConfig(h.Cfg, requestHost)
	blocks := defaultBlocksJSON()
	sf = models.Storefront{
		UserID:              userID,
		Domain:              defDomain,
		Slug:                fmt.Sprintf("store-%d-%d", userID, time.Now().Unix()%10000),
		SiteTitle:           "Cửa hàng của tôi",
		PrimaryColor:        "#059669",
		DraftBlocksJSON:     blocks,
		PublishedBlocksJSON: blocks,
		IsPublished:         true,
	}
	if createErr := h.DB.Create(&sf).Error; createErr != nil {
		return nil, createErr
	}
	return &sf, nil
}

func extractUserID(c *gin.Context) (uint, bool) {
	for _, key := range []string{"user_id", "userID", "uid"} {
		if val, exists := c.Get(key); exists {
			switch v := val.(type) {
			case uint:
				return v, true
			case int:
				if v > 0 {
					return uint(v), true
				}
			case int64:
				if v > 0 {
					return uint(v), true
				}
			case float64:
				if v > 0 {
					return uint(v), true
				}
			}
		}
	}
	return 0, false
}

// GetMyStorefront GET /api/v1/storefront
func (h *StorefrontHandler) GetMyStorefront(c *gin.Context) {
	uid, ok := extractUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Không xác định được tài khoản người dùng."})
		return
	}

	sf, err := h.getOrCreateForUser(uid, c.Request.Host)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tải được cấu hình trang bán hàng."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"storefront": sf})
}

type saveStorefrontDraftReq struct {
	Domain       string `json:"domain"`
	SiteTitle    string `json:"site_title"`
	LogoURL      string `json:"logo_url"`
	PrimaryColor string `json:"primary_color"`
	BlocksJSON   string `json:"draft_blocks_json"`
	ThemeJSON    *string `json:"draft_theme_json"` // nil = không đổi
}

// SaveDraft PUT /api/v1/storefront/draft
func (h *StorefrontHandler) SaveDraft(c *gin.Context) {
	uid, ok := extractUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Không xác định được tài khoản người dùng."})
		return
	}

	var req saveStorefrontDraftReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu lưu nháp không hợp lệ."})
		return
	}

	sf, err := h.getOrCreateForUser(uid, c.Request.Host)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tìm thấy cấu hình trang bán hàng."})
		return
	}

	if strings.TrimSpace(req.Domain) != "" {
		sf.Domain = normalizeHost(req.Domain)
	}
	if strings.TrimSpace(req.SiteTitle) != "" {
		sf.SiteTitle = strings.TrimSpace(req.SiteTitle)
	}
	sf.LogoURL = strings.TrimSpace(req.LogoURL)
	if strings.TrimSpace(req.PrimaryColor) != "" {
		sf.PrimaryColor = strings.TrimSpace(req.PrimaryColor)
	}
	if strings.TrimSpace(req.BlocksJSON) != "" {
		clean, sanErr := sanitizeBlocksJSON(req.BlocksJSON)
		if sanErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": sanErr.Error()})
			return
		}
		sf.DraftBlocksJSON = clean
	}

	if err := h.DB.Save(sf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu bản nháp thất bại."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Đã lưu bản nháp trang bán hàng.",
		"storefront": sf,
	})
}

// Publish POST /api/v1/storefront/publish
func (h *StorefrontHandler) Publish(c *gin.Context) {
	uid, ok := extractUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Không xác định được tài khoản người dùng."})
		return
	}

	sf, err := h.getOrCreateForUser(uid, c.Request.Host)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tìm thấy cấu hình trang bán hàng."})
		return
	}

	if strings.TrimSpace(sf.DraftBlocksJSON) == "" {
		sf.DraftBlocksJSON = defaultBlocksJSON()
	}
	cleanPub, sanErr := sanitizeBlocksJSON(sf.DraftBlocksJSON)
	if sanErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": sanErr.Error()})
		return
	}
	sf.PublishedBlocksJSON = cleanPub
	sf.IsPublished = true

	if err := h.DB.Save(sf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xuất bản trang bán hàng thất bại."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    fmt.Sprintf("Đã xuất bản lên tên miền %s.", sf.Domain),
		"storefront": sf,
	})
}

// GetPublicStorefront GET /api/v1/storefront/public?slug=... hoặc ?domain=... (hoặc tự đọc Host)
// Multi-tenant: nếu có slug mà không thấy thì trả 404, KHÔNG fallback sang site của người khác.
func (h *StorefrontHandler) GetPublicStorefront(c *gin.Context) {
	qSlug := strings.ToLower(strings.TrimSpace(c.Query("slug")))
	qDomain := normalizeHost(c.Query("domain"))
	if qDomain == "" {
		qDomain = normalizeHost(c.Request.Host)
	}

	var sf models.Storefront
	found := false

	if qSlug != "" {
		found = h.DB.Where("slug = ? AND is_published = ?", qSlug, true).First(&sf).Error == nil
	} else if qDomain != "" {
		found = h.DB.Where("LOWER(domain) = ? AND is_published = ?", qDomain, true).
			Order("id ASC").First(&sf).Error == nil
		// Tương thích cũ: trang /store trên chính domain của hệ thống hiển thị site published đầu tiên.
		if !found && qDomain == defaultDomainFromConfig(h.Cfg, "") {
			found = h.DB.Where("is_published = ?", true).Order("id ASC").First(&sf).Error == nil
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chưa có trang bán hàng nào được xuất bản."})
		return
	}

	// Danh sách sản phẩm đang bán (không lộ số lượng tồn kho, chỉ còn/hết hàng)
	var prods []models.Product
	h.DB.Where("storefront_id = ? AND is_active = ?", sf.ID, true).
		Order("sort_order ASC, id ASC").Find(&prods)
	pub := make([]gin.H, 0, len(prods))
	for _, p := range prods {
		pub = append(pub, gin.H{
			"id":            p.ID,
			"name":          p.Name,
			"description":   p.Description,
			"price":         p.Price,
			"compare_price": p.ComparePrice,
			"image_url":     p.ImageURL,
			"in_stock":      p.Stock != 0,
		})
	}

	c.Header("Cache-Control", "public, max-age=30")
	c.JSON(http.StatusOK, gin.H{
		"id":            sf.ID,
		"slug":          sf.Slug,
		"site_title":    sf.SiteTitle,
		"logo_url":      sf.LogoURL,
		"primary_color": sf.PrimaryColor,
		"blocks_json":   sf.PublishedBlocksJSON,
		"theme_json":    sf.PublishedThemeJSON,
		"products":      pub,
		"updated_at":    sf.UpdatedAt,
	})
}
