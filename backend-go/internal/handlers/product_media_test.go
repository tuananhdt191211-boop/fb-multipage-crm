package handlers

// Test tích hợp: sản phẩm (mô tả HTML, bộ sưu tập ảnh/video), thư viện media (PostgreSQL và kho S3 thật).
//   TEST_DATABASE_URL=postgres://fbcrm:fbcrm_secret_password@localhost:5432/fbcrm_test?sslmode=disable \
//   TEST_S3_ENDPOINT=http://localhost:18000 TEST_S3_ACCESS=accessKey1 TEST_S3_SECRET=verySecretKey1 TEST_S3_BUCKET=pp-test TEST_S3_REGION=us-east-1 \
//   go test ./internal/handlers -run 'Product|SiteMedia' -v
// (không có TEST_S3_* thì các phần thử kho ngoài được bỏ qua)

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ảnh PNG 1x1 hợp lệ
var tinyPNG = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0x0D, 'I', 'H', 'D', 'R', 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1F, 0x15, 0xC4, 0x89,
	0, 0, 0, 0x0D, 'I', 'D', 'A', 'T', 0x78, 0x9C, 0x63, 0xF8, 0xCF, 0xC0, 0xF0, 0x1F, 0, 5, 0, 1, 0xFF, 0x89, 0x99, 0x3D, 0x1D, 0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xAE, 0x42, 0x60, 0x82}

type productEnv struct {
	db  *gorm.DB
	h   *StorefrontHandler
	r   *gin.Engine
	sf  models.Storefront
	uid uint
}

func newProductEnv(t *testing.T) *productEnv {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("đặt TEST_DATABASE_URL (database tên kết thúc bằng _test) để chạy test tích hợp")
	}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("không kết nối được database test: %v", err)
	}
	var name string
	db.Raw("SELECT current_database()").Scan(&name)
	if !strings.HasSuffix(name, "_test") {
		t.Fatalf("TỪ CHỐI chạy trên database %q: tên phải kết thúc bằng _test", name)
	}
	h := NewStorefrontHandler(db, &config.Config{FrontendURL: "http://localhost:4173"})
	db.Exec("TRUNCATE products, site_media, storefronts RESTART IDENTITY CASCADE")
	sf := models.Storefront{UserID: 7, Slug: "test-shop", SiteTitle: "Shop thử", IsPublished: true, PrimaryColor: "#059669"}
	if err := db.Create(&sf).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	adm := r.Group("/api/v1/storefront", func(c *gin.Context) { c.Set("user_id", uint(7)) })
	adm.GET("/media-config", h.MediaConfig)
	adm.GET("/sites/:id/products", h.ListProducts)
	adm.POST("/sites/:id/products", h.CreateProduct)
	adm.GET("/sites/:id/products/:pid", h.GetProduct)
	adm.PUT("/sites/:id/products/:pid", h.UpdateProduct)
	adm.PATCH("/sites/:id/products/:pid", h.PatchProduct)
	adm.POST("/sites/:id/media", h.UploadSiteMedia)
	adm.POST("/sites/:id/media/video", h.UploadSiteVideo)
	adm.POST("/sites/:id/media/import", h.ImportSiteMedia)
	adm.GET("/sites/:id/media", h.ListSiteMedia)
	adm.DELETE("/sites/:id/media/:mid", h.DeleteSiteMedia)
	r.GET("/api/v1/storefront/media/:mid", h.ServeSiteMedia)
	r.GET("/api/v1/storefront/public", h.GetPublicStorefront)
	r.GET("/api/v1/storefront/public/products/:pid", h.GetPublicProduct)
	return &productEnv{db: db, h: h, r: r, sf: sf, uid: 7}
}

func (e *productEnv) do(method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func (e *productEnv) upload(path, field, filename string, data []byte, extra map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile(field, filename)
	_, _ = fw.Write(data)
	for k, v := range extra {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func (e *productEnv) base() string { return fmt.Sprintf("/api/v1/storefront/sites/%d", e.sf.ID) }

func TestProductRichDescriptionAndMedia(t *testing.T) {
	e := newProductEnv(t)
	media := []map[string]any{
		{"type": "image", "url": "https://cdn.example.com/a.jpg", "alt": "Ảnh chính"},
		{"type": "image", "url": "javascript:alert(1)"},
		{"type": "video", "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
		{"type": "video", "url": "https://cdn.example.com/clip.mp4", "poster": "https://cdn.example.com/p.jpg"},
		{"type": "video", "url": "https://evil.example.com/page.html"},
		{"type": "embed", "url": "https://vimeo.com/123456789"},
		{"type": "image", "url": "https://cdn.example.com/b.jpg"},
	}
	w, out := e.do("POST", e.base()+"/products", map[string]any{
		"name": "Micro Hamax", "description": `<h1>Giới thiệu</h1><p onclick="x()">Mô tả <strong>đậm</strong></p><script>alert(1)</script><iframe src="https://evil"></iframe>`,
		"price": 259000, "compare_price": 399000, "cost_price": 120000, "stock": -1, "media": media,
		"sku": "HX-01", "barcode": "893000", "source_url": "https://www.aliexpress.com/item/1005008867854647.html", "tags": "micro, không dây, Micro,  ",
	})
	if w.Code != 201 {
		t.Fatalf("tạo sản phẩm: %d %s", w.Code, w.Body.String())
	}
	p := out["product"].(map[string]any)
	id := fmt.Sprintf("%.0f", p["id"].(float64))

	desc := p["description"].(string)
	if !strings.Contains(desc, "<h2>Giới thiệu</h2>") || !strings.Contains(desc, "<strong>đậm</strong>") {
		t.Errorf("mô tả mất định dạng: %q", desc)
	}
	for _, bad := range []string{"script", "onclick", "iframe", "alert"} {
		if strings.Contains(desc, bad) {
			t.Errorf("mô tả còn %q: %q", bad, desc)
		}
	}
	gotMedia := p["media"].([]any)
	if len(gotMedia) != 5 { // ảnh a, youtube, mp4, vimeo, ảnh b (javascript: và .html bị loại)
		t.Fatalf("bộ sưu tập có %d mục, muốn 5: %v", len(gotMedia), gotMedia)
	}
	m1 := gotMedia[1].(map[string]any)
	if m1["type"] != "embed" || m1["url"] != "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ" || !strings.Contains(m1["poster"].(string), "i.ytimg.com/vi/dQw4w9WgXcQ") {
		t.Errorf("YouTube chưa chuẩn hoá thành trang nhúng: %v", m1)
	}
	if m2 := gotMedia[2].(map[string]any); m2["type"] != "video" || m2["poster"] != "https://cdn.example.com/p.jpg" {
		t.Errorf("video mp4 sai: %v", m2)
	}
	if m3 := gotMedia[3].(map[string]any); m3["type"] != "embed" || m3["url"] != "https://player.vimeo.com/video/123456789" {
		t.Errorf("Vimeo sai: %v", m3)
	}
	if p["image_url"] != "https://cdn.example.com/a.jpg" {
		t.Errorf("ảnh bìa phải là ảnh đầu tiên, được %v", p["image_url"])
	}
	if p["tags"] != "micro, không dây" {
		t.Errorf("tags chưa làm sạch/khử trùng lặp: %q", p["tags"])
	}

	// danh sách nhẹ: không có mô tả đầy đủ, có tóm tắt + số media + cờ video
	_, list := e.do("GET", e.base()+"/products?lite=1", nil)
	lp := list["products"].([]any)[0].(map[string]any)
	if _, has := lp["description"]; has {
		t.Error("danh sách lite không được kèm mô tả đầy đủ")
	}
	if lp["media_count"].(float64) != 5 || lp["has_video"] != true || !strings.Contains(lp["summary"].(string), "Giới thiệu") || lp["cost_price"].(float64) != 120000 {
		t.Errorf("dòng lite sai: %v", lp)
	}
	// danh sách đầy đủ (trình dựng web): có mô tả + tóm tắt
	_, full := e.do("GET", e.base()+"/products", nil)
	fp := full["products"].([]any)[0].(map[string]any)
	if fp["description"] == "" || fp["summary"] == "" {
		t.Errorf("danh sách đầy đủ thiếu mô tả/tóm tắt: %v", fp)
	}
	// chi tiết
	if _, det := e.do("GET", e.base()+"/products/"+id, nil); det["product"].(map[string]any)["source_url"] != "https://www.aliexpress.com/item/1005008867854647.html" {
		t.Errorf("chi tiết thiếu nguồn hàng: %v", det)
	}

	// PATCH chỉ đổi trạng thái, KHÔNG đụng mô tả / ảnh
	if w, _ := e.do("PATCH", e.base()+"/products/"+id, map[string]any{"is_active": false}); w.Code != 200 {
		t.Fatalf("patch: %d %s", w.Code, w.Body.String())
	}
	var row models.Product
	e.db.First(&row, id)
	if row.IsActive || row.Description == "" || len(row.Media) != 5 {
		t.Errorf("patch làm mất dữ liệu: active=%v desc=%d media=%d", row.IsActive, len(row.Description), len(row.Media))
	}

	// sản phẩm đang ẩn: web công khai không thấy, API chi tiết công khai trả 404
	if w, _ := e.do("GET", "/api/v1/storefront/public/products/"+id+"?slug=test-shop", nil); w.Code != 404 {
		t.Errorf("sản phẩm ẩn mà công khai vẫn thấy: %d", w.Code)
	}
	e.do("PATCH", e.base()+"/products/"+id, map[string]any{"is_active": true})

	// công khai: danh sách nhẹ (không mô tả đầy đủ) + chi tiết đầy đủ (không lộ giá nhập, nguồn hàng, SKU tồn kho...)
	_, pub := e.do("GET", "/api/v1/storefront/public?slug=test-shop", nil)
	card := pub["products"].([]any)[0].(map[string]any)
	if _, has := card["description"]; has || card["summary"] == "" || card["in_stock"] != true {
		t.Errorf("thẻ công khai sai: %v", card)
	}
	_, detail := e.do("GET", "/api/v1/storefront/public/products/"+id+"?slug=test-shop", nil)
	pp := detail["product"].(map[string]any)
	if pp["sku"] != "HX-01" || len(pp["media"].([]any)) != 5 || !strings.Contains(pp["description"].(string), "<strong>") {
		t.Errorf("chi tiết công khai sai: %v", pp)
	}
	for _, secret := range []string{"cost_price", "source_url", "tags", "stock", "barcode", "user_id"} {
		if _, leaked := pp[secret]; leaked {
			t.Errorf("chi tiết công khai lộ %q", secret)
		}
	}
	// site khác không xem được sản phẩm của site này
	if w, _ := e.do("GET", "/api/v1/storefront/public/products/"+id+"?slug=khong-co", nil); w.Code != 404 {
		t.Errorf("slug lạ phải 404, được %d", w.Code)
	}

	// PUT thay thế toàn bộ
	w, out = e.do("PUT", e.base()+"/products/"+id, map[string]any{"name": "Micro Hamax Pro", "description": "Văn bản thuần\nxuống dòng", "price": 300000, "stock": 5, "media": []map[string]any{}, "image_url": "https://cdn.example.com/legacy.jpg"})
	if w.Code != 200 {
		t.Fatalf("put: %d %s", w.Code, w.Body.String())
	}
	up := out["product"].(map[string]any)
	if up["description"] != "<p>Văn bản thuần<br>xuống dòng</p>" || up["image_url"] != "https://cdn.example.com/legacy.jpg" || len(up["media"].([]any)) != 1 {
		t.Errorf("PUT với ảnh kiểu cũ / mô tả thuần sai: %v", up)
	}

	// kiểm tra đầu vào
	for name, body := range map[string]map[string]any{
		"thiếu tên":     {"name": " "},
		"giá âm":        {"name": "x", "price": -1},
		"giá nhập âm":   {"name": "x", "cost_price": -5},
		"mô tả quá dài": {"name": "x", "description": "<p>" + strings.Repeat("a", 700000) + "</p>"},
	} {
		if w, _ := e.do("POST", e.base()+"/products", body); w.Code != 400 {
			t.Errorf("%s: phải 400, được %d", name, w.Code)
		}
	}
}

func TestLegacyProductStillWorks(t *testing.T) {
	e := newProductEnv(t)
	// sản phẩm tạo bằng bản cũ: chỉ có ImageURL và mô tả thuần, chưa có media_json
	legacy := models.Product{StorefrontID: e.sf.ID, UserID: 7, Name: "SP cũ", Description: "Mô tả cũ\n\nĐoạn hai", Price: 100000, ImageURL: "https://cdn.example.com/old.jpg", Stock: -1, IsActive: true}
	if err := e.db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprint(legacy.ID)
	_, det := e.do("GET", e.base()+"/products/"+id, nil)
	p := det["product"].(map[string]any)
	if p["description"] != "<p>Mô tả cũ</p><p>Đoạn hai</p>" {
		t.Errorf("mô tả cũ chưa đổi sang HTML: %q", p["description"])
	}
	if ms := p["media"].([]any); len(ms) != 1 || ms[0].(map[string]any)["url"] != "https://cdn.example.com/old.jpg" {
		t.Errorf("ảnh cũ chưa thành bộ sưu tập: %v", p["media"])
	}
	_, pub := e.do("GET", "/api/v1/storefront/public/products/"+id+"?slug=test-shop", nil)
	if pub["product"].(map[string]any)["description"] != "<p>Mô tả cũ</p><p>Đoạn hai</p>" {
		t.Errorf("công khai: %v", pub)
	}
	// đơn hàng vẫn tạo được: giá/tồn kho/ảnh bìa không đổi nghĩa
	if legacy.Price != 100000 || coverOf(legacy.Media) != "" {
		_ = legacy // (AfterFind chưa chạy với struct vừa Create — kiểm tra bằng bản đọc lại)
	}
	var re models.Product
	e.db.First(&re, legacy.ID)
	if coverOf(re.Media) != "https://cdn.example.com/old.jpg" {
		t.Errorf("ảnh bìa từ media: %q", coverOf(re.Media))
	}
}

func TestSiteMediaPostgresMode(t *testing.T) {
	e := newProductEnv(t)
	e.h.Store = nil
	// cấu hình: không có kho ngoài → tắt video
	if _, cfg := e.do("GET", "/api/v1/storefront/media-config", nil); cfg["video_upload"] != false || cfg["storage"] != "db" {
		t.Errorf("media-config sai: %v", cfg)
	}
	w, out := e.upload(e.base()+"/media", "file", "anh.png", tinyPNG, map[string]string{"width": "1", "height": "1"})
	if w.Code != 201 {
		t.Fatalf("tải ảnh: %d %s", w.Code, w.Body.String())
	}
	m := out["media"].(map[string]any)
	url := m["url"].(string)
	if !strings.Contains(url, "/api/v1/storefront/media/") || m["kind"] != "image" {
		t.Fatalf("ảnh trong PostgreSQL phải phục vụ qua backend: %v", m)
	}
	sw := httptest.NewRecorder()
	e.r.ServeHTTP(sw, httptest.NewRequest("GET", url[strings.Index(url, "/api/"):], nil))
	if sw.Code != 200 || !bytes.Equal(sw.Body.Bytes(), tinyPNG) || sw.Header().Get("Content-Type") != "image/png" {
		t.Errorf("phục vụ ảnh sai: %d %s", sw.Code, sw.Header().Get("Content-Type"))
	}
	// không phải ảnh
	if w, _ := e.upload(e.base()+"/media", "file", "x.png", []byte("<svg onload=alert(1)>"), nil); w.Code != 400 {
		t.Errorf("file không phải ảnh phải 400, được %d", w.Code)
	}
	// video khi chưa có kho ngoài
	if w, o := e.upload(e.base()+"/media/video", "file", "a.mp4", append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2'}, make([]byte, 100)...), nil); w.Code != 503 || !strings.Contains(fmt.Sprint(o["error"]), "R2") {
		t.Errorf("video khi chưa có kho phải 503 kèm hướng dẫn: %d %v", w.Code, o)
	}
	// danh sách chỉ ảnh; xoá
	_, list := e.do("GET", e.base()+"/media", nil)
	if len(list["media"].([]any)) != 1 {
		t.Errorf("danh sách ảnh: %v", list)
	}
	if w, _ := e.do("DELETE", fmt.Sprintf("%s/media/%.0f", e.base(), m["id"].(float64)), nil); w.Code != 200 {
		t.Errorf("xoá: %d", w.Code)
	}
}

func TestSiteMediaImportFromURL(t *testing.T) {
	e := newProductEnv(t)
	e.h.Store = nil
	// Máy chủ nguồn thử ở 127.0.0.1: mặc định bị chặn (SSRF)
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(tinyPNG)
		case "/fake.png":
			_, _ = w.Write([]byte("<html>không phải ảnh</html>"))
		default:
			w.WriteHeader(404)
		}
	}))
	defer src.Close()
	_, out := e.do("POST", e.base()+"/media/import", map[string]any{"urls": []string{src.URL + "/ok.png"}})
	r0 := out["results"].([]any)[0].(map[string]any)
	if r0["error"] == nil || r0["media"] != nil {
		t.Fatalf("127.0.0.1 phải bị chặn: %v", r0)
	}
	// cổng lạ cũng bị từ chối khi chưa bật cờ kiểm thử
	if _, o := e.do("POST", e.base()+"/media/import", map[string]any{"urls": []string{"https://example.com:8443/a.png", "ftp://x/a.png", "not a url"}}); true {
		for _, r := range o["results"].([]any) {
			if r.(map[string]any)["media"] != nil {
				t.Errorf("link không hợp lệ / cổng lạ phải bị từ chối: %v", r)
			}
		}
	}
	// cho phép máy chủ thử (cổng ngẫu nhiên, 127.0.0.1) rồi kiểm tra đường tải thành công
	old, oldPorts := mediaFetchClient, allowCustomFetchPorts
	mediaFetchClient = func() *http.Client { return &http.Client{Timeout: 5 * time.Second} }
	allowCustomFetchPorts = true
	defer func() { mediaFetchClient, allowCustomFetchPorts = old, oldPorts }()
	_, out = e.do("POST", e.base()+"/media/import", map[string]any{"urls": []string{src.URL + "/ok.png", src.URL + "/fake.png", src.URL + "/missing.png"}})
	res := out["results"].([]any)
	okRes := res[0].(map[string]any)
	if okRes["media"] == nil || !strings.Contains(okRes["media"].(map[string]any)["url"].(string), "/api/v1/storefront/media/") {
		t.Fatalf("tải ảnh hợp lệ phải thành công: %v", okRes)
	}
	if res[1].(map[string]any)["error"] == nil || res[2].(map[string]any)["error"] == nil {
		t.Errorf("file không phải ảnh / 404 phải báo lỗi: %v", res[1:])
	}
	if e.h.siteMediaCount(e.sf.ID) != 1 {
		t.Errorf("chỉ ảnh hợp lệ được lưu vào thư viện, có %d", e.h.siteMediaCount(e.sf.ID))
	}
	if w, _ := e.do("POST", e.base()+"/media/import", map[string]any{"urls": []string{}}); w.Code != 400 {
		t.Errorf("danh sách rỗng phải 400, được %d", w.Code)
	}
}

func TestSiteMediaObjectStorageMode(t *testing.T) {
	ep := os.Getenv("TEST_S3_ENDPOINT")
	if ep == "" {
		t.Skip("đặt TEST_S3_* để thử với kho S3 thật")
	}
	e := newProductEnv(t)
	cfg := storage.Config{Endpoint: ep, Region: os.Getenv("TEST_S3_REGION"), Bucket: os.Getenv("TEST_S3_BUCKET"), AccessKey: os.Getenv("TEST_S3_ACCESS"), SecretKey: os.Getenv("TEST_S3_SECRET"), PublicBase: "https://cdn.test.example/media"}
	e.h.Store = storage.New(cfg)
	ctx := context.Background()
	// bucket có sẵn từ test của gói storage; tạo nếu chưa có
	mk, _ := http.NewRequest(http.MethodPut, ep+"/"+cfg.Bucket, nil)
	_ = mk

	if _, c := e.do("GET", "/api/v1/storefront/media-config", nil); c["video_upload"] != true || c["storage"] != "s3" {
		t.Errorf("media-config: %v", c)
	}

	// ảnh → kho ngoài, URL công khai thuộc PublicBase, không lưu bytea trong DB
	w, out := e.upload(e.base()+"/media", "file", "anh.png", tinyPNG, nil)
	if w.Code != 201 {
		t.Fatalf("tải ảnh: %d %s", w.Code, w.Body.String())
	}
	img := out["media"].(map[string]any)
	if !strings.HasPrefix(img["url"].(string), "https://cdn.test.example/media/sites/") || !strings.HasSuffix(img["url"].(string), ".png") {
		t.Errorf("URL ảnh sai: %v", img["url"])
	}
	var row SiteMedia
	e.db.First(&row, img["id"])
	if len(row.Data) != 0 || row.StorageKey == "" {
		t.Errorf("ảnh ở kho ngoài không được lưu bytea: data=%d key=%q", len(row.Data), row.StorageKey)
	}
	got, err := e.h.Store.Get(ctx, row.StorageKey)
	if err != nil || !bytes.Equal(got, tinyPNG) {
		t.Fatalf("đọc lại ảnh từ kho: %v", err)
	}

	// video MP4 giả (đủ hộp ftyp) → kho ngoài
	mp4 := append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0, 'm', 'p', '4', '2'}, bytes.Repeat([]byte{7}, 3<<20)...)
	w, out = e.upload(e.base()+"/media/video", "file", "san-pham.mp4", mp4, nil)
	if w.Code != 201 {
		t.Fatalf("tải video: %d %s", w.Code, w.Body.String())
	}
	vid := out["media"].(map[string]any)
	if vid["kind"] != "video" || !strings.HasSuffix(vid["url"].(string), ".mp4") {
		t.Errorf("video: %v", vid)
	}
	var vrow SiteMedia
	e.db.First(&vrow, vid["id"])
	if back, err := e.h.Store.Get(ctx, vrow.StorageKey); err != nil || !bytes.Equal(back, mp4) {
		t.Fatalf("video đọc lại không khớp: %v", err)
	}
	// danh sách: mặc định chỉ ảnh, kind=video chỉ video
	if _, l := e.do("GET", e.base()+"/media", nil); len(l["media"].([]any)) != 1 {
		t.Errorf("danh sách ảnh: %v", l)
	}
	if _, l := e.do("GET", e.base()+"/media?kind=video", nil); len(l["media"].([]any)) != 1 || l["media"].([]any)[0].(map[string]any)["kind"] != "video" {
		t.Errorf("danh sách video: %v", l)
	}
	// định dạng không hỗ trợ
	if w, _ := e.upload(e.base()+"/media/video", "file", "a.mp4", []byte("không phải video, chỉ là chữ thôi"), nil); w.Code != 400 {
		t.Errorf("file không phải video phải 400, được %d", w.Code)
	}
	// xoá → mất cả trong kho
	if w, _ := e.do("DELETE", fmt.Sprintf("%s/media/%.0f", e.base(), vid["id"].(float64)), nil); w.Code != 200 {
		t.Fatalf("xoá video: %d", w.Code)
	}
	if _, err := e.h.Store.Get(ctx, vrow.StorageKey); err == nil {
		t.Error("xoá xong mà file vẫn còn trong kho")
	}
	_ = e.h.Store.Delete(ctx, row.StorageKey)

	// sản phẩm dùng video/ảnh ở kho ngoài vẫn qua bộ lọc URL
	_, p := e.do("POST", e.base()+"/products", map[string]any{"name": "Có video", "media": []map[string]any{{"type": "image", "url": img["url"]}, {"type": "video", "url": vid["url"], "poster": img["url"]}}})
	if len(p["product"].(map[string]any)["media"].([]any)) != 2 {
		t.Errorf("sản phẩm mất media ở kho ngoài: %v", p)
	}
}

func TestSniffVideo(t *testing.T) {
	for name, head := range map[string][]byte{
		"mp4":  {0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'},
		"mov":  {0, 0, 0, 0x14, 'f', 't', 'y', 'p', 'q', 't', ' ', ' '},
		"webm": {0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81},
	} {
		if _, _, ok := sniffVideo(head); !ok {
			t.Errorf("%s phải được nhận", name)
		}
	}
	for name, head := range map[string][]byte{"png": tinyPNG[:12], "html": []byte("<html><body>"), "ngắn": {1, 2}, "rỗng": nil} {
		if _, _, ok := sniffVideo(head); ok {
			t.Errorf("%s không được nhận là video", name)
		}
	}
}

func TestParseVideoLink(t *testing.T) {
	cases := []struct {
		in, typ, url string
		ok           bool
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=10", "embed", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", true},
		{"https://youtu.be/dQw4w9WgXcQ?si=abc", "embed", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", true},
		{"https://www.youtube.com/shorts/dQw4w9WgXcQ", "embed", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", true},
		{"https://www.youtube.com/embed/dQw4w9WgXcQ", "embed", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", true},
		{"https://vimeo.com/123456789", "embed", "https://player.vimeo.com/video/123456789", true},
		{"https://player.vimeo.com/video/123456789?h=abc", "embed", "https://player.vimeo.com/video/123456789", true},
		{"https://cdn.example.com/a/b/clip.MP4?x=1", "video", "https://cdn.example.com/a/b/clip.MP4?x=1", true},
		{"https://cdn.example.com/clip.webm", "video", "https://cdn.example.com/clip.webm", true},
		{"https://evil.com/watch?v=dQw4w9WgXcQ", "", "", false},
		{"https://www.youtube.com/watch?v=<script>", "", "", false},
		{"https://example.com/page.html", "", "", false},
		{"javascript:alert(1)", "", "", false},
		{"/uploads/a.mp4", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		typ, u, _, ok := parseVideoLink(c.in)
		if ok != c.ok || typ != c.typ || u != c.url {
			t.Errorf("parseVideoLink(%q) = (%q,%q,%v), muốn (%q,%q,%v)", c.in, typ, u, ok, c.typ, c.url, c.ok)
		}
	}
}

func TestDetectImageType(t *testing.T) {
	avif := append([]byte{0, 0, 0, 0x1C, 'f', 't', 'y', 'p', 'a', 'v', 'i', 'f'}, make([]byte, 20)...)
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"png", tinyPNG, "image/png"},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F', 0, 1}, "image/jpeg"},
		{"gif", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), "image/gif"},
		{"webp", []byte("RIFF\x24\x00\x00\x00WEBPVP8 "), "image/webp"},
		{"avif", avif, "image/avif"},
		{"svg bị từ chối", []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`), ""},
		{"html bị từ chối", []byte("<html><script>alert(1)</script></html>"), ""},
		{"mp4 không phải ảnh", []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0}, ""},
		{"rỗng", nil, ""},
	}
	for _, c := range cases {
		got, ok := detectImageType(c.data)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%s: detectImageType = (%q,%v), muốn %q", c.name, got, ok, c.want)
		}
	}
}

// Dòng sản phẩm tạo trước khi có các cột mới (media_json, sku, cost_price...) có giá trị NULL: đọc phải vẫn bình thường.
func TestProductRowsWithNullNewColumns(t *testing.T) {
	e := newProductEnv(t)
	res := e.db.Exec(`INSERT INTO products (storefront_id, user_id, name, description, price, compare_price, image_url, stock, is_active, sort_order, created_at, updated_at)
		VALUES (?, 7, 'SP trước khi nâng cấp', 'Mô tả cũ', 50000, 0, 'https://cdn.example.com/old.jpg', -1, true, 0, now(), now())`, e.sf.ID)
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	var id uint
	e.db.Raw(`SELECT id FROM products WHERE name = 'SP trước khi nâng cấp'`).Scan(&id)
	var nulls int64
	e.db.Raw(`SELECT count(*) FROM products WHERE id = ? AND media_json IS NULL AND sku IS NULL AND cost_price IS NULL AND tags IS NULL`, id).Scan(&nulls)
	if nulls != 1 {
		t.Fatalf("giả lập sai: cột mới không NULL (%d)", nulls)
	}
	for _, path := range []string{e.base() + "/products?lite=1", e.base() + "/products", fmt.Sprintf("%s/products/%d", e.base(), id), fmt.Sprintf("/api/v1/storefront/public/products/%d?slug=test-shop", id), "/api/v1/storefront/public?slug=test-shop"} {
		if w, _ := e.do("GET", path, nil); w.Code != 200 {
			t.Errorf("GET %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	// sửa bình thường rồi lưu lại
	w, out := e.do("PUT", fmt.Sprintf("%s/products/%d", e.base(), id), map[string]any{"name": "SP đã nâng cấp", "price": 60000, "stock": -1, "media": []map[string]any{{"type": "image", "url": "https://cdn.example.com/new.jpg"}}})
	if w.Code != 200 || out["product"].(map[string]any)["image_url"] != "https://cdn.example.com/new.jpg" {
		t.Errorf("lưu lại sản phẩm cũ: %d %s", w.Code, w.Body.String())
	}
}
