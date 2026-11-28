package handlers

// Nhập sản phẩm từ link hoặc từ nội dung trang dán vào (AliExpress, 1688, Taobao, Shopify, website bất kỳ).
//
// Thực tế về nguồn hàng:
//   - 1688 / Taobao / API chi tiết của AliExpress chặn máy chủ bằng captcha trượt ("punish") → KHÔNG lấy được từ máy chủ.
//   - Trang AliExpress chỉ trả về tiêu đề + ảnh cho máy chủ (giá, mô tả, phân loại nạp bằng JavaScript sau đó).
//   - Website thường (Shopify, WooCommerce...) có JSON-LD / OpenGraph nên lấy được khá đủ.
// Vì vậy có 2 đường: dán link (lấy được gì lấy, báo rõ phần thiếu) và DÁN NỘI DUNG TRANG (Ctrl+A, Ctrl+C trên trang đang mở
// trong trình duyệt của bạn rồi Ctrl+V) — chạy với mọi nguồn vì trình duyệt của bạn đã vượt được captcha.
// AI (nếu đã cấu hình) đọc chữ của trang, bỏ phần rác, dịch sang tiếng Việt và tách tên / mô tả / giá / phân loại.

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

const (
	maxImportHTMLBytes = 3 << 20
	maxImportTextRunes = 12000 // chữ gửi cho AI
	maxImportImagesOut = 40
	maxImportVideosOut = 5
)

// importFetchClient: client tải trang nguồn (test thay bằng bản cho phép máy chủ thử ở 127.0.0.1)
var importFetchClient = func() *http.Client { return newSafeHTTPClient(25 * time.Second) }

type importOption struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type importFX struct {
	Currency   string  `json:"currency"`
	VNDPerUnit float64 `json:"vnd_per_unit"`
}

type productDraft struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"` // HTML đã lọc
	Images       []string       `json:"images"`
	Videos       []string       `json:"videos"`
	Price        float64        `json:"price"` // theo tiền tệ của trang nguồn; 0 = chưa nhận ra
	ComparePrice float64        `json:"compare_price"`
	Currency     string         `json:"currency"`
	SKU          string         `json:"sku,omitempty"`
	Options      []importOption `json:"options,omitempty"`
	SourceURL    string         `json:"source_url"`
	Source       string         `json:"source"` // aliexpress | 1688 | taobao | shopify | generic | paste
	FX           *importFX      `json:"fx,omitempty"`
	Warnings     []string       `json:"warnings"`
	Blocked      bool           `json:"blocked"`
	Translated   bool           `json:"translated"`
}

// ----------------------------------------------------------------- Phân tích HTML

type pageInfo struct {
	Title    string
	Desc     string
	H1       string
	Images   []string
	Videos   []string
	Text     string
	Price    float64
	Compare  float64
	Currency string
	SKU      string
}

func metaContent(meta map[string][]string, keys ...string) string {
	for _, k := range keys {
		for _, v := range meta[k] {
			if v = strings.TrimSpace(v); v != "" {
				return v
			}
		}
	}
	return ""
}

var (
	thumbSuffixRe = regexp.MustCompile(`(?i)(\.(?:jpe?g|png|webp|gif))_[0-9]+x[0-9]+[^/]*$`)
	sizeHintRe    = regexp.MustCompile(`(?i)[_\-.](\d{2,4})x(\d{2,4})(?:[a-z]\d+)?(?:[._]|$)`)
	noiseImgRe    = regexp.MustCompile(`(?i)(logo|icon|sprite|avatar|favicon|/flags?/|badge|payment|loading|placeholder|blank\.|pixel|spacer|captcha|qrcode|qr-code|\.svg(\?|$)|\.ico(\?|$)|1x1|/emoji|emoticon|star-?rating|tracking|beacon)`)
	videoURLRe    = regexp.MustCompile(`https?:\\?/\\?/[^"'\s<>\\]+?\.mp4[^"'\s<>\\]*`)
	imgPathListRe = regexp.MustCompile(`"imagePathList"\s*:\s*(\[[^\]]*\])`)
	srcsetItemRe  = regexp.MustCompile(`\s*([^\s,]+)(?:\s+(\d+)[wx])?\s*(?:,|$)`)
)

// canonicalImage bỏ hậu tố thu nhỏ kiểu AliExpress/Alibaba ("...jpg_80x80.jpg", "...jpg_960x960q75.jpg_.avif") để lấy ảnh gốc
func canonicalImage(u string) string {
	if m := thumbSuffixRe.FindStringSubmatchIndex(u); m != nil && (strings.Contains(u, "alicdn") || strings.Contains(u, "aliexpress") || strings.Contains(u, "alibaba")) {
		return u[:m[3]]
	}
	return u
}

func normalizeImageURL(raw string, base *url.URL) string {
	raw = strings.TrimSpace(html.UnescapeString(raw))
	raw = strings.ReplaceAll(raw, `\/`, "/")
	if raw == "" || strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "blob:") || strings.HasPrefix(raw, "javascript:") {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if !u.IsAbs() {
		if base == nil {
			return ""
		}
		u = base.ResolveReference(u)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

// sizeHint đọc kích thước ghi trong URL ảnh (vd. "..._80x80.jpg"); ok=false nếu không có
func sizeHint(u string) (w, h int, ok bool) {
	m := sizeHintRe.FindStringSubmatch(u)
	if m == nil {
		return 0, 0, false
	}
	w, _ = strconv.Atoi(m[1])
	h, _ = strconv.Atoi(m[2])
	return w, h, true
}

func imageKey(u string) string {
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://"))
}

func bestFromSrcset(set string) string {
	best, bestW := "", -1
	for _, m := range srcsetItemRe.FindAllStringSubmatch(set, -1) {
		w := 0
		if m[2] != "" {
			w, _ = strconv.Atoi(m[2])
		}
		if m[1] != "" && w >= bestW {
			best, bestW = m[1], w
		}
	}
	return best
}

type imgCollector struct {
	base *url.URL
	seen map[string]bool
	list []string
}

// add thêm một ảnh. force = ảnh chắc chắn của sản phẩm (JSON-LD, og:image, danh sách ảnh của trang) nên không lọc.
// Với ảnh trong thân trang: ảnh thu nhỏ rất bé (≤100px — dải ảnh nhỏ của chính sản phẩm) được đổi về bản gốc;
// ảnh cỡ nhỏ-vừa (≤250px — sản phẩm gợi ý, icon...) bị bỏ; icon / logo / cờ / mã QR... theo tên file cũng bỏ.
func (c *imgCollector) add(raw string, force bool) {
	u := normalizeImageURL(raw, c.base)
	if u == "" {
		return
	}
	if !force && noiseImgRe.MatchString(strings.ToLower(u)) {
		return
	}
	if w, h, ok := sizeHint(u); ok {
		switch {
		case force || (w <= 100 && h <= 100):
			u = canonicalImage(u)
		case w <= 250 && h <= 250:
			return
		}
	}
	k := imageKey(u)
	if c.seen[k] || len(c.list) >= maxImportImagesOut {
		return
	}
	c.seen[k] = true
	c.list = append(c.list, u)
}

var blockTags = map[string]bool{
	"p": true, "div": true, "br": true, "li": true, "tr": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"section": true, "article": true, "ul": true, "ol": true, "table": true, "header": true, "footer": true, "dt": true, "dd": true, "blockquote": true,
}
var skipTextTags = map[string]bool{"script": true, "style": true, "noscript": true, "svg": true, "head": true, "template": true, "iframe": true, "canvas": true, "select": true, "option": true, "button": true}

func firstText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			sb.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(strings.Join(strings.Fields(sb.String()), " "))
}

// parsePage đọc HTML (cả trang hoặc đoạn dán từ clipboard) thành thông tin sản phẩm thô.
func parsePage(htmlStr string, base *url.URL) pageInfo {
	var info pageInfo
	root, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return info
	}
	meta := map[string][]string{}
	var ldBlocks []string
	imgs := &imgCollector{base: base, seen: map[string]bool{}}
	var bodyImgs, videos []string
	var text strings.Builder
	var title string

	var walk func(n *html.Node, inVideo, noText bool)
	walk = func(n *html.Node, inVideo, noText bool) {
		if n.Type == html.TextNode {
			if !noText {
				text.WriteString(n.Data)
			}
			return
		}
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, inVideo, noText)
			}
			return
		}
		tag := strings.ToLower(n.Data)
		switch tag {
		case "title":
			if title == "" {
				title = firstText(n)
			}
		case "meta":
			key := strings.ToLower(attrOf(n, "property"))
			if key == "" {
				key = strings.ToLower(attrOf(n, "name"))
			}
			if key == "" {
				key = strings.ToLower(attrOf(n, "itemprop"))
			}
			if key != "" {
				meta[key] = append(meta[key], attrOf(n, "content"))
			}
		case "script":
			if strings.Contains(strings.ToLower(attrOf(n, "type")), "ld+json") {
				ldBlocks = append(ldBlocks, firstChildText(n))
			}
		case "h1":
			if info.H1 == "" {
				info.H1 = firstText(n)
			}
		case "img":
			for _, k := range []string{"src", "data-src", "data-original", "data-lazy-src", "data-lazy", "data-image"} {
				if v := attrOf(n, k); v != "" {
					bodyImgs = append(bodyImgs, v)
					break
				}
			}
			for _, k := range []string{"srcset", "data-srcset"} {
				if v := bestFromSrcset(attrOf(n, k)); v != "" {
					bodyImgs = append(bodyImgs, v)
					break
				}
			}
		case "video":
			if v := attrOf(n, "src"); v != "" {
				videos = append(videos, v)
			}
			if p := attrOf(n, "poster"); p != "" {
				bodyImgs = append(bodyImgs, p)
			}
			inVideo = true
		case "source":
			if inVideo {
				if v := attrOf(n, "src"); v != "" {
					videos = append(videos, v)
				}
			} else if v := bestFromSrcset(attrOf(n, "srcset")); v != "" {
				bodyImgs = append(bodyImgs, v)
			}
		}
		childNoText := noText || skipTextTags[tag] // vẫn duyệt tiếp (meta, title, JSON-LD nằm trong head) nhưng không lấy chữ
		if blockTags[tag] && !childNoText {
			text.WriteString("\n")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inVideo, childNoText)
		}
		if blockTags[tag] && !childNoText {
			text.WriteString("\n")
		}
	}
	walk(root, false, false)

	// JSON-LD Product
	for _, raw := range ldBlocks {
		for _, p := range ldProducts(raw) {
			if info.Title == "" {
				info.Title = strAny(p["name"])
			}
			if info.Desc == "" {
				info.Desc = strAny(p["description"])
			}
			if info.SKU == "" {
				info.SKU = strAny(p["sku"])
			}
			for _, im := range strListAny(p["image"]) {
				imgs.add(im, true)
			}
			if vd, ok := p["video"].(map[string]any); ok {
				videos = append(videos, strAny(vd["contentUrl"]))
			}
			price, low, cur := ldOffer(p["offers"])
			if info.Price == 0 {
				info.Price, info.Currency = firstNonZero(price, low), cur
			}
		}
	}
	if info.Title == "" {
		info.Title = metaContent(meta, "og:title", "twitter:title")
	}
	if info.Desc == "" {
		info.Desc = metaContent(meta, "og:description", "description", "twitter:description")
	}
	if info.Price == 0 {
		if v, err := strconv.ParseFloat(strings.ReplaceAll(metaContent(meta, "product:price:amount", "og:price:amount", "price"), ",", "."), 64); err == nil && v > 0 {
			info.Price = v
			info.Currency = strings.ToUpper(metaContent(meta, "product:price:currency", "og:price:currency", "pricecurrency"))
		}
	}
	for _, k := range []string{"og:image", "og:image:url", "og:image:secure_url", "twitter:image", "twitter:image:src"} {
		for _, v := range meta[k] {
			imgs.add(v, true)
		}
	}
	for _, v := range meta["og:video"] {
		videos = append(videos, v)
	}
	for _, v := range meta["og:video:url"] {
		videos = append(videos, v)
	}
	for _, v := range meta["og:video:secure_url"] {
		videos = append(videos, v)
	}
	// AliExpress (trang nạp bằng JS): danh sách ảnh sản phẩm nằm trong JSON "imagePathList"
	if m := imgPathListRe.FindStringSubmatch(htmlStr); m != nil {
		var list []string
		if json.Unmarshal([]byte(m[1]), &list) == nil {
			for _, v := range list {
				imgs.add(v, true)
			}
		}
	}
	for _, v := range bodyImgs {
		imgs.add(v, false)
	}
	for _, m := range videoURLRe.FindAllString(htmlStr, 20) {
		videos = append(videos, strings.ReplaceAll(m, `\/`, "/"))
	}

	info.Title = firstNonEmpty(info.Title, info.H1, title)
	info.Images = imgs.list
	info.Videos = cleanVideoURLs(videos, base)
	info.Text = tidyText(text.String())
	if info.Price == 0 {
		info.Price, info.Currency = findPriceInText(info.Text)
	}
	return info
}

func firstChildText(n *html.Node) string {
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
	}
	return sb.String()
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstNonZero(v ...float64) float64 {
	for _, x := range v {
		if x > 0 {
			return x
		}
	}
	return 0
}

func tidyText(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, ln := range lines {
		ln = strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(ln, " ", " ")), " "))
		if ln == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, ln)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func cleanVideoURLs(in []string, base *url.URL) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(html.UnescapeString(v))
		if v == "" || strings.HasPrefix(v, "blob:") || strings.HasPrefix(v, "data:") {
			continue
		}
		if strings.HasPrefix(v, "//") {
			v = "https:" + v
		}
		u, err := url.Parse(v)
		if err != nil {
			continue
		}
		if !u.IsAbs() && base != nil {
			u = base.ResolveReference(u)
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || seen[u.String()] || len(out) >= maxImportVideosOut {
			continue
		}
		seen[u.String()] = true
		out = append(out, u.String())
	}
	return out
}

// ----------------------------------------------------------------- JSON-LD

func ldProducts(raw string) []map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return nil
	}
	var out []map[string]any
	var visit func(x any, depth int)
	visit = func(x any, depth int) {
		if depth > 5 {
			return
		}
		switch t := x.(type) {
		case []any:
			for _, e := range t {
				visit(e, depth+1)
			}
		case map[string]any:
			if isProductType(t["@type"]) {
				out = append(out, t)
			}
			if g, ok := t["@graph"]; ok {
				visit(g, depth+1)
			}
		}
	}
	visit(v, 0)
	return out
}

func isProductType(t any) bool {
	switch v := t.(type) {
	case string:
		return strings.EqualFold(v, "Product") || strings.EqualFold(v, "ProductGroup")
	case []any:
		for _, e := range v {
			if isProductType(e) {
				return true
			}
		}
	}
	return false
}

func strAny(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return ""
}

func strListAny(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, e := range t {
			out = append(out, strListAny(e)...)
		}
		return out
	case map[string]any:
		return []string{strAny(t["url"])}
	}
	return nil
}

func numAny(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		f, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(t), ",", "."), 64)
		return f
	}
	return 0
}

func ldOffer(v any) (price, low float64, cur string) {
	var one func(o map[string]any)
	one = func(o map[string]any) {
		if price == 0 {
			price = numAny(o["price"])
		}
		if low == 0 {
			low = numAny(o["lowPrice"])
		}
		if cur == "" {
			cur = strings.ToUpper(strAny(o["priceCurrency"]))
		}
		if ps, ok := o["priceSpecification"].(map[string]any); ok && price == 0 {
			price = numAny(ps["price"])
			if cur == "" {
				cur = strings.ToUpper(strAny(ps["priceCurrency"]))
			}
		}
	}
	switch t := v.(type) {
	case map[string]any:
		one(t)
	case []any:
		for _, e := range t {
			if m, ok := e.(map[string]any); ok {
				one(m)
			}
		}
	}
	return
}

// ----------------------------------------------------------------- Giá trong văn bản

var priceTextRes = []struct {
	re  *regexp.Regexp
	cur string
}{
	{regexp.MustCompile(`(?i)(?:US\s?\$|USD\s?|\$)\s?(\d[\d.,]*)`), "USD"},
	{regexp.MustCompile(`(?i)(\d[\d.,]*)\s?(?:₫|đ|VND|vnđ)`), "VND"},
	{regexp.MustCompile(`(?i)(?:¥|￥|RMB\s?|CNY\s?)\s?(\d[\d.,]*)`), "CNY"},
	{regexp.MustCompile(`(\d[\d.,]*)\s?(?:元|RMB|CNY)`), "CNY"},
	{regexp.MustCompile(`€\s?(\d[\d.,]*)`), "EUR"},
	{regexp.MustCompile(`£\s?(\d[\d.,]*)`), "GBP"},
}

// parseAmount hiểu cả "1.234,56", "1,234.56", "259.000", "12,5"
func parseAmount(s, cur string) float64 {
	s = strings.Trim(strings.TrimSpace(s), ".,")
	if s == "" {
		return 0
	}
	dot, comma := strings.LastIndex(s, "."), strings.LastIndex(s, ",")
	switch {
	case dot >= 0 && comma >= 0:
		if comma > dot {
			s = strings.ReplaceAll(strings.ReplaceAll(s, ".", ""), ",", ".")
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	case comma >= 0:
		if after := len(s) - comma - 1; after == 3 || cur == "VND" {
			s = strings.ReplaceAll(s, ",", "")
		} else {
			s = strings.ReplaceAll(s, ",", ".")
		}
	case dot >= 0:
		if after := len(s) - dot - 1; (after == 3 && (cur == "VND" || strings.Count(s, ".") > 1)) || strings.Count(s, ".") > 1 {
			s = strings.ReplaceAll(s, ".", "")
		} else if after == 3 && cur == "VND" {
			s = strings.ReplaceAll(s, ".", "")
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0
	}
	return f
}

// findPriceInText lấy giá xuất hiện SỚM NHẤT trong văn bản trang (giá hiển thị trước thường là giá chính)
func findPriceInText(text string) (float64, string) {
	if len(text) > 30000 {
		text = text[:30000]
	}
	bestPos, best, bestCur := -1, 0.0, ""
	for _, pr := range priceTextRes {
		for _, m := range pr.re.FindAllStringSubmatchIndex(text, 6) {
			amt := parseAmount(text[m[2]:m[3]], pr.cur)
			if amt <= 0 || amt > 1e10 {
				continue
			}
			if bestPos < 0 || m[0] < bestPos {
				bestPos, best, bestCur = m[0], amt, pr.cur
			}
			break
		}
	}
	return best, bestCur
}

// ----------------------------------------------------------------- Nguồn & chặn

func detectSource(u *url.URL) string {
	if u == nil {
		return "paste"
	}
	h := strings.ToLower(u.Hostname())
	switch {
	case strings.Contains(h, "aliexpress."):
		return "aliexpress"
	case strings.Contains(h, "1688.com"):
		return "1688"
	case strings.Contains(h, "taobao.com") || strings.Contains(h, "tmall.com"):
		return "taobao"
	case strings.Contains(u.Path, "/products/"):
		return "shopify"
	}
	return "generic"
}

// detectBlocked nhận ra trang chặn bot / yêu cầu xác thực thay vì trang sản phẩm
func detectBlocked(body string, status int) string {
	l := strings.ToLower(body)
	if len(l) > 60000 {
		l = l[:60000]
	}
	switch {
	case strings.Contains(l, "_____tmd_____") || strings.Contains(l, "x5secdata") || strings.Contains(l, "/punish"):
		return "Trang nguồn chặn máy chủ bằng xác thực trượt (captcha). Hãy dùng cách “Dán nội dung trang”."
	case strings.Contains(l, "captcha interception") || strings.Contains(l, "unusual traffic") || strings.Contains(l, "are you a robot") || strings.Contains(l, "access denied"):
		return "Trang nguồn nghi ngờ truy cập tự động và chặn lại. Hãy dùng cách “Dán nội dung trang”."
	case status == http.StatusForbidden || status == http.StatusTooManyRequests || status == http.StatusUnauthorized:
		return fmt.Sprintf("Trang nguồn từ chối truy cập (HTTP %d). Hãy dùng cách “Dán nội dung trang”.", status)
	}
	return ""
}

// ----------------------------------------------------------------- Shopify (.js)

var shopifyHandleRe = regexp.MustCompile(`^(.*?/products/[^/?#]+)`)

func shopifyJSURL(u *url.URL) string {
	m := shopifyHandleRe.FindStringSubmatch(u.Path)
	if m == nil {
		return ""
	}
	c := *u
	c.Path = strings.TrimSuffix(m[1], ".js") + ".js"
	c.RawQuery, c.Fragment = "", ""
	return c.String()
}

func applyShopifyJS(body []byte, base *url.URL, d *productDraft) bool {
	var p struct {
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		Price        float64  `json:"price"` // đơn vị nhỏ nhất (cent)
		ComparePrice float64  `json:"compare_at_price"`
		Images       []string `json:"images"`
		Options      []struct {
			Name   string   `json:"name"`
			Values []string `json:"values"`
		} `json:"options"`
		Variants []struct {
			SKU string `json:"sku"`
		} `json:"variants"`
	}
	if json.Unmarshal(body, &p) != nil || p.Title == "" {
		return false
	}
	d.Name = p.Title
	d.Description = sanitizeRichHTML(p.Description)
	d.Price, d.ComparePrice = p.Price/100, p.ComparePrice/100
	imgs := &imgCollector{base: base, seen: map[string]bool{}}
	for _, im := range p.Images {
		imgs.add(im, true)
	}
	d.Images = imgs.list
	for _, o := range p.Options {
		if o.Name != "" && !(len(o.Values) == 1 && o.Values[0] == "Default Title") {
			d.Options = append(d.Options, importOption{Name: o.Name, Values: o.Values})
		}
	}
	if len(p.Variants) > 0 {
		d.SKU = p.Variants[0].SKU
	}
	d.Source = "shopify"
	return true
}

// ----------------------------------------------------------------- AI

type aiProduct struct {
	Name         string         `json:"name"`
	DescriptionH string         `json:"description_html"`
	Price        *float64       `json:"price"`
	ComparePrice *float64       `json:"compare_price"`
	Currency     string         `json:"currency"`
	Options      []importOption `json:"options"`
}

func (h *StorefrontHandler) aiStructureProduct(info pageInfo, langName string) (aiProduct, error) {
	var res aiProduct
	text := info.Text
	if r := []rune(text); len(r) > maxImportTextRunes {
		text = string(r[:maxImportTextRunes])
	}
	hints, _ := json.Marshal(map[string]any{"title": info.Title, "description": truncRunes(info.Desc, 400), "price": info.Price, "currency": info.Currency})
	system := "You extract e-commerce product data from the text of a product web page (copied from AliExpress, 1688, Taobao, Shopify or a similar shop). " +
		"The text is noisy: it may contain menus, reviews, recommendations, shipping, seller and footer text — ignore everything that is not about the MAIN product. " +
		"Return ONLY a JSON object: {\"name\": string, \"description_html\": string, \"price\": number|null, \"compare_price\": number|null, \"currency\": string|null, \"options\": [{\"name\": string, \"values\": [string]}]}. " +
		"Rules: write name, description_html and options in " + langName + " (translate faithfully; keep brand names, model numbers, units and numbers). " +
		"name: a concise product title (max 120 characters), no promotional spam, no emoji. " +
		"description_html: a clean description using ONLY <p>, <ul>, <li>, <strong>, <h3>, <br>; bullet the key features and specifications; never invent facts; leave out shipping, seller, return-policy and review boilerplate. " +
		"price: the current selling price of the main product as a plain number in the currency shown on the page (if a range, use the lowest); compare_price: the struck-through original price if shown, else null; " +
		"currency: the ISO 4217 code (USD, CNY, VND, EUR...) or null if unknown (¥/￥/元 means CNY). " +
		"options: variant groups such as Color, Size or Model with their values (max 3 groups, max 30 values each), else []."
	user := "HINTS (from page metadata, may be incomplete): " + string(hints) + "\n\nPAGE TEXT:\n" + text
	reply, err := h.AI.callUniversalChatAI(system, user, true)
	if err != nil {
		return res, err
	}
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
	if err := json.Unmarshal([]byte(reply), &res); err != nil {
		return res, fmt.Errorf("AI trả về dữ liệu không đọc được")
	}
	return res, nil
}

func cleanOptions(in []importOption) []importOption {
	out := []importOption{}
	for _, o := range in {
		name := truncRunes(strings.TrimSpace(o.Name), 40)
		if name == "" || len(out) >= 3 {
			continue
		}
		seen := map[string]bool{}
		vals := []string{}
		for _, v := range o.Values {
			v = truncRunes(strings.TrimSpace(v), 60)
			if v != "" && !seen[strings.ToLower(v)] && len(vals) < 30 {
				seen[strings.ToLower(v)] = true
				vals = append(vals, v)
			}
		}
		if len(vals) > 0 {
			out = append(out, importOption{Name: name, Values: vals})
		}
	}
	return out
}

// ----------------------------------------------------------------- Giới hạn tần suất

var importLimiter = struct {
	sync.Mutex
	hits map[uint][]time.Time
}{hits: map[uint][]time.Time{}}

func allowImport(uid uint) bool {
	importLimiter.Lock()
	defer importLimiter.Unlock()
	now := time.Now()
	recent := importLimiter.hits[uid][:0]
	for _, t := range importLimiter.hits[uid] {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 15 {
		importLimiter.hits[uid] = recent
		return false
	}
	importLimiter.hits[uid] = append(recent, now)
	return true
}

// ----------------------------------------------------------------- Handler

// ImportProduct POST /api/v1/storefront/sites/:id/products/import
// {url?, html?, text?, lang?, translate?} → {draft}. Chỉ trả bản nháp, KHÔNG tạo sản phẩm: giao diện cho chủ shop xem lại.
func (h *StorefrontHandler) ImportProduct(c *gin.Context) {
	if _, ok := h.loadOwned(c); !ok {
		return
	}
	uid, _ := extractUserID(c)
	if !allowImport(uid) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Bạn thao tác quá nhanh, vui lòng thử lại sau ít phút."})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportHTMLBytes+(256<<10))
	var req struct {
		URL       string `json:"url"`
		HTML      string `json:"html"`
		Text      string `json:"text"`
		Lang      string `json:"lang"`
		Translate *bool  `json:"translate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ hoặc quá lớn."})
		return
	}
	req.URL, req.HTML, req.Text = strings.TrimSpace(req.URL), strings.TrimSpace(req.HTML), strings.TrimSpace(req.Text)
	if req.URL == "" && req.HTML == "" && req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hãy dán link sản phẩm hoặc nội dung trang."})
		return
	}
	langName := "Vietnamese"
	if n, ok := uiLangNames[strings.ToLower(req.Lang)]; ok && req.Lang != "" {
		langName = n
	}
	wantAI := h.AI != nil && (req.Translate == nil || *req.Translate)

	d := &productDraft{Images: []string{}, Videos: []string{}, Warnings: []string{}}
	var base *url.URL
	var info pageInfo

	switch {
	case req.HTML != "" || req.Text != "":
		// Nội dung dán từ trình duyệt của người dùng (đã vượt được captcha)
		d.Source = "paste"
		if req.URL != "" {
			if u, err := url.Parse(req.URL); err == nil && u.Host != "" {
				base = u
				d.SourceURL = req.URL
				if s := detectSource(u); s != "generic" && s != "shopify" {
					d.Source = s
				}
			}
		}
		if req.HTML != "" {
			if len(req.HTML) > maxImportHTMLBytes {
				req.HTML = req.HTML[:maxImportHTMLBytes]
			}
			info = parsePage(req.HTML, base)
		}
		if info.Text == "" {
			info.Text = tidyText(req.Text)
		}
	default:
		u, err := url.Parse(req.URL)
		if err != nil || checkFetchURL(u) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Link không hợp lệ."})
			return
		}
		base = u
		d.SourceURL = u.String()
		d.Source = detectSource(u)
		client := importFetchClient()
		body, _, final, err := fetchLimited(client, u.String(), maxImportHTMLBytes, map[string]string{
			"Accept": "text/html,application/xhtml+xml,*/*;q=0.8", "Accept-Language": "vi,en;q=0.9",
		})
		if err != nil {
			d.Blocked = true
			d.Warnings = append(d.Warnings, "Không mở được link: "+shortErr(err)+". Hãy dùng cách “Dán nội dung trang”.")
			c.JSON(http.StatusOK, gin.H{"draft": d})
			return
		}
		if fu, err := url.Parse(final); err == nil && fu.Host != "" {
			base = fu
		}
		info = parsePage(string(body), base)
		// Chỉ coi là bị chặn khi có dấu hiệu chặn VÀ không đọc được gì về sản phẩm (tránh nhận nhầm trang thật)
		if msg := detectBlocked(string(body), 200); msg != "" && info.Title == "" && len(info.Images) == 0 {
			d.Blocked = true
			d.Warnings = append(d.Warnings, msg)
			c.JSON(http.StatusOK, gin.H{"draft": d})
			return
		}
		// Shopify: lấy đủ dữ liệu từ <sản phẩm>.js
		if js := shopifyJSURL(u); js != "" && info.Title != "" && d.Source != "aliexpress" {
			if jb, _, _, err := fetchLimited(client, js, 2<<20, map[string]string{"Accept": "application/json"}); err == nil && applyShopifyJS(jb, base, d) {
				info.Images = append(append([]string{}, d.Images...), info.Images...)
			}
		}
		if d.Source == "aliexpress" && info.Price == 0 && info.Text == "" {
			d.Warnings = append(d.Warnings, "AliExpress chỉ cho máy chủ lấy tiêu đề và ảnh (giá, mô tả, phân loại nạp bằng JavaScript). Hãy nhập giá thủ công hoặc dùng “Dán nội dung trang” để lấy đầy đủ.")
		}
	}

	// Hợp nhất kết quả phân tích thô
	if d.Name == "" {
		d.Name = truncRunes(info.Title, 255)
	}
	if d.Description == "" && info.Desc != "" && !strings.HasPrefix(info.Desc, "Smarter Shopping") {
		d.Description = sanitizeRichHTML(plainToRichHTML(info.Desc))
	}
	if d.Price == 0 {
		d.Price, d.Currency = info.Price, info.Currency
	}
	if d.SKU == "" {
		d.SKU = truncRunes(info.SKU, 100)
	}
	d.Images = dedupeStrings(append(d.Images, info.Images...), maxImportImagesOut)
	d.Videos = dedupeStrings(append(d.Videos, info.Videos...), maxImportVideosOut)

	// AI: đọc chữ của trang, bỏ phần rác, dịch, tách tên / mô tả / giá / phân loại
	if wantAI && (len([]rune(info.Text)) >= 80 || info.Title != "") {
		ai, err := h.aiStructureProduct(info, langName)
		switch {
		case err != nil:
			log.Printf("[import] AI lỗi: %v", err)
			d.Warnings = append(d.Warnings, "AI chưa xử lý được nội dung (thường do giới hạn lượt gọi hoặc chưa cấu hình khoá AI) — đã dùng dữ liệu đọc trực tiếp từ trang.")
		default:
			d.Translated = true
			if n := truncRunes(strings.TrimSpace(ai.Name), 255); n != "" {
				d.Name = n
			}
			if desc := sanitizeRichHTML(ai.DescriptionH); desc != "" {
				d.Description = desc
			}
			if ai.Price != nil && *ai.Price > 0 {
				d.Price = *ai.Price
				if cur := strings.ToUpper(strings.TrimSpace(ai.Currency)); len(cur) == 3 {
					d.Currency = cur
				}
			}
			if ai.ComparePrice != nil && *ai.ComparePrice > d.Price {
				d.ComparePrice = *ai.ComparePrice
			}
			if len(ai.Options) > 0 {
				d.Options = cleanOptions(ai.Options)
			}
		}
	}
	if !d.Translated && d.Description == "" && info.Text != "" && d.Source == "paste" {
		// Không có AI: chừa lại đoạn đầu của chữ trang để người dùng tự chỉnh
		d.Description = sanitizeRichHTML(plainToRichHTML(truncRunes(info.Text, 2500)))
		d.Warnings = append(d.Warnings, "Chưa dùng được AI nên mô tả là chữ thô của trang (chưa dịch / lọc). Bạn nên chỉnh lại.")
	}
	d.Options = cleanOptions(d.Options)
	if d.Name == "" && len(d.Images) == 0 {
		d.Blocked = true
		d.Warnings = append(d.Warnings, "Không đọc được dữ liệu sản phẩm từ nội dung này. Hãy thử “Dán nội dung trang” khi trang đã hiện đầy đủ (cuộn xuống hết để ảnh tải xong).")
	}
	if d.Price == 0 {
		d.Currency = ""
	}
	if d.Currency == "" && d.Price > 0 {
		d.Currency = "USD"
	}
	// Tỉ giá để đổi sang VND
	if d.Price > 0 && d.Currency != "VND" {
		if rates, _, err := h.fetchRates(); err == nil {
			if r := rates[d.Currency]; r > 0 {
				d.FX = &importFX{Currency: d.Currency, VNDPerUnit: 1 / r}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"draft": d})
}

func dedupeStrings(in []string, max int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		k := imageKey(s)
		if s == "" || seen[k] || len(out) >= max {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}
