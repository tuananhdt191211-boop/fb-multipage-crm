package handlers

// SEO + landing page (giai đoạn 2):
//   - Cài đặt riêng của từng trang (storefront_pages.{draft,published}_settings_json): tiêu đề SEO, mô tả, ảnh chia sẻ, noindex,
//     và "landing page" (ẩn dải thông báo + đầu trang / chân trang dùng chung).
//   - Cài đặt SEO mặc định của cả website (storefronts.{draft,published}_seo_json): mô tả, ảnh chia sẻ, ẩn cả website khỏi công cụ tìm kiếm.
//   - GET /storefront/public/seo   — thông tin SEO đã tính sẵn cho một đường dẫn (cho Cloudflare Function chèn vào HTML và cho trình duyệt).
//   - GET /storefront/public/sitemap.xml — sơ đồ trang (trang chủ, trang đang hiện không noindex, sản phẩm đang bán).
//   - POST /storefront/sites/:id/seo/suggest — AI gợi ý tiêu đề / mô tả từ nội dung trang.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
)

// ----------------------------------------------------------------- Cài đặt trang / website

type PageSettings struct {
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
	SEOImage       string `json:"seo_image"`
	NoIndex        bool   `json:"noindex"`     // không cho công cụ tìm kiếm lập chỉ mục trang này (và không đưa vào sitemap)
	HideHeader     bool   `json:"hide_header"` // landing page: ẩn dải thông báo + đầu trang
	HideFooter     bool   `json:"hide_footer"` // landing page: ẩn chân trang
}

type SiteSEO struct {
	Description string `json:"description"` // mô tả mặc định khi trang không có mô tả riêng
	Image       string `json:"image"`       // ảnh chia sẻ mặc định (mặc định dùng logo)
	NoIndex     bool   `json:"noindex"`     // ẩn CẢ website khỏi công cụ tìm kiếm
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(htmlToText(s)), " ")
	return cutRunes(s, n)
}

func parseSettings(raw string) PageSettings {
	var s PageSettings
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &s)
	}
	return s
}

func cleanSettings(s PageSettings) PageSettings {
	s.SEOTitle = oneLine(s.SEOTitle, 120)
	s.SEODescription = oneLine(s.SEODescription, 320)
	s.SEOImage = safeMediaURL(s.SEOImage)
	return s
}

// sanitizeSettingsJSON: chuỗi JSON cài đặt trang đã lọc (chuỗi rỗng / lỗi → mặc định)
func sanitizeSettingsJSON(raw string) string {
	b, _ := json.Marshal(cleanSettings(parseSettings(raw)))
	return string(b)
}

func parseSiteSEO(raw string) SiteSEO {
	var s SiteSEO
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &s)
	}
	return s
}

func sanitizeSiteSEOJSON(raw string) string {
	s := parseSiteSEO(raw)
	s.Description = oneLine(s.Description, 320)
	s.Image = safeMediaURL(s.Image)
	b, _ := json.Marshal(s)
	return string(b)
}

// copySettingsForDuplicate: bản sao trang giữ kiểu landing nhưng bỏ tiêu đề / mô tả SEO (tránh trùng nội dung SEO giữa hai trang)
func copySettingsForDuplicate(raw string) string {
	s := parseSettings(raw)
	s.SEOTitle, s.SEODescription = "", ""
	return sanitizeSettingsJSON(settingsJSON(s))
}

// composeForSettings ghép phần dùng chung + nội dung trang, bỏ phần bị ẩn theo cài đặt (landing page)
func composeForSettings(shared, page []StorefrontBlock, s PageSettings) []StorefrontBlock {
	use := make([]StorefrontBlock, 0, len(shared))
	for _, b := range shared {
		if s.HideHeader && (b.Type == "announcement" || b.Type == "header") {
			continue
		}
		if s.HideFooter && b.Type == "footer" {
			continue
		}
		use = append(use, b)
	}
	return composeBlocks(use, page)
}

// ----------------------------------------------------------------- SEO đã tính sẵn cho một đường dẫn

type seoProduct struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
	SKU         string `json:"sku"`
	Price       int64  `json:"price"`
	Currency    string `json:"currency"`
	InStock     bool   `json:"in_stock"`
}

type seoMeta struct {
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	Image         string      `json:"image"`
	NoIndex       bool        `json:"noindex"`
	Type          string      `json:"type"` // website | product
	SiteName      string      `json:"site_name"`
	Path          string      `json:"path"`           // đường dẫn trong website: "/", "/pages/<trang>", "/p/<id>"
	CanonicalBase string      `json:"canonical_base"` // https://<tên miền riêng> khi đã kết nối xong; rỗng = dùng nơi đang truy cập
	Product       *seoProduct `json:"product,omitempty"`
}

// textOfBlocks lấy một đoạn chữ đầu tiên đáng làm mô tả (phụ đề hero, nội dung văn bản...)
func autoDescription(blocks []StorefrontBlock) string {
	pick := func(b StorefrontBlock, keys ...string) string {
		for _, k := range keys {
			if s, ok := b.Data[k].(string); ok {
				if t := oneLine(s, 400); utf8.RuneCountInString(t) >= 25 {
					return t
				}
			}
		}
		return ""
	}
	for _, b := range blocks {
		if !b.Enabled {
			continue
		}
		var t string
		switch b.Type {
		case "hero":
			t = pick(b, "subtitle")
		case "richtext", "image_text":
			t = pick(b, "body")
		case "cta":
			t = pick(b, "subtitle")
		case "banner":
			if slides, ok := b.Data["slides"].([]interface{}); ok && len(slides) > 0 {
				if m, ok := slides[0].(map[string]interface{}); ok {
					if s, ok := m["text"].(string); ok {
						if x := oneLine(s, 400); utf8.RuneCountInString(x) >= 25 {
							t = x
						}
					}
				}
			}
		}
		if t != "" {
			return cutWords(t, 155)
		}
	}
	return ""
}

// cutWords cắt tại ranh giới từ, không quá n ký tự
func cutWords(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n]
	out := string(r)
	if i := strings.LastIndexAny(out, " ,;.!?—-"); i > n/2 {
		out = out[:i]
	}
	return strings.TrimRight(out, " ,;.:—-")
}

// canonicalBase: tên miền riêng đang hoạt động của website (nếu có)
func canonicalBase(sf *models.Storefront) string {
	d := strings.ToLower(strings.TrimSpace(sf.Domain))
	if d != "" && sf.DomainStatus == "active" {
		return "https://" + d
	}
	return ""
}

func (h *StorefrontHandler) homeSettings(sf *models.Storefront) (models.StorefrontPage, PageSettings) {
	home, _ := h.homePage(sf)
	return home, parseSettings(home.PublishedSettingsJSON)
}

// seoFor tính SEO của một đường dẫn trong website (đường dẫn đã bỏ tiền tố /s/<slug>)
func (h *StorefrontHandler) seoFor(sf *models.Storefront, path string) seoMeta {
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	path = "/" + strings.Trim(path, "/")
	site := parseSiteSEO(sf.PublishedSEOJSON)
	m := seoMeta{Type: "website", SiteName: sf.SiteTitle, Path: path, CanonicalBase: canonicalBase(sf), NoIndex: site.NoIndex}
	m.Image = firstNonEmpty(site.Image, sf.LogoURL)
	m.Description = site.Description
	segs := strings.Split(strings.Trim(path, "/"), "/")

	switch {
	case path == "/":
		home, st := h.homeSettings(sf)
		m.Title = firstNonEmpty(st.SEOTitle, sf.SiteTitle)
		m.Description = firstNonEmpty(st.SEODescription, site.Description, autoDescription(parseBlocks(home.PublishedBlocksJSON)))
		m.Image = firstNonEmpty(st.SEOImage, m.Image)
		m.NoIndex = m.NoIndex || st.NoIndex
	case segs[0] == "pages" && len(segs) == 2:
		var p models.StorefrontPage
		if h.DB.Where("storefront_id = ? AND published_slug = ? AND published_visible = ? AND kind <> ?", sf.ID, segs[1], true, models.PageKindHome).
			Limit(1).Find(&p).RowsAffected == 0 {
			m.Title, m.NoIndex = sf.SiteTitle, true
			break
		}
		st := parseSettings(p.PublishedSettingsJSON)
		m.Title = firstNonEmpty(st.SEOTitle, p.PublishedTitle+" – "+sf.SiteTitle)
		m.Description = firstNonEmpty(st.SEODescription, site.Description, autoDescription(parseBlocks(p.PublishedBlocksJSON)))
		m.Image = firstNonEmpty(st.SEOImage, m.Image)
		m.NoIndex = m.NoIndex || st.NoIndex
	case segs[0] == "p" && len(segs) == 2:
		id, _ := strconv.Atoi(segs[1])
		var p models.Product
		if id <= 0 || h.DB.Where("id = ? AND storefront_id = ? AND is_active = ?", id, sf.ID, true).Limit(1).Find(&p).RowsAffected == 0 {
			m.Title, m.NoIndex = sf.SiteTitle, true
			break
		}
		sum := productSummary(p.Description)
		m.Type = "product"
		m.Title = p.Name + " – " + sf.SiteTitle
		m.Description = firstNonEmpty(cutWords(sum, 155), site.Description)
		m.Image = firstNonEmpty(p.ImageURL, m.Image)
		m.Product = &seoProduct{Name: p.Name, Description: cutWords(sum, 300), Image: p.ImageURL, SKU: p.SKU, Price: p.Price, Currency: "VND", InStock: p.Stock != 0}
	default: // giỏ hàng, thanh toán, tài khoản, đơn hàng: không đưa lên công cụ tìm kiếm
		m.Title, m.NoIndex = sf.SiteTitle, true
	}
	m.Title = oneLine(m.Title, 160)
	m.Description = oneLine(m.Description, 320)
	return m
}

// GetPublicSEO GET /api/v1/storefront/public/seo?slug=...|domain=...&path=/pages/gioi-thieu
func (h *StorefrontHandler) GetPublicSEO(c *gin.Context) {
	sf, ok := h.findPublishedSite(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chưa có trang bán hàng nào được xuất bản."})
		return
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, h.seoFor(sf, c.Query("path")))
}

// ----------------------------------------------------------------- Sitemap

var (
	sitemapOriginRe = regexp.MustCompile(`^https?://[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:\d{1,5})?$`)
	sitemapPrefixRe = regexp.MustCompile(`^(/s/[a-z0-9]+(-[a-z0-9]+)*)?$`)
)

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// GetPublicSitemap GET /api/v1/storefront/public/sitemap.xml?slug=...|domain=...&origin=https://host&prefix=/s/<slug>
func (h *StorefrontHandler) GetPublicSitemap(c *gin.Context) {
	sf, ok := h.findPublishedSite(c)
	if !ok {
		c.String(http.StatusNotFound, "not found")
		return
	}
	origin := strings.ToLower(strings.TrimRight(c.Query("origin"), "/"))
	prefix := c.Query("prefix")
	if !sitemapOriginRe.MatchString(origin) || !sitemapPrefixRe.MatchString(prefix) {
		c.String(http.StatusBadRequest, "bad origin or prefix")
		return
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	add := func(path string, mod time.Time) {
		fmt.Fprintf(&b, "  <url><loc>%s</loc><lastmod>%s</lastmod></url>\n", xmlEscape(origin+prefix+path), mod.UTC().Format("2006-01-02"))
	}
	if !parseSiteSEO(sf.PublishedSEOJSON).NoIndex {
		home, hs := h.homeSettings(sf)
		if !hs.NoIndex {
			add("", maxTime(sf.UpdatedAt, home.UpdatedAt))
		}
		var pages []models.StorefrontPage
		h.DB.Select("id, published_slug, published_settings_json, updated_at").
			Where("storefront_id = ? AND published_visible = ? AND kind <> ?", sf.ID, true, models.PageKindHome).
			Order("position ASC, id ASC").Find(&pages)
		for _, p := range pages {
			if !parseSettings(p.PublishedSettingsJSON).NoIndex {
				add("/pages/"+p.PublishedSlug, p.UpdatedAt)
			}
		}
		var prods []models.Product
		h.DB.Select("id, updated_at").Where("storefront_id = ? AND is_active = ?", sf.ID, true).Order("sort_order ASC, id ASC").Limit(2000).Find(&prods)
		for _, p := range prods {
			add("/p/"+strconv.Itoa(int(p.ID)), p.UpdatedAt)
		}
	}
	b.WriteString("</urlset>\n")
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(b.String()))
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// ----------------------------------------------------------------- AI gợi ý SEO

// Mỗi khách hàng gọi AI gợi ý SEO tối đa 40 lần / giờ (khoá AI là của hệ thống, dùng chung)
var seoAILimiter = struct {
	sync.Mutex
	hits map[uint][]time.Time
}{hits: map[uint][]time.Time{}}

const seoAIPerHour = 40

func seoAIAllow(ws uint) bool {
	seoAILimiter.Lock()
	defer seoAILimiter.Unlock()
	cut := time.Now().Add(-time.Hour)
	kept := seoAILimiter.hits[ws][:0]
	for _, t := range seoAILimiter.hits[ws] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= seoAIPerHour {
		seoAILimiter.hits[ws] = kept
		return false
	}
	seoAILimiter.hits[ws] = append(kept, time.Now())
	return true
}

type seoSuggestReq struct {
	Title    string `json:"title"`    // tên trang
	Content  string `json:"content"`  // văn bản của trang (gồm cả phần chưa lưu), tối đa ~6000 ký tự
	Keywords string `json:"keywords"` // từ khoá chính chủ shop muốn nhắm tới (không bắt buộc)
	Lang     string `json:"lang"`     // ngôn ngữ viết, mặc định vi
	IsHome   bool   `json:"is_home"`
}

func parseAIObject(reply string) (map[string]any, error) {
	reply = strings.TrimSpace(reply)
	reply = strings.TrimPrefix(reply, "```json")
	reply = strings.TrimPrefix(reply, "```")
	reply = strings.TrimSuffix(reply, "```")
	if i := strings.Index(reply, "{"); i > 0 {
		reply = reply[i:]
	}
	if j := strings.LastIndex(reply, "}"); j >= 0 && j < len(reply)-1 {
		reply = reply[:j+1]
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(reply), &raw); err != nil {
		return nil, errAIBadFormat
	}
	return raw, nil
}

func aiString(raw map[string]any, key string) string {
	if v, ok := raw[key].(string); ok {
		return strings.Trim(strings.Join(strings.Fields(htmlToText(v)), " "), " \"'“”‘’")
	}
	return ""
}

// SuggestSEO POST /api/v1/storefront/sites/:id/seo/suggest
func (h *StorefrontHandler) SuggestSEO(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req seoSuggestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	if h.AI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Chưa cấu hình AI."})
		return
	}
	req.Title = oneLine(req.Title, 160)
	req.Content = cutRunes(strings.TrimSpace(req.Content), 6000)
	req.Keywords = oneLine(req.Keywords, 200)
	if req.Title == "" && req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trang chưa có nội dung để AI viết SEO. Hãy thêm nội dung trước."})
		return
	}
	langName := "Vietnamese"
	if n, ok := i18nLangNames[req.Lang]; ok && req.Lang != "" {
		langName = n
	}
	if !seoAIAllow(sf.UserID) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": fmt.Sprintf("Bạn đã dùng hết %d lượt AI SEO trong giờ này. Hãy thử lại sau.", seoAIPerHour), "code": "rate_limited"})
		return
	}

	system := "You are an expert SEO copywriter for small e-commerce websites. " +
		"Write the SEO title and meta description for ONE web page, in " + langName + ", based only on the page content provided. " +
		"Rules: title at most 60 characters, put the main keyword first and the brand at the end only if it fits; " +
		"description between 110 and 155 characters, natural and persuasive, ends with a soft call to action; " +
		"never invent prices, discounts, certifications, awards or claims that are not in the content; no emoji, no quotation marks, no HTML; " +
		"do not repeat the same phrase in the title and description. " +
		"Also return up to 6 relevant keyword phrases. Return ONLY a JSON object: {\"title\": string, \"description\": string, \"keywords\": [string]}."
	var user strings.Builder
	fmt.Fprintf(&user, "Website name: %s\n", sf.SiteTitle)
	if req.IsHome {
		user.WriteString("Page type: home page\n")
	}
	if req.Title != "" {
		fmt.Fprintf(&user, "Page name: %s\n", req.Title)
	}
	if req.Keywords != "" {
		fmt.Fprintf(&user, "Main keywords the owner wants to target: %s\n", req.Keywords)
	}
	user.WriteString("Page content:\n" + req.Content)

	reply, err := h.AI.callUniversalChatAI(system, user.String(), true)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI chưa trả lời được: " + cutRunes(err.Error(), 200)})
		return
	}
	raw, err := parseAIObject(reply)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI trả lời sai định dạng. Hãy thử lại."})
		return
	}
	title := cutWords(aiString(raw, "title"), 65)
	desc := cutWords(aiString(raw, "description"), 160)
	if title == "" && desc == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI không viết được gợi ý. Hãy thử lại."})
		return
	}
	kws := []string{}
	if arr, ok := raw["keywords"].([]any); ok {
		for _, k := range arr {
			if s, ok := k.(string); ok {
				if s = oneLine(s, 60); s != "" && len(kws) < 6 {
					kws = append(kws, s)
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"title": title, "description": desc, "keywords": kws})
}

func settingsJSON(s PageSettings) string {
	b, _ := json.Marshal(s)
	return string(b)
}
