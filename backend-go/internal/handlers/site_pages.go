package handlers

// Nhiều trang cho một website (giai đoạn 1):
//   - Dải thông báo, đầu trang (header) và chân trang (footer) là phần DÙNG CHUNG: lưu ở Storefront.{Draft,Published}SharedJSON,
//     sửa một lần hiện ở mọi trang. Mỗi trang (bảng storefront_pages) chỉ giữ các section riêng của nó.
//   - Trang chủ là trang kind=home (slug "home", đường dẫn "/"); trang khác ở /pages/<slug>.
//   - Giữ tương thích bản cũ: Storefront.{Draft,Published}BlocksJSON vẫn là trang chủ "ghép sẵn" (chung + trang chủ);
//     PUT /draft kiểu cũ (chỉ draft_blocks_json) được tách ngược lại thành phần chung + trang chủ.
//   - Website cũ (một mảng section) tự được chuyển thành "trang chủ + phần chung" khi máy chủ khởi động (migrateStorefrontPages).

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxPagesPerSite = 30
	homeSlug        = "home"
)

// sharedRank: thứ tự của phần dùng chung — dải thông báo, rồi header ở trên cùng; footer ở dưới cùng
var sharedRank = map[string]int{"announcement": 0, "header": 1, "footer": 2}

func isSharedType(t string) bool { _, ok := sharedRank[t]; return ok }

func parseBlocks(raw string) []StorefrontBlock {
	var out []StorefrontBlock
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

func marshalBlocks(b []StorefrontBlock) string {
	if b == nil {
		b = []StorefrontBlock{}
	}
	out, _ := json.Marshal(b)
	return string(out)
}

// splitBlocks tách danh sách section "ghép sẵn" thành phần dùng chung (đúng thứ tự thông báo → header → footer) và phần riêng của trang
func splitBlocks(blocks []StorefrontBlock) (shared, page []StorefrontBlock) {
	for _, b := range blocks {
		if isSharedType(b.Type) {
			shared = append(shared, b)
		} else {
			page = append(page, b)
		}
	}
	sort.SliceStable(shared, func(i, j int) bool { return sharedRank[shared[i].Type] < sharedRank[shared[j].Type] })
	return
}

// composeBlocks ghép phần dùng chung + section của trang: [thông báo, header] + trang + [footer]
func composeBlocks(shared, page []StorefrontBlock) []StorefrontBlock {
	out := make([]StorefrontBlock, 0, len(shared)+len(page))
	for _, b := range shared {
		if sharedRank[b.Type] < sharedRank["footer"] {
			out = append(out, b)
		}
	}
	out = append(out, page...)
	for _, b := range shared {
		if sharedRank[b.Type] >= sharedRank["footer"] {
			out = append(out, b)
		}
	}
	return out
}

// sanitizeSharedJSON chỉ giữ các section dùng chung hợp lệ, đã lọc an toàn
func sanitizeSharedJSON(raw string) (string, error) {
	shared, _ := splitBlocks(parseBlocks(raw))
	return sanitizeBlocksJSON(marshalBlocks(shared))
}

// sanitizePageBlocksJSON bỏ các section dùng chung (nằm ở phần chung, không thuộc riêng trang nào) rồi lọc an toàn
func sanitizePageBlocksJSON(raw string) (string, error) {
	_, page := splitBlocks(parseBlocks(raw))
	return sanitizeBlocksJSON(marshalBlocks(page))
}

func newBlockID(typ string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", typ, hex.EncodeToString(b))
}

// reidBlocks cấp id mới cho mọi section (bản sao trang không được trùng id với trang gốc vì bản dịch gắn theo id)
func reidBlocks(blocks []StorefrontBlock) []StorefrontBlock {
	out := make([]StorefrontBlock, len(blocks))
	for i, b := range blocks {
		b.ID = newBlockID(b.Type)
		out[i] = b
	}
	return out
}

// ----------------------------------------------------------------- Đường dẫn trang

var pageSlugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validatePageSlug(slug string) string {
	switch {
	case slug == "" || len(slug) > 60:
		return "Đường dẫn trang chỉ dài tối đa 60 ký tự và không được để trống."
	case !pageSlugRe.MatchString(slug):
		return "Đường dẫn chỉ gồm chữ thường không dấu, số và dấu gạch ngang (vd. chinh-sach-doi-tra)."
	case slug == homeSlug:
		return "Đường dẫn \"home\" dành riêng cho trang chủ."
	}
	return ""
}

// uniquePageSlug thêm hậu tố -2, -3... nếu đường dẫn đã có trong website
func (h *StorefrontHandler) uniquePageSlug(siteID uint, base string, exceptID uint) string {
	base = strings.Trim(base, "-")
	if base == "" || base == homeSlug {
		base = "trang"
	}
	if len(base) > 50 {
		base = strings.Trim(base[:50], "-")
	}
	slug := base
	for i := 2; i < 200; i++ {
		var n int64
		h.DB.Model(&models.StorefrontPage{}).Where("storefront_id = ? AND slug = ? AND id <> ?", siteID, slug, exceptID).Count(&n)
		if n == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return slug
}

// ----------------------------------------------------------------- Chuyển dữ liệu cũ / tạo trang chủ

// migrateStorefrontPages: mọi website chưa có trang nào → tạo trang chủ từ mảng section cũ, tách phần dùng chung.
// Chạy mỗi lần khởi động; idempotent (chỉ đụng website chưa có trang).
func migrateStorefrontPages(db *gorm.DB) {
	_ = db.AutoMigrate(&models.StorefrontPage{})
	var sites []models.Storefront
	db.Where("id NOT IN (SELECT DISTINCT storefront_id FROM storefront_pages)").Find(&sites)
	for i := range sites {
		_ = seedHomePage(db, &sites[i])
	}
}

// seedHomePage tạo trang chủ + phần dùng chung từ DraftBlocksJSON / PublishedBlocksJSON hiện có của website
func seedHomePage(db *gorm.DB, sf *models.Storefront) error {
	draft := parseBlocks(sf.DraftBlocksJSON)
	pubRaw := sf.PublishedBlocksJSON
	if strings.TrimSpace(pubRaw) == "" {
		pubRaw = sf.DraftBlocksJSON
	}
	published := parseBlocks(pubRaw)
	dShared, dPage := splitBlocks(draft)
	pShared, pPage := splitBlocks(published)
	return db.Transaction(func(tx *gorm.DB) error {
		var n int64
		tx.Model(&models.StorefrontPage{}).Where("storefront_id = ?", sf.ID).Count(&n)
		if n > 0 {
			return nil
		}
		home := models.StorefrontPage{
			StorefrontID: sf.ID, Slug: homeSlug, Title: "Trang chủ", Kind: models.PageKindHome, Position: 0, IsVisible: true,
			DraftBlocksJSON: marshalBlocks(dPage), PublishedBlocksJSON: marshalBlocks(pPage),
			PublishedTitle: "Trang chủ", PublishedSlug: homeSlug, PublishedVisible: true,
		}
		if err := tx.Create(&home).Error; err != nil {
			return err
		}
		sf.DraftSharedJSON, sf.PublishedSharedJSON = marshalBlocks(dShared), marshalBlocks(pShared)
		return tx.Model(&models.Storefront{}).Where("id = ?", sf.ID).UpdateColumns(map[string]any{
			"draft_shared_json": sf.DraftSharedJSON, "published_shared_json": sf.PublishedSharedJSON,
		}).Error
	})
}

// pagesOf: các trang của website theo thứ tự hiển thị; website chưa có trang thì tạo trang chủ trước
func (h *StorefrontHandler) pagesOf(sf *models.Storefront) []models.StorefrontPage {
	var pages []models.StorefrontPage
	h.DB.Where("storefront_id = ?", sf.ID).Order("position ASC, id ASC").Find(&pages)
	if len(pages) == 0 {
		_ = seedHomePage(h.DB, sf)
		h.DB.Where("storefront_id = ?", sf.ID).Order("position ASC, id ASC").Find(&pages)
	}
	return pages
}

func (h *StorefrontHandler) homePage(sf *models.Storefront) (models.StorefrontPage, bool) {
	for _, p := range h.pagesOf(sf) {
		if p.Kind == models.PageKindHome {
			return p, true
		}
	}
	return models.StorefrontPage{}, false
}

// refreshLegacyBlocks cập nhật cột "ghép sẵn" (trang chủ + phần chung) cho giao diện / API bản cũ
func (h *StorefrontHandler) refreshLegacyBlocks(sf *models.Storefront, published bool) {
	home, ok := h.homePage(sf)
	if !ok {
		return
	}
	if published {
		sf.PublishedBlocksJSON = marshalBlocks(composeBlocks(parseBlocks(sf.PublishedSharedJSON), parseBlocks(home.PublishedBlocksJSON)))
		return
	}
	sf.DraftBlocksJSON = marshalBlocks(composeBlocks(parseBlocks(sf.DraftSharedJSON), parseBlocks(home.DraftBlocksJSON)))
}

// applyLegacyDraft: PUT /draft kiểu cũ gửi mảng section "ghép sẵn" → tách thành phần chung + trang chủ
func (h *StorefrontHandler) applyLegacyDraft(sf *models.Storefront, raw string) error {
	clean, err := sanitizeBlocksJSON(raw)
	if err != nil {
		return err
	}
	shared, page := splitBlocks(parseBlocks(clean))
	home, ok := h.homePage(sf)
	if !ok {
		return fmt.Errorf("Không tìm thấy trang chủ của website.")
	}
	sf.DraftSharedJSON = marshalBlocks(shared)
	if err := h.DB.Model(&models.StorefrontPage{}).Where("id = ?", home.ID).Update("draft_blocks_json", marshalBlocks(page)).Error; err != nil {
		return err
	}
	sf.DraftBlocksJSON = marshalBlocks(composeBlocks(shared, page))
	return nil
}

type draftPageReq struct {
	ID         uint    `json:"id"`
	BlocksJSON *string `json:"draft_blocks_json"` // nil = không đổi nội dung (chỉ đổi cài đặt)
	Settings   *string `json:"draft_settings_json"`
}

// applyDraftContent áp nội dung của một yêu cầu lưu nháp vào website:
//   - kiểu mới: draft_shared_json (đầu/chân trang dùng chung), draft_menus_json, draft_seo_json + pages[] (chỉ các trang có thay đổi,
//     nội dung và / hoặc cài đặt SEO + landing);
//   - kiểu cũ: draft_blocks_json là mảng "ghép sẵn" → tách thành phần chung + trang chủ.
//
// Kiểm tra hết trước khi ghi nên một yêu cầu sai không để lại dữ liệu nửa chừng.
func (h *StorefrontHandler) applyDraftContent(sf *models.Storefront, req *saveStorefrontDraftReq) error {
	if req.SharedJSON == nil && len(req.Pages) == 0 && req.MenusJSON == nil && req.SEOJSON == nil {
		if strings.TrimSpace(req.BlocksJSON) == "" {
			return nil
		}
		return h.applyLegacyDraft(sf, req.BlocksJSON)
	}
	if len(req.Pages) > maxPagesPerSite {
		return fmt.Errorf("Mỗi website tối đa %d trang.", maxPagesPerSite)
	}
	pages := h.pagesOf(sf)
	byID := make(map[uint]models.StorefrontPage, len(pages))
	for _, p := range pages {
		byID[p.ID] = p
	}
	updates := make(map[uint]map[string]any, len(req.Pages))
	cleanedBlocks := map[uint]string{}
	for _, pr := range req.Pages {
		p, ok := byID[pr.ID]
		if !ok {
			return fmt.Errorf("Trang không thuộc website này.")
		}
		u := updates[pr.ID]
		if u == nil {
			u = map[string]any{}
			updates[pr.ID] = u
		}
		if pr.BlocksJSON != nil {
			c, err := sanitizePageBlocksJSON(*pr.BlocksJSON)
			if err != nil {
				return fmt.Errorf("Trang \"%s\": %s", p.Title, err.Error())
			}
			u["draft_blocks_json"] = c
			cleanedBlocks[pr.ID] = c
		}
		if pr.Settings != nil {
			c := sanitizeSettingsJSON(*pr.Settings)
			u["draft_settings_json"] = c
		}
	}
	shared := sf.DraftSharedJSON
	if req.SharedJSON != nil {
		c, err := sanitizeSharedJSON(*req.SharedJSON)
		if err != nil {
			return err
		}
		shared = c
	}
	menus, seo := sf.DraftMenusJSON, sf.DraftSEOJSON
	if req.MenusJSON != nil {
		c, err := sanitizeMenusJSON(*req.MenusJSON)
		if err != nil {
			return err
		}
		menus = c
	}
	if req.SEOJSON != nil {
		seo = sanitizeSiteSEOJSON(*req.SEOJSON)
	}
	if len(updates) > 0 {
		err := h.DB.Transaction(func(tx *gorm.DB) error {
			for id, u := range updates {
				if len(u) == 0 {
					continue
				}
				if err := tx.Model(&models.StorefrontPage{}).Where("id = ?", id).Updates(u).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("Lưu nội dung các trang thất bại.")
		}
	}
	sf.DraftSharedJSON, sf.DraftMenusJSON, sf.DraftSEOJSON = shared, menus, seo
	for _, p := range pages {
		if p.Kind == models.PageKindHome {
			raw := p.DraftBlocksJSON
			if c, ok := cleanedBlocks[p.ID]; ok {
				raw = c
			}
			// Cột cũ của bản nháp luôn ghép ĐỦ phần dùng chung (không áp landing): trình chỉnh sửa bản cũ lưu lại sẽ không làm mất đầu / chân trang
			sf.DraftBlocksJSON = marshalBlocks(composeBlocks(parseBlocks(shared), parseBlocks(raw)))
		}
	}
	return nil
}

// publishAllPages sao nháp → xuất bản cho phần dùng chung, menu, SEO website và mọi trang (nội dung + cài đặt)
func (h *StorefrontHandler) publishAllPages(sf *models.Storefront) error {
	shared, err := sanitizeSharedJSON(sf.DraftSharedJSON)
	if err != nil {
		return err
	}
	menus, err := sanitizeMenusJSON(sf.DraftMenusJSON)
	if err != nil {
		return err
	}
	seo := sanitizeSiteSEOJSON(sf.DraftSEOJSON)
	pages := h.pagesOf(sf)
	cleaned := make([]string, len(pages))
	settings := make([]string, len(pages))
	for i, p := range pages {
		c, err := sanitizePageBlocksJSON(p.DraftBlocksJSON)
		if err != nil {
			return fmt.Errorf("Trang \"%s\": %s", p.Title, err.Error())
		}
		cleaned[i] = c
		settings[i] = sanitizeSettingsJSON(p.DraftSettingsJSON)
	}
	return h.DB.Transaction(func(tx *gorm.DB) error {
		for i, p := range pages {
			visible := p.IsVisible || p.Kind == models.PageKindHome
			if err := tx.Model(&models.StorefrontPage{}).Where("id = ?", p.ID).Updates(map[string]any{
				"published_blocks_json": cleaned[i], "published_title": p.Title, "published_slug": p.Slug, "published_visible": visible,
				"published_settings_json": settings[i],
			}).Error; err != nil {
				return err
			}
			if p.Kind == models.PageKindHome {
				sf.PublishedBlocksJSON = marshalBlocks(composeForSettings(parseBlocks(shared), parseBlocks(cleaned[i]), parseSettings(settings[i])))
			}
		}
		sf.PublishedSharedJSON, sf.PublishedMenusJSON, sf.PublishedSEOJSON = shared, menus, seo
		return nil
	})
}

// ----------------------------------------------------------------- JSON cho trình chỉnh sửa

type adminPage struct {
	ID                    uint   `json:"id"`
	Slug                  string `json:"slug"`
	Title                 string `json:"title"`
	Kind                  string `json:"kind"`
	Position              int    `json:"position"`
	IsVisible             bool   `json:"is_visible"`
	PublishedVisible      bool   `json:"published_visible"`
	DraftBlocksJSON       string `json:"draft_blocks_json"`
	DraftSettingsJSON     string `json:"draft_settings_json"`
	PublishedSettingsJSON string `json:"published_settings_json"`
}

func (h *StorefrontHandler) adminPages(sf *models.Storefront) []adminPage {
	pages := h.pagesOf(sf)
	out := make([]adminPage, 0, len(pages))
	for _, p := range pages {
		out = append(out, adminPage{p.ID, p.Slug, p.Title, p.Kind, p.Position, p.IsVisible || p.Kind == models.PageKindHome, p.PublishedVisible, p.DraftBlocksJSON, p.DraftSettingsJSON, p.PublishedSettingsJSON})
	}
	return out
}

func (h *StorefrontHandler) siteWithPages(sf *models.Storefront) gin.H {
	return gin.H{"site": sf, "pages": h.adminPages(sf), "page_templates": pageTemplateList(), "max_pages": maxPagesPerSite}
}

// ----------------------------------------------------------------- API chủ shop

func (h *StorefrontHandler) loadOwnedPage(c *gin.Context, sf *models.Storefront) (models.StorefrontPage, bool) {
	id, err := strconv.Atoi(c.Param("pid"))
	var p models.StorefrontPage
	if err != nil || id <= 0 || h.DB.Where("id = ? AND storefront_id = ?", id, sf.ID).Limit(1).Find(&p).RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy trang."})
		return p, false
	}
	return p, true
}

// ListPages GET /api/v1/storefront/sites/:id/pages
func (h *StorefrontHandler) ListPages(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"pages": h.adminPages(sf), "page_templates": pageTemplateList(), "max_pages": maxPagesPerSite})
}

type createPageReq struct {
	Title    string `json:"title"`
	Slug     string `json:"slug"`
	Template string `json:"template"` // blank | about | contact | return | shipping | privacy | terms | payment
}

// CreatePage POST /api/v1/storefront/sites/:id/pages
func (h *StorefrontHandler) CreatePage(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req createPageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	tplKey := strings.TrimSpace(req.Template)
	if tplKey == "" {
		tplKey = "blank"
	}
	tpl, ok := pageTemplates[tplKey]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mẫu trang không tồn tại."})
		return
	}
	title := trimTo(req.Title, 160)
	if title == "" {
		title = tpl.Title
	}
	existing := h.pagesOf(sf)
	if len(existing) >= maxPagesPerSite {
		c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Mỗi website tối đa %d trang.", maxPagesPerSite), "code": "limit_reached"})
		return
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		base := slugify(title)
		if base == "" {
			base = tpl.Slug
		}
		slug = h.uniquePageSlug(sf.ID, base, 0)
	} else {
		if msg := validatePageSlug(slug); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		var n int64
		h.DB.Model(&models.StorefrontPage{}).Where("storefront_id = ? AND slug = ?", sf.ID, slug).Count(&n)
		if n > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Đường dẫn này đã dùng cho một trang khác."})
			return
		}
	}
	blocks, err := sanitizeBlocksJSON(marshalBlocks(tpl.Blocks(title)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không dựng được nội dung trang mẫu."})
		return
	}
	pos := 0
	for _, p := range existing {
		if p.Position >= pos {
			pos = p.Position + 1
		}
	}
	page := models.StorefrontPage{
		StorefrontID: sf.ID, Slug: slug, Title: title, Kind: tpl.Kind, Position: pos, IsVisible: true,
		DraftBlocksJSON: blocks, PublishedBlocksJSON: "[]",
	}
	if err := h.DB.Create(&page).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo trang thất bại."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Đã tạo trang. Bấm Xuất bản để khách xem được.", "page": adminPageOf(page), "pages": h.adminPages(sf)})
}

func adminPageOf(p models.StorefrontPage) adminPage {
	return adminPage{p.ID, p.Slug, p.Title, p.Kind, p.Position, p.IsVisible || p.Kind == models.PageKindHome, p.PublishedVisible, p.DraftBlocksJSON, p.DraftSettingsJSON, p.PublishedSettingsJSON}
}

type updatePageReq struct {
	Title     *string `json:"title"`
	Slug      *string `json:"slug"`
	IsVisible *bool   `json:"is_visible"`
}

// UpdatePage PATCH /api/v1/storefront/sites/:id/pages/:pid — đổi tên, đường dẫn, ẩn/hiện (có hiệu lực công khai sau khi Xuất bản)
func (h *StorefrontHandler) UpdatePage(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	p, ok := h.loadOwnedPage(c, sf)
	if !ok {
		return
	}
	var req updatePageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	updates := map[string]any{}
	if req.Title != nil {
		t := trimTo(*req.Title, 160)
		if t == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tên trang không được để trống."})
			return
		}
		updates["title"] = t
	}
	if req.Slug != nil && p.Kind != models.PageKindHome {
		slug := strings.ToLower(strings.TrimSpace(*req.Slug))
		if msg := validatePageSlug(slug); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		var n int64
		h.DB.Model(&models.StorefrontPage{}).Where("storefront_id = ? AND slug = ? AND id <> ?", sf.ID, slug, p.ID).Count(&n)
		if n > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Đường dẫn này đã dùng cho một trang khác."})
			return
		}
		updates["slug"] = slug
	}
	if req.IsVisible != nil && p.Kind != models.PageKindHome {
		updates["is_visible"] = *req.IsVisible
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không có gì để cập nhật."})
		return
	}
	if err := h.DB.Model(&models.StorefrontPage{}).Where("id = ?", p.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không cập nhật được trang."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật trang.", "pages": h.adminPages(sf)})
}

// DeletePage DELETE /api/v1/storefront/sites/:id/pages/:pid (không xoá được trang chủ)
func (h *StorefrontHandler) DeletePage(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	p, ok := h.loadOwnedPage(c, sf)
	if !ok {
		return
	}
	if p.Kind == models.PageKindHome {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không thể xoá trang chủ."})
		return
	}
	h.DB.Delete(&models.StorefrontPage{}, p.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Đã xoá trang.", "pages": h.adminPages(sf)})
}

// DuplicatePage POST /api/v1/storefront/sites/:id/pages/:pid/duplicate
func (h *StorefrontHandler) DuplicatePage(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	p, ok := h.loadOwnedPage(c, sf)
	if !ok {
		return
	}
	pages := h.pagesOf(sf)
	if len(pages) >= maxPagesPerSite {
		c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Mỗi website tối đa %d trang.", maxPagesPerSite), "code": "limit_reached"})
		return
	}
	kind := p.Kind
	if kind == models.PageKindHome {
		kind = models.PageKindPage
	}
	pos := 0
	for _, x := range pages {
		if x.Position >= pos {
			pos = x.Position + 1
		}
	}
	base := p.Slug
	if p.Kind == models.PageKindHome {
		base = "trang-chu"
	}
	cp := models.StorefrontPage{
		StorefrontID: sf.ID, Slug: h.uniquePageSlug(sf.ID, base+"-ban-sao", 0), Title: trimTo(p.Title+" (bản sao)", 160), Kind: kind, Position: pos,
		IsVisible:       false, // bản sao ẩn cho tới khi chủ shop sửa xong và bật hiển thị
		DraftBlocksJSON: marshalBlocks(reidBlocks(parseBlocks(p.DraftBlocksJSON))), PublishedBlocksJSON: "[]",
		DraftSettingsJSON: copySettingsForDuplicate(p.DraftSettingsJSON),
	}
	// Lưu ý: cột is_visible có default:true nên GORM đổi false lúc Create thành true → ghi riêng sau khi tạo
	if err := h.DB.Create(&cp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Nhân bản trang thất bại."})
		return
	}
	h.DB.Model(&models.StorefrontPage{}).Where("id = ?", cp.ID).Update("is_visible", false)
	cp.IsVisible = false
	c.JSON(http.StatusCreated, gin.H{"message": "Đã nhân bản trang (đang ẩn).", "page": adminPageOf(cp), "pages": h.adminPages(sf)})
}

// ReorderPages PUT /api/v1/storefront/sites/:id/pages/order {ids:[...]} — trang chủ luôn ở đầu
func (h *StorefrontHandler) ReorderPages(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	pages := h.pagesOf(sf)
	byID := map[uint]models.StorefrontPage{}
	for _, p := range pages {
		byID[p.ID] = p
	}
	order := []uint{}
	seen := map[uint]bool{}
	for _, p := range pages { // trang chủ đứng đầu
		if p.Kind == models.PageKindHome {
			order = append(order, p.ID)
			seen[p.ID] = true
		}
	}
	for _, id := range req.IDs {
		if _, ok := byID[id]; ok && !seen[id] {
			order = append(order, id)
			seen[id] = true
		}
	}
	for _, p := range pages { // trang không có trong danh sách giữ nguyên thứ tự cũ ở cuối
		if !seen[p.ID] {
			order = append(order, p.ID)
		}
	}
	h.DB.Transaction(func(tx *gorm.DB) error {
		for i, id := range order {
			tx.Model(&models.StorefrontPage{}).Where("id = ?", id).UpdateColumn("position", i)
		}
		return nil
	})
	c.JSON(http.StatusOK, gin.H{"pages": h.adminPages(sf)})
}

// ----------------------------------------------------------------- API công khai

// GetPublicPage GET /api/v1/storefront/public/pages/:pslug?slug=... — một trang (nội dung riêng của trang; header/footer lấy từ trang chủ)
func (h *StorefrontHandler) GetPublicPage(c *gin.Context) {
	sfp, found := h.findPublishedSite(c)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chưa có trang bán hàng nào được xuất bản."})
		return
	}
	pslug := strings.ToLower(strings.TrimSpace(c.Param("pslug")))
	var p models.StorefrontPage
	if pslug == "" || h.DB.Where("storefront_id = ? AND published_slug = ? AND published_visible = ? AND kind <> ?", sfp.ID, pslug, true, models.PageKindHome).
		Limit(1).Find(&p).RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy trang này."})
		return
	}
	c.Header("Cache-Control", "public, max-age=30")
	c.JSON(http.StatusOK, gin.H{"slug": p.PublishedSlug, "title": p.PublishedTitle, "kind": p.Kind, "blocks_json": p.PublishedBlocksJSON, "settings_json": p.PublishedSettingsJSON})
}

// publicPageList: các trang đã xuất bản và đang hiện (trừ trang chủ), để giao diện dựng liên kết
func (h *StorefrontHandler) publicPageList(siteID uint) []gin.H {
	var pages []models.StorefrontPage
	h.DB.Select("id, published_slug, published_title, kind, position").
		Where("storefront_id = ? AND published_visible = ? AND kind <> ?", siteID, true, models.PageKindHome).
		Order("position ASC, id ASC").Find(&pages)
	out := make([]gin.H, 0, len(pages))
	for _, p := range pages {
		out = append(out, gin.H{"slug": p.PublishedSlug, "title": p.PublishedTitle, "kind": p.Kind})
	}
	return out
}
