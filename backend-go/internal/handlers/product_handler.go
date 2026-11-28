package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxProductsPerSite  = 200
	maxProductMedia     = 20
	maxProductDescChars = 200000
)

type productReq struct {
	Name         string                `json:"name"`
	Description  string                `json:"description"`
	Price        int64                 `json:"price"`
	ComparePrice int64                 `json:"compare_price"`
	ImageURL     string                `json:"image_url"` // bản cũ chỉ gửi 1 ảnh
	Media        []models.ProductMedia `json:"media"`
	Stock        int                   `json:"stock"`
	IsActive     *bool                 `json:"is_active"`
	SortOrder    int                   `json:"sort_order"`
	SKU          string                `json:"sku"`
	Barcode      string                `json:"barcode"`
	CostPrice    int64                 `json:"cost_price"`
	SourceURL    string                `json:"source_url"`
	Tags         string                `json:"tags"`
}

// ----------------------------------------------------------------- Video / nhúng

var (
	ytIDRe      = regexp.MustCompile(`^[A-Za-z0-9_-]{6,20}$`)
	vimeoIDRe   = regexp.MustCompile(`^\d{5,12}$`)
	videoExtRe  = regexp.MustCompile(`(?i)\.(mp4|webm|mov|m4v)$`)
	youtubeHost = map[string]bool{"youtube.com": true, "www.youtube.com": true, "m.youtube.com": true, "youtube-nocookie.com": true, "www.youtube-nocookie.com": true}
)

// parseVideoLink nhận một link video bất kỳ: YouTube / Vimeo → trang nhúng; file .mp4/.webm/... → video trực tiếp.
func parseVideoLink(raw string) (typ, link, poster string, ok bool) {
	v := safeMediaURL(raw)
	if v == "" || strings.HasPrefix(v, "/") { // video luôn là địa chỉ tuyệt đối (kho lưu trữ / link ngoài)
		return "", "", "", false
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return "", "", "", false
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case youtubeHost[host]:
		id := u.Query().Get("v")
		if id == "" {
			segs := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(segs) >= 2 && (segs[0] == "embed" || segs[0] == "shorts" || segs[0] == "live" || segs[0] == "v") {
				id = segs[1]
			}
		}
		if ytIDRe.MatchString(id) {
			return "embed", "https://www.youtube-nocookie.com/embed/" + id, "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg", true
		}
	case host == "youtu.be":
		id := strings.Trim(u.Path, "/")
		if ytIDRe.MatchString(id) {
			return "embed", "https://www.youtube-nocookie.com/embed/" + id, "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg", true
		}
	case host == "vimeo.com" || host == "www.vimeo.com" || host == "player.vimeo.com":
		segs := strings.Split(strings.Trim(u.Path, "/"), "/")
		id := segs[len(segs)-1]
		if len(segs) >= 2 && segs[0] == "video" {
			id = segs[1]
		}
		if vimeoIDRe.MatchString(id) {
			return "embed", "https://player.vimeo.com/video/" + id, "", true
		}
	}
	if videoExtRe.MatchString(path.Base(u.Path)) {
		return "video", v, "", true
	}
	return "", "", "", false
}

// cleanProductMedia lọc bộ sưu tập: chỉ giữ URL an toàn, tối đa maxProductMedia, đảm bảo có ảnh bìa nếu có thể.
func cleanProductMedia(in []models.ProductMedia, legacyImage string) []models.ProductMedia {
	out := make([]models.ProductMedia, 0, len(in))
	for _, m := range in {
		if len(out) >= maxProductMedia {
			break
		}
		alt := truncRunes(strings.TrimSpace(m.Alt), 200)
		if strings.EqualFold(m.Type, "image") {
			if u := safeMediaURL(m.URL); u != "" {
				out = append(out, models.ProductMedia{Type: "image", URL: u, Alt: alt})
			}
			continue
		}
		if typ, link, poster, ok := parseVideoLink(m.URL); ok {
			if p := safeMediaURL(m.Poster); p != "" {
				poster = p
			}
			out = append(out, models.ProductMedia{Type: typ, URL: link, Poster: poster, Alt: alt})
		}
	}
	if len(out) == 0 {
		if u := safeMediaURL(legacyImage); u != "" {
			out = append(out, models.ProductMedia{Type: "image", URL: u})
		}
	}
	return out
}

// coverOf: ảnh bìa = ảnh đầu tiên; chỉ có video thì dùng ảnh bìa của video
func coverOf(media []models.ProductMedia) string {
	for _, m := range media {
		if m.Type == "image" {
			return m.URL
		}
	}
	for _, m := range media {
		if m.Poster != "" {
			return m.Poster
		}
	}
	return ""
}

func cleanTags(raw string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == ';' }) {
		t = truncRunes(strings.TrimSpace(t), 40)
		k := strings.ToLower(t)
		if t == "" || seen[k] || len(out) >= 20 {
			continue
		}
		seen[k] = true
		out = append(out, t)
	}
	return strings.Join(out, ", ")
}

func cleanSourceURL(raw string) string {
	v := strings.TrimSpace(raw)
	if len(v) > 2000 {
		return ""
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return v
}

// normalize làm sạch dữ liệu và trả về thông báo lỗi (rỗng nếu hợp lệ).
func (r *productReq) normalize() string {
	r.Name = strings.TrimSpace(r.Name)
	switch {
	case r.Name == "" || len([]rune(r.Name)) > 255:
		return "Tên sản phẩm bắt buộc và tối đa 255 ký tự."
	case len(r.Description) > 3*maxProductDescChars:
		return "Mô tả quá dài."
	case r.Price < 0 || r.Price > 10000000000:
		return "Giá bán không hợp lệ."
	case r.ComparePrice < 0 || r.ComparePrice > 10000000000:
		return "Giá gốc không hợp lệ."
	case r.CostPrice < 0 || r.CostPrice > 10000000000:
		return "Giá nhập không hợp lệ."
	case r.Stock < -1 || r.Stock > 1000000:
		return "Tồn kho không hợp lệ (dùng -1 nếu không giới hạn)."
	}
	desc := strings.TrimSpace(r.Description)
	if desc != "" && !looksLikeHTML(desc) {
		desc = plainToRichHTML(desc)
	}
	r.Description = sanitizeRichHTML(desc)
	if len(r.Description) > maxProductDescChars {
		return "Mô tả quá dài (tối đa 200.000 ký tự)."
	}
	r.Media = cleanProductMedia(r.Media, r.ImageURL)
	r.ImageURL = coverOf(r.Media)
	r.SKU = truncRunes(strings.TrimSpace(r.SKU), 100)
	r.Barcode = truncRunes(strings.TrimSpace(r.Barcode), 100)
	r.SourceURL = cleanSourceURL(r.SourceURL)
	r.Tags = cleanTags(r.Tags)
	return ""
}

func (r *productReq) apply(p *models.Product) {
	p.Name = r.Name
	p.Description = r.Description
	p.Price = r.Price
	p.ComparePrice = r.ComparePrice
	p.ImageURL = r.ImageURL
	b, _ := json.Marshal(r.Media)
	p.MediaJSON = string(b)
	p.Media = r.Media
	p.Stock = r.Stock
	p.SortOrder = r.SortOrder
	p.SKU = r.SKU
	p.Barcode = r.Barcode
	p.CostPrice = r.CostPrice
	p.SourceURL = r.SourceURL
	p.Tags = r.Tags
}

// descriptionOut: bản cũ lưu văn bản thuần → hiển thị dưới dạng HTML đoạn văn
func descriptionOut(s string) string {
	if s != "" && !looksLikeHTML(s) {
		return plainToRichHTML(s)
	}
	return s
}

// productSummary: tóm tắt ngắn dạng chữ thuần để hiện trên thẻ sản phẩm
func productSummary(descHTML string) string {
	txt := strings.Join(strings.Fields(htmlToText(descriptionOut(descHTML))), " ")
	if r := []rune(txt); len(r) > 160 {
		return strings.TrimSpace(string(r[:160])) + "…"
	}
	return txt
}

// ----------------------------------------------------------------- Admin

// productLite là dòng trong danh sách quản lý: nhẹ, không kèm mô tả đầy đủ / toàn bộ bộ sưu tập
type productLite struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Summary    string `json:"summary"`
	Price      int64  `json:"price"`
	Compare    int64  `json:"compare_price"`
	ImageURL   string `json:"image_url"`
	Stock      int    `json:"stock"`
	IsActive   bool   `json:"is_active"`
	SortOrder  int    `json:"sort_order"`
	SKU        string `json:"sku"`
	CostPrice  int64  `json:"cost_price"`
	SourceURL  string `json:"source_url"`
	Tags       string `json:"tags"`
	MediaCount int    `json:"media_count"`
	HasVideo   bool   `json:"has_video"`
}

func toLite(p models.Product) productLite {
	l := productLite{
		ID: p.ID, Name: p.Name, Summary: productSummary(p.Description), Price: p.Price, Compare: p.ComparePrice, ImageURL: p.ImageURL,
		Stock: p.Stock, IsActive: p.IsActive, SortOrder: p.SortOrder, SKU: p.SKU, CostPrice: p.CostPrice, SourceURL: p.SourceURL,
		Tags: p.Tags, MediaCount: len(p.Media),
	}
	for _, m := range p.Media {
		if m.Type != "image" {
			l.HasVideo = true
		}
	}
	return l
}

// productFull: sản phẩm đầy đủ kèm tóm tắt chữ thuần (cho thẻ sản phẩm)
type productFull struct {
	models.Product
	Summary string `json:"summary"`
}

// ListProducts GET /api/v1/storefront/sites/:id/products[?lite=1]
// lite=1 (trang quản lý sản phẩm): không kèm mô tả đầy đủ để danh sách nhẹ; mặc định trả đủ (trình dựng web cần để dịch).
func (h *StorefrontHandler) ListProducts(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	q := h.DB.Where("storefront_id = ?", sf.ID).Order("sort_order ASC, id ASC")
	lite := c.Query("lite") == "1"
	if lite {
		q = q.Select("id, storefront_id, user_id, name, LEFT(description, 1500) AS description, price, compare_price, image_url, stock, is_active, sort_order, sku, barcode, cost_price, source_url, tags, media_json, created_at, updated_at")
	}
	var items []models.Product
	if err := q.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tải được danh sách sản phẩm."})
		return
	}
	if lite {
		out := make([]productLite, 0, len(items))
		for _, p := range items {
			out = append(out, toLite(p))
		}
		c.JSON(http.StatusOK, gin.H{"products": out, "max_products": maxProductsPerSite})
		return
	}
	full := make([]productFull, 0, len(items))
	for _, p := range items {
		p.Description = descriptionOut(p.Description)
		full = append(full, productFull{Product: p, Summary: productSummary(p.Description)})
	}
	c.JSON(http.StatusOK, gin.H{"products": full, "max_products": maxProductsPerSite})
}

// GetProduct GET /api/v1/storefront/sites/:id/products/:pid — đầy đủ, dùng khi mở trình sửa
func (h *StorefrontHandler) GetProduct(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var p models.Product
	if err := h.DB.Where("id = ? AND storefront_id = ?", c.Param("pid"), sf.ID).First(&p).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy sản phẩm."})
		return
	}
	p.Description = descriptionOut(p.Description)
	c.JSON(http.StatusOK, gin.H{"product": p})
}

// CreateProduct POST /api/v1/storefront/sites/:id/products
func (h *StorefrontHandler) CreateProduct(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu sản phẩm không hợp lệ."})
		return
	}
	if msg := req.normalize(); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	var count int64
	h.DB.Model(&models.Product{}).Where("storefront_id = ?", sf.ID).Count(&count)
	if count >= maxProductsPerSite {
		c.JSON(http.StatusForbidden, gin.H{"error": "Đã đạt số sản phẩm tối đa của website."})
		return
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	p := models.Product{StorefrontID: sf.ID, UserID: sf.UserID, IsActive: active}
	req.apply(&p)
	if err := h.DB.Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo sản phẩm thất bại."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"product": p})
}

// UpdateProduct PUT /api/v1/storefront/sites/:id/products/:pid
func (h *StorefrontHandler) UpdateProduct(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var p models.Product
	if err := h.DB.Where("id = ? AND storefront_id = ?", c.Param("pid"), sf.ID).First(&p).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy sản phẩm."})
		return
	}
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu sản phẩm không hợp lệ."})
		return
	}
	if msg := req.normalize(); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	req.apply(&p)
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}
	if err := h.DB.Save(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu sản phẩm thất bại."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"product": p})
}

// PatchProduct PATCH /api/v1/storefront/sites/:id/products/:pid — chỉ đổi trạng thái hiển thị / thứ tự (không đụng mô tả, ảnh)
func (h *StorefrontHandler) PatchProduct(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req struct {
		IsActive  *bool `json:"is_active"`
		SortOrder *int  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu sản phẩm không hợp lệ."})
		return
	}
	updates := map[string]interface{}{}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không có gì để cập nhật."})
		return
	}
	res := h.DB.Model(&models.Product{}).Where("id = ? AND storefront_id = ?", c.Param("pid"), sf.ID).Updates(updates)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu sản phẩm thất bại."})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy sản phẩm."})
		return
	}
	var p models.Product
	h.DB.Where("id = ?", c.Param("pid")).First(&p)
	c.JSON(http.StatusOK, gin.H{"product": toLite(p)})
}

// DeleteProduct DELETE /api/v1/storefront/sites/:id/products/:pid
func (h *StorefrontHandler) DeleteProduct(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	res := h.DB.Where("id = ? AND storefront_id = ?", c.Param("pid"), sf.ID).Delete(&models.Product{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa sản phẩm thất bại."})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy sản phẩm."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã xóa sản phẩm."})
}

// ----------------------------------------------------------------- Công khai

// publicProductCard: dòng nhẹ trong danh sách web công khai (không kèm mô tả đầy đủ)
func publicProductCard(p models.Product) gin.H {
	return gin.H{
		"id":            p.ID,
		"name":          p.Name,
		"summary":       productSummary(p.Description),
		"price":         p.Price,
		"compare_price": p.ComparePrice,
		"image_url":     p.ImageURL,
		"in_stock":      p.Stock != 0,
	}
}

func (h *StorefrontHandler) publishedProducts(sf uint) []models.Product {
	var prods []models.Product
	h.DB.Select("id, storefront_id, name, LEFT(description, 1500) AS description, price, compare_price, image_url, stock, is_active, sort_order").
		Where("storefront_id = ? AND is_active = ?", sf, true).
		Order("sort_order ASC, id ASC").Find(&prods)
	return prods
}

// GetPublicProduct GET /api/v1/storefront/public/products/:pid?slug=... (hoặc ?domain=...)
// Đầy đủ mô tả, bộ sưu tập ảnh/video. Không lộ giá nhập, SKU nội bộ, nguồn hàng, tồn kho cụ thể.
func (h *StorefrontHandler) GetPublicProduct(c *gin.Context) {
	sf, ok := h.findPublishedSite(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy cửa hàng."})
		return
	}
	var p models.Product
	err := h.DB.Where("id = ? AND storefront_id = ? AND is_active = ?", c.Param("pid"), sf.ID, true).First(&p).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy sản phẩm."})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tải được sản phẩm."})
		}
		return
	}
	c.Header("Cache-Control", "public, max-age=30")
	c.JSON(http.StatusOK, gin.H{"product": gin.H{
		"id":            p.ID,
		"name":          p.Name,
		"description":   descriptionOut(p.Description),
		"price":         p.Price,
		"compare_price": p.ComparePrice,
		"image_url":     p.ImageURL,
		"media":         p.Media,
		"sku":           p.SKU,
		"in_stock":      p.Stock != 0,
	}})
}
