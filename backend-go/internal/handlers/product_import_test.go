package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"fb-multipage-crm/internal/config"

	"github.com/gin-gonic/gin"
)

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in, cur string
		want    float64
	}{
		{"1,234.56", "USD", 1234.56}, {"1.234,56", "EUR", 1234.56}, {"259.000", "VND", 259000}, {"259,000", "VND", 259000},
		{"12,5", "EUR", 12.5}, {"3,000", "USD", 3000}, {"12.99", "USD", 12.99}, {"1.234.567", "VND", 1234567}, {"35.80", "CNY", 35.8},
		{"0", "USD", 0}, {"", "USD", 0}, {"...", "USD", 0},
	}
	for _, c := range cases {
		if got := parseAmount(c.in, c.cur); got != c.want {
			t.Errorf("parseAmount(%q,%s) = %v, muốn %v", c.in, c.cur, got, c.want)
		}
	}
}

func TestFindPriceInText(t *testing.T) {
	cases := []struct {
		text string
		want float64
		cur  string
	}{
		{"Giá: 259.000₫ (giảm từ 399.000₫)", 259000, "VND"},
		{"US $12.99  US $20.00 free shipping", 12.99, "USD"},
		{"价格 ￥ 35.80 起", 35.8, "CNY"},
		{"起批价 12.50元-15.00元", 12.5, "CNY"},
		{"Only €19,90 today", 19.9, "EUR"},
		{"Chưa có giá ở đây", 0, ""},
		{"Mua 3 tặng 1, bảo hành 12 tháng", 0, ""},
	}
	for _, c := range cases {
		got, cur := findPriceInText(c.text)
		if got != c.want || cur != c.cur {
			t.Errorf("findPriceInText(%q) = (%v,%q), muốn (%v,%q)", c.text, got, cur, c.want, c.cur)
		}
	}
}

func TestCanonicalImage(t *testing.T) {
	cases := map[string]string{
		"https://ae-pic-a1.aliexpress-media.com/kf/Sabc.jpg_80x80.jpg":            "https://ae-pic-a1.aliexpress-media.com/kf/Sabc.jpg",
		"https://ae-pic-a1.aliexpress-media.com/kf/Sabc.jpg_960x960q75.jpg_.avif": "https://ae-pic-a1.aliexpress-media.com/kf/Sabc.jpg",
		"https://cbu01.alicdn.com/img/ibank/O1CN01x.jpg_.webp":                    "https://cbu01.alicdn.com/img/ibank/O1CN01x.jpg_.webp", // không có kích thước: giữ nguyên
		"https://shop.example.com/img/a_80x80.jpg":                                "https://shop.example.com/img/a_80x80.jpg",             // không phải CDN Alibaba: không đụng
	}
	for in, want := range cases {
		if got := canonicalImage(in); got != want {
			t.Errorf("canonicalImage(%q) = %q, muốn %q", in, got, want)
		}
	}
}

// Trang AliExpress thật (cắt gọn): tiêu đề + ảnh có sẵn, giá / mô tả thì không (trang nạp bằng JavaScript)
func TestParseRealAliExpressPage(t *testing.T) {
	raw, err := os.ReadFile("testdata/aliexpress_item.html")
	if err != nil {
		t.Fatal(err)
	}
	info := parsePage(string(raw), mustURL("https://vi.aliexpress.com/item/1005008867854647.html"))
	if !strings.HasPrefix(info.Title, "2.4G Wireless Lavalier Microphone") {
		t.Errorf("tiêu đề: %q", info.Title)
	}
	if len(info.Images) != 6 {
		t.Fatalf("muốn 6 ảnh sản phẩm (imagePathList, không lặp, không ảnh thu nhỏ), được %d: %v", len(info.Images), info.Images)
	}
	for _, im := range info.Images {
		if !strings.HasPrefix(im, "https://ae-pic-a1.aliexpress-media.com/kf/") || strings.Contains(im, "_80x80") {
			t.Errorf("ảnh lạ: %s", im)
		}
	}
	if info.Images[0] != "https://ae-pic-a1.aliexpress-media.com/kf/Sba4b82637c374244a9ca143370c7cc8fz.jpg" {
		t.Errorf("ảnh đầu phải là ảnh chính: %s", info.Images[0])
	}
	if info.Price != 0 {
		t.Errorf("trang này không có giá, được %v", info.Price)
	}
	if detectSource(mustURL("https://vi.aliexpress.com/item/1.html")) != "aliexpress" || detectSource(mustURL("https://detail.1688.com/offer/1.html")) != "1688" ||
		detectSource(mustURL("https://item.taobao.com/item.htm?id=1")) != "taobao" || detectSource(mustURL("https://shop.com/products/x")) != "shopify" || detectSource(mustURL("https://shop.com/a")) != "generic" {
		t.Error("detectSource sai")
	}
}

const jsonLDPage = `<!doctype html><html><head><title>Tai nghe Bluetooth | Shop</title>
<meta property="og:title" content="Tai nghe OG"><meta property="og:image" content="/img/og.jpg">
<script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"Organization","name":"Shop"},{"@type":["Product"],"name":"Tai nghe Bluetooth X1","description":"Âm thanh trong, pin 30 giờ","sku":"X1-BLK",
"image":["https://cdn.shop.vn/p/1.jpg",{"@type":"ImageObject","url":"https://cdn.shop.vn/p/2.jpg"}],"offers":{"@type":"Offer","price":"259000","priceCurrency":"vnd"}}]}</script></head>
<body><h1>Tai nghe X1 (H1)</h1><img src="/img/logo.png"><img src="/img/gallery-3.jpg"><img src="https://cdn.shop.vn/p/1.jpg"><img src="/img/icon-cart.svg">
<img src="https://cdn.shop.vn/rec/other_120x120.jpg"><img data-src="//cdn.shop.vn/lazy-4.jpg"><img srcset="/img/s-300.jpg 300w, /img/s-900.jpg 900w">
<video><source src="/v/demo.mp4"></video><script>var x="https://cdn.shop.vn/v/extra.mp4";</script></body></html>`

func TestParseJSONLDPage(t *testing.T) {
	info := parsePage(jsonLDPage, mustURL("https://shop.vn/products/x1"))
	if info.Title != "Tai nghe Bluetooth X1" || info.Desc != "Âm thanh trong, pin 30 giờ" || info.SKU != "X1-BLK" {
		t.Errorf("JSON-LD: %+v", info)
	}
	if info.Price != 259000 || info.Currency != "VND" {
		t.Errorf("giá từ offers: %v %s", info.Price, info.Currency)
	}
	want := []string{"https://cdn.shop.vn/p/1.jpg", "https://cdn.shop.vn/p/2.jpg", "https://shop.vn/img/og.jpg", "https://shop.vn/img/gallery-3.jpg", "https://cdn.shop.vn/lazy-4.jpg", "https://shop.vn/img/s-900.jpg"}
	if strings.Join(info.Images, "\n") != strings.Join(want, "\n") {
		t.Errorf("ảnh:\n got: %v\nwant: %v", info.Images, want)
	}
	if strings.Join(info.Videos, ",") != "https://shop.vn/v/demo.mp4,https://cdn.shop.vn/v/extra.mp4" {
		t.Errorf("video: %v", info.Videos)
	}
	if strings.Contains(info.Text, "var x=") || !strings.Contains(info.Text, "Tai nghe X1 (H1)") {
		t.Errorf("văn bản trang: %q", info.Text)
	}
}

// Nội dung dán từ trang 1688 (tiếng Trung): ảnh 80x80 của dải ảnh nhỏ → lấy bản gốc; ảnh 220x220 của sản phẩm gợi ý → bỏ
const pastedChinese = `<meta charset="utf-8"><!--StartFragment--><div><h1>2024新款无线领夹麦克风 降噪直播收音麦</h1><span>¥12.50-15.00</span><p>起批量 2个起批</p>
<img src="https://cbu01.alicdn.com/img/ibank/O1CN01main1.jpg"><img src="https://cbu01.alicdn.com/img/ibank/O1CN01main2.jpg_.webp"><img src="https://cbu01.alicdn.com/img/ibank/O1CN01thumb.jpg_80x80.jpg"><img src="https://cbu01.alicdn.com/img/ibank/O1CN01rec.jpg_220x220.jpg">
<img src="/relative/only.jpg"><img src="data:image/gif;base64,R0lGODlhAQABAAAAACw="><ul><li>颜色：黑色</li><li>颜色：白色</li></ul></div><!--EndFragment-->`

func TestParsePastedChinesePage(t *testing.T) {
	info := parsePage(pastedChinese, nil)
	if info.Title != "2024新款无线领夹麦克风 降噪直播收音麦" {
		t.Errorf("tiêu đề: %q", info.Title)
	}
	if strings.Join(info.Images, ",") != "https://cbu01.alicdn.com/img/ibank/O1CN01main1.jpg,https://cbu01.alicdn.com/img/ibank/O1CN01main2.jpg_.webp,https://cbu01.alicdn.com/img/ibank/O1CN01thumb.jpg" {
		t.Errorf("ảnh (80x80 → bản gốc, 220x220 bỏ, ảnh tương đối khi không có link gốc bỏ, data: bỏ): %v", info.Images)
	}
	if info.Price != 12.5 || info.Currency != "CNY" {
		t.Errorf("giá nhân dân tệ: %v %s", info.Price, info.Currency)
	}
	if !strings.Contains(info.Text, "起批量") || !strings.Contains(info.Text, "颜色：白色") {
		t.Errorf("văn bản: %q", info.Text)
	}
}

func TestDetectBlocked(t *testing.T) {
	punish := `<script>sessionStorage.x5referer = window.location.href;var url = "https://detail.1688.com//offer/1.html/_____tmd_____/punish?x5secdata=abc&x5step=1";</script>`
	if detectBlocked(punish, 200) == "" {
		t.Error("trang punish của 1688 phải nhận ra là bị chặn")
	}
	if detectBlocked("<html>bình thường</html>", 200) != "" {
		t.Error("trang bình thường không bị chặn")
	}
	if detectBlocked("", 403) == "" || detectBlocked("", 429) == "" {
		t.Error("HTTP 403/429 phải coi là bị chặn")
	}
}

func TestApplyShopifyJS(t *testing.T) {
	js := `{"title":"Áo thun","description":"<p>Mềm <script>x</script>mại</p>","price":19900,"compare_at_price":29900,"images":["//cdn.shopify.com/s/a.jpg","//cdn.shopify.com/s/b.jpg"],
"options":[{"name":"Size","values":["S","M"]},{"name":"Title","values":["Default Title"]}],"variants":[{"sku":"TS-1"}]}`
	d := &productDraft{}
	if !applyShopifyJS([]byte(js), mustURL("https://shop.com/products/ao"), d) {
		t.Fatal("không đọc được JSON Shopify")
	}
	if d.Name != "Áo thun" || d.Price != 199 || d.ComparePrice != 299 || d.SKU != "TS-1" || len(d.Images) != 2 || len(d.Options) != 1 || d.Options[0].Name != "Size" {
		t.Errorf("draft Shopify sai: %+v", d)
	}
	if strings.Contains(d.Description, "script") {
		t.Errorf("mô tả chưa được lọc: %q", d.Description)
	}
	if shopifyJSURL(mustURL("https://shop.com/collections/all/products/ao?variant=1")) != "https://shop.com/collections/all/products/ao.js" {
		t.Errorf("shopifyJSURL: %s", shopifyJSURL(mustURL("https://shop.com/collections/all/products/ao?variant=1")))
	}
}

// ----------------------------------------------------------------- handler (AI giả + trang nguồn giả)

type importEnv struct {
	*productEnv
	aiCalls  int
	lastUser string
	aiFail   bool
}

func newImportEnv(t *testing.T) *importEnv {
	pe := newProductEnv(t)
	ie := &importEnv{productEnv: pe}

	// Máy chủ AI giả (OpenAI-compatible)
	aiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ie.aiCalls++
		if ie.aiFail {
			w.WriteHeader(429)
			return
		}
		var req struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		_ = json.Unmarshal(body, &req)
		ie.lastUser = req.Messages[len(req.Messages)-1].Content
		out := `{"name":"Micro cài áo không dây chống ồn","description_html":"<p>Micro nhỏ gọn, thu âm rõ.</p><ul><li>Chống ồn</li><li>Pin 8 giờ</li></ul><script>x</script>","price":12.5,"compare_price":20,"currency":"CNY","options":[{"name":"Màu","values":["Đen","Trắng","Đen"]}]}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": out}}}})
	}))
	t.Cleanup(aiSrv.Close)
	t.Setenv("AI_BASE_URL", aiSrv.URL)
	t.Setenv("AI_API_KEY", "test")
	t.Setenv("AI_MODEL", "fake")
	pe.h.AI = NewAIHandler(pe.db, &config.Config{})

	// Tỉ giá cố định (khỏi gọi mạng): 1 VND = 1/25000 USD = 1/3500 CNY
	rateCache.Lock()
	rateCache.rates = map[string]float64{"USD": 1.0 / 25000, "CNY": 1.0 / 3500}
	rateCache.updated, rateCache.fetched = time.Now(), time.Now()
	rateCache.Unlock()

	// Cho phép máy chủ thử ở 127.0.0.1 với cổng ngẫu nhiên
	oldClient, oldPorts := importFetchClient, allowCustomFetchPorts
	importFetchClient = func() *http.Client { return &http.Client{Timeout: 5 * time.Second} }
	allowCustomFetchPorts = true
	t.Cleanup(func() { importFetchClient, allowCustomFetchPorts = oldClient, oldPorts })
	importLimiter.Lock()
	importLimiter.hits = map[uint][]time.Time{}
	importLimiter.Unlock()

	pe.r.POST("/api/v1/storefront/sites/:id/products/import", func(c *gin.Context) { c.Set("user_id", uint(7)); pe.h.ImportProduct(c) })
	return ie
}

func (e *importEnv) importReq(body map[string]any) (int, map[string]any) {
	w, out := e.do("POST", e.base()+"/products/import", body)
	return w.Code, out
}

func TestImportProductFromURLAndPasteWithAI(t *testing.T) {
	e := newImportEnv(t)
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/item/1.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, jsonLDPage)
		case "/offer/2.html": // giả lập 1688 chặn bằng captcha trượt
			fmt.Fprint(w, `<script>var url="https://x/_____tmd_____/punish?x5secdata=abc";</script>`)
		case "/forbidden":
			w.WriteHeader(403)
		default:
			w.WriteHeader(404)
		}
	}))
	defer site.Close()

	// 1) Link trang có JSON-LD + AI (dịch / tách)
	code, out := e.importReq(map[string]any{"url": site.URL + "/item/1.html"})
	if code != 200 {
		t.Fatalf("import link: %d %v", code, out)
	}
	d := out["draft"].(map[string]any)
	if d["translated"] != true || d["name"] != "Micro cài áo không dây chống ồn" {
		t.Errorf("AI chưa áp dụng: %v", d)
	}
	desc := d["description"].(string)
	if !strings.Contains(desc, "<li>Chống ồn</li>") || strings.Contains(desc, "script") {
		t.Errorf("mô tả AI chưa lọc: %q", desc)
	}
	if d["price"].(float64) != 12.5 || d["currency"] != "CNY" || d["compare_price"].(float64) != 20 {
		t.Errorf("giá: %v %v", d["price"], d["currency"])
	}
	fx := d["fx"].(map[string]any)
	if fx["currency"] != "CNY" || fx["vnd_per_unit"].(float64) < 3499 || fx["vnd_per_unit"].(float64) > 3501 {
		t.Errorf("tỉ giá: %v", fx)
	}
	opts := d["options"].([]any)
	if len(opts) != 1 || len(opts[0].(map[string]any)["values"].([]any)) != 2 {
		t.Errorf("phân loại phải khử trùng lặp: %v", opts)
	}
	if len(d["images"].([]any)) != 6 || len(d["videos"].([]any)) != 2 {
		t.Errorf("ảnh/video từ trang: %v / %v", d["images"], d["videos"])
	}
	if !strings.Contains(e.lastUser, "Tai nghe X1 (H1)") || !strings.Contains(e.lastUser, "HINTS") {
		t.Errorf("AI không nhận được chữ của trang: %.200q", e.lastUser)
	}
	if d["source_url"] != site.URL+"/item/1.html" {
		t.Errorf("source_url: %v", d["source_url"])
	}

	// 2) Dán nội dung trang tiếng Trung (không cần link) → AI dịch
	code, out = e.importReq(map[string]any{"html": pastedChinese, "url": "https://detail.1688.com/offer/724158207862.html", "lang": "vi"})
	d = out["draft"].(map[string]any)
	if code != 200 || d["source"] != "1688" || d["translated"] != true || len(d["images"].([]any)) != 4 {
		t.Errorf("dán nội dung 1688: %d %v", code, d)
	}
	if !strings.Contains(e.lastUser, "颜色：白色") || !strings.Contains(e.lastUser, "Vietnamese") && !strings.Contains(e.lastUser, "HINTS") {
		t.Errorf("AI phải nhận chữ Trung của trang dán: %.160q", e.lastUser)
	}

	// 3) Link bị chặn bằng captcha → báo rõ, KHÔNG gọi AI
	calls := e.aiCalls
	_, out = e.importReq(map[string]any{"url": site.URL + "/offer/2.html"})
	d = out["draft"].(map[string]any)
	if d["blocked"] != true || len(d["warnings"].([]any)) == 0 || !strings.Contains(d["warnings"].([]any)[0].(string), "Dán nội dung trang") || e.aiCalls != calls {
		t.Errorf("trang chặn: %v (AI calls %d→%d)", d, calls, e.aiCalls)
	}
	// HTTP 403 → cũng báo hướng dẫn dán nội dung
	_, out = e.importReq(map[string]any{"url": site.URL + "/forbidden"})
	if out["draft"].(map[string]any)["blocked"] != true {
		t.Errorf("403 phải báo blocked: %v", out)
	}

	// 4) AI lỗi → vẫn trả dữ liệu đọc trực tiếp từ trang + cảnh báo
	e.aiFail = true
	_, out = e.importReq(map[string]any{"url": site.URL + "/item/1.html"})
	d = out["draft"].(map[string]any)
	if d["translated"] == true || d["name"] != "Tai nghe Bluetooth X1" || d["price"].(float64) != 259000 || d["currency"] != "VND" || len(d["warnings"].([]any)) == 0 {
		t.Errorf("AI lỗi phải rơi về dữ liệu của trang: %v", d)
	}
	e.aiFail = false

	// 5) Tắt dịch → không gọi AI
	calls = e.aiCalls
	_, out = e.importReq(map[string]any{"url": site.URL + "/item/1.html", "translate": false})
	if e.aiCalls != calls || out["draft"].(map[string]any)["name"] != "Tai nghe Bluetooth X1" {
		t.Errorf("translate=false không được gọi AI")
	}

	// 6) Đầu vào sai
	for name, body := range map[string]map[string]any{
		"rỗng":              {},
		"link không hợp lệ": {"url": "ftp://x/a"},
		"link nội bộ":       {"url": "http://localhost/admin"},
	} {
		if code, _ := e.importReq(body); code != 400 {
			t.Errorf("%s: phải 400, được %d", name, code)
		}
	}
}

func TestImportProductRateLimit(t *testing.T) {
	e := newImportEnv(t)
	got429 := false
	for i := 0; i < 20; i++ {
		if code, _ := e.importReq(map[string]any{"text": "Sản phẩm thử số " + fmt.Sprint(i)}); code == 429 {
			got429 = true
			if i < 15 {
				t.Fatalf("bị giới hạn quá sớm ở lần %d", i+1)
			}
			break
		}
	}
	if !got429 {
		t.Error("gọi liên tục phải bị giới hạn tần suất")
	}
}
