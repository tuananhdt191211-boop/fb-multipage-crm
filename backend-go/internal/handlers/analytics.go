package handlers

// Thống kê website bán hàng (kiểu Shopify / ShopBase):
//   - Trang bán hàng gửi lô sự kiện nhỏ (xem trang, xem sản phẩm, thêm giỏ, từng bước thanh toán, click, tìm kiếm...)
//     về POST /storefront/public/track. Không lưu IP, không lưu họ tên / số điện thoại — chỉ mã ẩn danh của trình duyệt.
//   - Mỗi lượt truy cập là một "phiên" (store_sessions): nguồn, thiết bị, nước, bước xa nhất trong phễu (stage),
//     bước hiện tại (để xem trực tiếp khách đang ở đâu), giỏ hàng gần nhất. Sự kiện chi tiết ở store_events.
//   - Đơn hàng gắn với phiên qua track_sid nên thống kê được doanh thu theo nguồn / thiết bị / nước.
//   - Dữ liệu cũ được dọn định kỳ để không làm đầy database (xem RunAnalyticsJanitor).
//
// Báo cáo nằm ở analytics_report.go.

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Các bước của phễu (Stage = bước xa nhất mà phiên đã đạt)
const (
	stageVisit       = 0
	stageProduct     = 1 // xem sản phẩm
	stageAddToCart   = 2 // thêm vào giỏ
	stageCheckout    = 3 // mở trang thanh toán
	stageInfo        = 4 // bắt đầu nhập thông tin giao hàng
	stageInfoDone    = 5 // đã điền đủ họ tên, số điện thoại, địa chỉ
	stagePayment     = 6 // chọn phương thức thanh toán
	stageSubmit      = 7 // bấm đặt hàng / thanh toán
	stageOrderPlaced = 8 // đơn hàng đã được tạo (do máy chủ ghi nhận)
)

type StoreSession struct {
	ID           uint      `gorm:"primaryKey" json:"-"`
	StorefrontID uint      `gorm:"not null;uniqueIndex:idx_ss_site_sid,priority:1;index:idx_ss_site_start,priority:1;index:idx_ss_site_seen,priority:1"`
	SID          string    `gorm:"column:sid;size:40;not null;uniqueIndex:idx_ss_site_sid,priority:2"`
	VisitorID    string    `gorm:"size:40;index"`
	StartedAt    time.Time `gorm:"index:idx_ss_site_start,priority:2"`
	LastSeen     time.Time `gorm:"index:idx_ss_site_seen,priority:2"`
	Source       string    `gorm:"size:60"`  // utm_source hoặc tên miền giới thiệu; "direct" = vào thẳng
	Medium       string    `gorm:"size:20"`  // direct | search | social | email | paid | referral
	Campaign     string    `gorm:"size:80"`  // utm_campaign
	Landing      string    `gorm:"size:200"` // trang đầu tiên khách vào
	Device       string    `gorm:"size:10"`  // mobile | tablet | desktop
	Browser      string    `gorm:"size:20"`
	OS           string    `gorm:"size:20"`
	Country      string    `gorm:"size:2"` // mã nước (ước lượng theo ngôn ngữ trình duyệt, hoặc theo Cloudflare nếu có)
	Lang         string    `gorm:"size:8"`
	Market       string    `gorm:"size:8"`              // thị trường (nước / tiền tệ) khách đang chọn
	Returning    bool      `gorm:"column:is_returning"` // khách quay lại (trình duyệt đã từng vào web này)
	Pages        int       // số lượt xem trang
	Clicks       int
	Events       int
	Stage        int
	LastEvent    string `gorm:"size:24"` // bước hiện tại: sự kiện "có ý nghĩa" gần nhất
	LastPath     string `gorm:"size:200"`
	CartValue    int64  // giá trị giỏ hàng gần nhất (VND)
	CartItems    int
	CartJSON     string `gorm:"type:text"` // tối đa 10 dòng: product_id, variant_id, qty, price, name
	Orders       int
}

type StoreEvent struct {
	ID           uint      `gorm:"primaryKey" json:"-"`
	StorefrontID uint      `gorm:"not null;index:idx_se_site_time,priority:1;index:idx_se_site_type_time,priority:1"`
	SID          string    `gorm:"column:sid;size:40;index:idx_se_sid"`
	Type         string    `gorm:"size:24;not null;index:idx_se_site_type_time,priority:2"`
	CreatedAt    time.Time `gorm:"index:idx_se_site_time,priority:2;index:idx_se_site_type_time,priority:3"`
	Path         string    `gorm:"size:200"`
	ProductID    uint
	Label        string `gorm:"size:120"` // chữ trên nút / từ khoá tìm kiếm / phương thức thanh toán
	Target       string `gorm:"size:160"` // đích của click (đường dẫn hoặc tên miền ngoài)
	Section      string `gorm:"size:24"`  // loại section chứa click (header, hero, products...)
	Value        int64  // giá trị (VND), số kết quả tìm kiếm...
}

func migrateAnalytics(db *gorm.DB) {
	_ = db.AutoMigrate(&StoreSession{}, &StoreEvent{})
}

type trackEventDef struct {
	stage int  // bước phễu mà sự kiện này đạt được
	sets  bool // có đổi "bước hiện tại" của phiên không
	store bool // có lưu thành dòng sự kiện không
}

var trackTypes = map[string]trackEventDef{
	"page_view":          {0, true, true},
	"product_view":       {stageProduct, true, true},
	"add_to_cart":        {stageAddToCart, true, true},
	"remove_from_cart":   {0, false, true},
	"cart_open":          {0, true, true},
	"checkout_start":     {stageCheckout, true, true},
	"checkout_info":      {stageInfo, true, true},
	"checkout_info_done": {stageInfoDone, true, true},
	"checkout_payment":   {stagePayment, true, true},
	"checkout_submit":    {stageSubmit, true, true},
	"checkout_error":     {0, false, true},
	"click":              {0, false, true},
	"search":             {0, true, true},
	"market_change":      {0, false, true},
	"heartbeat":          {0, false, false},
}

type trackEvent struct {
	T   string `json:"t"`
	P   string `json:"p"`
	Pid uint   `json:"pid"`
	L   string `json:"l"`
	Tg  string `json:"tg"`
	Sec string `json:"sec"`
	V   int64  `json:"v"`
}

type trackCartItem struct {
	P  uint   `json:"p"`
	V  uint   `json:"v"`
	Q  int    `json:"q"`
	Pr int64  `json:"pr"`
	Nm string `json:"nm"`
}

type trackEnvelope struct {
	Slug string `json:"slug"`
	SID  string `json:"sid"`
	VID  string `json:"vid"`
	Ctx  struct {
		Ref       string `json:"ref"`
		UTMSource string `json:"us"`
		UTMMedium string `json:"um"`
		UTMCamp   string `json:"uc"`
		FBCLID    bool   `json:"fb"`
		Landing   string `json:"landing"`
		Lang      string `json:"lang"`
		Market    string `json:"market"`
		Country   string `json:"country"`
		Returning bool   `json:"ret"`
	} `json:"ctx"`
	Cart *struct {
		Items []trackCartItem `json:"items"`
	} `json:"cart"`
	Events []trackEvent `json:"events"`
}

const (
	maxTrackBody   = 32 << 10
	maxTrackEvents = 25
)

var trackIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{8,40}$`)

// ----------------------------------------------------------------- Chống lạm dụng

// Mỗi IP tối đa 90 lô / phút; mỗi website tối đa 6000 sự kiện / phút (bảo vệ database khi bị bơm dữ liệu)
var trackLimiter = struct {
	sync.Mutex
	hits  map[string][]time.Time
	sites map[uint]*siteBudget
}{hits: map[string][]time.Time{}, sites: map[uint]*siteBudget{}}

type siteBudget struct {
	minute int64
	n      int
}

func allowTrack(ip string, siteID uint, events int) bool {
	trackLimiter.Lock()
	defer trackLimiter.Unlock()
	now := time.Now()
	if len(trackLimiter.hits) > 20000 { // dọn bớt để không phình RAM
		trackLimiter.hits = map[string][]time.Time{}
	}
	keep := trackLimiter.hits[ip][:0]
	for _, t := range trackLimiter.hits[ip] {
		if now.Sub(t) < time.Minute {
			keep = append(keep, t)
		}
	}
	if len(keep) >= 90 {
		trackLimiter.hits[ip] = keep
		return false
	}
	trackLimiter.hits[ip] = append(keep, now)
	if siteID != 0 {
		m := now.Unix() / 60
		b := trackLimiter.sites[siteID]
		if b == nil || b.minute != m {
			b = &siteBudget{minute: m}
			trackLimiter.sites[siteID] = b
		}
		if b.n+events > 6000 {
			return false
		}
		b.n += events
	}
	return true
}

var botUARe = regexp.MustCompile(`(?i)(bot\b|bot/|crawl|spider|slurp|facebookexternalhit|preview|lighthouse|pingdom|uptime|monitor|curl/|wget/|python-requests|go-http-client|httpclient|axios/|node-fetch)`)

func isBotUA(ua string) bool {
	ua = strings.TrimSpace(ua)
	return ua == "" || botUARe.MatchString(ua)
}

// ----------------------------------------------------------------- Tra cứu website (có đệm để không đọc database mỗi lần)

type trackSiteInfo struct {
	ID      uint
	Slug    string
	Domain  string
	Expires time.Time
	Missing bool
}

var trackSiteCache = struct {
	sync.RWMutex
	m map[string]trackSiteInfo
}{m: map[string]trackSiteInfo{}}

func (h *StorefrontHandler) trackSite(slug string) (trackSiteInfo, bool) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" || len(slug) > 80 {
		return trackSiteInfo{}, false
	}
	trackSiteCache.RLock()
	hit, ok := trackSiteCache.m[slug]
	trackSiteCache.RUnlock()
	if ok && time.Now().Before(hit.Expires) {
		return hit, !hit.Missing
	}
	info := trackSiteInfo{Slug: slug, Expires: time.Now().Add(time.Minute)}
	if sf, found := h.publishedSiteBySlug(slug); found {
		info.ID, info.Domain = sf.ID, sf.Domain
	} else {
		info.Missing = true
		info.Expires = time.Now().Add(15 * time.Second)
	}
	trackSiteCache.Lock()
	if len(trackSiteCache.m) > 5000 {
		trackSiteCache.m = map[string]trackSiteInfo{}
	}
	trackSiteCache.m[slug] = info
	trackSiteCache.Unlock()
	return info, !info.Missing
}

// ----------------------------------------------------------------- Làm sạch dữ liệu từ trình duyệt

func trackStr(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return trimTo(s, n)
}

var numericSegRe = regexp.MustCompile(`^\d{1,12}$`)

// cleanTrackPath: chỉ giữ đường dẫn tương đối của cửa hàng. Bỏ tham số (có thể chứa mã đơn / token),
// bỏ tiền tố /s/<slug>, gộp trang đơn hàng thành /order/*
func cleanTrackPath(slug, p string) string {
	p = strings.TrimSpace(p)
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if p == "" || p[0] != '/' {
		return "/"
	}
	if prefix := "/s/" + slug; p == prefix || strings.HasPrefix(p, prefix+"/") {
		p = strings.TrimPrefix(p, prefix)
		if p == "" {
			p = "/"
		}
	}
	segs := strings.Split(strings.Trim(p, "/"), "/")
	switch segs[0] {
	case "order":
		return "/order/*"
	case "p":
		if len(segs) >= 2 && numericSegRe.MatchString(segs[1]) {
			return "/p/" + segs[1]
		}
		return "/p"
	case "checkout", "account":
		return "/" + segs[0]
	}
	return trackStr(p, 120)
}

func cleanTrackTarget(slug, t string) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return ""
	}
	if strings.HasPrefix(t, "/") || strings.HasPrefix(t, "#") {
		if strings.HasPrefix(t, "#") {
			return trackStr(t, 60)
		}
		return cleanTrackPath(slug, t)
	}
	if strings.HasPrefix(t, "tel:") {
		return "tel:"
	}
	if strings.HasPrefix(t, "mailto:") {
		return "mailto:"
	}
	if u, err := url.Parse(t); err == nil && u.Host != "" {
		return trackStr(strings.TrimPrefix(normalizeHost(u.Host), "www."), 100)
	}
	return ""
}

// ----------------------------------------------------------------- Phân loại nguồn truy cập, thiết bị

var searchHosts = []string{"google.", "bing.", "yahoo.", "duckduckgo.", "coccoc.", "baidu.", "yandex.", "ecosia.", "naver."}
var socialHosts = []string{"facebook.", "fb.com", "fb.me", "l.facebook.", "m.facebook.", "instagram.", "tiktok.", "youtube.", "youtu.be", "zalo.", "twitter.", "t.co", "x.com", "pinterest.", "linkedin.", "reddit.", "threads.", "messenger."}

// classifyTraffic trả về (nguồn, kênh, chiến dịch). own = các tên miền của chính website (không tính là giới thiệu).
func classifyTraffic(ref, utmSource, utmMedium, utmCampaign string, fb bool, own ...string) (string, string, string) {
	utmSource = strings.ToLower(trackStr(utmSource, 60))
	utmMedium = strings.ToLower(trackStr(utmMedium, 20))
	utmCampaign = trackStr(utmCampaign, 80)
	if utmSource != "" {
		medium := utmMedium
		switch {
		case medium == "cpc" || medium == "ppc" || medium == "paid" || medium == "paidsocial" || medium == "display":
			medium = "paid"
		case medium == "email" || medium == "newsletter":
			medium = "email"
		case medium == "":
			medium = nameMedium(utmSource)
		}
		return utmSource, medium, utmCampaign
	}
	host := ""
	if u, err := url.Parse(strings.TrimSpace(ref)); err == nil {
		host = strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	}
	for _, o := range own {
		if o != "" && host == strings.TrimPrefix(strings.ToLower(normalizeHost(o)), "www.") {
			host = ""
		}
	}
	if fb && host == "" {
		return "facebook", "social", utmCampaign
	}
	if host == "" {
		return "direct", "direct", utmCampaign
	}
	return trackStr(host, 60), hostMedium(host), utmCampaign
}

// nameMedium: kênh của tên nguồn không có dấu chấm (utm_source=facebook, tiktok, google...)
func nameMedium(name string) string {
	if strings.Contains(name, ".") {
		return hostMedium(name)
	}
	for _, s := range []string{"google", "bing", "yahoo", "duckduckgo", "coccoc"} {
		if strings.Contains(name, s) {
			return "search"
		}
	}
	for _, s := range []string{"facebook", "fb", "instagram", "ig", "tiktok", "youtube", "zalo", "twitter", "pinterest", "linkedin", "reddit", "threads", "messenger"} {
		if name == s || (len(s) > 3 && strings.Contains(name, s)) {
			return "social"
		}
	}
	return "referral"
}

func hostMedium(host string) string {
	for _, s := range searchHosts {
		if strings.Contains(host, s) {
			return "search"
		}
	}
	for _, s := range socialHosts {
		if strings.Contains(host, s) {
			return "social"
		}
	}
	return "referral"
}

func parseDevice(ua string) (device, browser, osName string) {
	l := strings.ToLower(ua)
	switch {
	case strings.Contains(l, "ipad") || strings.Contains(l, "tablet") || (strings.Contains(l, "android") && !strings.Contains(l, "mobile")):
		device = "tablet"
	case strings.Contains(l, "mobi") || strings.Contains(l, "iphone") || strings.Contains(l, "ipod"):
		device = "mobile"
	default:
		device = "desktop"
	}
	switch {
	case strings.Contains(l, "edg/") || strings.Contains(l, "edga/") || strings.Contains(l, "edgios/"):
		browser = "Edge"
	case strings.Contains(l, "opr/") || strings.Contains(l, "opera"):
		browser = "Opera"
	case strings.Contains(l, "coc_coc"):
		browser = "Cốc Cốc"
	case strings.Contains(l, "fban") || strings.Contains(l, "fbav") || strings.Contains(l, "instagram"):
		browser = "Facebook/Instagram"
	case strings.Contains(l, "zalo"):
		browser = "Zalo"
	case strings.Contains(l, "firefox") || strings.Contains(l, "fxios"):
		browser = "Firefox"
	case strings.Contains(l, "chrome") || strings.Contains(l, "crios"):
		browser = "Chrome"
	case strings.Contains(l, "safari"):
		browser = "Safari"
	default:
		browser = "Khác"
	}
	switch {
	case strings.Contains(l, "windows"):
		osName = "Windows"
	case strings.Contains(l, "iphone") || strings.Contains(l, "ipad") || strings.Contains(l, "ipod"):
		osName = "iOS"
	case strings.Contains(l, "android"):
		osName = "Android"
	case strings.Contains(l, "mac os") || strings.Contains(l, "macintosh"):
		osName = "macOS"
	case strings.Contains(l, "linux") || strings.Contains(l, "cros"):
		osName = "Linux"
	default:
		osName = "Khác"
	}
	return
}

var countryRe = regexp.MustCompile(`^[A-Za-z]{2}$`)

// ----------------------------------------------------------------- Nhận sự kiện

// TrackEvents POST /api/v1/storefront/public/track
// Nội dung là JSON (gửi dạng text/plain để navigator.sendBeacon không cần preflight CORS). Luôn trả 204 khi nhận hoặc bỏ qua êm.
func (h *StorefrontHandler) TrackEvents(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxTrackBody+1))
	if err != nil || len(raw) > maxTrackBody {
		c.Status(http.StatusRequestEntityTooLarge)
		return
	}
	var env trackEnvelope
	if json.Unmarshal(raw, &env) != nil || !trackIDRe.MatchString(env.SID) || !trackIDRe.MatchString(env.VID) {
		c.Status(http.StatusBadRequest)
		return
	}
	if isBotUA(c.GetHeader("User-Agent")) {
		c.Status(http.StatusNoContent)
		return
	}
	site, ok := h.trackSite(env.Slug)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	if len(env.Events) > maxTrackEvents {
		env.Events = env.Events[:maxTrackEvents]
	}
	if !allowTrack(c.ClientIP(), site.ID, len(env.Events)) {
		c.Status(http.StatusTooManyRequests)
		return
	}

	now := time.Now()
	var rows []StoreEvent
	pv, clicks, maxStage := 0, 0, 0
	lastEvent, lastPath, firstPath := "", "", ""
	for _, ev := range env.Events {
		def, known := trackTypes[ev.T]
		if !known {
			continue
		}
		path := cleanTrackPath(site.Slug, ev.P)
		switch ev.T {
		case "page_view":
			pv++
			if firstPath == "" {
				firstPath = path
			}
		case "click":
			clicks++
		}
		if def.stage > maxStage {
			maxStage = def.stage
		}
		if def.sets {
			lastEvent, lastPath = ev.T, path
		}
		if def.store {
			rows = append(rows, StoreEvent{
				StorefrontID: site.ID, SID: env.SID, Type: ev.T, CreatedAt: now, Path: path,
				ProductID: ev.Pid, Label: trackStr(ev.L, 120), Target: cleanTrackTarget(site.Slug, ev.Tg),
				Section: trackStr(ev.Sec, 24), Value: ev.V,
			})
		}
	}

	// Giỏ hàng gần nhất (giá trị tính lại ở máy chủ, không tin con số trình duyệt gửi)
	hasCart := env.Cart != nil
	var cartValue int64
	cartItems := 0
	cartJSON := ""
	if hasCart {
		items := env.Cart.Items
		if len(items) > 10 {
			items = items[:10]
		}
		clean := make([]trackCartItem, 0, len(items))
		for _, it := range items {
			if it.P == 0 || it.Q <= 0 || it.Q > 99 || it.Pr < 0 || it.Pr > 1_000_000_000_000 {
				continue
			}
			clean = append(clean, trackCartItem{P: it.P, V: it.V, Q: it.Q, Pr: it.Pr, Nm: trackStr(it.Nm, 60)})
			cartValue += int64(it.Q) * it.Pr
			cartItems += it.Q
		}
		if b, err := json.Marshal(clean); err == nil && len(clean) > 0 {
			cartJSON = string(b)
		}
	}

	ua := c.GetHeader("User-Agent")
	device, browser, osName := parseDevice(ua)
	country := strings.ToUpper(strings.TrimSpace(c.GetHeader("CF-IPCountry"))) // có khi web chạy sau Cloudflare
	if !countryRe.MatchString(country) || country == "XX" || country == "T1" {
		country = strings.ToUpper(strings.TrimSpace(env.Ctx.Country))
		if !countryRe.MatchString(country) {
			country = ""
		}
	}
	ownHosts := []string{site.Domain}
	if h.Cfg != nil {
		if u, err := url.Parse(strings.TrimSpace(h.Cfg.FrontendURL)); err == nil {
			ownHosts = append(ownHosts, u.Host)
		}
	}
	source, medium, campaign := classifyTraffic(env.Ctx.Ref, env.Ctx.UTMSource, env.Ctx.UTMMedium, env.Ctx.UTMCamp, env.Ctx.FBCLID, ownHosts...)
	landing := cleanTrackPath(site.Slug, env.Ctx.Landing)
	if env.Ctx.Landing == "" {
		landing = firstPath
	}

	sess := StoreSession{
		StorefrontID: site.ID, SID: env.SID, VisitorID: env.VID, StartedAt: now, LastSeen: now,
		Source: source, Medium: medium, Campaign: campaign, Landing: landing,
		Device: device, Browser: browser, OS: osName, Country: country,
		Lang: trackStr(env.Ctx.Lang, 8), Market: trackStr(env.Ctx.Market, 8), Returning: env.Ctx.Returning,
		Pages: pv, Clicks: clicks, Events: len(rows), Stage: maxStage, LastEvent: lastEvent, LastPath: lastPath,
		CartValue: cartValue, CartItems: cartItems, CartJSON: cartJSON,
	}
	assign := map[string]any{
		"last_seen": now,
		"pages":     gorm.Expr("store_sessions.pages + ?", pv),
		"clicks":    gorm.Expr("store_sessions.clicks + ?", clicks),
		"events":    gorm.Expr("store_sessions.events + ?", len(rows)),
		"stage":     gorm.Expr("GREATEST(store_sessions.stage, ?)", maxStage),
	}
	if lastEvent != "" {
		assign["last_event"] = lastEvent
		assign["last_path"] = lastPath
	}
	if hasCart {
		assign["cart_value"] = cartValue
		assign["cart_items"] = cartItems
		assign["cart_json"] = cartJSON
	}
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "storefront_id"}, {Name: "sid"}},
			DoUpdates: clause.Assignments(assign),
		}).Create(&sess).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			return tx.Create(&rows).Error
		}
		return nil
	})
	if err != nil {
		log.Printf("[track] lỗi ghi thống kê: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

// sessionOrdered ghi nhận phiên đã đặt hàng (gọi từ CreatePublicOrder; lỗi không làm hỏng đơn hàng)
func (h *StorefrontHandler) sessionOrdered(siteID uint, sid string) {
	if !trackIDRe.MatchString(sid) {
		return
	}
	h.DB.Model(&StoreSession{}).Where("storefront_id = ? AND sid = ?", siteID, sid).Updates(map[string]any{
		"stage":      gorm.Expr("GREATEST(stage, ?)", stageOrderPlaced),
		"orders":     gorm.Expr("orders + 1"),
		"last_event": "order_placed",
		"last_seen":  time.Now(),
	})
}

// ----------------------------------------------------------------- Dọn dữ liệu cũ

const (
	analyticsEventDays   = 60
	analyticsSessionDays = 180
)

var janitorOnce sync.Once

// RunAnalyticsJanitor xoá sự kiện > 60 ngày, phiên > 180 ngày (mỗi ngày một lần, xoá từng đợt nhỏ)
func (h *StorefrontHandler) RunAnalyticsJanitor() {
	janitorOnce.Do(func() {
		go func() {
			time.Sleep(2 * time.Minute)
			for {
				h.purgeAnalytics(time.Now())
				time.Sleep(24 * time.Hour)
			}
		}()
	})
}

func (h *StorefrontHandler) purgeAnalytics(now time.Time) {
	ev := now.AddDate(0, 0, -analyticsEventDays)
	ss := now.AddDate(0, 0, -analyticsSessionDays)
	for i := 0; i < 200; i++ {
		r := h.DB.Exec("DELETE FROM store_events WHERE id IN (SELECT id FROM store_events WHERE created_at < ? LIMIT 5000)", ev)
		if r.Error != nil || r.RowsAffected == 0 {
			break
		}
	}
	for i := 0; i < 200; i++ {
		r := h.DB.Exec("DELETE FROM store_sessions WHERE id IN (SELECT id FROM store_sessions WHERE started_at < ? LIMIT 5000)", ss)
		if r.Error != nil || r.RowsAffected == 0 {
			break
		}
	}
}
