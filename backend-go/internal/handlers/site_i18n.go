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

	// 2. Dịch phần còn thiếu theo từng nhóm
	pending := make([]string, 0, len(hashes))
	for hs := range hashes {
		pending = append(pending, hs)
	}
	sort.Strings(pending)
	failed := 0
	for start := 0; start < len(pending); start += 50 {
		end := start + 50
		if end > len(pending) {
			end = len(pending)
		}
		batch := map[string]string{}
		for i, hs := range pending[start:end] {
			batch[fmt.Sprintf("t%d", i)] = srcByHash[hs]
		}
		res, err := h.aiTranslateBatch(langName, batch)
		if err != nil {
			failed += end - start
			continue
		}
		for i, hs := range pending[start:end] {
			tr := strings.TrimSpace(res[fmt.Sprintf("t%d", i)])
			if tr == "" {
				failed++
				continue
			}
			for _, k := range hashes[hs] {
				out[k] = tr
			}
			h.DB.Create(&TranslationCache{Lang: req.Lang, SrcHash: hs, Text: tr})
		}
	}
	resp := gin.H{"translations": out}
	if failed > 0 {
		resp["failed"] = failed
		resp["warning"] = fmt.Sprintf("Còn %d câu AI chưa dịch được (có thể do giới hạn lượt gọi). Bấm dịch lại để thử tiếp.", failed)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *StorefrontHandler) aiTranslateBatch(langName string, batch map[string]string) (map[string]string, error) {
	src, _ := json.Marshal(batch)
	system := "You are a professional e-commerce translator and copywriter. " +
		"Translate every value of the given JSON object from Vietnamese into natural, fluent " + langName + " suitable for an online shop. " +
		"Rules: keep the same keys; keep placeholders like {n}, {name}, {code} exactly; keep brand names, product codes, emoji, numbers, URLs and e-mail addresses unchanged; " +
		"keep line breaks; do not add explanations. Return ONLY a JSON object with the same keys."
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
	var res map[string]string
	if err := json.Unmarshal([]byte(reply), &res); err != nil {
		return nil, fmt.Errorf("AI trả về dữ liệu không đọc được")
	}
	return res, nil
}
