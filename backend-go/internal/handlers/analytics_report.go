package handlers

// Báo cáo thống kê cho chủ shop (analytics.go lo phần thu thập).
//   GET /storefront/sites/:id/analytics?from=YYYY-MM-DD&to=YYYY-MM-DD&tzo=420   tổng quan, phễu, biểu đồ, bảng phân tích
//   GET /storefront/sites/:id/analytics/live                                     khách đang online và đang ở bước nào
//   GET /storefront/sites/:id/analytics/abandoned?from=&to=&tzo=                 giỏ hàng / thanh toán bỏ dở
// from/to là ngày theo múi giờ của chủ shop (tzo = số phút lệch so với UTC, vd. Việt Nam 420).

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
)

const (
	liveWindow       = 5 * time.Minute  // còn hoạt động trong 5 phút gần nhất = đang online
	abandonAfter     = 30 * time.Minute // im lặng quá 30 phút mà chưa đặt hàng = bỏ dở
	maxAnalyticsDays = 366
)

type analyticsRange struct {
	From, To     time.Time // [From, To) theo UTC
	Prev, PrevTo time.Time // kỳ liền trước, cùng độ dài
	Tzo          int
	Days         int
}

// parseAnalyticsRange đọc from/to/tzo; mặc định 7 ngày gần nhất
func parseAnalyticsRange(c *gin.Context) (analyticsRange, bool) {
	tzo, err := strconv.Atoi(c.DefaultQuery("tzo", "420"))
	if err != nil || tzo < -720 || tzo > 840 {
		tzo = 420
	}
	loc := time.FixedZone("u", tzo*60)
	today := time.Now().In(loc)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	parse := func(s string, def time.Time) (time.Time, bool) {
		if strings.TrimSpace(s) == "" {
			return def, true
		}
		t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), loc)
		return t, err == nil
	}
	from, ok1 := parse(c.Query("from"), today.AddDate(0, 0, -6))
	to, ok2 := parse(c.Query("to"), today)
	if !ok1 || !ok2 || to.Before(from) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Khoảng ngày không hợp lệ."})
		return analyticsRange{}, false
	}
	end := to.AddDate(0, 0, 1) // gồm cả ngày "to"
	days := int(end.Sub(from).Round(24*time.Hour) / (24 * time.Hour))
	if days > maxAnalyticsDays {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chỉ xem được tối đa 366 ngày mỗi lần."})
		return analyticsRange{}, false
	}
	return analyticsRange{
		From: from.UTC(), To: end.UTC(),
		Prev: from.AddDate(0, 0, -days).UTC(), PrevTo: from.UTC(),
		Tzo: tzo, Days: days,
	}, true
}

// ----------------------------------------------------------------- Số liệu tổng

type analyticsTotals struct {
	Sessions           int64   `json:"sessions"`
	Visitors           int64   `json:"visitors"`
	NewVisitors        int64   `json:"new_visitors"`
	ReturningVisitors  int64   `json:"returning_visitors"`
	PageViews          int64   `json:"page_views"`
	Orders             int64   `json:"orders"`
	CancelledOrders    int64   `json:"cancelled_orders"`
	PaidOrders         int64   `json:"paid_orders"`
	Revenue            int64   `json:"revenue"`
	AOV                int64   `json:"aov"`
	ConversionRate     float64 `json:"conversion_rate"`
	AddToCartRate      float64 `json:"add_to_cart_rate"`
	CheckoutRate       float64 `json:"checkout_rate"`
	BounceRate         float64 `json:"bounce_rate"`
	AvgSessionSec      int64   `json:"avg_session_sec"`
	AbandonedCarts     int64   `json:"abandoned_carts"`
	AbandonedCheckouts int64   `json:"abandoned_checkouts"`
	AbandonedValue     int64   `json:"abandoned_value"`
}

type funnelRow struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

type sessionAgg struct {
	Sessions     int64
	Visitors     int64
	NewVisitors  int64
	PageViews    int64
	Bounced      int64
	AvgSec       float64
	Product      int64
	AddToCart    int64
	Checkout     int64
	Info         int64
	InfoDone     int64
	Payment      int64
	Submit       int64
	Ordered      int64
	AbCarts      int64
	AbCheckouts  int64
	AbandonValue int64
}

func (h *StorefrontHandler) sessionAggregates(siteID uint, from, to time.Time) sessionAgg {
	var a sessionAgg
	cutoff := time.Now().Add(-abandonAfter)
	err := h.DB.Raw(`
SELECT COUNT(*) AS sessions,
  COUNT(DISTINCT visitor_id) AS visitors,
  COUNT(DISTINCT visitor_id) FILTER (WHERE NOT is_returning) AS new_visitors,
  COALESCE(SUM(pages), 0) AS page_views,
  COUNT(*) FILTER (WHERE pages <= 1 AND clicks = 0 AND stage = 0 AND orders = 0) AS bounced,
  COALESCE(AVG(EXTRACT(EPOCH FROM (last_seen - started_at))) FILTER (WHERE last_seen > started_at), 0) AS avg_sec,
  COUNT(*) FILTER (WHERE stage >= ?) AS product,
  COUNT(*) FILTER (WHERE stage >= ?) AS add_to_cart,
  COUNT(*) FILTER (WHERE stage >= ?) AS checkout,
  COUNT(*) FILTER (WHERE stage >= ?) AS info,
  COUNT(*) FILTER (WHERE stage >= ?) AS info_done,
  COUNT(*) FILTER (WHERE stage >= ?) AS payment,
  COUNT(*) FILTER (WHERE stage >= ?) AS submit,
  COUNT(*) FILTER (WHERE stage >= ?) AS ordered,
  COUNT(*) FILTER (WHERE stage = ? AND orders = 0 AND last_seen < ?) AS ab_carts,
  COUNT(*) FILTER (WHERE stage >= ? AND stage < ? AND orders = 0 AND last_seen < ?) AS ab_checkouts,
  COALESCE(SUM(cart_value) FILTER (WHERE stage >= ? AND stage < ? AND orders = 0 AND last_seen < ?), 0) AS abandon_value
FROM store_sessions WHERE storefront_id = ? AND started_at >= ? AND started_at < ?`,
		stageProduct, stageAddToCart, stageCheckout, stageInfo, stageInfoDone, stagePayment, stageSubmit, stageOrderPlaced,
		stageAddToCart, cutoff,
		stageCheckout, stageOrderPlaced, cutoff,
		stageAddToCart, stageOrderPlaced, cutoff,
		siteID, from, to).Scan(&a).Error
	if err != nil {
		log.Printf("[analytics] lỗi thống kê phiên: %v", err)
	}
	return a
}

// orderStats gom đơn hàng trong kỳ (không tính đơn đã huỷ vào doanh thu)
type orderStats struct {
	Orders, Cancelled, Paid, Revenue int64
	PerDay                           map[string]*dayPoint
	Methods                          map[string]*methodStat
	Products                         map[uint]*productStat
	BySID                            map[string]int64 // track_sid -> doanh thu
	CountBySID                       map[string]int64 // track_sid -> số đơn
}

type dayPoint struct {
	Orders  int64
	Revenue int64
}

type methodStat struct {
	Orders  int64
	Paid    int64
	Revenue int64
}

type productStat struct {
	Name    string
	Orders  int64
	Qty     int64
	Revenue int64
}

func (h *StorefrontHandler) orderAggregates(siteID uint, from, to time.Time, tzo int) orderStats {
	st := orderStats{PerDay: map[string]*dayPoint{}, Methods: map[string]*methodStat{}, Products: map[uint]*productStat{}, BySID: map[string]int64{}, CountBySID: map[string]int64{}}
	var orders []ShopOrder
	h.DB.Select("id, total, status, payment_status, payment_method, items_json, track_sid, created_at").
		Where("storefront_id = ? AND created_at >= ? AND created_at < ?", siteID, from, to).
		Order("id DESC").Limit(20000).Find(&orders)
	off := time.Duration(tzo) * time.Minute
	for _, o := range orders {
		if o.Status == "cancelled" {
			st.Cancelled++
			continue
		}
		st.Orders++
		st.Revenue += o.Total
		if o.PaymentStatus == "paid" {
			st.Paid++
		}
		day := o.CreatedAt.UTC().Add(off).Format("2006-01-02")
		dp := st.PerDay[day]
		if dp == nil {
			dp = &dayPoint{}
			st.PerDay[day] = dp
		}
		dp.Orders++
		dp.Revenue += o.Total
		m := st.Methods[o.PaymentMethod]
		if m == nil {
			m = &methodStat{}
			st.Methods[o.PaymentMethod] = m
		}
		m.Orders++
		m.Revenue += o.Total
		if o.PaymentStatus == "paid" {
			m.Paid++
		}
		if o.TrackSID != "" {
			st.BySID[o.TrackSID] += o.Total
			st.CountBySID[o.TrackSID]++
		}
		seen := map[uint]bool{}
		for _, it := range o.Items {
			ps := st.Products[it.ProductID]
			if ps == nil {
				ps = &productStat{Name: it.Name}
				st.Products[it.ProductID] = ps
			}
			if !seen[it.ProductID] {
				ps.Orders++
				seen[it.ProductID] = true
			}
			ps.Qty += int64(it.Qty)
			ps.Revenue += it.LineTotal
		}
	}
	return st
}

func ratio(a, b int64) float64 {
	if b <= 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func (h *StorefrontHandler) totalsFor(siteID uint, from, to time.Time, tzo int) (analyticsTotals, sessionAgg, orderStats) {
	sa := h.sessionAggregates(siteID, from, to)
	os := h.orderAggregates(siteID, from, to, tzo)
	t := analyticsTotals{
		Sessions: sa.Sessions, Visitors: sa.Visitors, NewVisitors: sa.NewVisitors,
		ReturningVisitors: sa.Visitors - sa.NewVisitors, PageViews: sa.PageViews,
		Orders: os.Orders, CancelledOrders: os.Cancelled, PaidOrders: os.Paid, Revenue: os.Revenue,
		AddToCartRate: ratio(sa.AddToCart, sa.Sessions), CheckoutRate: ratio(sa.Checkout, sa.Sessions),
		BounceRate: ratio(sa.Bounced, sa.Sessions), AvgSessionSec: int64(sa.AvgSec),
		AbandonedCarts: sa.AbCarts, AbandonedCheckouts: sa.AbCheckouts, AbandonedValue: sa.AbandonValue,
	}
	if os.Orders > 0 {
		t.AOV = os.Revenue / os.Orders
	}
	// Tỷ lệ chuyển đổi = đơn hàng / lượt truy cập (đơn từ trình duyệt không bật theo dõi vẫn được tính, nên giới hạn ở 100%)
	t.ConversionRate = ratio(os.Orders, sa.Sessions)
	if t.ConversionRate > 1 {
		t.ConversionRate = 1
	}
	return t, sa, os
}

// ----------------------------------------------------------------- Bảng phân tích

type breakdownRow struct {
	Name     string  `json:"name"`
	Extra    string  `json:"extra,omitempty"` // kênh (medium) của nguồn
	Sessions int64   `json:"sessions"`
	Orders   int64   `json:"orders"`
	Revenue  int64   `json:"revenue"`
	Rate     float64 `json:"conversion_rate"`
}

var breakdownCols = map[string]bool{"source": true, "device": true, "country": true, "landing": true, "market": true, "browser": true, "os": true}

func (h *StorefrontHandler) breakdown(siteID uint, col string, from, to time.Time, limit int, revBySID map[string]int64, ordersBySID map[string]int64) []breakdownRow {
	if !breakdownCols[col] {
		return nil
	}
	extra := "''"
	if col == "source" {
		extra = "MAX(medium)"
	}
	type row struct {
		Name     string
		Extra    string
		Sessions int64
		Sids     string
	}
	var rows []row
	h.DB.Raw(`SELECT COALESCE(NULLIF(`+col+`, ''), '-') AS name, `+extra+` AS extra, COUNT(*) AS sessions,
  COALESCE(string_agg(CASE WHEN orders > 0 THEN sid END, ',' ), '') AS sids
FROM store_sessions WHERE storefront_id = ? AND started_at >= ? AND started_at < ?
GROUP BY 1 ORDER BY sessions DESC, name LIMIT ?`, siteID, from, to, limit).Scan(&rows)
	out := make([]breakdownRow, 0, len(rows))
	for _, r := range rows {
		br := breakdownRow{Name: r.Name, Extra: r.Extra, Sessions: r.Sessions}
		for _, sid := range strings.Split(r.Sids, ",") {
			if sid == "" {
				continue
			}
			if n, ok := ordersBySID[sid]; ok {
				br.Orders += n
				br.Revenue += revBySID[sid]
			}
		}
		br.Rate = ratio(br.Orders, br.Sessions)
		out = append(out, br)
	}
	return out
}

type topProduct struct {
	ProductID uint   `json:"product_id"`
	Name      string `json:"name"`
	Views     int64  `json:"views"`
	Adds      int64  `json:"adds"`
	Orders    int64  `json:"orders"`
	Qty       int64  `json:"qty"`
	Revenue   int64  `json:"revenue"`
}

type clickRow struct {
	Section string `json:"section"`
	Label   string `json:"label"`
	Target  string `json:"target"`
	Count   int64  `json:"count"`
}

type searchRow struct {
	Term    string  `json:"term"`
	Count   int64   `json:"count"`
	Results float64 `json:"avg_results"`
}

// AnalyticsOverview GET /api/v1/storefront/sites/:id/analytics
func (h *StorefrontHandler) AnalyticsOverview(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	rg, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	totals, sa, ost := h.totalsFor(sf.ID, rg.From, rg.To, rg.Tzo)
	prev, _, _ := h.totalsFor(sf.ID, rg.Prev, rg.PrevTo, rg.Tzo)

	// Đơn theo phiên (để gắn doanh thu với nguồn / thiết bị / nước)
	ordersBySID := ost.CountBySID

	// Chuỗi theo ngày
	type dayRow struct {
		D        string
		Sessions int64
		Visitors int64
	}
	var days []dayRow
	h.DB.Raw(`SELECT to_char(started_at + make_interval(mins => ?::int), 'YYYY-MM-DD') AS d, COUNT(*) AS sessions, COUNT(DISTINCT visitor_id) AS visitors
FROM store_sessions WHERE storefront_id = ? AND started_at >= ? AND started_at < ? GROUP BY 1`, rg.Tzo, sf.ID, rg.From, rg.To).Scan(&days)
	sessByDay := map[string]dayRow{}
	for _, d := range days {
		sessByDay[d.D] = d
	}
	loc := time.FixedZone("u", rg.Tzo*60)
	type seriesPoint struct {
		Date     string `json:"date"`
		Sessions int64  `json:"sessions"`
		Visitors int64  `json:"visitors"`
		Orders   int64  `json:"orders"`
		Revenue  int64  `json:"revenue"`
	}
	series := make([]seriesPoint, 0, rg.Days)
	for i := 0; i < rg.Days; i++ {
		d := rg.From.In(loc).AddDate(0, 0, i).Format("2006-01-02")
		p := seriesPoint{Date: d, Sessions: sessByDay[d].Sessions, Visitors: sessByDay[d].Visitors}
		if op := ost.PerDay[d]; op != nil {
			p.Orders, p.Revenue = op.Orders, op.Revenue
		}
		series = append(series, p)
	}

	// Khung giờ (theo giờ của chủ shop)
	type hourRow struct {
		H int
		N int64
	}
	var hrs []hourRow
	h.DB.Raw(`SELECT EXTRACT(HOUR FROM started_at + make_interval(mins => ?::int))::int AS h, COUNT(*) AS n
FROM store_sessions WHERE storefront_id = ? AND started_at >= ? AND started_at < ? GROUP BY 1`, rg.Tzo, sf.ID, rg.From, rg.To).Scan(&hrs)
	hours := make([]int64, 24)
	for _, r := range hrs {
		if r.H >= 0 && r.H < 24 {
			hours[r.H] = r.N
		}
	}

	// Sản phẩm: lượt xem / thêm giỏ từ sự kiện, đơn từ đơn hàng
	type pe struct {
		ProductID uint
		Views     int64
		Adds      int64
	}
	var pes []pe
	h.DB.Raw(`SELECT product_id,
  COUNT(*) FILTER (WHERE type = 'product_view') AS views,
  COUNT(*) FILTER (WHERE type = 'add_to_cart') AS adds
FROM store_events WHERE storefront_id = ? AND created_at >= ? AND created_at < ? AND product_id > 0 AND type IN ('product_view','add_to_cart')
GROUP BY product_id ORDER BY views DESC LIMIT 200`, sf.ID, rg.From, rg.To).Scan(&pes)
	prod := map[uint]*topProduct{}
	for _, p := range pes {
		prod[p.ProductID] = &topProduct{ProductID: p.ProductID, Views: p.Views, Adds: p.Adds}
	}
	for pid, ps := range ost.Products {
		tp := prod[pid]
		if tp == nil {
			tp = &topProduct{ProductID: pid}
			prod[pid] = tp
		}
		tp.Name, tp.Orders, tp.Qty, tp.Revenue = ps.Name, ps.Orders, ps.Qty, ps.Revenue
	}
	if len(prod) > 0 {
		ids := make([]uint, 0, len(prod))
		for id := range prod {
			ids = append(ids, id)
		}
		var names []struct {
			ID   uint
			Name string
		}
		h.DB.Model(&models.Product{}).Select("id, name").Where("storefront_id = ? AND id IN ?", sf.ID, ids).Scan(&names)
		for _, n := range names {
			prod[n.ID].Name = n.Name
		}
	}
	products := make([]topProduct, 0, len(prod))
	for _, p := range prod {
		if p.Name == "" {
			p.Name = "#" + strconv.Itoa(int(p.ProductID))
		}
		products = append(products, *p)
	}
	sort.Slice(products, func(i, j int) bool {
		if products[i].Revenue != products[j].Revenue {
			return products[i].Revenue > products[j].Revenue
		}
		if products[i].Views != products[j].Views {
			return products[i].Views > products[j].Views
		}
		return products[i].ProductID < products[j].ProductID
	})
	if len(products) > 15 {
		products = products[:15]
	}

	// Click nhiều nhất, tìm kiếm, số lần thêm / bỏ giỏ...
	clicks := []clickRow{} // luôn là mảng (không phải null) để giao diện không lỗi khi trống
	h.DB.Raw(`SELECT COALESCE(section, '') AS section, label, COALESCE(target, '') AS target, COUNT(*) AS count
FROM store_events WHERE storefront_id = ? AND type = 'click' AND created_at >= ? AND created_at < ? AND label <> ''
GROUP BY section, label, target ORDER BY count DESC, label LIMIT 20`, sf.ID, rg.From, rg.To).Scan(&clicks)
	searches := []searchRow{}
	h.DB.Raw(`SELECT LOWER(label) AS term, COUNT(*) AS count, COALESCE(AVG(value), 0) AS results
FROM store_events WHERE storefront_id = ? AND type = 'search' AND created_at >= ? AND created_at < ? AND label <> ''
GROUP BY 1 ORDER BY count DESC, term LIMIT 15`, sf.ID, rg.From, rg.To).Scan(&searches)
	var evCounts []struct {
		Type string
		N    int64
	}
	h.DB.Raw(`SELECT type, COUNT(*) AS n FROM store_events WHERE storefront_id = ? AND created_at >= ? AND created_at < ?
AND type IN ('add_to_cart','remove_from_cart','cart_open','checkout_error','click','search') GROUP BY type`, sf.ID, rg.From, rg.To).Scan(&evCounts)
	events := map[string]int64{}
	for _, e := range evCounts {
		events[e.Type] = e.N
	}
	perPage := []struct {
		Path  string `json:"path"`
		Views int64  `json:"views"`
	}{}
	h.DB.Raw(`SELECT path, COUNT(*) AS views FROM store_events WHERE storefront_id = ? AND type = 'page_view' AND created_at >= ? AND created_at < ?
GROUP BY path ORDER BY views DESC, path LIMIT 12`, sf.ID, rg.From, rg.To).Scan(&perPage)

	methods := make([]gin.H, 0, len(ost.Methods))
	for m, s := range ost.Methods {
		methods = append(methods, gin.H{"method": m, "orders": s.Orders, "paid": s.Paid, "revenue": s.Revenue})
	}
	sort.Slice(methods, func(i, j int) bool { return methods[i]["orders"].(int64) > methods[j]["orders"].(int64) })

	funnel := []funnelRow{
		{"sessions", sa.Sessions}, {"product_view", sa.Product}, {"add_to_cart", sa.AddToCart}, {"checkout", sa.Checkout},
		{"info", sa.Info}, {"info_done", sa.InfoDone}, {"payment", sa.Payment}, {"submit", sa.Submit}, {"ordered", sa.Ordered},
	}

	c.JSON(http.StatusOK, gin.H{
		"range":     gin.H{"from": rg.From, "to": rg.To, "days": rg.Days, "tzo": rg.Tzo},
		"totals":    totals,
		"prev":      prev,
		"series":    series,
		"funnel":    funnel,
		"hours":     hours,
		"sources":   h.breakdown(sf.ID, "source", rg.From, rg.To, 12, ost.BySID, ordersBySID),
		"devices":   h.breakdown(sf.ID, "device", rg.From, rg.To, 5, ost.BySID, ordersBySID),
		"browsers":  h.breakdown(sf.ID, "browser", rg.From, rg.To, 8, ost.BySID, ordersBySID),
		"countries": h.breakdown(sf.ID, "country", rg.From, rg.To, 12, ost.BySID, ordersBySID),
		"landing":   h.breakdown(sf.ID, "landing", rg.From, rg.To, 10, ost.BySID, ordersBySID),
		"pages":     perPage,
		"products":  products,
		"clicks":    clicks,
		"searches":  searches,
		"events":    events,
		"methods":   methods,
	})
}

// ----------------------------------------------------------------- Xem trực tiếp

// liveStep gom (sự kiện gần nhất, trang) thành bước khách đang ở
func liveStep(lastEvent, lastPath string, orders int) string {
	switch lastEvent {
	case "order_placed":
		return "ordered"
	case "checkout_submit":
		return "submit"
	case "checkout_payment":
		return "payment"
	case "checkout_info_done":
		return "info_done"
	case "checkout_info":
		return "info"
	case "checkout_start":
		return "checkout"
	case "add_to_cart", "cart_open":
		return "cart"
	case "product_view":
		return "product"
	}
	switch {
	case strings.HasPrefix(lastPath, "/order/"):
		return "ordered"
	case lastPath == "/checkout":
		return "checkout"
	case strings.HasPrefix(lastPath, "/p/"):
		return "product"
	}
	return "browsing"
}

type cartLine struct {
	P  uint   `json:"p"`
	V  uint   `json:"v,omitempty"`
	Q  int    `json:"q"`
	Pr int64  `json:"pr"`
	Nm string `json:"nm"`
}

func cartLines(js string) []cartLine {
	if js == "" {
		return []cartLine{}
	}
	var out []cartLine
	if json.Unmarshal([]byte(js), &out) != nil {
		return []cartLine{}
	}
	return out
}

// AnalyticsLive GET /api/v1/storefront/sites/:id/analytics/live
func (h *StorefrontHandler) AnalyticsLive(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	since := time.Now().Add(-liveWindow)
	var list []StoreSession
	h.DB.Where("storefront_id = ? AND last_seen >= ?", sf.ID, since).Order("last_seen DESC").Limit(100).Find(&list)
	steps := map[string]int{}
	out := make([]gin.H, 0, len(list))
	for _, s := range list {
		step := liveStep(s.LastEvent, s.LastPath, s.Orders)
		steps[step]++
		sid := s.SID
		if len(sid) > 6 {
			sid = sid[:6]
		}
		out = append(out, gin.H{
			"id": s.ID, "sid": sid, "step": step, "stage": s.Stage, "last_event": s.LastEvent, "last_path": s.LastPath,
			"device": s.Device, "browser": s.Browser, "country": s.Country, "source": s.Source, "medium": s.Medium,
			"returning": s.Returning, "cart_value": s.CartValue, "cart_items": s.CartItems, "cart": cartLines(s.CartJSON),
			"orders": s.Orders, "started_at": s.StartedAt, "last_seen": s.LastSeen, "pages": s.Pages,
		})
	}
	c.JSON(http.StatusOK, gin.H{"online": len(list), "steps": steps, "sessions": out, "window_seconds": int(liveWindow.Seconds()), "now": time.Now()})
}

// AnalyticsAbandoned GET /api/v1/storefront/sites/:id/analytics/abandoned
// Phiên đã thêm giỏ / vào thanh toán nhưng không đặt hàng và im lặng quá 30 phút
func (h *StorefrontHandler) AnalyticsAbandoned(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	rg, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	var list []StoreSession
	h.DB.Where("storefront_id = ? AND started_at >= ? AND started_at < ? AND stage >= ? AND stage < ? AND orders = 0 AND last_seen < ?",
		sf.ID, rg.From, rg.To, stageAddToCart, stageOrderPlaced, time.Now().Add(-abandonAfter)).
		Order("last_seen DESC").Limit(100).Find(&list)
	out := make([]gin.H, 0, len(list))
	for _, s := range list {
		out = append(out, gin.H{
			"id": s.ID, "stage": s.Stage, "last_event": s.LastEvent, "device": s.Device, "country": s.Country,
			"source": s.Source, "medium": s.Medium, "returning": s.Returning, "cart_value": s.CartValue,
			"cart_items": s.CartItems, "cart": cartLines(s.CartJSON), "started_at": s.StartedAt, "last_seen": s.LastSeen,
		})
	}
	c.JSON(http.StatusOK, gin.H{"abandoned": out})
}
