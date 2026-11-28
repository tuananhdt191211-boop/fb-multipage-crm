package handlers

// Test nhiều trang: tách / ghép phần dùng chung, chuyển dữ liệu cũ, API trang, xuất bản, công khai, quyền sở hữu.
//   TEST_DATABASE_URL=postgres://fbcrm:fbcrm_secret_password@localhost:5432/fbcrm_test?sslmode=disable go test ./internal/handlers -run Page -v

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
)

func tb(id, typ string) StorefrontBlock {
	return StorefrontBlock{ID: id, Type: typ, Enabled: true, Data: map[string]interface{}{"x": id}}
}

func typesOf(bs []StorefrontBlock) string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Type
	}
	return strings.Join(out, ",")
}

func TestPageSplitAndCompose(t *testing.T) {
	in := []StorefrontBlock{tb("1", "header"), tb("2", "hero"), tb("3", "footer"), tb("4", "announcement"), tb("5", "products")}
	shared, page := splitBlocks(in)
	if got := typesOf(shared); got != "announcement,header,footer" {
		t.Fatalf("shared order: %s", got)
	}
	if got := typesOf(page); got != "hero,products" {
		t.Fatalf("page: %s", got)
	}
	if got := typesOf(composeBlocks(shared, page)); got != "announcement,header,hero,products,footer" {
		t.Fatalf("compose: %s", got)
	}
	if got := typesOf(composeBlocks(nil, page)); got != "hero,products" {
		t.Fatalf("no shared: %s", got)
	}
	// phần riêng của trang không bao giờ chứa loại dùng chung
	raw := marshalBlocks(in)
	clean, err := sanitizePageBlocksJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := typesOf(parseBlocks(clean)); got != "hero,products" {
		t.Fatalf("sanitizePage kept shared types: %s", got)
	}
	cs, err := sanitizeSharedJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := typesOf(parseBlocks(cs)); got != "announcement,header,footer" {
		t.Fatalf("sanitizeShared: %s", got)
	}
	// reid: id mới, không trùng, giữ loại
	re := reidBlocks(page)
	if re[0].ID == page[0].ID || re[0].ID == re[1].ID || re[0].Type != "hero" {
		t.Fatalf("reid: %+v", re)
	}
}

func TestPageSafeLinkAndSlug(t *testing.T) {
	for in, want := range map[string]string{
		"page:chinh-sach-doi-tra": "page:chinh-sach-doi-tra", "PAGE:Lien-He": "page:lien-he", "page:home": "page:home",
		"page:": "", "page:bad slug": "", "page:../x": "", "page:" + strings.Repeat("a", 61): "",
		"javascript:alert(1)": "", "#faq": "#faq", "/pages/x": "/pages/x", "https://a.vn": "https://a.vn",
	} {
		if got := safeLink(in); got != want {
			t.Errorf("safeLink(%q) = %q, want %q", in, got, want)
		}
	}
	for slug, ok := range map[string]bool{"lien-he": true, "a1": true, "home": false, "Lien-He": false, "a_b": false, "": false, "-a": false, strings.Repeat("a", 61): false} {
		if (validatePageSlug(slug) == "") != ok {
			t.Errorf("validatePageSlug(%q) ok=%v", slug, validatePageSlug(slug) == "")
		}
	}
	for k, tpl := range pageTemplates { // mọi mẫu đều qua bộ lọc an toàn và có đường dẫn hợp lệ
		if validatePageSlug(tpl.Slug) != "" {
			t.Errorf("mẫu %s có slug không hợp lệ: %s", k, tpl.Slug)
		}
		if _, err := sanitizeBlocksJSON(marshalBlocks(tpl.Blocks(tpl.Title))); err != nil {
			t.Errorf("mẫu %s: %v", k, err)
		}
		if _, page := splitBlocks(tpl.Blocks(tpl.Title)); len(page) != len(tpl.Blocks(tpl.Title)) {
			t.Errorf("mẫu %s chứa section dùng chung", k)
		}
	}
}

type pagesEnv struct {
	*productEnv
}

func newPagesEnv(t *testing.T) *pagesEnv {
	e := newProductEnv(t)
	pg := e.r.Group("/pg", func(c *gin.Context) { // X-U: giả lập tài khoản khác
		uid := uint(7)
		if c.GetHeader("X-U") == "8" {
			uid = 8
		}
		c.Set("user_id", uid)
	})
	pg.GET("/sites/:id", e.h.GetSite)
	pg.PUT("/sites/:id/draft", e.h.SaveSiteDraft)
	pg.POST("/sites/:id/publish", e.h.PublishSite)
	pg.GET("/sites/:id/pages", e.h.ListPages)
	pg.POST("/sites/:id/pages", e.h.CreatePage)
	pg.PUT("/sites/:id/pages/order", e.h.ReorderPages)
	pg.PATCH("/sites/:id/pages/:pid", e.h.UpdatePage)
	pg.DELETE("/sites/:id/pages/:pid", e.h.DeletePage)
	pg.POST("/sites/:id/pages/:pid/duplicate", e.h.DuplicatePage)
	pg.DELETE("/sites/:id", e.h.DeleteSite)
	e.r.GET("/api/v1/storefront/public/pages/:pslug", e.h.GetPublicPage)
	return &pagesEnv{e}
}

func (e *pagesEnv) call(method, path, user string, body any) (int, map[string]any) {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.Header.Set("X-U", user)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (e *pagesEnv) api(method, suffix string, body any) (int, map[string]any) {
	return e.call(method, fmt.Sprintf("/pg/sites/%d%s", e.sf.ID, suffix), "", body)
}

func (e *pagesEnv) public(slug string) (int, map[string]any) {
	return e.call("GET", "/api/v1/storefront/public/pages/"+slug+"?slug=test-shop", "", nil)
}

func (e *pagesEnv) publicHome() map[string]any {
	_, out := e.call("GET", "/api/v1/storefront/public?slug=test-shop", "", nil)
	return out
}

func pagesOfResp(out map[string]any) []map[string]any {
	arr, _ := out["pages"].([]any)
	res := make([]map[string]any, 0, len(arr))
	for _, a := range arr {
		res = append(res, a.(map[string]any))
	}
	return res
}

func findPage(out map[string]any, slug string) map[string]any {
	for _, p := range pagesOfResp(out) {
		if p["slug"] == slug {
			return p
		}
	}
	return nil
}

func blocksFrom(raw any) []StorefrontBlock { s, _ := raw.(string); return parseBlocks(s) }

// Website cũ (một mảng section): chuyển thành trang chủ + phần chung, trang công khai không đổi, chạy lại không nhân đôi
func TestPageMigrationOfLegacySite(t *testing.T) {
	e := newPagesEnv(t)
	draft := []StorefrontBlock{tb("a", "announcement"), tb("h", "header"), tb("hero", "hero"), tb("pr", "products"), tb("ft", "footer")}
	published := []StorefrontBlock{tb("h", "header"), tb("hero", "hero"), tb("ft", "footer")}
	e.db.Model(&models.Storefront{}).Where("id = ?", e.sf.ID).Updates(map[string]any{
		"draft_blocks_json": marshalBlocks(draft), "published_blocks_json": marshalBlocks(published),
	})
	before := e.publicHome()["blocks_json"]

	migrateStorefrontPages(e.db)
	migrateStorefrontPages(e.db) // idempotent

	var pages []models.StorefrontPage
	e.db.Where("storefront_id = ?", e.sf.ID).Find(&pages)
	if len(pages) != 1 || pages[0].Kind != models.PageKindHome || pages[0].Slug != "home" || !pages[0].PublishedVisible || !pages[0].IsVisible {
		t.Fatalf("trang chủ: %+v", pages)
	}
	if got := typesOf(parseBlocks(pages[0].DraftBlocksJSON)); got != "hero,products" {
		t.Fatalf("nháp trang chủ: %s", got)
	}
	if got := typesOf(parseBlocks(pages[0].PublishedBlocksJSON)); got != "hero" {
		t.Fatalf("đã xuất bản trang chủ: %s", got)
	}
	var sf models.Storefront
	e.db.First(&sf, e.sf.ID)
	if typesOf(parseBlocks(sf.DraftSharedJSON)) != "announcement,header,footer" || typesOf(parseBlocks(sf.PublishedSharedJSON)) != "header,footer" {
		t.Fatalf("shared: %s | %s", sf.DraftSharedJSON, sf.PublishedSharedJSON)
	}
	// khách xem trang chủ: nội dung y hệt trước khi chuyển (cột cũ không bị đụng)
	if after := e.publicHome()["blocks_json"]; after != before {
		t.Fatalf("trang công khai đổi sau khi chuyển:\n%v\n%v", before, after)
	}
	// GetSite trả danh sách trang + mẫu
	code, out := e.api("GET", "", nil)
	if code != 200 || len(pagesOfResp(out)) != 1 || out["max_pages"] != float64(maxPagesPerSite) || len(out["page_templates"].([]any)) < 6 {
		t.Fatalf("GetSite: %d %v", code, out)
	}
}

func TestPageCreateTemplatesAndValidation(t *testing.T) {
	e := newPagesEnv(t)
	code, out := e.api("POST", "/pages", map[string]any{"template": "return"})
	if code != 201 {
		t.Fatalf("tạo trang đổi trả: %d %v", code, out)
	}
	p := out["page"].(map[string]any)
	if p["slug"] != "chinh-sach-doi-tra" || p["kind"] != "policy" || p["title"] != "Chính sách đổi trả" || p["published_visible"] != false {
		t.Fatalf("page: %v", p)
	}
	if bs := blocksFrom(p["draft_blocks_json"]); len(bs) < 4 || bs[0].Type != "richtext" || bs[0].Data["heading"] != "Chính sách đổi trả" {
		t.Fatalf("nội dung mẫu: %v", p["draft_blocks_json"])
	}
	// cùng tên → đường dẫn tự thêm hậu tố
	_, out = e.api("POST", "/pages", map[string]any{"template": "return"})
	if got := out["page"].(map[string]any)["slug"]; got != "chinh-sach-doi-tra-2" {
		t.Fatalf("slug trùng: %v", got)
	}
	// tên tuỳ ý → slug theo tên; mẫu trắng
	_, out = e.api("POST", "/pages", map[string]any{"title": "Khuyến mãi Tết", "template": "blank"})
	if got := out["page"].(map[string]any)["slug"]; got != "khuyen-mai-tet" {
		t.Fatalf("slug theo tên: %v", got)
	}
	// slug tự nhập: sai định dạng / trùng / dành riêng
	if code, _ = e.api("POST", "/pages", map[string]any{"title": "X", "slug": "Sai Dau"}); code != 400 {
		t.Fatalf("slug sai: %d", code)
	}
	if code, _ = e.api("POST", "/pages", map[string]any{"title": "X", "slug": "home"}); code != 400 {
		t.Fatalf("slug home: %d", code)
	}
	if code, _ = e.api("POST", "/pages", map[string]any{"title": "X", "slug": "khuyen-mai-tet"}); code != 409 {
		t.Fatalf("slug trùng: %d", code)
	}
	if code, _ = e.api("POST", "/pages", map[string]any{"template": "khong-co"}); code != 400 {
		t.Fatalf("mẫu lạ: %d", code)
	}
	// tên chỉ có ký tự đặc biệt → dùng slug của mẫu
	if _, out = e.api("POST", "/pages", map[string]any{"title": "???", "template": "about"}); out["page"].(map[string]any)["slug"] != "gioi-thieu" {
		t.Fatalf("slug mẫu: %v", out)
	}
	// giới hạn số trang
	for i := 0; ; i++ {
		code, _ = e.api("POST", "/pages", map[string]any{"title": fmt.Sprintf("Trang %d", i)})
		if code != 201 {
			break
		}
		if i > maxPagesPerSite {
			t.Fatal("không chặn giới hạn trang")
		}
	}
	if code != 403 {
		t.Fatalf("vượt giới hạn: %d", code)
	}
	var n int64
	e.db.Model(&models.StorefrontPage{}).Where("storefront_id = ?", e.sf.ID).Count(&n)
	if n != maxPagesPerSite {
		t.Fatalf("số trang = %d", n)
	}
}

func TestPageDraftPublishAndPublicView(t *testing.T) {
	e := newPagesEnv(t)
	_, out := e.api("POST", "/pages", map[string]any{"template": "contact"})
	pid := uint(out["page"].(map[string]any)["id"].(float64))

	// chưa xuất bản → khách không xem được
	if code, _ := e.public("lien-he"); code != 404 {
		t.Fatalf("trang chưa xuất bản: %d", code)
	}
	// lưu nháp kiểu mới: đầu trang dùng chung + nội dung trang; block "header" lọt vào nội dung trang bị loại
	shared := marshalBlocks([]StorefrontBlock{tb("sh", "header"), tb("sf", "footer")})
	pageBlocks := marshalBlocks([]StorefrontBlock{tb("c1", "contact"), tb("bad", "header"), {ID: "r1", Type: "richtext", Enabled: true, Data: map[string]interface{}{"body": "xin chào", "link": "javascript:x"}}})
	code, out := e.api("PUT", "/draft", map[string]any{"draft_shared_json": shared, "pages": []any{map[string]any{"id": pid, "draft_blocks_json": pageBlocks}}})
	if code != 200 {
		t.Fatalf("lưu nháp: %d %v", code, out)
	}
	var pg models.StorefrontPage
	e.db.First(&pg, pid)
	if typesOf(parseBlocks(pg.DraftBlocksJSON)) != "contact,richtext" {
		t.Fatalf("nháp trang: %s", pg.DraftBlocksJSON)
	}
	if parseBlocks(pg.DraftBlocksJSON)[1].Data["link"] != "" {
		t.Fatalf("link nguy hiểm còn nguyên: %s", pg.DraftBlocksJSON)
	}
	site := out["site"].(map[string]any)
	if typesOf(parseBlocks(site["draft_shared_json"].(string))) != "header,footer" || typesOf(blocksFrom(site["draft_blocks_json"])) != "header,footer" {
		t.Fatalf("shared / ghép sẵn: %v", site)
	}
	// vẫn chưa xuất bản
	if code, _ = e.public("lien-he"); code != 404 {
		t.Fatalf("sau lưu nháp: %d", code)
	}

	code, out = e.api("POST", "/publish", nil)
	if code != 200 {
		t.Fatalf("xuất bản: %d %v", code, out)
	}
	code, out = e.public("lien-he")
	if code != 200 || out["title"] != "Liên hệ" || out["kind"] != "page" || typesOf(blocksFrom(out["blocks_json"])) != "contact,richtext" {
		t.Fatalf("trang công khai: %d %v", code, out)
	}
	// trang chủ công khai: ghép đầu / chân trang chung; danh sách trang chỉ gồm trang đang hiện
	home := e.publicHome()
	if typesOf(blocksFrom(home["blocks_json"])) != "header,footer" {
		t.Fatalf("trang chủ ghép: %v", home["blocks_json"])
	}
	list := pagesOfResp(home)
	if len(list) != 1 || list[0]["slug"] != "lien-he" || list[0]["title"] != "Liên hệ" {
		t.Fatalf("danh sách trang công khai: %v", list)
	}
	// trang chủ không mở qua /pages/home; trang không tồn tại; site khác không thấy
	if code, _ = e.public("home"); code != 404 {
		t.Fatalf("home qua /pages: %d", code)
	}
	if code, _ = e.public("khong-co"); code != 404 {
		t.Fatalf("trang lạ: %d", code)
	}

	// sửa nháp + đổi tên / đường dẫn → khách vẫn thấy bản cũ cho tới khi xuất bản lại
	e.api("PUT", "/draft", map[string]any{"pages": []any{map[string]any{"id": pid, "draft_blocks_json": marshalBlocks([]StorefrontBlock{tb("n1", "faq")})}}})
	if code, _ = e.api("PATCH", fmt.Sprintf("/pages/%d", pid), map[string]any{"title": "Liên hệ mới", "slug": "lien-he-moi"}); code != 200 {
		t.Fatalf("đổi tên: %d", code)
	}
	if _, out = e.public("lien-he"); out["title"] != "Liên hệ" || typesOf(blocksFrom(out["blocks_json"])) != "contact,richtext" {
		t.Fatalf("khách thấy bản nháp: %v", out)
	}
	if code, _ = e.public("lien-he-moi"); code != 404 {
		t.Fatalf("slug mới lộ trước khi xuất bản: %d", code)
	}
	e.api("POST", "/publish", nil)
	if code, out = e.public("lien-he-moi"); code != 200 || out["title"] != "Liên hệ mới" || typesOf(blocksFrom(out["blocks_json"])) != "faq" {
		t.Fatalf("sau xuất bản lại: %d %v", code, out)
	}
	if code, _ = e.public("lien-he"); code != 404 {
		t.Fatalf("slug cũ còn sống: %d", code)
	}

	// ẩn trang: có hiệu lực công khai sau khi xuất bản
	e.api("PATCH", fmt.Sprintf("/pages/%d", pid), map[string]any{"is_visible": false})
	if code, _ = e.public("lien-he-moi"); code != 200 {
		t.Fatalf("ẩn có hiệu lực ngay (chưa xuất bản): %d", code)
	}
	e.api("POST", "/publish", nil)
	if code, _ = e.public("lien-he-moi"); code != 404 {
		t.Fatalf("trang ẩn vẫn xem được: %d", code)
	}
	if list = pagesOfResp(e.publicHome()); len(list) != 0 {
		t.Fatalf("trang ẩn còn trong danh sách: %v", list)
	}
	// gỡ xuất bản website → trang con cũng không xem được
	e.api("PATCH", fmt.Sprintf("/pages/%d", pid), map[string]any{"is_visible": true})
	e.api("POST", "/publish", nil)
	e.db.Model(&models.Storefront{}).Where("id = ?", e.sf.ID).Update("is_published", false)
	if code, _ = e.public("lien-he-moi"); code != 404 {
		t.Fatalf("site chưa xuất bản mà trang vẫn xem được: %d", code)
	}
}

func TestPageSaveValidationIsAtomic(t *testing.T) {
	e := newPagesEnv(t)
	_, out := e.api("POST", "/pages", map[string]any{"template": "about"})
	pid := uint(out["page"].(map[string]any)["id"].(float64))
	var before models.StorefrontPage
	e.db.First(&before, pid)

	bad := `[{"id":"x","type":"khong-co","enabled":true,"data":{}}]`
	good := marshalBlocks([]StorefrontBlock{tb("g", "faq")})
	// block sai loại ở một trang → cả yêu cầu bị từ chối, trang khác không bị ghi
	_, out = e.api("POST", "/pages", map[string]any{"template": "blank"})
	pid2 := uint(out["page"].(map[string]any)["id"].(float64))
	code, _ := e.api("PUT", "/draft", map[string]any{"pages": []any{
		map[string]any{"id": pid2, "draft_blocks_json": good}, map[string]any{"id": pid, "draft_blocks_json": bad},
	}})
	if code != 400 {
		t.Fatalf("block sai loại: %d", code)
	}
	var p2 models.StorefrontPage
	e.db.First(&p2, pid2)
	if strings.Contains(p2.DraftBlocksJSON, `"g"`) {
		t.Fatalf("ghi nửa chừng: %s", p2.DraftBlocksJSON)
	}
	var after models.StorefrontPage
	e.db.First(&after, pid)
	if after.DraftBlocksJSON != before.DraftBlocksJSON {
		t.Fatal("trang bị ghi dù yêu cầu bị từ chối")
	}
	// id trang của website khác
	other := models.Storefront{UserID: 7, Slug: "other-shop", SiteTitle: "Khác"}
	e.db.Create(&other)
	otherPage := models.StorefrontPage{StorefrontID: other.ID, Slug: "home", Title: "Trang chủ", Kind: models.PageKindHome, IsVisible: true}
	e.db.Create(&otherPage)
	if code, _ = e.api("PUT", "/draft", map[string]any{"pages": []any{map[string]any{"id": otherPage.ID, "draft_blocks_json": good}}}); code != 400 {
		t.Fatalf("trang của website khác: %d", code)
	}
}

// Giao diện / API bản cũ gửi mảng "ghép sẵn": tách ngược thành phần chung + trang chủ, trang khác không bị đụng
func TestPageLegacyDraftAndPublishCompat(t *testing.T) {
	e := newPagesEnv(t)
	_, out := e.api("POST", "/pages", map[string]any{"template": "about"})
	pid := uint(out["page"].(map[string]any)["id"].(float64))
	var about models.StorefrontPage
	e.db.First(&about, pid)

	legacy := marshalBlocks([]StorefrontBlock{tb("h", "header"), tb("hero", "hero"), tb("ft", "footer")})
	code, out := e.api("PUT", "/draft", map[string]any{"draft_blocks_json": legacy, "site_title": "Tên mới"})
	if code != 200 {
		t.Fatalf("lưu nháp kiểu cũ: %d %v", code, out)
	}
	site := out["site"].(map[string]any)
	if site["site_title"] != "Tên mới" || typesOf(blocksFrom(site["draft_blocks_json"])) != "header,hero,footer" || typesOf(parseBlocks(site["draft_shared_json"].(string))) != "header,footer" {
		t.Fatalf("site: %v", site)
	}
	var home models.StorefrontPage
	e.db.Where("storefront_id = ? AND kind = ?", e.sf.ID, models.PageKindHome).First(&home)
	if typesOf(parseBlocks(home.DraftBlocksJSON)) != "hero" {
		t.Fatalf("trang chủ: %s", home.DraftBlocksJSON)
	}
	var again models.StorefrontPage
	e.db.First(&again, pid)
	if again.DraftBlocksJSON != about.DraftBlocksJSON {
		t.Fatal("trang khác bị đổi bởi lưu kiểu cũ")
	}
	e.api("POST", "/publish", nil)
	if got := typesOf(blocksFrom(e.publicHome()["blocks_json"])); got != "header,hero,footer" {
		t.Fatalf("xuất bản: %s", got)
	}
	// cột cũ published_blocks_json được cập nhật cho bản cũ đọc
	var sf models.Storefront
	e.db.First(&sf, e.sf.ID)
	if typesOf(parseBlocks(sf.PublishedBlocksJSON)) != "header,hero,footer" {
		t.Fatalf("cột cũ: %s", sf.PublishedBlocksJSON)
	}
	// xuất bản trang "about" cùng lần đó
	if code, _ := e.public("gioi-thieu"); code != 200 {
		t.Fatalf("trang giới thiệu sau publish: %d", code)
	}
}

func TestPageManageDuplicateDeleteReorderAndOwnership(t *testing.T) {
	e := newPagesEnv(t)
	mk := func(tpl string) (uint, string) {
		_, out := e.api("POST", "/pages", map[string]any{"template": tpl})
		p := out["page"].(map[string]any)
		return uint(p["id"].(float64)), p["slug"].(string)
	}
	a, _ := mk("about")
	b, _ := mk("contact")
	c, _ := mk("privacy")
	var homeID uint
	e.db.Model(&models.StorefrontPage{}).Where("storefront_id = ? AND kind = ?", e.sf.ID, "home").Select("id").Scan(&homeID)

	// nhân bản: ẩn, id section mới, tên "(bản sao)"
	code, out := e.api("POST", fmt.Sprintf("/pages/%d/duplicate", a), nil)
	if code != 201 {
		t.Fatalf("nhân bản: %d %v", code, out)
	}
	cp := out["page"].(map[string]any)
	if cp["is_visible"] != false || !strings.Contains(cp["title"].(string), "bản sao") || cp["slug"] != "gioi-thieu-ban-sao" {
		t.Fatalf("bản sao: %v", cp)
	}
	var orig, copyP models.StorefrontPage
	e.db.First(&orig, a)
	e.db.First(&copyP, uint(cp["id"].(float64)))
	ob, cb := parseBlocks(orig.DraftBlocksJSON), parseBlocks(copyP.DraftBlocksJSON)
	if len(ob) != len(cb) || ob[0].ID == cb[0].ID || ob[0].Type != cb[0].Type {
		t.Fatalf("id section bản sao: %v %v", ob, cb)
	}
	// nhân bản trang chủ → thành trang thường (không có 2 trang chủ)
	code, out = e.api("POST", fmt.Sprintf("/pages/%d/duplicate", homeID), nil)
	if code != 201 || out["page"].(map[string]any)["kind"] != "page" {
		t.Fatalf("nhân bản trang chủ: %d %v", code, out)
	}

	// sắp xếp: trang chủ luôn đầu, id lạ bị bỏ qua
	code, out = e.api("PUT", "/pages/order", map[string]any{"ids": []uint{c, 99999, b, a}})
	if code != 200 {
		t.Fatalf("sắp xếp: %d", code)
	}
	var slugs []string
	for _, p := range pagesOfResp(out) {
		slugs = append(slugs, p["slug"].(string))
	}
	if len(slugs) < 5 || slugs[0] != "home" || slugs[1] != "chinh-sach-bao-mat" || slugs[2] != "lien-he" || slugs[3] != "gioi-thieu" {
		t.Fatalf("thứ tự: %v", slugs)
	}

	// không xoá / ẩn / đổi đường dẫn trang chủ
	if code, _ = e.api("DELETE", fmt.Sprintf("/pages/%d", homeID), nil); code != 400 {
		t.Fatalf("xoá trang chủ: %d", code)
	}
	e.api("PATCH", fmt.Sprintf("/pages/%d", homeID), map[string]any{"is_visible": false, "slug": "doi-ten", "title": "Nhà"})
	var home models.StorefrontPage
	e.db.First(&home, homeID)
	if !home.IsVisible || home.Slug != "home" || home.Title != "Nhà" {
		t.Fatalf("trang chủ: %+v", home)
	}
	if code, _ = e.api("PATCH", fmt.Sprintf("/pages/%d", a), map[string]any{"title": "  "}); code != 400 {
		t.Fatalf("tên rỗng: %d", code)
	}
	if code, _ = e.api("PATCH", fmt.Sprintf("/pages/%d", a), map[string]any{"slug": "lien-he"}); code != 409 {
		t.Fatalf("slug trùng: %d", code)
	}
	if code, _ = e.api("PATCH", fmt.Sprintf("/pages/%d", a), map[string]any{}); code != 400 {
		t.Fatalf("không có gì để đổi: %d", code)
	}

	// tài khoản khác: không thấy / không sửa được gì (404 như website không tồn tại)
	base := fmt.Sprintf("/pg/sites/%d", e.sf.ID)
	for _, tc := range [][2]string{{"GET", base}, {"GET", base + "/pages"}, {"POST", base + "/pages"}, {"PUT", base + "/draft"}, {"POST", base + "/publish"},
		{"PATCH", fmt.Sprintf("%s/pages/%d", base, a)}, {"DELETE", fmt.Sprintf("%s/pages/%d", base, a)}, {"POST", fmt.Sprintf("%s/pages/%d/duplicate", base, a)}, {"PUT", base + "/pages/order"}} {
		if code, _ := e.call(tc[0], tc[1], "8", map[string]any{"title": "x", "ids": []uint{a}}); code != 404 {
			t.Errorf("%s %s bởi tài khoản khác: %d", tc[0], tc[1], code)
		}
	}
	// id trang của website khác trong URL của mình
	other := models.Storefront{UserID: 7, Slug: "other-shop2", SiteTitle: "Khác"}
	e.db.Create(&other)
	op := models.StorefrontPage{StorefrontID: other.ID, Slug: "x", Title: "X", Kind: models.PageKindPage, IsVisible: true}
	e.db.Create(&op)
	if code, _ = e.api("DELETE", fmt.Sprintf("/pages/%d", op.ID), nil); code != 404 {
		t.Fatalf("xoá trang của website khác: %d", code)
	}

	// xoá trang + xoá website dọn sạch bảng trang
	if code, _ = e.api("DELETE", fmt.Sprintf("/pages/%d", b), nil); code != 200 {
		t.Fatalf("xoá trang: %d", code)
	}
	var n int64
	e.db.Model(&models.StorefrontPage{}).Where("id = ?", b).Count(&n)
	if n != 0 {
		t.Fatal("trang chưa bị xoá")
	}
	if code, _ = e.api("DELETE", "", nil); code != 200 {
		t.Fatalf("xoá website: %d", code)
	}
	e.db.Model(&models.StorefrontPage{}).Where("storefront_id = ?", e.sf.ID).Count(&n)
	if n != 0 {
		t.Fatalf("còn %d trang mồ côi sau khi xoá website", n)
	}
}

// Website tạo mới (CreateSite) và website tạo bằng đường cũ đều có sẵn trang chủ + phần dùng chung
func TestPageNewSitesGetHomePage(t *testing.T) {
	e := newPagesEnv(t)
	pg := e.r.Group("/pgn", func(c *gin.Context) { c.Set("user_id", uint(7)) })
	pg.POST("/sites", e.h.CreateSite)
	for _, tpl := range []string{"", "wellness"} {
		code, out := e.call("POST", "/pgn/sites", "", map[string]any{"site_title": "Shop " + tpl + "x", "template": tpl})
		if code != 201 {
			t.Fatalf("tạo site %q: %d %v", tpl, code, out)
		}
		id := uint(out["site"].(map[string]any)["id"].(float64))
		var pages []models.StorefrontPage
		e.db.Where("storefront_id = ?", id).Find(&pages)
		if len(pages) != 1 || pages[0].Kind != models.PageKindHome {
			t.Fatalf("site %q: %+v", tpl, pages)
		}
		var sf models.Storefront
		e.db.First(&sf, id)
		sh := parseBlocks(sf.DraftSharedJSON)
		want := "header" // mẫu mặc định chỉ có đầu trang; mẫu wellness có đủ thông báo + đầu + chân trang
		if tpl == "wellness" {
			want = "announcement,header,footer"
		}
		if typesOf(sh) != want {
			t.Fatalf("site %q phần dùng chung = %s, muốn %s", tpl, typesOf(sh), want)
		}
		if _, page := splitBlocks(parseBlocks(pages[0].DraftBlocksJSON)); len(page) != len(parseBlocks(pages[0].DraftBlocksJSON)) {
			t.Fatalf("trang chủ còn section dùng chung")
		}
	}
	// getOrCreateForUser (đường /storefront cũ)
	sf, err := e.h.getOrCreateForUser(4242, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	var n int64
	e.db.Model(&models.StorefrontPage{}).Where("storefront_id = ?", sf.ID).Count(&n)
	if n != 1 {
		t.Fatalf("getOrCreateForUser: %d trang", n)
	}
}
