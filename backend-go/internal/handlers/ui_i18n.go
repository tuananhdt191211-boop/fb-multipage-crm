package handlers

// Đa ngôn ngữ cho giao diện QUẢN TRỊ PagePilot (người mua phần mềm ở nước ngoài dùng bằng tiếng của họ).
//
// Cách hoạt động:
//   - Giao diện được viết bằng tiếng Việt. Script web-vue/scripts/extract-ui-strings.mjs quét mã nguồn và lập
//     "danh mục" các câu chữ giao diện (uicatalog/catalog.vi.json) kèm bản tiếng Anh (uicatalog/en.json).
//     Hai file này được nhúng (go:embed) vào chương trình.
//   - Trình duyệt dịch tại chỗ những câu CÓ trong danh mục (so khớp nguyên văn). Nội dung của khách hàng
//     (tin nhắn, tên, sản phẩm...) không nằm trong danh mục nên không bao giờ bị dịch hay gửi đi đâu.
//   - Tiếng Anh có sẵn trong gói web. Ngôn ngữ khác được AI dịch từ danh mục (chỉ danh mục, không nhận chữ tuỳ ý
//     từ người dùng) rồi lưu chung vào bảng translation_caches (tiền tố "ui|" để tách khỏi bản dịch website):
//     người đầu tiên chọn một ngôn ngữ làm AI dịch xong, những người sau dùng luôn bản đã dịch.
//
//	GET  /api/v1/ui-i18n/:lang        (công khai) bản dịch đã có của ngôn ngữ đó: {lang,total,count,translations}
//	POST /api/v1/ui-i18n/:lang/fill   (đăng nhập) nhờ AI dịch tiếp một đợt các câu còn thiếu

import (
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:embed uicatalog/catalog.vi.json
var uiCatalogRaw []byte

//go:embed uicatalog/en.json
var uiEnglishRaw []byte

// Ngôn ngữ của giao diện quản trị: gồm các ngôn ngữ của website bán hàng + một số ngôn ngữ châu Á.
// (Không có ngôn ngữ viết từ phải sang trái vì bố cục chưa hỗ trợ.)
var uiLangNames = func() map[string]string {
	m := make(map[string]string, len(i18nLangNames)+8)
	for k, v := range i18nLangNames {
		m[k] = v
	}
	m["zh"] = "Simplified Chinese"
	m["ja"] = "Japanese"
	m["ko"] = "Korean"
	m["th"] = "Thai"
	m["id"] = "Indonesian"
	m["ms"] = "Malay"
	return m
}()

const (
	uiFillBatch  = 40               // số câu mỗi lần gọi AI
	uiFillBudget = 40 * time.Second // quá thời gian này thì không mở thêm đợt mới trong cùng một yêu cầu
	uiCacheTTL   = time.Minute      // bản dịch trong RAM
)

// ----------------------------------------------------------------- Danh mục nhúng sẵn

type uiCatalogData struct {
	list   []string          // các câu tiếng Việt, thứ tự cố định
	en     map[string]string // câu tiếng Việt -> tiếng Anh
	byHash map[string]string // băm("ui|"+câu) -> câu tiếng Việt
	err    error
}

var (
	uiCatOnce sync.Once
	uiCat     uiCatalogData
)

func uiSrcHash(vi string) string { return srcHash("ui|" + vi) }

func uiCatalog() *uiCatalogData {
	uiCatOnce.Do(func() {
		if err := json.Unmarshal(uiCatalogRaw, &uiCat.list); err != nil {
			uiCat.err = fmt.Errorf("danh mục giao diện hỏng: %w", err)
			return
		}
		if err := json.Unmarshal(uiEnglishRaw, &uiCat.en); err != nil {
			uiCat.err = fmt.Errorf("bản tiếng Anh của giao diện hỏng: %w", err)
			return
		}
		uiCat.byHash = make(map[string]string, len(uiCat.list))
		for _, s := range uiCat.list {
			uiCat.byHash[uiSrcHash(s)] = s
		}
	})
	return &uiCat
}

var uiPlaceholderRe = regexp.MustCompile(`\{\d+\}`)

func uiPlaceholders(s string) string {
	ph := uiPlaceholderRe.FindAllString(s, -1)
	sort.Strings(ph)
	return strings.Join(ph, ",")
}

// uiTranslationOK: bản dịch phải giữ nguyên các biến {0} {1}..., không rỗng, không dài bất thường,
// và không phải chính câu tiếng Việt gốc (AI "quên" dịch).
func uiTranslationOK(vi, en, tr string) bool {
	tr = strings.TrimSpace(tr)
	if tr == "" || uiPlaceholders(vi) != uiPlaceholders(tr) {
		return false
	}
	limit := 5*len([]rune(en)) + 40
	if n := 5*len([]rune(vi)) + 40; n > limit {
		limit = n
	}
	if len([]rune(tr)) > limit {
		return false
	}
	if tr == vi && strings.IndexFunc(vi, func(r rune) bool { return r > 127 && unicode.IsLetter(r) }) >= 0 {
		return false
	}
	return true
}

// ----------------------------------------------------------------- Handler

type UIi18nHandler struct {
	DB *gorm.DB
	AI *AIHandler // gán trong main.go

	// translate là chỗ gọi AI; mặc định dùng AI của hệ thống (test thay bằng bản giả)
	translate func(langName string, batch map[string]uiSrcPair) (map[string]string, error)

	mu    sync.Mutex
	cache map[string]*uiLangPayload
	locks map[string]*sync.Mutex
}

type uiSrcPair struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

type uiLangPayload struct {
	at    time.Time
	body  []byte
	etag  string
	count int
}

func NewUIi18nHandler(db *gorm.DB) *UIi18nHandler {
	migrateTranslationCache(db)
	return &UIi18nHandler{DB: db, cache: map[string]*uiLangPayload{}, locks: map[string]*sync.Mutex{}}
}

// lang chuẩn hoá + kiểm tra; trả mã lỗi HTTP khác 0 nếu không hợp lệ
func uiLangParam(c *gin.Context) (string, int, string) {
	lang := strings.ToLower(strings.TrimSpace(c.Param("lang")))
	if lang == "vi" {
		return "", http.StatusBadRequest, "Tiếng Việt là ngôn ngữ gốc của giao diện."
	}
	if _, ok := uiLangNames[lang]; !ok {
		return "", http.StatusNotFound, "Ngôn ngữ chưa được hỗ trợ."
	}
	return lang, 0, ""
}

// loadFromDB: các câu của danh mục đã có bản dịch trong bộ đệm. Không cần tiếng Anh vì đã có sẵn trong gói web.
func (h *UIi18nHandler) loadFromDB(lang string) (map[string]string, error) {
	cat := uiCatalog()
	if cat.err != nil {
		return nil, cat.err
	}
	hashes := make([]string, 0, len(cat.byHash))
	for hs := range cat.byHash {
		hashes = append(hashes, hs)
	}
	var rows []TranslationCache
	if err := h.DB.Select("src_hash", "text").Where("lang = ? AND src_hash IN ?", lang, hashes).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		if vi, ok := cat.byHash[r.SrcHash]; ok && strings.TrimSpace(r.Text) != "" {
			out[vi] = r.Text
		}
	}
	return out, nil
}

func (h *UIi18nHandler) payload(lang string) (*uiLangPayload, error) {
	h.mu.Lock()
	p := h.cache[lang]
	h.mu.Unlock()
	if p != nil && time.Since(p.at) < uiCacheTTL {
		return p, nil
	}

	var tr map[string]string
	if lang == "en" {
		cat := uiCatalog()
		if cat.err != nil {
			return nil, cat.err
		}
		tr = cat.en
	} else {
		var err error
		if tr, err = h.loadFromDB(lang); err != nil {
			if p != nil { // lỗi database: dùng tạm bản cũ
				return p, nil
			}
			return nil, err
		}
	}
	body, _ := json.Marshal(gin.H{"lang": lang, "total": len(uiCatalog().list), "count": len(tr), "translations": tr})
	sum := sha1.Sum(body)
	np := &uiLangPayload{at: time.Now(), body: body, etag: `"` + hex.EncodeToString(sum[:8]) + `"`, count: len(tr)}
	h.mu.Lock()
	h.cache[lang] = np
	h.mu.Unlock()
	return np, nil
}

func (h *UIi18nHandler) invalidate(lang string) {
	h.mu.Lock()
	delete(h.cache, lang)
	h.mu.Unlock()
}

// Get GET /api/v1/ui-i18n/:lang (công khai: trang đăng nhập cũng cần)
func (h *UIi18nHandler) Get(c *gin.Context) {
	lang, status, msg := uiLangParam(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	p, err := h.payload(lang)
	if err != nil {
		log.Printf("[ui-i18n] không đọc được bản dịch %s: %v", lang, err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Chưa tải được bản dịch giao diện, thử lại sau."})
		return
	}
	c.Header("ETag", p.etag)
	c.Header("Cache-Control", "public, max-age=60")
	if c.GetHeader("If-None-Match") == p.etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", p.body)
}

func (h *UIi18nHandler) langLock(lang string) *sync.Mutex {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.locks[lang] == nil {
		h.locks[lang] = &sync.Mutex{}
	}
	return h.locks[lang]
}

// Fill POST /api/v1/ui-i18n/:lang/fill — dịch tiếp một đợt các câu còn thiếu (trong khoảng 40 giây).
// Trả {total, count, remaining, added:{vi: bản dịch}}. Giao diện gọi lặp lại cho tới khi remaining = 0.
func (h *UIi18nHandler) Fill(c *gin.Context) {
	lang, status, msg := uiLangParam(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	if lang == "en" {
		c.JSON(http.StatusOK, gin.H{"total": len(uiCatalog().list), "count": len(uiCatalog().list), "remaining": 0, "added": gin.H{}})
		return
	}
	cat := uiCatalog()
	if cat.err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": cat.err.Error()})
		return
	}

	lk := h.langLock(lang)
	if !lk.TryLock() { // đang có người khác dịch ngôn ngữ này
		c.JSON(http.StatusOK, gin.H{"busy": true, "total": len(cat.list)})
		return
	}
	defer lk.Unlock()

	have, err := h.loadFromDB(lang)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Chưa đọc được bản dịch đã lưu, thử lại sau."})
		return
	}
	missing := make([]string, 0, len(cat.list)-len(have))
	for _, s := range cat.list {
		if _, ok := have[s]; !ok {
			missing = append(missing, s)
		}
	}
	resp := gin.H{"total": len(cat.list)}
	if len(missing) == 0 {
		resp["count"], resp["remaining"], resp["added"] = len(cat.list), 0, gin.H{}
		c.JSON(http.StatusOK, resp)
		return
	}

	translate := h.translate
	if translate == nil {
		if h.AI == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Chưa cấu hình AI nên chưa dịch được giao diện sang ngôn ngữ này."})
			return
		}
		translate = h.aiTranslateUI
	}

	langName := uiLangNames[lang]
	added := map[string]string{}
	var lastErr error
	deadline := time.Now().Add(uiFillBudget)
	for start := 0; start < len(missing) && time.Now().Before(deadline); start += uiFillBatch {
		end := start + uiFillBatch
		if end > len(missing) {
			end = len(missing)
		}
		batch := make(map[string]uiSrcPair, end-start)
		ids := make([]string, 0, end-start)
		for i, vi := range missing[start:end] {
			id := fmt.Sprintf("t%d", i)
			batch[id] = uiSrcPair{VI: vi, EN: cat.en[vi]}
			ids = append(ids, vi)
		}
		res, err := translate(langName, batch)
		if err != nil {
			lastErr = err
			break // thường là hết lượt gọi AI; lần gọi sau thử tiếp
		}
		rows := make([]TranslationCache, 0, len(ids))
		for i, vi := range ids {
			tr := strings.TrimSpace(res[fmt.Sprintf("t%d", i)])
			if !uiTranslationOK(vi, cat.en[vi], tr) {
				continue
			}
			added[vi] = tr
			rows = append(rows, TranslationCache{Lang: lang, SrcHash: uiSrcHash(vi), Text: tr})
		}
		if len(rows) > 0 {
			if err := h.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
				log.Printf("[ui-i18n] lưu bản dịch %s lỗi: %v", lang, err)
			}
		}
	}
	if len(added) > 0 {
		h.invalidate(lang)
	}
	remaining := len(missing) - len(added)
	resp["count"] = len(cat.list) - remaining
	resp["remaining"] = remaining
	resp["added"] = added
	if lastErr != nil && len(added) == 0 {
		log.Printf("[ui-i18n] AI dịch %s lỗi: %v", lang, lastErr)
		resp["warning"] = "AI chưa dịch được (có thể do giới hạn lượt gọi hoặc chưa có API key). Hệ thống sẽ thử lại sau."
	}
	c.JSON(http.StatusOK, resp)
}

func (h *UIi18nHandler) aiTranslateUI(langName string, batch map[string]uiSrcPair) (map[string]string, error) {
	src, _ := json.Marshal(batch)
	system := "You are a professional software localization translator. " +
		"Translate the user-interface texts of \"PagePilot\", a CRM web app for managing Facebook Fanpages " +
		"(merged multi-Page Messenger inbox, automatic customer-care messages, post scheduling, competitor analysis, " +
		"a drag-and-drop online store builder with orders and payments), into natural, concise " + langName + ". " +
		"The input is a JSON object: each key is an id and each value is {\"vi\": Vietnamese original, \"en\": English reference}. " +
		"Use the Vietnamese as the source of meaning and the English as a terminology hint. " +
		"Return ONLY a JSON object with the same keys where each value is the translated text as a plain string. " +
		"Rules: keep placeholders like {0}, {1} exactly as written; keep brand and product names untranslated " +
		"(PagePilot, Facebook, Messenger, Meta, Meta Business, Instagram, TikTok, YouTube, Zalo, VNPay, PayPal, Stripe, Cloudflare, Gemini, Groq, OpenAI, Google); " +
		"keep emoji, symbols, numbers, URLs, e-mail addresses, code snippets, file names and keyboard shortcuts (Ctrl+S) unchanged; " +
		"keep a trailing \"...\" or \":\" or \"→\" if the original has one; use short UI wording for buttons and labels, " +
		"and the same capitalization style as the English reference; do not add explanations."
	reply, err := h.AI.callUniversalChatAI(system, string(src), true)
	if err != nil {
		return nil, err
	}
	return parseAIStringMap(reply)
}

// parseAIStringMap đọc JSON {id: "chuỗi"} từ câu trả lời của AI (bỏ rào ```json ...```)
func parseAIStringMap(reply string) (map[string]string, error) {
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
		return nil, fmt.Errorf("AI trả về dữ liệu không đọc được")
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		switch t := v.(type) {
		case string:
			out[k] = t
		case map[string]any: // AI lỡ trả {"text": "..."}
			for _, key := range []string{"text", "translation", "tr"} {
				if s, ok := t[key].(string); ok {
					out[k] = s
					break
				}
			}
		}
	}
	return out, nil
}
