package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
)

const (
	uaMobile  = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	uaDesktop = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
)

func newAnalyticsEnv(t *testing.T) *productEnv {
	e := newProductEnv(t)
	e.db.Exec("TRUNCATE store_events, store_sessions RESTART IDENTITY")
	trackLimiter.Lock()
	trackLimiter.hits = map[string][]time.Time{}
	trackLimiter.sites = map[uint]*siteBudget{}
	trackLimiter.Unlock()
	trackSiteCache.Lock()
	trackSiteCache.m = map[string]trackSiteInfo{}
	trackSiteCache.Unlock()
	e.r.POST("/api/v1/storefront/public/track", e.h.TrackEvents)
	e.r.GET("/api/v1/storefront/sites/:id/analytics", func(c *gin.Context) { c.Set("user_id", uint(7)); e.h.AnalyticsOverview(c) })
	e.r.GET("/api/v1/storefront/sites/:id/analytics/live", func(c *gin.Context) { c.Set("user_id", uint(7)); e.h.AnalyticsLive(c) })
	e.r.GET("/api/v1/storefront/sites/:id/analytics/abandoned", func(c *gin.Context) { c.Set("user_id", uint(7)); e.h.AnalyticsAbandoned(c) })
	return e
}

// track gửi một lô sự kiện như trình duyệt khách (text/plain, giống navigator.sendBeacon)
func (e *productEnv) track(body map[string]any, ua string, hdr map[string]string) int {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storefront/public/track", bytes.NewReader(b))
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("User-Agent", ua)
	req.RemoteAddr = "203.0.113.9:5555"
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w.Code
}

func env(sid string, events ...map[string]any) map[string]any {
	return map[string]any{"slug": "test-shop", "sid": sid, "vid": "visitor-0001", "events": events}
}

func ev(t string, kv ...any) map[string]any {
	m := map[string]any{"t": t, "p": "/"}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

func (e *productEnv) session(sid string) StoreSession {
	var s StoreSession
	e.db.Where("storefront_id = ? AND sid = ?", e.sf.ID, sid).First(&s)
	return s
}

func TestTrackBuildsSessionFunnelAndNeverStoresSecrets(t *testing.T) {
	e := newAnalyticsEnv(t)
	body := env("sess-aaaa-0001",
		ev("page_view", "p", "/s/test-shop/"),
		ev("product_view", "p", "/s/test-shop/p/12?utm=1", "pid", 12),
		ev("add_to_cart", "p", "/s/test-shop/p/12", "pid", 12, "v", 190000, "l", "Áo thun"),
		ev("click", "p", "/", "l", "Thêm vào giỏ", "sec", "products", "tg", "/p/12"),
		ev("heartbeat"),
		ev("page_view", "p", "/s/test-shop/order/DH24112800001?t=SUPERSECRETTOKEN"),
		ev("bogus_type"),
	)
	body["ctx"] = map[string]any{"ref": "https://www.google.com/search?q=x", "landing": "/s/test-shop/", "lang": "en", "market": "us", "country": "us", "ret": true}
	body["cart"] = map[string]any{"items": []map[string]any{{"p": 12, "v": 0, "q": 2, "pr": 190000, "nm": "Áo thun"}, {"p": 0, "q": 1, "pr": 5}}}
	if code := e.track(body, uaMobile, nil); code != 204 {
		t.Fatalf("status %d", code)
	}
	s := e.session("sess-aaaa-0001")
	if s.ID == 0 {
		t.Fatal("chưa tạo phiên")
	}
	if s.Stage != stageAddToCart || s.Pages != 2 || s.Clicks != 1 || s.Events != 5 {
		t.Errorf("phiên: stage=%d pages=%d clicks=%d events=%d", s.Stage, s.Pages, s.Clicks, s.Events)
	}
	if s.Source != "google.com" || s.Medium != "search" || s.Device != "mobile" || s.OS != "iOS" || s.Browser != "Safari" || s.Country != "US" || !s.Returning || s.Landing != "/" {
		t.Errorf("thuộc tính phiên sai: %+v", s)
	}
	if s.CartValue != 380000 || s.CartItems != 2 || !strings.Contains(s.CartJSON, "Áo thun") {
		t.Errorf("giỏ hàng: %d %d %s", s.CartValue, s.CartItems, s.CartJSON)
	}
	if s.LastEvent != "page_view" || s.LastPath != "/order/*" {
		t.Errorf("bước hiện tại: %s %s", s.LastEvent, s.LastPath)
	}
	var evs []StoreEvent
	e.db.Where("storefront_id = ?", e.sf.ID).Order("id").Find(&evs)
	if len(evs) != 5 {
		t.Fatalf("lưu %d sự kiện (heartbeat và loại lạ không được lưu)", len(evs))
	}
	for _, x := range evs {
		if strings.Contains(x.Path+x.Label+x.Target, "SECRET") || strings.Contains(x.Path, "DH2411") || strings.Contains(x.Path, "?") {
			t.Errorf("rò rỉ mã đơn / token / tham số: %+v", x)
		}
	}
	if evs[1].Path != "/p/12" || evs[1].ProductID != 12 || evs[3].Section != "products" || evs[3].Target != "/p/12" || evs[4].Path != "/order/*" {
		t.Errorf("sự kiện sai: %+v", evs)
	}
}

func TestTrackStageNeverGoesBackAndClicksDontChangeStep(t *testing.T) {
	e := newAnalyticsEnv(t)
	sid := "sess-bbbb-0002"
	e.track(env(sid, ev("page_view"), ev("checkout_start", "p", "/checkout"), ev("checkout_info", "p", "/checkout"), ev("checkout_payment", "p", "/checkout", "l", "cod")), uaDesktop, nil)
	s := e.session(sid)
	if s.Stage != stagePayment || s.LastEvent != "checkout_payment" || s.Device != "desktop" {
		t.Fatalf("%+v", s)
	}
	// Quay về trang chủ, click, heartbeat: bước xa nhất giữ nguyên, bước hiện tại đổi theo trang
	e.track(env(sid, ev("click", "l", "Logo"), ev("heartbeat")), uaDesktop, nil)
	s = e.session(sid)
	if s.Stage != stagePayment || s.LastEvent != "checkout_payment" || s.Clicks != 1 {
		t.Errorf("click/heartbeat không được đổi bước: %+v", s)
	}
	e.track(env(sid, ev("page_view", "p", "/")), uaDesktop, nil)
	s = e.session(sid)
	if s.Stage != stagePayment || s.LastEvent != "page_view" || s.LastPath != "/" || s.Pages != 2 {
		t.Errorf("%+v", s)
	}
	if liveStep(s.LastEvent, s.LastPath, 0) != "browsing" || liveStep("checkout_info_done", "/checkout", 0) != "info_done" || liveStep("page_view", "/checkout", 0) != "checkout" || liveStep("order_placed", "/", 1) != "ordered" {
		t.Error("liveStep sai")
	}
}

func TestTrackRejectsBadTrafficQuietly(t *testing.T) {
	e := newAnalyticsEnv(t)
	good := func() map[string]any { return env("sess-cccc-0003", ev("page_view")) }
	if c := e.track(good(), "Googlebot/2.1 (+http://www.google.com/bot.html)", nil); c != 204 {
		t.Errorf("bot: %d", c)
	}
	if c := e.track(good(), "", nil); c != 204 {
		t.Errorf("không UA: %d", c)
	}
	if e.session("sess-cccc-0003").ID != 0 {
		t.Error("bot không được tạo phiên")
	}
	b := good()
	b["slug"] = "khong-ton-tai"
	if c := e.track(b, uaDesktop, nil); c != 404 {
		t.Errorf("slug lạ: %d", c)
	}
	b = good()
	b["sid"] = "x y"
	if c := e.track(b, uaDesktop, nil); c != 400 {
		t.Errorf("sid sai: %d", c)
	}
	// website chưa xuất bản không nhận dữ liệu
	e.db.Model(&models.Storefront{}).Where("id = ?", e.sf.ID).Update("is_published", false)
	trackSiteCache.Lock()
	trackSiteCache.m = map[string]trackSiteInfo{}
	trackSiteCache.Unlock()
	if c := e.track(good(), uaDesktop, nil); c != 404 {
		t.Errorf("chưa xuất bản: %d", c)
	}
	e.db.Model(&models.Storefront{}).Where("id = ?", e.sf.ID).Update("is_published", true)
	trackSiteCache.Lock()
	trackSiteCache.m = map[string]trackSiteInfo{}
	trackSiteCache.Unlock()

	// quá lớn
	big := good()
	big["events"] = []map[string]any{ev("click", "l", strings.Repeat("x", 40<<10))}
	if c := e.track(big, uaDesktop, nil); c != 413 {
		t.Errorf("quá lớn: %d", c)
	}
	// nhiều sự kiện: chỉ nhận 25 đầu
	many := []map[string]any{}
	for i := 0; i < 40; i++ {
		many = append(many, ev("click", "l", fmt.Sprintf("n%d", i)))
	}
	b = good()
	b["events"] = many
	e.track(b, uaDesktop, nil)
	var n int64
	e.db.Model(&StoreEvent{}).Where("storefront_id = ?", e.sf.ID).Count(&n)
	if n != 25 {
		t.Errorf("nhận %d sự kiện, muốn 25", n)
	}
	// giới hạn theo IP
	codes := map[int]int{}
	for i := 0; i < 100; i++ {
		codes[e.track(env("sess-dddd-0004", ev("heartbeat")), uaDesktop, nil)]++
	}
	if codes[429] == 0 || codes[204] > 90 {
		t.Errorf("không giới hạn theo IP: %v", codes)
	}
}

func TestTrackTrafficClassification(t *testing.T) {
	cases := []struct {
		ref, us, um, uc string
		fb              bool
		src, med        string
	}{
		{"", "", "", "", false, "direct", "direct"},
		{"https://www.google.com/", "", "", "", false, "google.com", "search"},
		{"https://l.facebook.com/l.php?u=x", "", "", "", false, "l.facebook.com", "social"},
		{"https://blog.example.org/post", "", "", "", false, "blog.example.org", "referral"},
		{"", "Newsletter", "email", "tet", false, "newsletter", "email"},
		{"", "fb", "cpc", "", false, "fb", "paid"},
		{"", "tiktok", "", "", false, "tiktok", "social"},
		{"", "", "", "", true, "facebook", "social"},
		{"https://shop.example.com/p/1", "", "", "", false, "direct", "direct"}, // tên miền của chính shop
	}
	for _, c := range cases {
		s, m, _ := classifyTraffic(c.ref, c.us, c.um, c.uc, c.fb, "shop.example.com")
		if s != c.src || m != c.med {
			t.Errorf("%+v → %s/%s", c, s, m)
		}
	}
	if d, b, o := parseDevice(uaDesktop); d != "desktop" || b != "Chrome" || o != "Windows" {
		t.Errorf("desktop: %s %s %s", d, b, o)
	}
	if d, _, o := parseDevice("Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 Chrome/120 Mobile Safari/537.36"); d != "mobile" || o != "Android" {
		t.Errorf("android: %s %s", d, o)
	}
	if d, _, _ := parseDevice("Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605 Safari/604"); d != "tablet" {
		t.Errorf("ipad: %s", d)
	}
	for in, want := range map[string]string{
		"/s/shop/p/12?x=1": "/p/12", "/p/12abc": "/p", "/order/DH1?t=abc": "/order/*", "/s/shop/order/DH1": "/order/*",
		"/checkout#x": "/checkout", "": "/", "javascript:alert(1)": "/", "/s/shop": "/", "/account/x": "/account",
	} {
		if got := cleanTrackPath("shop", in); got != want {
			t.Errorf("cleanTrackPath(%q) = %q, muốn %q", in, got, want)
		}
	}
	if cleanTrackTarget("shop", "https://www.facebook.com/page?x=1") != "facebook.com" || cleanTrackTarget("shop", "tel:0912345678") != "tel:" || cleanTrackTarget("shop", "/s/shop/p/3?a=1") != "/p/3" {
		t.Error("cleanTrackTarget sai")
	}
}

// Dữ liệu cố định để kiểm tra báo cáo
func seedAnalytics(e *productEnv, now time.Time) {
	mk := func(sid, vid, source, medium, device, country string, ret bool, stage, orders, pages, clicks int, ago time.Duration, lastEvent string, cart int64) {
		started := now.Add(-ago)
		e.db.Create(&StoreSession{StorefrontID: e.sf.ID, SID: sid, VisitorID: vid, StartedAt: started, LastSeen: started.Add(2 * time.Minute),
			Source: source, Medium: medium, Device: device, Browser: "Chrome", OS: "Windows", Country: country, Landing: "/", Returning: ret,
			Pages: pages, Clicks: clicks, Stage: stage, Orders: orders, LastEvent: lastEvent, LastPath: "/checkout", CartValue: cart, CartItems: 1,
			CartJSON: `[{"p":5,"q":1,"pr":` + fmt.Sprint(cart) + `,"nm":"Áo"}]`})
	}
	// trong kỳ (3 ngày gần nhất)
	mk("s-0000-0001", "v1", "google.com", "search", "desktop", "VN", false, 8, 1, 4, 3, 2*time.Hour, "order_placed", 500000)
	mk("s-0000-0002", "v2", "facebook", "social", "mobile", "VN", false, 8, 1, 5, 4, 3*time.Hour, "order_placed", 300000)
	mk("s-0000-0003", "v3", "facebook", "social", "mobile", "US", true, 3, 0, 3, 2, 4*time.Hour, "checkout_start", 200000)   // bỏ thanh toán
	mk("s-0000-0004", "v4", "direct", "direct", "desktop", "VN", false, 6, 0, 3, 1, 5*time.Hour, "checkout_payment", 100000) // bỏ thanh toán
	mk("s-0000-0005", "v5", "direct", "direct", "desktop", "VN", true, 2, 0, 2, 1, 26*time.Hour, "add_to_cart", 50000)       // bỏ giỏ
	mk("s-0000-0006", "v6", "google.com", "search", "mobile", "VN", false, 1, 0, 2, 0, 27*time.Hour, "product_view", 0)
	mk("s-0000-0007", "v7", "google.com", "search", "mobile", "VN", false, 0, 0, 1, 0, 28*time.Hour, "page_view", 0) // thoát ngay
	// kỳ trước (4 ngày trước) — để so sánh
	mk("s-0000-0008", "v8", "direct", "direct", "desktop", "VN", false, 8, 1, 4, 3, 4*24*time.Hour, "order_placed", 100000)
	mk("s-0000-0009", "v9", "direct", "direct", "desktop", "VN", false, 0, 0, 1, 0, 4*24*time.Hour+time.Hour, "page_view", 0)

	order := func(code, sid string, total int64, status, pay, payStatus string, ago time.Duration) {
		items, _ := json.Marshal([]ShopOrderItem{{ProductID: 5, Name: "Áo", Price: total, Qty: 1, LineTotal: total}})
		e.db.Create(&ShopOrder{StorefrontID: e.sf.ID, UserID: 7, Code: code, AccessToken: "tok" + code, ItemsJSON: string(items), Subtotal: total, Total: total,
			PaymentMethod: pay, PaymentStatus: payStatus, Status: status, TrackSID: sid, CreatedAt: now.Add(-ago)})
	}
	order("DHA1", "s-0000-0001", 500000, "new", "vnpay", "paid", 2*time.Hour)
	order("DHA2", "s-0000-0002", 300000, "completed", "cod", "unpaid", 3*time.Hour)
	order("DHA3", "", 999000, "cancelled", "cod", "unpaid", 3*time.Hour) // đã huỷ: không tính doanh thu
	order("DHB1", "s-0000-0008", 100000, "new", "cod", "unpaid", 4*24*time.Hour)
}

func (e *productEnv) getJSON(path string) map[string]any {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out == nil {
		out = map[string]any{}
	}
	out["_status"] = float64(w.Code)
	return out
}

func num(m map[string]any, k string) float64 { v, _ := m[k].(float64); return v }

func TestAnalyticsOverviewNumbers(t *testing.T) {
	e := newAnalyticsEnv(t)
	now := time.Now()
	seedAnalytics(e, now)
	from := now.UTC().AddDate(0, 0, -2).Format("2006-01-02")
	to := now.UTC().Format("2006-01-02")
	out := e.getJSON(fmt.Sprintf("%s/analytics?from=%s&to=%s&tzo=0", e.base(), from, to))
	if num(out, "_status") != 200 {
		t.Fatalf("%v", out)
	}
	tot := out["totals"].(map[string]any)
	if num(tot, "sessions") != 7 || num(tot, "visitors") != 7 || num(tot, "orders") != 2 || num(tot, "revenue") != 800000 || num(tot, "aov") != 400000 {
		t.Errorf("tổng: %v", tot)
	}
	if num(tot, "cancelled_orders") != 1 || num(tot, "paid_orders") != 1 || num(tot, "page_views") != 20 {
		t.Errorf("tổng 2: %v", tot)
	}
	if d := num(tot, "conversion_rate") - 2.0/7.0; d > 0.001 || d < -0.001 {
		t.Errorf("tỷ lệ chuyển đổi %v", tot["conversion_rate"])
	}
	if num(tot, "new_visitors") != 5 || num(tot, "returning_visitors") != 2 {
		t.Errorf("khách mới / cũ: %v", tot)
	}
	// phiên 7 thoát ngay (1 trang, không click); các phiên khác thì không
	if num(tot, "bounce_rate") < 0.14 || num(tot, "bounce_rate") > 0.15 {
		t.Errorf("tỷ lệ thoát %v", tot["bounce_rate"])
	}
	// bỏ dở: phiên 3 + 4 (thanh toán), phiên 5 (giỏ)
	if num(tot, "abandoned_checkouts") != 2 || num(tot, "abandoned_carts") != 1 || num(tot, "abandoned_value") != 350000 {
		t.Errorf("bỏ dở: %v", tot)
	}
	prev := out["prev"].(map[string]any)
	if num(prev, "sessions") != 2 || num(prev, "orders") != 1 || num(prev, "revenue") != 100000 {
		t.Errorf("kỳ trước: %v", prev)
	}
	// phễu: 7 phiên → 5 xem SP (stage>=1) → 4 giỏ → 3...
	want := map[string]float64{"sessions": 7, "product_view": 6, "add_to_cart": 5, "checkout": 4, "info": 3, "info_done": 3, "payment": 3, "submit": 2, "ordered": 2}
	for _, r := range out["funnel"].([]any) {
		row := r.(map[string]any)
		if k := row["key"].(string); num(row, "count") != want[k] {
			t.Errorf("phễu %s = %v, muốn %v", k, row["count"], want[k])
		}
	}
	// chuỗi theo ngày: đủ 3 ngày, tổng khớp
	series := out["series"].([]any)
	if len(series) != 3 {
		t.Fatalf("series %d", len(series))
	}
	var sSum, oSum, rSum float64
	for _, p := range series {
		pm := p.(map[string]any)
		sSum += num(pm, "sessions")
		oSum += num(pm, "orders")
		rSum += num(pm, "revenue")
	}
	if sSum != 7 || oSum != 2 || rSum != 800000 {
		t.Errorf("tổng chuỗi: %v %v %v", sSum, oSum, rSum)
	}
	// nguồn: google 3 phiên 1 đơn 500k; facebook 2 phiên 1 đơn 300k
	srcs := map[string]map[string]any{}
	for _, r := range out["sources"].([]any) {
		row := r.(map[string]any)
		srcs[row["name"].(string)] = row
	}
	if num(srcs["google.com"], "sessions") != 3 || num(srcs["google.com"], "orders") != 1 || num(srcs["google.com"], "revenue") != 500000 || srcs["google.com"]["extra"] != "search" {
		t.Errorf("nguồn google: %v", srcs["google.com"])
	}
	if num(srcs["facebook"], "sessions") != 2 || num(srcs["facebook"], "revenue") != 300000 {
		t.Errorf("nguồn facebook: %v", srcs["facebook"])
	}
	devs := map[string]float64{}
	for _, r := range out["devices"].([]any) {
		devs[r.(map[string]any)["name"].(string)] = num(r.(map[string]any), "sessions")
	}
	if devs["mobile"] != 4 || devs["desktop"] != 3 {
		t.Errorf("thiết bị: %v", devs)
	}
	// trang được xem: JSON phải có khoá path / views (không phải Path / Views)
	if pg, _ := out["pages"].([]any); len(pg) > 0 {
		if _, ok := pg[0].(map[string]any)["path"]; !ok {
			t.Errorf("pages thiếu khoá path: %v", pg[0])
		}
	}
	// phương thức thanh toán + sản phẩm bán chạy
	if ms := out["methods"].([]any); len(ms) != 2 {
		t.Errorf("phương thức: %v", ms)
	}
	prods := out["products"].([]any)
	if len(prods) != 1 || num(prods[0].(map[string]any), "revenue") != 800000 || num(prods[0].(map[string]any), "qty") != 2 {
		t.Errorf("sản phẩm: %v", prods)
	}
}

// Trang trống: mọi danh sách phải là mảng rỗng (null làm giao diện báo lỗi)
func TestAnalyticsEmptyRangeReturnsEmptyArrays(t *testing.T) {
	e := newAnalyticsEnv(t)
	out := e.getJSON(e.base() + "/analytics?from=2001-01-01&to=2001-01-02&tzo=0")
	for _, k := range []string{"series", "funnel", "hours", "sources", "devices", "browsers", "countries", "landing", "pages", "products", "clicks", "searches", "methods"} {
		if _, ok := out[k].([]any); !ok {
			t.Errorf("%s phải là mảng, nhận %T", k, out[k])
		}
	}
	if _, ok := out["events"].(map[string]any); !ok {
		t.Errorf("events phải là object")
	}
}

func TestAnalyticsRangeValidationAndOwnership(t *testing.T) {
	e := newAnalyticsEnv(t)
	if num(e.getJSON(e.base()+"/analytics?from=2024-13-40"), "_status") != 400 {
		t.Error("ngày sai phải 400")
	}
	if num(e.getJSON(e.base()+"/analytics?from=2024-01-01&to=2023-01-01"), "_status") != 400 {
		t.Error("to < from phải 400")
	}
	if num(e.getJSON(e.base()+"/analytics?from=2020-01-01&to=2024-01-01"), "_status") != 400 {
		t.Error("quá 366 ngày phải 400")
	}
	if num(e.getJSON(e.base()+"/analytics"), "_status") != 200 {
		t.Error("mặc định 7 ngày phải 200")
	}
	// website của người khác
	other := models.Storefront{UserID: 8, Slug: "shop-khac", SiteTitle: "Khác", IsPublished: true}
	e.db.Create(&other)
	for _, p := range []string{"/analytics", "/analytics/live", "/analytics/abandoned"} {
		if c := num(e.getJSON(fmt.Sprintf("/api/v1/storefront/sites/%d%s", other.ID, p)), "_status"); c != 404 {
			t.Errorf("%s của người khác: %v", p, c)
		}
	}
}

func TestAnalyticsLiveAndAbandoned(t *testing.T) {
	e := newAnalyticsEnv(t)
	now := time.Now()
	add := func(sid, lastEvent, path string, seenAgo time.Duration, stage, orders int, cart int64) {
		e.db.Create(&StoreSession{StorefrontID: e.sf.ID, SID: sid, VisitorID: "v-" + sid, StartedAt: now.Add(-seenAgo - 10*time.Minute), LastSeen: now.Add(-seenAgo),
			Device: "mobile", Country: "VN", Source: "facebook", Medium: "social", Stage: stage, Orders: orders, LastEvent: lastEvent, LastPath: path,
			CartValue: cart, CartItems: 2, CartJSON: `[{"p":9,"q":2,"pr":` + fmt.Sprint(cart/2) + `,"nm":"Giày"}]`})
	}
	add("live-0000-0001", "checkout_info", "/checkout", 20*time.Second, stageInfo, 0, 400000)
	add("live-0000-0002", "checkout_payment", "/checkout", 2*time.Minute, stagePayment, 0, 200000)
	add("live-0000-0003", "product_view", "/p/9", 1*time.Minute, stageProduct, 0, 0)
	add("live-0000-0004", "page_view", "/", 4*time.Minute, stageVisit, 0, 0)
	add("live-0000-0005", "checkout_info", "/checkout", 10*time.Minute, stageInfo, 0, 600000)      // quá 5 phút: không online; bỏ dở? chưa (≤30 phút)
	add("live-0000-0006", "checkout_start", "/checkout", 50*time.Minute, stageCheckout, 0, 800000) // bỏ dở
	add("live-0000-0007", "add_to_cart", "/", 2*time.Hour, stageAddToCart, 0, 120000)              // bỏ giỏ
	add("live-0000-0008", "order_placed", "/order/*", 2*time.Hour, stageOrderPlaced, 1, 900000)    // đã đặt: không bỏ dở

	live := e.getJSON(e.base() + "/analytics/live")
	if num(live, "online") != 4 {
		t.Fatalf("online %v", live["online"])
	}
	steps := live["steps"].(map[string]any)
	if num(steps, "info") != 1 || num(steps, "payment") != 1 || num(steps, "product") != 1 || num(steps, "browsing") != 1 {
		t.Errorf("các bước: %v", steps)
	}
	first := live["sessions"].([]any)[0].(map[string]any)
	if first["sid"] != "live-0" || first["step"] != "info" || num(first, "cart_value") != 400000 || len(first["cart"].([]any)) != 1 {
		t.Errorf("khách mới nhất: %v", first)
	}

	ab := e.getJSON(e.base() + "/analytics/abandoned?tzo=0")["abandoned"].([]any)
	if len(ab) != 2 {
		t.Fatalf("bỏ dở %d, muốn 2: %v", len(ab), ab)
	}
	if num(ab[0].(map[string]any), "cart_value") != 800000 || num(ab[1].(map[string]any), "cart_value") != 120000 {
		t.Errorf("thứ tự / giá trị: %v", ab)
	}
}

func TestTrackedOrderLinksSessionAndAttributesRevenue(t *testing.T) {
	e := newAnalyticsEnv(t)
	p := models.Product{StorefrontID: e.sf.ID, Name: "Áo thử", Price: 150000, Stock: -1, IsActive: true}
	e.db.Create(&p)
	sid := "sess-eeee-0005"
	b := env(sid, ev("page_view"), ev("product_view", "p", "/p/1", "pid", p.ID), ev("add_to_cart", "pid", p.ID), ev("checkout_start", "p", "/checkout"), ev("checkout_submit", "p", "/checkout"))
	b["ctx"] = map[string]any{"ref": "https://zalo.me/x"}
	e.track(b, uaMobile, nil)
	w, out := e.do("POST", "/api/v1/storefront/public/orders", map[string]any{
		"slug": "test-shop", "items": []map[string]any{{"product_id": p.ID, "qty": 2}},
		"customer_name": "Khách", "phone": "0912345678", "address": "1 Đường A", "payment_method": "cod", "track_sid": sid,
	})
	if w.Code != 201 {
		t.Fatalf("đặt hàng: %d %v", w.Code, out)
	}
	s := e.session(sid)
	if s.Stage != stageOrderPlaced || s.Orders != 1 || s.LastEvent != "order_placed" {
		t.Fatalf("phiên sau khi đặt: %+v", s)
	}
	var o ShopOrder
	e.db.Where("code = ?", out["code"]).First(&o)
	if o.TrackSID != sid {
		t.Errorf("đơn chưa gắn phiên: %q", o.TrackSID)
	}
	// track_sid sai định dạng bị bỏ qua, đơn vẫn tạo được
	w, out = e.do("POST", "/api/v1/storefront/public/orders", map[string]any{
		"slug": "test-shop", "items": []map[string]any{{"product_id": p.ID, "qty": 1}},
		"customer_name": "Khách", "phone": "0912345678", "address": "1 Đường A", "payment_method": "cod", "track_sid": "bad sid!",
	})
	if w.Code != 201 {
		t.Fatalf("đặt hàng 2: %d %v", w.Code, out)
	}
	to := time.Now().UTC().Format("2006-01-02")
	res := e.getJSON(fmt.Sprintf("%s/analytics?from=%s&to=%s&tzo=0", e.base(), to, to))
	tot := res["totals"].(map[string]any)
	if num(tot, "orders") != 2 || num(tot, "revenue") != 450000 {
		t.Errorf("tổng: %v", tot)
	}
	src := res["sources"].([]any)[0].(map[string]any)
	if src["name"] != "zalo.me" || src["extra"] != "social" || num(src, "orders") != 1 || num(src, "revenue") != 300000 {
		t.Errorf("nguồn: %v", src)
	}
	prods := res["products"].([]any)[0].(map[string]any)
	if prods["name"] != "Áo thử" || num(prods, "views") != 1 || num(prods, "adds") != 1 || num(prods, "qty") != 3 {
		t.Errorf("sản phẩm: %v", prods)
	}
}

func TestAnalyticsPurgeKeepsRecentData(t *testing.T) {
	e := newAnalyticsEnv(t)
	now := time.Now()
	e.db.Create(&StoreEvent{StorefrontID: e.sf.ID, SID: "a", Type: "click", CreatedAt: now.AddDate(0, 0, -61)})
	e.db.Create(&StoreEvent{StorefrontID: e.sf.ID, SID: "b", Type: "click", CreatedAt: now.AddDate(0, 0, -5)})
	e.db.Create(&StoreSession{StorefrontID: e.sf.ID, SID: "old-0000-0001", StartedAt: now.AddDate(0, 0, -181), LastSeen: now.AddDate(0, 0, -181)})
	e.db.Create(&StoreSession{StorefrontID: e.sf.ID, SID: "new-0000-0001", StartedAt: now.AddDate(0, 0, -100), LastSeen: now.AddDate(0, 0, -100)})
	e.h.purgeAnalytics(now)
	var ne, ns int64
	e.db.Model(&StoreEvent{}).Count(&ne)
	e.db.Model(&StoreSession{}).Count(&ns)
	if ne != 1 || ns != 1 {
		t.Errorf("còn %d sự kiện, %d phiên (muốn 1 và 1)", ne, ns)
	}
}
