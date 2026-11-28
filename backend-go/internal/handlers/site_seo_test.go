package handlers

// Giai đoạn 2: menu quản lý riêng, SEO từng trang, sitemap, landing page (ẩn đầu / chân trang), AI gợi ý SEO.
//   TEST_DATABASE_URL=postgres://fbcrm:fbcrm_secret_password@localhost:5432/fbcrm_test?sslmode=disable go test ./internal/handlers -run 'Menu|SEO|Sitemap|Landing' -v

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
)

func TestMenusSanitize(t *testing.T) {
	raw := `[
	 {"id":"main","name":"  Menu   chính ","items":[
	   {"label":"Trang chủ","link":"page:home"},
	   {"label":"  ","link":"#x"},
	   {"label":"Chính sách","link":"","children":[
	      {"label":"Đổi trả","link":"page:chinh-sach-doi-tra","children":[{"label":"quá sâu","link":"#"}]},
	      {"label":"Xấu","link":"javascript:alert(1)"}]},
	   {"label":"Liên hệ","link":"https://example.com"}]},
	 {"id":"main","name":"","items":[]},
	 {"id":"Bad Id!","name":"X","items":[{"label":"a","link":"#a"}]}
	]`
	out, err := sanitizeMenusJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var ms []siteMenu
	_ = json.Unmarshal([]byte(out), &ms)
	if len(ms) != 3 || ms[0].ID != "main" || ms[0].Name != "Menu chính" {
		t.Fatalf("menus: %s", out)
	}
	if ms[1].ID == "main" || ms[2].ID == "bad-id" || ms[1].Name != "Menu" || ms[2].ID == ms[1].ID {
		t.Fatalf("id phải duy nhất & hợp lệ: %s", out)
	}
	it := ms[0].Items
	if len(it) != 3 || it[0].Link != "page:home" || it[1].Label != "Chính sách" || it[2].Link != "https://example.com" {
		t.Fatalf("mục: %+v", it)
	}
	ch := it[1].Children
	if len(ch) != 2 || ch[0].Link != "page:chinh-sach-doi-tra" || len(ch[0].Children) != 0 || ch[1].Link != "" {
		t.Fatalf("con (tối đa 2 cấp, link xấu bị bỏ): %+v", ch)
	}
	if o, _ := sanitizeMenusJSON(""); o != "[]" {
		t.Fatalf("rỗng: %q", o)
	}
	if _, err := sanitizeMenusJSON("{không phải mảng"); err == nil {
		t.Fatal("JSON hỏng phải báo lỗi")
	}
	// giới hạn số menu / mục
	var big []siteMenu
	for i := 0; i < 15; i++ {
		var items []menuItem
		for j := 0; j < 60; j++ {
			items = append(items, menuItem{Label: fmt.Sprintf("m%d", j), Link: "#"})
		}
		big = append(big, siteMenu{ID: fmt.Sprintf("m%d", i), Name: "x", Items: items})
	}
	bb, _ := json.Marshal(big)
	o2, _ := sanitizeMenusJSON(string(bb))
	var m2 []siteMenu
	_ = json.Unmarshal([]byte(o2), &m2)
	if len(m2) != maxMenus || len(m2[0].Items) != maxMenuItems {
		t.Fatalf("giới hạn: %d menu, %d mục", len(m2), len(m2[0].Items))
	}
}

func TestLandingComposeAndSettings(t *testing.T) {
	shared := []StorefrontBlock{tb("a", "announcement"), tb("h", "header"), tb("f", "footer")}
	page := []StorefrontBlock{tb("1", "hero"), tb("2", "faq")}
	cases := map[string]PageSettings{
		"announcement,header,hero,faq,footer": {},
		"hero,faq,footer":                     {HideHeader: true},
		"announcement,header,hero,faq":        {HideFooter: true},
		"hero,faq":                            {HideHeader: true, HideFooter: true},
	}
	for want, s := range cases {
		if got := typesOf(composeForSettings(shared, page, s)); got != want {
			t.Errorf("%+v → %s, muốn %s", s, got, want)
		}
	}
	// làm sạch: độ dài, ảnh không an toàn, thẻ HTML
	clean := parseSettings(sanitizeSettingsJSON(`{"seo_title":"  <b>Tiêu   đề</b> ","seo_description":"` + strings.Repeat("a", 500) + `","seo_image":"javascript:alert(1)","noindex":true,"hide_header":true}`))
	if clean.SEOTitle != "Tiêu đề" || len([]rune(clean.SEODescription)) != 320 || clean.SEOImage != "" || !clean.NoIndex || !clean.HideHeader || clean.HideFooter {
		t.Fatalf("settings: %+v", clean)
	}
	if s := parseSettings(sanitizeSettingsJSON("không phải json")); s != (PageSettings{}) {
		t.Fatalf("JSON hỏng phải ra mặc định: %+v", s)
	}
	if got := cutWords("Một hai ba bốn năm sáu bảy tám chín mười", 20); got != "Một hai ba bốn năm" {
		t.Fatalf("cutWords: %q", got)
	}
}

type seoEnv struct{ *pagesEnv }

func newSEOEnv(t *testing.T) *seoEnv {
	e := newPagesEnv(t)
	e.r.GET("/api/v1/storefront/public/seo", e.h.GetPublicSEO)
	e.r.GET("/api/v1/storefront/public/sitemap.xml", e.h.GetPublicSitemap)
	return &seoEnv{e}
}

func (e *seoEnv) seo(path string) map[string]any {
	_, out := e.call("GET", "/api/v1/storefront/public/seo?slug=test-shop&path="+path, "", nil)
	return out
}

func (e *seoEnv) sitemap(q string) (int, string) {
	req := httptest.NewRequest("GET", "/api/v1/storefront/public/sitemap.xml?slug=test-shop"+q, nil)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

func (e *seoEnv) newPage(tpl string) (uint, string) {
	_, out := e.api("POST", "/pages", map[string]any{"template": tpl})
	p := out["page"].(map[string]any)
	return uint(p["id"].(float64)), p["slug"].(string)
}

func str(v any) string { s, _ := v.(string); return s }

// Lưu cài đặt trang / menu / SEO website không đụng nội dung trang; xuất bản mới sao sang bản công khai
func TestSEOSettingsSaveAndPublish(t *testing.T) {
	e := newSEOEnv(t)
	about, aboutSlug := e.newPage("about")
	var before models.StorefrontPage
	e.db.First(&before, about)

	menus := `[{"id":"main","name":"Menu chính","items":[{"label":"Giới thiệu","link":"page:gioi-thieu"}]}]`
	seo := `{"description":"Mô tả mặc định của cả website","image":"https://cdn.example.com/share.jpg","noindex":false}`
	set := `{"seo_title":"Giới thiệu thương hiệu","seo_description":"Câu chuyện của chúng tôi","noindex":false,"hide_header":true,"hide_footer":false}`
	code, out := e.api("PUT", "/draft", map[string]any{
		"draft_menus_json": menus, "draft_seo_json": seo, "draft_shared_json": marshalBlocks([]StorefrontBlock{tb("h", "header"), tb("f", "footer")}),
		"pages": []any{map[string]any{"id": about, "draft_settings_json": set}}, // chỉ cài đặt, KHÔNG có nội dung
	})
	if code != 200 {
		t.Fatalf("lưu nháp: %d %v", code, out)
	}
	var after models.StorefrontPage
	e.db.First(&after, about)
	if after.DraftBlocksJSON != before.DraftBlocksJSON {
		t.Fatal("chỉ gửi cài đặt mà nội dung trang bị đổi")
	}
	if p := parseSettings(after.DraftSettingsJSON); p.SEOTitle != "Giới thiệu thương hiệu" || !p.HideHeader {
		t.Fatalf("settings nháp: %s", after.DraftSettingsJSON)
	}
	site := out["site"].(map[string]any)
	if !strings.Contains(str(site["draft_menus_json"]), "page:gioi-thieu") || !strings.Contains(str(site["draft_seo_json"]), "Mô tả mặc định") {
		t.Fatalf("site: %v", site)
	}
	if pg := findPage(out, aboutSlug); pg == nil || !strings.Contains(str(pg["draft_settings_json"]), "Giới thiệu thương hiệu") {
		t.Fatalf("trang trả về thiếu settings: %v", pg)
	}
	// chưa xuất bản: bản công khai chưa có gì
	home := e.publicHome()
	if str(home["menus_json"]) != "[]" || str(home["seo_json"]) != "{}" {
		t.Fatalf("chưa xuất bản mà lộ menu / SEO: %v %v", home["menus_json"], home["seo_json"])
	}
	e.api("POST", "/publish", nil)
	home = e.publicHome()
	if !strings.Contains(str(home["menus_json"]), "page:gioi-thieu") || !strings.Contains(str(home["seo_json"]), "share.jpg") {
		t.Fatalf("sau xuất bản: %v / %v", home["menus_json"], home["seo_json"])
	}
	if typesOf(parseBlocks(str(home["shared_json"]))) != "header,footer" {
		t.Fatalf("shared_json: %v", home["shared_json"])
	}
	if home["home_settings_json"] == nil || parseSettings(str(home["home_settings_json"])).HideHeader {
		t.Fatalf("home_settings_json: %v", home["home_settings_json"])
	}
	_, pg := e.public(aboutSlug)
	if ps := parseSettings(str(pg["settings_json"])); ps.SEOTitle != "Giới thiệu thương hiệu" || !ps.HideHeader {
		t.Fatalf("trang công khai thiếu settings_json: %v", pg)
	}
	// nhân bản trang: giữ landing, bỏ tiêu đề / mô tả SEO
	_, dup := e.api("POST", fmt.Sprintf("/pages/%d/duplicate", about), nil)
	ds := parseSettings(str(dup["page"].(map[string]any)["draft_settings_json"]))
	if ds.SEOTitle != "" || ds.SEODescription != "" || !ds.HideHeader {
		t.Fatalf("bản sao: %+v", ds)
	}
}

// Trang chủ là landing: cột ghép sẵn (cho bản cũ) bỏ đầu / chân trang, nhưng phần dùng chung vẫn còn để trang khác dùng
func TestLandingHomePublish(t *testing.T) {
	e := newSEOEnv(t)
	e.api("PUT", "/draft", map[string]any{"draft_shared_json": marshalBlocks([]StorefrontBlock{tb("h", "header"), tb("f", "footer")})})
	_, out := e.api("GET", "", nil)
	home := out["pages"].([]any)[0].(map[string]any)
	hid := uint(home["id"].(float64))
	e.api("PUT", "/draft", map[string]any{"pages": []any{map[string]any{"id": hid, "draft_blocks_json": marshalBlocks([]StorefrontBlock{tb("x", "hero")}), "draft_settings_json": `{"hide_header":true,"hide_footer":true}`}}})
	// bản nháp cũ luôn ghép đủ
	var sf models.Storefront
	e.db.First(&sf, e.sf.ID)
	if typesOf(parseBlocks(sf.DraftBlocksJSON)) != "header,hero,footer" {
		t.Fatalf("cột nháp cũ phải ghép đủ: %s", sf.DraftBlocksJSON)
	}
	e.api("POST", "/publish", nil)
	pub := e.publicHome()
	if typesOf(blocksFrom(pub["blocks_json"])) != "hero" {
		t.Fatalf("landing: blocks_json công khai = %s", typesOf(blocksFrom(pub["blocks_json"])))
	}
	if typesOf(blocksFrom(pub["shared_json"])) != "header,footer" || typesOf(blocksFrom(pub["home_blocks_json"])) != "hero" {
		t.Fatalf("shared / home riêng: %v / %v", pub["shared_json"], pub["home_blocks_json"])
	}
	if s := parseSettings(str(pub["home_settings_json"])); !s.HideHeader || !s.HideFooter {
		t.Fatalf("home_settings_json: %v", pub["home_settings_json"])
	}
}

func TestSEOMetaResolution(t *testing.T) {
	e := newSEOEnv(t)
	// trang chủ có phụ đề dài → mô tả tự lấy từ đó
	_, out := e.api("GET", "", nil)
	hid := uint(out["pages"].([]any)[0].(map[string]any)["id"].(float64))
	hero := StorefrontBlock{ID: "h1", Type: "hero", Enabled: true, Data: map[string]interface{}{"title": "Chào", "subtitle": "Mỹ phẩm thảo dược chăm sóc da nhạy cảm, dịu nhẹ và an toàn cho cả gia đình bạn mỗi ngày — giao nhanh toàn quốc."}}
	e.api("PUT", "/draft", map[string]any{"pages": []any{map[string]any{"id": hid, "draft_blocks_json": marshalBlocks([]StorefrontBlock{hero})}}})
	about, aboutSlug := e.newPage("about")
	priv, privSlug := e.newPage("privacy")
	hidden, hiddenSlug := e.newPage("terms")
	e.api("PUT", "/draft", map[string]any{"pages": []any{
		map[string]any{"id": about, "draft_settings_json": `{"seo_title":"Giới thiệu – SEO riêng","seo_description":"Mô tả riêng","seo_image":"https://cdn.example.com/about.jpg"}`},
		map[string]any{"id": priv, "draft_settings_json": `{"noindex":true}`},
	}})
	e.api("PATCH", fmt.Sprintf("/pages/%d", hidden), map[string]any{"is_visible": false})
	prod := models.Product{StorefrontID: e.sf.ID, UserID: 7, Name: "Kem dưỡng", Description: "<p>Kem dưỡng ẩm dịu nhẹ cho da khô, dùng được mỗi ngày.</p>", Price: 250000, Stock: -1, IsActive: true, ImageURL: "https://cdn.example.com/kem.jpg", SKU: "KEM-1"}
	e.db.Create(&prod)
	e.db.Create(&models.Product{StorefrontID: e.sf.ID, UserID: 7, Name: "Ẩn", Price: 1, Stock: -1, IsActive: false})

	// trước khi xuất bản: chỉ có trang chủ (website đã xuất bản từ lúc tạo nhưng trang con chưa)
	if m := e.seo("/pages/" + aboutSlug); m["noindex"] != true {
		t.Fatalf("trang chưa xuất bản phải noindex: %v", m)
	}
	e.api("POST", "/publish", nil)

	m := e.seo("/")
	if m["title"] != "Shop thử" || !strings.Contains(str(m["description"]), "Mỹ phẩm thảo dược") || m["noindex"] != false || m["type"] != "website" || m["site_name"] != "Shop thử" {
		t.Fatalf("trang chủ: %v", m)
	}
	if len([]rune(str(m["description"]))) > 155 {
		t.Fatalf("mô tả tự động quá dài: %d", len([]rune(str(m["description"]))))
	}
	m = e.seo("/pages/" + aboutSlug)
	if m["title"] != "Giới thiệu – SEO riêng" || m["description"] != "Mô tả riêng" || m["image"] != "https://cdn.example.com/about.jpg" || m["path"] != "/pages/"+aboutSlug {
		t.Fatalf("trang có SEO riêng: %v", m)
	}
	m = e.seo("/pages/" + privSlug)
	if m["noindex"] != true || m["title"] != "Chính sách bảo mật – Shop thử" {
		t.Fatalf("noindex + tiêu đề mặc định: %v", m)
	}
	if m = e.seo("/pages/" + hiddenSlug); m["noindex"] != true {
		t.Fatalf("trang ẩn: %v", m)
	}
	m = e.seo(fmt.Sprintf("/p/%d", prod.ID))
	pr, _ := m["product"].(map[string]any)
	if m["type"] != "product" || m["title"] != "Kem dưỡng – Shop thử" || pr["price"] != float64(250000) || pr["currency"] != "VND" || pr["in_stock"] != true || pr["sku"] != "KEM-1" || m["image"] != "https://cdn.example.com/kem.jpg" {
		t.Fatalf("sản phẩm: %v", m)
	}
	if !strings.Contains(str(m["description"]), "Kem dưỡng ẩm") {
		t.Fatalf("mô tả sản phẩm: %v", m["description"])
	}
	for _, p := range []string{"/checkout", "/account", "/order/DH1", "/p/99999", "/pages/khong-co"} {
		if m = e.seo(p); m["noindex"] != true {
			t.Errorf("%s phải noindex: %v", p, m)
		}
	}
	// mô tả mặc định của website được dùng khi trang không có mô tả riêng và không tự rút được
	e.api("PUT", "/draft", map[string]any{"draft_seo_json": `{"description":"Mô tả cả website","image":"https://cdn.example.com/site.jpg"}`})
	e.api("POST", "/publish", nil)
	if m = e.seo("/pages/" + privSlug); m["description"] != "Mô tả cả website" || m["image"] != "https://cdn.example.com/site.jpg" {
		t.Fatalf("mặc định website: %v", m)
	}
	// tên miền riêng đang hoạt động → canonical_base
	e.db.Model(&models.Storefront{}).Where("id = ?", e.sf.ID).Updates(map[string]any{"domain": "www.cuahang.vn", "domain_status": "active"})
	if m = e.seo("/"); m["canonical_base"] != "https://www.cuahang.vn" {
		t.Fatalf("canonical_base: %v", m["canonical_base"])
	}
	e.db.Model(&models.Storefront{}).Where("id = ?", e.sf.ID).Update("domain_status", "pending")
	if m = e.seo("/"); m["canonical_base"] != "" {
		t.Fatalf("tên miền chưa hoạt động không được làm canonical: %v", m["canonical_base"])
	}
	// ẩn cả website khỏi công cụ tìm kiếm
	e.api("PUT", "/draft", map[string]any{"draft_seo_json": `{"noindex":true}`})
	e.api("POST", "/publish", nil)
	if m = e.seo("/"); m["noindex"] != true {
		t.Fatalf("noindex cả site: %v", m)
	}
	if code, _ := e.call("GET", "/api/v1/storefront/public/seo?slug=khong-co&path=/", "", nil); code != 404 {
		t.Fatalf("site lạ: %d", code)
	}
}

func TestSitemap(t *testing.T) {
	e := newSEOEnv(t)
	about, aboutSlug := e.newPage("about")
	priv, privSlug := e.newPage("privacy")
	hiddenID, hiddenSlug := e.newPage("terms")
	_, paySlug := e.newPage("payment")
	e.api("PUT", "/draft", map[string]any{"pages": []any{map[string]any{"id": priv, "draft_settings_json": `{"noindex":true}`}}})
	e.api("PATCH", fmt.Sprintf("/pages/%d", hiddenID), map[string]any{"is_visible": false})
	_ = about
	p1 := models.Product{StorefrontID: e.sf.ID, UserID: 7, Name: "A & B <đặc biệt>", Price: 1, Stock: -1, IsActive: true}
	e.db.Create(&p1)
	e.db.Create(&models.Product{StorefrontID: e.sf.ID, UserID: 7, Name: "Ẩn", Price: 1, Stock: -1, IsActive: false})
	e.api("POST", "/publish", nil)

	code, xml := e.sitemap("&origin=https://fb-multipage-crm.pages.dev&prefix=/s/test-shop")
	if code != 200 || !strings.HasPrefix(xml, `<?xml version="1.0" encoding="UTF-8"?>`) || !strings.Contains(xml, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`) {
		t.Fatalf("sitemap: %d %s", code, xml)
	}
	base := "https://fb-multipage-crm.pages.dev/s/test-shop"
	for _, want := range []string{"<loc>" + base + "</loc>", "<loc>" + base + "/pages/" + aboutSlug + "</loc>", "<loc>" + base + "/pages/" + paySlug + "</loc>", fmt.Sprintf("<loc>%s/p/%d</loc>", base, p1.ID)} {
		if !strings.Contains(xml, want) {
			t.Errorf("thiếu %s\n%s", want, xml)
		}
	}
	for _, bad := range []string{"/pages/" + privSlug, "/pages/" + hiddenSlug, fmt.Sprintf("/p/%d<", p1.ID+1)} {
		if strings.Contains(xml, bad) {
			t.Errorf("không được có %s trong sitemap (noindex / ẩn / sản phẩm tắt)", bad)
		}
	}
	if !strings.Contains(xml, "<lastmod>") {
		t.Error("thiếu lastmod")
	}
	// tên miền riêng: không có tiền tố /s/<slug>
	_, xml = e.sitemap("&origin=https://www.cuahang.vn&prefix=")
	if !strings.Contains(xml, "<loc>https://www.cuahang.vn</loc>") || !strings.Contains(xml, "https://www.cuahang.vn/pages/"+aboutSlug) {
		t.Errorf("sitemap tên miền riêng: %s", xml)
	}
	// tham số xấu bị từ chối (không phản chiếu chuỗi tuỳ ý vào XML)
	for _, q := range []string{"&origin=javascript:alert(1)&prefix=", "&origin=https://a.com&prefix=/evil", "&origin=https://a.com/x&prefix=", "&origin=https://a.com\"><x>&prefix=", ""} {
		if code, _ := e.sitemap(q); code != 400 {
			t.Errorf("%q → %d, muốn 400", q, code)
		}
	}
	// noindex cả website → sitemap rỗng
	e.api("PUT", "/draft", map[string]any{"draft_seo_json": `{"noindex":true}`})
	e.api("POST", "/publish", nil)
	if _, xml = e.sitemap("&origin=https://a.com&prefix="); strings.Contains(xml, "<loc>") {
		t.Errorf("website noindex mà sitemap còn URL: %s", xml)
	}
	// site không tồn tại
	req := httptest.NewRequest("GET", "/api/v1/storefront/public/sitemap.xml?slug=khong-co&origin=https://a.com&prefix=", nil)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("site lạ: %d", w.Code)
	}
}

// ----------------------------------------------------------------- AI gợi ý SEO

type fakeSEOAI struct {
	mu    sync.Mutex
	calls int
	users []string
	reply string
}

func (f *fakeSEOAI) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	var req struct {
		Messages []struct{ Content string } `json:"messages"`
	}
	_ = json.Unmarshal(body, &req)
	f.users = append(f.users, req.Messages[len(req.Messages)-1].Content)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": f.reply}}}})
}

func newSEOAIEnv(t *testing.T, f *fakeSEOAI) *seoEnv {
	e := newSEOEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	t.Setenv("AI_BASE_URL", srv.URL)
	t.Setenv("AI_API_KEY", "test")
	t.Setenv("AI_MODEL", "fake")
	e.h.AI = NewAIHandler(e.db, &config.Config{})
	pg := e.r.Group("/pgai", func(c *gin.Context) { c.Set("user_id", uint(7)) })
	pg.POST("/sites/:id/seo/suggest", e.h.SuggestSEO)
	seoAILimiter.Lock()
	seoAILimiter.hits = map[uint][]time.Time{}
	seoAILimiter.Unlock()
	return e
}

func (e *seoEnv) suggest(body map[string]any) (int, map[string]any) {
	return e.call("POST", fmt.Sprintf("/pgai/sites/%d/seo/suggest", e.sf.ID), "", body)
}

func TestSuggestSEOWithAI(t *testing.T) {
	f := &fakeSEOAI{reply: "```json\n{\"title\":\"\\\"Kem dưỡng ẩm thảo dược cho da nhạy cảm – Dịu nhẹ mỗi ngày tại Shop thử\\\"\",\"description\":\"<b>Kem dưỡng</b> ẩm thảo dược dịu nhẹ, phù hợp da nhạy cảm. Đặt mua ngay hôm nay để da mềm mịn và khoẻ mạnh hơn mỗi ngày nhé bạn ơi!\",\"keywords\":[\"kem dưỡng ẩm\",\"  da nhạy cảm \",\"\",\"a\",\"b\",\"c\",\"d\",\"e\",\"f\"]}\n```"}
	e := newSEOAIEnv(t, f)
	code, out := e.suggest(map[string]any{"title": "Trang chủ", "content": "Kem dưỡng ẩm thảo dược cho da nhạy cảm.\nGiao nhanh toàn quốc.", "keywords": "kem dưỡng ẩm", "is_home": true})
	if code != 200 {
		t.Fatalf("gợi ý: %d %v", code, out)
	}
	title, desc := str(out["title"]), str(out["description"])
	if title == "" || len([]rune(title)) > 65 || strings.ContainsAny(title, `"<>`) {
		t.Fatalf("tiêu đề: %q", title)
	}
	if desc == "" || len([]rune(desc)) > 160 || strings.Contains(desc, "<") {
		t.Fatalf("mô tả: %q", desc)
	}
	kws, _ := out["keywords"].([]any)
	if len(kws) != 6 || kws[1] != "da nhạy cảm" {
		t.Fatalf("từ khoá: %v", kws)
	}
	u := f.users[0]
	if !strings.Contains(u, "Website name: Shop thử") || !strings.Contains(u, "home page") || !strings.Contains(u, "Main keywords the owner wants to target: kem dưỡng ẩm") || !strings.Contains(u, "Giao nhanh toàn quốc") {
		t.Fatalf("lời nhắn gửi AI: %s", u)
	}
	// thiếu nội dung
	if code, _ = e.suggest(map[string]any{"title": "", "content": "  "}); code != 400 {
		t.Fatalf("thiếu nội dung: %d", code)
	}
	// AI trả sai định dạng
	f.reply = "Xin lỗi tôi không biết"
	if code, _ = e.suggest(map[string]any{"title": "x", "content": "nội dung"}); code != 502 {
		t.Fatalf("sai định dạng: %d", code)
	}
	// giới hạn lượt
	f.reply = `{"title":"A","description":"B","keywords":[]}`
	seoAILimiter.Lock()
	seoAILimiter.hits = map[uint][]time.Time{}
	seoAILimiter.Unlock()
	last := 0
	for i := 0; i < seoAIPerHour+3; i++ {
		last, _ = e.suggest(map[string]any{"title": "x", "content": "nội dung"})
		if last == 429 {
			break
		}
	}
	if last != 429 {
		t.Fatalf("phải chặn sau %d lượt/giờ, nhận %d", seoAIPerHour, last)
	}
	// chưa cấu hình AI
	e.h.AI = nil
	if code, _ = e.suggest(map[string]any{"content": "x"}); code != 503 {
		t.Fatalf("chưa có AI: %d", code)
	}
}
