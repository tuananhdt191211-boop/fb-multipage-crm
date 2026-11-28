package handlers

// Bán hàng quốc tế: nhiều ngôn ngữ + nhiều tiền tệ cho website bán hàng.
// - i18n JSON (nháp / xuất bản): thị trường đang bật + bản dịch theo ngôn ngữ
//   {"markets":["us","de"],"default":"vn","tr":{"de":{"b":{...},"p":{...},"ui":{...}}}}
// - Tỉ giá: ExchangeRate-API (miễn phí, cập nhật hằng ngày), gốc USD rồi quy ra VND, đệm 6 giờ.
// - Dịch bằng AI: chủ shop gửi các chuỗi tiếng Việt, AI dịch theo từng ngôn ngữ; kết quả đệm
//   trong bảng translation_caches (dùng chung mọi website) để không dịch lại câu đã dịch.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Ngôn ngữ hỗ trợ (khớp web-vue/src/lib/markets.ts)
var i18nLangNames = map[string]string{
	"vi": "Vietnamese", "en": "English", "de": "German", "fr": "French", "it": "Italian", "es": "Spanish",
	"nl": "Dutch", "pt": "Portuguese", "pl": "Polish", "cs": "Czech", "sk": "Slovak", "sv": "Swedish",
	"da": "Danish", "fi": "Finnish", "nb": "Norwegian (Bokmål)", "el": "Greek", "hu": "Hungarian",
	"ro": "Romanian", "bg": "Bulgarian", "hr": "Croatian", "sl": "Slovenian", "et": "Estonian",
	"lv": "Latvian", "lt": "Lithuanian", "uk": "Ukrainian",
}

var marketCodeRe = regexp.MustCompile(`^[a-z]{2}(-[a-z]{2})?$`)

const maxI18nBytes = 3 << 20 // 3MB

// sanitizeI18nJSON: giữ đúng cấu trúc, chỉ chuỗi; giới hạn kích thước
func sanitizeI18nJSON(raw string) string {
	if raw == "" || len(raw) > maxI18nBytes {
		return ""
	}
	var in struct {
		Markets []string                             `json:"markets"`
		Default string                               `json:"default"`
		Tr      map[string]map[string]map[string]any `json:"tr"`
	}
	if json.Unmarshal([]byte(raw), &in) != nil {
		return ""
	}
	out := map[string]any{}
	seen := map[string]bool{}
	markets := []string{}
	for _, m := range in.Markets {
		if marketCodeRe.MatchString(m) && !seen[m] && len(markets) < 60 {
			seen[m] = true
			markets = append(markets, m)
		}
	}
	out["markets"] = markets
	if marketCodeRe.MatchString(in.Default) {
		out["default"] = in.Default
	}
	tr := map[string]any{}
	for lang, parts := range in.Tr {
		if _, ok := i18nLangNames[lang]; !ok || lang == "vi" {
			continue
		}
		cleanParts := map[string]any{}
		for _, part := range []string{"b", "p", "ui"} {
			src, ok := parts[part]
			if !ok {
				continue
			}
			cleanParts[part] = cleanStringTree(src, 0)
		}
		tr[lang] = cleanParts
	}
	out["tr"] = tr
	b, _ := json.Marshal(out)
	return string(b)
}

// cleanStringTree giữ lại map lồng nhau (tối đa 3 tầng) có giá trị chuỗi
func cleanStringTree(v any, depth int) any {
	switch t := v.(type) {
	case string:
		if len(t) > 5000 {
			return t[:5000]
		}
		return t
	case map[string]any:
		if depth > 3 {
			return map[string]any{}
		}
		out := map[string]any{}
		for k, vv := range t {
			if len(k) > 200 {
				continue
			}
			if c := cleanStringTree(vv, depth+1); c != nil {
				out[k] = c
			}
		}
		return out
	}
	return nil
}

// ----------------------------------------------------------------- Tỉ giá

var rateCache = struct {
	sync.Mutex
	rates   map[string]float64 // 1 VND = ? đơn vị ngoại tệ
	updated time.Time
	fetched time.Time
}{}

func (h *StorefrontHandler) fetchRates() (map[string]float64, time.Time, error) {
	rateCache.Lock()
	defer rateCache.Unlock()
	if rateCache.rates != nil && time.Since(rateCache.fetched) < 6*time.Hour {
		return rateCache.rates, rateCache.updated, nil
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://open.er-api.com/v6/latest/USD")
	if err == nil {
		defer resp.Body.Close()
		var data struct {
			Result string             `json:"result"`
			Unix   int64              `json:"time_last_update_unix"`
			Rates  map[string]float64 `json:"rates"`
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if json.Unmarshal(body, &data) == nil && data.Result == "success" && data.Rates["VND"] > 0 {
			vnd := data.Rates["VND"]
			rates := map[string]float64{}
			for code, r := range data.Rates {
				if r > 0 {
					rates[code] = r / vnd
				}
			}
			rateCache.rates = rates
			rateCache.updated = time.Unix(data.Unix, 0)
			rateCache.fetched = time.Now()
			return rates, rateCache.updated, nil
		}
		err = fmt.Errorf("nguồn tỉ giá trả dữ liệu không hợp lệ (HTTP %d)", resp.StatusCode)
	}
	if rateCache.rates != nil { // lỗi mạng: dùng tạm tỉ giá cũ
		return rateCache.rates, rateCache.updated, nil
	}
	return nil, time.Time{}, err
}

// PublicRates GET /api/v1/storefront/public/rates
func (h *StorefrontHandler) PublicRates(c *gin.Context) {
	rates, updated, err := h.fetchRates()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Chưa lấy được tỉ giá, thử lại sau."})
		return
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, gin.H{"base": "VND", "rates": rates, "updated_at": updated, "source": "https://www.exchangerate-api.com"})
}

// ----------------------------------------------------------------- Dịch bằng AI

type TranslationCache struct {
	ID        uint   `gorm:"primaryKey"`
	Lang      string `gorm:"size:8;not null;uniqueIndex:idx_tr_cache"`
	SrcHash   string `gorm:"size:64;not null;uniqueIndex:idx_tr_cache"`
	Text      string `gorm:"type:text;not null"`
	CreatedAt time.Time
}

func migrateTranslationCache(db *gorm.DB) {
	_ = db.AutoMigrate(&TranslationCache{})
}

func srcHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TranslateStrings POST /api/v1/storefront/sites/:id/translate {lang, texts:{key: "tiếng Việt"}}
// Trả {translations:{key: "bản dịch"}}. Câu đã dịch trước đây lấy từ bộ đệm.
func (h *StorefrontHandler) TranslateStrings(c *gin.Context) {
	if _, ok := h.loadOwned(c); !ok {
		return
	}
	var req struct {
		Lang  string            `json:"lang"`
		Texts map[string]string `json:"texts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	langName, ok := i18nLangNames[req.Lang]
	if !ok || req.Lang == "vi" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ngôn ngữ không được hỗ trợ."})
		return
	}
	if len(req.Texts) > 800 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Quá nhiều câu trong một lần dịch (tối đa 800)."})
		return
	}
	if h.AI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Chưa cấu hình AI."})
		return
	}

	out := map[string]string{}
	// 1. Lấy từ bộ đệm
	hashes := map[string][]string{} // hash -> các key
	srcByHash := map[string]string{}
	for k, v := range req.Texts {
		v = strings.TrimSpace(v)
		if v == "" || len(k) > 300 {
			continue
		}
		if len(v) > 4000 {
			v = v[:4000]
		}
		hs := srcHash(v)
		hashes[hs] = append(hashes[hs], k)
		srcByHash[hs] = v
	}
	if len(hashes) > 0 {
		list := make([]string, 0, len(hashes))
		for hs := range hashes {
			list = append(list, hs)
		}
		var cached []TranslationCache
		h.DB.Where("lang = ? AND src_hash IN ?", req.Lang, list).Find(&cached)
		for _, tc := range cached {
			for _, k := range hashes[tc.SrcHash] {
				out[k] = tc.Text
			}
			delete(hashes, tc.SrcHash)
		}
	}

	// 2. Dịch phần còn thiếu: chia lô theo số câu + số ký tự, tự thử lại khi AI bị giới hạn lượt gọi,
	// chia nhỏ lô khi AI trả sai định dạng / bỏ sót câu → hạn chế tối đa câu bị bỏ lại tiếng Việt
	pending := make([]string, 0, len(hashes))
	for hs := range hashes {
		pending = append(pending, hs)
	}
	sort.Strings(pending)
	translated := map[string]string{}
	deadline := time.Now().Add(translateBudget)
	var lastErr error
	for _, group := range splitTranslateBatches(pending, srcByHash) {
		if err := h.translateHashes(langName, group, srcByHash, deadline, 0, translated); err != nil {
			lastErr = err
		}
	}
	failed := 0
	for _, hs := range pending {
		tr, ok := translated[hs]
		if !ok {
			failed += len(hashes[hs])
			continue
		}
		for _, k := range hashes[hs] {
			out[k] = tr
		}
		h.DB.Create(&TranslationCache{Lang: req.Lang, SrcHash: hs, Text: tr})
	}
	resp := gin.H{"translations": out}
	if failed > 0 {
		resp["failed"] = failed
		msg := fmt.Sprintf("Còn %d câu AI chưa dịch được. Bấm dịch lại để thử tiếp.", failed)
		if lastErr != nil {
			detail := lastErr.Error()
			if r := []rune(detail); len(r) > 240 {
				detail = string(r[:240]) + "…"
			}
			resp["reason"] = detail
			msg += " Lý do: " + detail
		}
		resp["warning"] = msg
	}
	c.JSON(http.StatusOK, resp)
}

const (
	translateBatchMaxItems = 40
	translateBatchMaxChars = 6000
	translateBudget        = 75 * time.Second // một yêu cầu không chạy quá lâu (cổng Render ngắt sau ~100 giây)
	translateMaxDepth      = 4
)

// Thời gian chờ giữa các lần thử lại (biến để bài kiểm thử rút ngắn)
var translateRetryWait = 2 * time.Second

var errAIBadFormat = fmt.Errorf("AI trả về dữ liệu không đọc được")

// splitTranslateBatches chia các câu thành lô vừa phải (tối đa 40 câu và ~6000 ký tự mỗi lô)
func splitTranslateBatches(pending []string, src map[string]string) [][]string {
	var out [][]string
	var cur []string
	chars := 0
	for _, hs := range pending {
		n := len(src[hs])
		if len(cur) > 0 && (len(cur) >= translateBatchMaxItems || chars+n > translateBatchMaxChars) {
			out = append(out, cur)
			cur, chars = nil, 0
		}
		cur = append(cur, hs)
		chars += n
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

var placeholderRe = regexp.MustCompile(`\{[A-Za-z_][A-Za-z0-9_]*\}`)

// translationUsable: bản dịch không rỗng và giữ nguyên các chỗ trống như {n}, {name}
func translationUsable(src, tr string) bool {
	if strings.TrimSpace(tr) == "" {
		return false
	}
	for _, ph := range placeholderRe.FindAllString(src, -1) {
		if !strings.Contains(tr, ph) {
			return false
		}
	}
	return true
}

// translateHashes dịch một nhóm câu (khoá = mã băm) và ghi kết quả vào out.
// Thử lại khi AI lỗi tạm thời; AI trả sai định dạng hoặc bỏ sót câu thì chia nhỏ rồi thử lại phần thiếu.
func (h *StorefrontHandler) translateHashes(langName string, hs []string, src map[string]string, deadline time.Time, depth int, out map[string]string) error {
	if len(hs) == 0 {
		return nil
	}
	batch := make(map[string]string, len(hs))
	for i, x := range hs {
		batch[fmt.Sprintf("t%d", i)] = src[x]
	}
	var res map[string]string
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			wait := time.Duration(attempt) * translateRetryWait
			if time.Now().Add(wait).After(deadline) {
				break
			}
			time.Sleep(wait)
		}
		res, err = h.aiTranslateBatch(langName, batch)
		if err == nil || (errors.Is(err, errAIBadFormat) && len(hs) > 1) {
			break // sai định dạng: chia nhỏ ngay thay vì gọi lại cả lô
		}
	}
	split := func(parts ...[]string) error {
		var first error
		for _, p := range parts {
			if e := h.translateHashes(langName, p, src, deadline, depth+1, out); e != nil && first == nil {
				first = e
			}
		}
		return first
	}
	if err != nil {
		if errors.Is(err, errAIBadFormat) && len(hs) > 1 && depth < translateMaxDepth && time.Now().Before(deadline) {
			mid := len(hs) / 2
			return split(hs[:mid], hs[mid:])
		}
		return err
	}
	var missing []string
	for i, x := range hs {
		tr := strings.TrimSpace(res[fmt.Sprintf("t%d", i)])
		if !translationUsable(src[x], tr) {
			missing = append(missing, x)
			continue
		}
		out[x] = tr
	}
	if len(missing) == 0 {
		return nil
	}
	if depth >= translateMaxDepth || time.Now().After(deadline) {
		return fmt.Errorf("AI bỏ sót %d câu", len(missing))
	}
	if len(missing) == len(hs) && len(hs) > 1 { // cả lô hỏng: chia đôi
		mid := len(hs) / 2
		return split(hs[:mid], hs[mid:])
	}
	return split(missing) // chỉ thử lại phần bị bỏ sót
}

// aiTranslateBatch gửi một lô {t0: "tiếng Việt", ...} và đọc lại {t0: "bản dịch", ...}.
// Chấp nhận cả khi AI bọc thêm một tầng ({"translations": {...}}) hoặc trả số thay vì chuỗi.
func (h *StorefrontHandler) aiTranslateBatch(langName string, batch map[string]string) (map[string]string, error) {
	src, _ := json.Marshal(batch)
	system := "You are a professional e-commerce translator and copywriter. " +
		"Translate every value of the given JSON object from Vietnamese into natural, fluent " + langName + " suitable for an online shop. " +
		"Rules: keep the same keys; keep placeholders like {n}, {name}, {code} exactly; keep brand names, product codes, emoji, numbers, URLs and e-mail addresses unchanged; " +
		"when a value contains HTML (product descriptions), keep every tag and attribute exactly and translate only the visible text between tags; " +
		"keep line breaks; do not add explanations. Return ONLY a JSON object with the same keys, and every value must be a string."
	reply, err := h.AI.callUniversalChatAI(system, string(src), true)
	if err != nil {
		return nil, err
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
	var raw map[string]any
	if err := json.Unmarshal([]byte(reply), &raw); err != nil {
		return nil, errAIBadFormat
	}
	if inner, ok := raw["translations"].(map[string]any); ok && len(raw) == 1 {
		raw = inner
	}
	res := make(map[string]string, len(raw))
	for k, v := range raw {
		key := strings.ToLower(strings.TrimSpace(k))
		switch x := v.(type) {
		case string:
			res[key] = x
		case float64, bool:
			res[key] = fmt.Sprint(x)
		}
	}
	if len(res) == 0 {
		return nil, errAIBadFormat
	}
	return res, nil
}
