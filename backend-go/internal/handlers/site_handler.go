package handlers

// Multi-site: mỗi tài khoản có thể tạo nhiều website bán hàng.
// Dùng lại model Storefront (không cần migration mới).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxSitesPerUser = 5 // TODO: đưa vào gói dịch vụ / config
	maxBlocks       = 50
	maxBlocksBytes  = 512 * 1024
	maxHTMLBytes    = 50 * 1024
)

var (
	slugRe        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	colorRe       = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	reservedSlugs = map[string]bool{
		"api": true, "admin": true, "login": true, "register": true, "auth": true,
		"store": true, "sites": true, "s": true, "uploads": true, "health": true,
		"inbox": true, "pages": true, "posts": true, "roles": true, "broadcasts": true,
	}
	allowedBlockTypes = map[string]bool{
		"header": true, "hero": true, "products": true,
		"features": true, "testimonials": true, "cta": true,
		"richtext": true, "image": true, "gallery": true,
		"faq": true, "contact": true, "html": true,
		"announcement": true, "banner": true, "image_text": true,
		"collections": true, "icon_strip": true, "newsletter": true, "footer": true,
	}
)

// ---------- helpers ----------

var viTable = []struct {
	base  rune
	chars string
}{
	{'a', "àáạảãâầấậẩẫăằắặẳẵ"}, {'e', "èéẹẻẽêềếệểễ"}, {'i', "ìíịỉĩ"},
	{'o', "òóọỏõôồốộổỗơờớợởỡ"}, {'u', "ùúụủũưừứựửữ"}, {'y', "ỳýỵỷỹ"}, {'d', "đ"},
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := true
	for _, r := range s {
		for _, t := range viTable {
			if strings.ContainsRune(t.chars, r) {
				r = t.base
				break
			}
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	return out
}

func validateSlug(slug string) string {
	switch {
	case len(slug) < 3 || len(slug) > 40:
		return "Đường dẫn phải dài 3–40 ký tự."
	case !slugRe.MatchString(slug):
		return "Đường dẫn chỉ gồm chữ thường không dấu, số và dấu gạch ngang."
	case reservedSlugs[slug]:
		return "Đường dẫn này được hệ thống giữ chỗ, hãy chọn tên khác."
	}
	return ""
}

func (h *StorefrontHandler) slugTaken(slug string, exceptID uint) bool {
	var n int64
	h.DB.Model(&models.Storefront{}).Where("slug = ? AND id <> ?", slug, exceptID).Count(&n)
	return n > 0
}

func (h *StorefrontHandler) uniqueSlug(base string) string {
	if len(base) < 3 {
		base = "shop-" + base
	}
	slug := base
	for i := 2; h.slugTaken(slug, 0) || validateSlug(slug) != ""; i++ {
		slug = fmt.Sprintf("%s-%d", base, i)
		if i > 500 {
			break
		}
	}
	return slug
}

// safeLink chỉ cho phép http(s), mailto, tel, #anchor và đường dẫn tương đối.
func safeLink(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" || strings.HasPrefix(v, "#") || (strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//")) {
		return v
	}
	u, err := url.Parse(v)
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto", "tel":
		return v
	}
	return ""
}

// safeMediaURL chỉ cho phép http(s) và đường dẫn tương đối (ví dụ /uploads/...).
func safeMediaURL(raw string) string {
	v := strings.TrimSpace(raw)
	if strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") {
		return v
	}
	u, err := url.Parse(v)
	if err != nil {
		return ""
	}
	if s := strings.ToLower(u.Scheme); s == "http" || s == "https" {
		return v
	}
	return ""
}

func sanitizeValue(key string, v interface{}) interface{} {
	switch t := v.(type) {
	case string:
		k := strings.ToLower(key)
		if strings.HasSuffix(k, "_link") || k == "link" || k == "href" {
			return safeLink(t)
		}
		if strings.HasSuffix(k, "_url") || k == "url" {
			return safeMediaURL(t)
		}
		return t
	case map[string]interface{}:
		for kk, vv := range t {
			t[kk] = sanitizeValue(kk, vv)
		}
		return t
	case []interface{}:
		for i := range t {
			t[i] = sanitizeValue(key, t[i])
		}
		return t
	}
	return v
}

// normalizeStyle chỉ giữ màu hex hợp lệ và mức khoảng cách cho phép.
func normalizeStyle(data map[string]interface{}) {
	raw, ok := data["style"].(map[string]interface{})
	if !ok {
		delete(data, "style")
		return
	}
	clean := map[string]interface{}{}
	if bg, ok := raw["bg"].(string); ok && colorRe.MatchString(bg) {
		clean["bg"] = bg
	}
	if pad, ok := raw["pad"].(string); ok && (pad == "sm" || pad == "md" || pad == "lg") {
		clean["pad"] = pad
	}
	if css, ok := raw["css"].(string); ok {
		if css = cleanSectionCSS(css); css != "" {
			clean["css"] = css
		}
	}
	data["style"] = clean
}

var (
	cssStyleTagRe = regexp.MustCompile(`(?i)</?\s*style`)
	cssImportRe   = regexp.MustCompile(`(?i)@import[^;]*;?`)
	cssExprRe     = regexp.MustCompile(`(?i)expression\s*\(`)
)

// cleanSectionCSS: CSS tuỳ chỉnh của section — giới hạn 20KB, bỏ thẻ style / @import / expression().
// Frontend ghi CSS này dạng chữ thuần vào thẻ <style>, bọc trong selector của section.
func cleanSectionCSS(css string) string {
	if len(css) > 20000 {
		css = css[:20000]
	}
	css = cssStyleTagRe.ReplaceAllString(css, "")
	css = cssImportRe.ReplaceAllString(css, "")
	css = cssExprRe.ReplaceAllString(css, "")
	return strings.TrimSpace(css)
}

// normalizeHeader làm sạch cấu hình section Đầu trang.
func normalizeHeader(data map[string]interface{}) {
	// Menu: nhận chuỗi cũ hoặc {label, link}, tối đa 12 mục
	items := []interface{}{}
	if arr, ok := data["nav_items"].([]interface{}); ok {
		for _, it := range arr {
			if len(items) >= 12 {
				break
			}
			var label, link string
			switch v := it.(type) {
			case string:
				label = v
			case map[string]interface{}:
				label, _ = v["label"].(string)
				link, _ = v["link"].(string)
			}
			label = strings.TrimSpace(label)
			if label == "" {
				continue
			}
			if r := []rune(label); len(r) > 40 {
				label = string(r[:40])
			}
			items = append(items, map[string]interface{}{"label": label, "link": link})
		}
	}
	data["nav_items"] = items

	pick := func(key, def string, allowed ...string) {
		s, _ := data[key].(string)
		for _, a := range allowed {
			if s == a {
				return
			}
		}
		data[key] = def
	}
	pick("layout", "left", "left", "left_menu", "split", "center", "center_split")
	pick("account_style", "buttons", "buttons", "icon")
	pick("brand_size", "md", "sm", "md", "lg")
	pick("right_mode", "cta", "cta", "account")

	for _, k := range []string{"show_brand", "show_logo", "show_cta", "sticky", "border"} {
		if _, ok := data[k].(bool); !ok {
			delete(data, k)
		}
	}

	h, _ := data["logo_height"].(float64)
	if h < 20 {
		h = 40
	} else if h > 120 {
		h = 120
	}
	data["logo_height"] = h

	if tc, ok := data["text_color"].(string); !ok || !colorRe.MatchString(tc) {
		delete(data, "text_color")
	}
}

// sanitizeBlocksJSON kiểm tra loại block, giới hạn kích thước và lọc các link/URL nguy hiểm.
func sanitizeBlocksJSON(raw string) (string, error) {
	if len(raw) > maxBlocksBytes {
		return "", fmt.Errorf("Nội dung trang quá lớn (tối đa %d KB).", maxBlocksBytes/1024)
	}
	var blocks []StorefrontBlock
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		return "", fmt.Errorf("Cấu trúc danh sách block JSON không hợp lệ.")
	}
	if len(blocks) > maxBlocks {
		return "", fmt.Errorf("Tối đa %d block mỗi trang.", maxBlocks)
	}
	for i := range blocks {
		if !allowedBlockTypes[blocks[i].Type] {
			return "", fmt.Errorf("Loại block không được hỗ trợ: %q.", blocks[i].Type)
		}
		if blocks[i].Data == nil {
			blocks[i].Data = map[string]interface{}{}
		}
		for k, v := range blocks[i].Data {
			blocks[i].Data[k] = sanitizeValue(k, v)
		}

		normalizeStyle(blocks[i].Data)

		if blocks[i].Type == "header" {
			normalizeHeader(blocks[i].Data)
		}

		if blocks[i].Type == "html" {
			if s, _ := blocks[i].Data["html"].(string); len(s) > maxHTMLBytes {
				return "", fmt.Errorf("Khối HTML tối đa %d KB.", maxHTMLBytes/1024)
			}
			ht, _ := blocks[i].Data["height"].(float64)
			if ht < 80 {
				ht = 300
			} else if ht > 2000 {
				ht = 2000
			}
			blocks[i].Data["height"] = ht
		}
	}
	out, err := json.Marshal(blocks)
	return string(out), err
}

func (h *StorefrontHandler) loadOwned(c *gin.Context) (*models.Storefront, bool) {
	uid, ok := extractUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Không xác định được tài khoản người dùng."})
		return nil, false
	}
	var sf models.Storefront
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), uid).First(&sf).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy website."})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tải được website."})
		}
		return nil, false
	}
	return &sf, true
}

// ---------- handlers ----------

// ListSites GET /api/v1/storefront/sites
func (h *StorefrontHandler) ListSites(c *gin.Context) {
	uid, ok := extractUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Không xác định được tài khoản người dùng."})
		return
	}
	var sites []models.Storefront
	if err := h.DB.Where("user_id = ?", uid).Order("id ASC").Find(&sites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tải được danh sách website."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sites": sites, "max_sites": maxSitesPerUser})
}

type createSiteReq struct {
	SiteTitle string `json:"site_title"`
	Slug      string `json:"slug"`
	Template  string `json:"template"` // "" = mặc định, "wellness" = Mỹ phẩm / Sức khoẻ
}

// CreateSite POST /api/v1/storefront/sites
func (h *StorefrontHandler) CreateSite(c *gin.Context) {
	uid, ok := extractUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Không xác định được tài khoản người dùng."})
		return
	}
	var req createSiteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	title := strings.TrimSpace(req.SiteTitle)
	if title == "" || len([]rune(title)) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên website bắt buộc và tối đa 120 ký tự."})
		return
	}

	var count int64
	h.DB.Model(&models.Storefront{}).Where("user_id = ?", uid).Count(&count)
	if count >= maxSitesPerUser {
		c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Bạn đã đạt giới hạn %d website.", maxSitesPerUser)})
		return
	}

	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		slug = h.uniqueSlug(slugify(title))
	} else {
		if msg := validateSlug(slug); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		if h.slugTaken(slug, 0) {
			c.JSON(http.StatusConflict, gin.H{"error": "Đường dẫn đã có người dùng."})
			return
		}
	}

	blocks := defaultBlocksJSON()
	color := "#059669"
	tplJSON, tpl, useTpl := templateBlocksJSON(req.Template)
	if useTpl {
		blocks = tplJSON
		color = tpl.PrimaryColor
	}
	sf := models.Storefront{
		UserID:              uid,
		Domain:              "", // site mới truy cập qua /s/:slug; domain riêng gắn sau
		Slug:                slug,
		SiteTitle:           title,
		PrimaryColor:        color,
		DraftBlocksJSON:     blocks,
		PublishedBlocksJSON: blocks,
		IsPublished:         false,
	}
	if err := h.DB.Create(&sf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo website thất bại."})
		return
	}
	if useTpl {
		createTemplateProducts(h.DB, &sf, tpl)
	}
	c.JSON(http.StatusCreated, gin.H{"site": sf})
}

// GetSite GET /api/v1/storefront/sites/:id
func (h *StorefrontHandler) GetSite(c *gin.Context) {
	if sf, ok := h.loadOwned(c); ok {
		c.JSON(http.StatusOK, gin.H{"site": sf})
	}
}

// SaveSiteDraft PUT /api/v1/storefront/sites/:id/draft
func (h *StorefrontHandler) SaveSiteDraft(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req saveStorefrontDraftReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu lưu nháp không hợp lệ."})
		return
	}
	if t := strings.TrimSpace(req.SiteTitle); t != "" {
		sf.SiteTitle = t
	}
	sf.LogoURL = safeMediaURL(req.LogoURL)
	if col := strings.TrimSpace(req.PrimaryColor); colorRe.MatchString(col) {
		sf.PrimaryColor = col
	}
	if strings.TrimSpace(req.BlocksJSON) != "" {
		clean, err := sanitizeBlocksJSON(req.BlocksJSON)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sf.DraftBlocksJSON = clean
	}
	if req.ThemeJSON != nil {
		sf.DraftThemeJSON = sanitizeThemeJSON(*req.ThemeJSON)
	}
	if req.I18nJSON != nil {
		sf.DraftI18nJSON = sanitizeI18nJSON(*req.I18nJSON)
	}
	if err := h.DB.Save(sf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu bản nháp thất bại."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã lưu bản nháp.", "site": sf})
}

// PublishSite POST /api/v1/storefront/sites/:id/publish
func (h *StorefrontHandler) PublishSite(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	if strings.TrimSpace(sf.DraftBlocksJSON) == "" {
		sf.DraftBlocksJSON = defaultBlocksJSON()
	}
	clean, err := sanitizeBlocksJSON(sf.DraftBlocksJSON)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sf.PublishedBlocksJSON = clean
	sf.PublishedThemeJSON = sanitizeThemeJSON(sf.DraftThemeJSON)
	sf.PublishedI18nJSON = sanitizeI18nJSON(sf.DraftI18nJSON)
	sf.IsPublished = true
	if err := h.DB.Save(sf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xuất bản thất bại."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã xuất bản website.", "site": sf})
}

// UnpublishSite POST /api/v1/storefront/sites/:id/unpublish
func (h *StorefrontHandler) UnpublishSite(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	sf.IsPublished = false
	if err := h.DB.Save(sf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gỡ xuất bản thất bại."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã gỡ website khỏi internet.", "site": sf})
}

// DeleteSite DELETE /api/v1/storefront/sites/:id
func (h *StorefrontHandler) DeleteSite(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	// Xóa sản phẩm thuộc website này trước
	h.releaseDomainOnDelete(sf)
	h.DB.Where("storefront_id = ?", sf.ID).Delete(&models.Product{})
	if err := h.DB.Delete(sf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa website thất bại."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã xóa website."})
}
