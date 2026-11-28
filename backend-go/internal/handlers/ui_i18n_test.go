package handlers

// Test đa ngôn ngữ giao diện quản trị.
//   - Phần thuần: toàn vẹn danh mục nhúng sẵn, kiểm tra bản dịch, đọc JSON của AI.
//   - Phần tích hợp (cần TEST_DATABASE_URL, tên database kết thúc bằng _test):
//     TEST_DATABASE_URL=postgres://fbcrm:fbcrm_secret_password@localhost:5432/fbcrm_test?sslmode=disable go test ./internal/handlers -run UI -v

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestUICatalogIntegrity(t *testing.T) {
	cat := uiCatalog()
	if cat.err != nil {
		t.Fatal(cat.err)
	}
	if len(cat.list) < 1000 {
		t.Fatalf("danh mục chỉ có %d câu — chạy `npm run i18n:extract` trong web-vue?", len(cat.list))
	}
	if !sort.StringsAreSorted(cat.list) {
		t.Fatal("danh mục phải sắp xếp theo thứ tự chữ")
	}
	if len(cat.byHash) != len(cat.list) {
		t.Fatalf("trùng câu hoặc trùng băm: %d câu, %d băm", len(cat.list), len(cat.byHash))
	}
	if len(cat.en) != len(cat.list) {
		t.Fatalf("bản tiếng Anh có %d câu, danh mục có %d — chạy lại `npm run i18n:extract`", len(cat.en), len(cat.list))
	}
	for _, vi := range cat.list {
		en := strings.TrimSpace(cat.en[vi])
		if en == "" {
			t.Errorf("thiếu tiếng Anh cho %q", vi)
			continue
		}
		if uiPlaceholders(vi) != uiPlaceholders(en) {
			t.Errorf("sai biến {n}: %q -> %q", vi, en)
		}
	}
}

func TestUITranslationOK(t *testing.T) {
	cases := []struct {
		vi, en, tr string
		ok         bool
	}{
		{"Đã chọn {0} trang", "{0} pages selected", "{0} Seiten ausgewählt", true},
		{"Đã chọn {0} trang", "{0} pages selected", "Seiten ausgewählt", false}, // mất biến
		{"Đã chọn {0} trang", "{0} pages selected", "{0} {1} Seiten", false},    // thừa biến
		{"Lưu", "Save", "", false},
		{"Lưu", "Save", "   ", false},
		{"Lưu", "Save", "Lưu", false}, // AI quên dịch (giữ nguyên tiếng Việt)
		{"Lưu", "Save", "Speichern", true},
		{"Tổng", "Total", strings.Repeat("x", 500), false}, // dài bất thường
		{"Xin chào, {0}! {1}", "Hello, {0}! {1}", "{1} Hallo, {0}!", true},
	}
	for _, c := range cases {
		if got := uiTranslationOK(c.vi, c.en, c.tr); got != c.ok {
			t.Errorf("uiTranslationOK(%q,%q,%q) = %v, muốn %v", c.vi, c.en, c.tr, got, c.ok)
		}
	}
}

func TestParseAIStringMap(t *testing.T) {
	out, err := parseAIStringMap("Here you go:\n```json\n{\"t0\":\"Speichern\",\"t1\":{\"text\":\"Löschen\"},\"t2\":5}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if out["t0"] != "Speichern" || out["t1"] != "Löschen" {
		t.Fatalf("đọc sai: %v", out)
	}
	if _, ok := out["t2"]; ok {
		t.Fatal("giá trị không phải chuỗi mà vẫn nhận")
	}
	if _, err := parseAIStringMap("không phải json"); err == nil {
		t.Fatal("phải báo lỗi khi AI không trả JSON")
	}
}

func TestUILangParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for lang, want := range map[string]int{"de": 0, "EN": 0, "zh": 0, "vi": http.StatusBadRequest, "xx": http.StatusNotFound, "../etc": http.StatusNotFound} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "lang", Value: lang}}
		if _, status, _ := uiLangParam(c); status != want {
			t.Errorf("lang=%q: status %d, muốn %d", lang, status, want)
		}
	}
}

// ----------------------------------------------------------------- tích hợp

func openUITestDB(t *testing.T) *gorm.DB {
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
	migrateTranslationCache(db)
	db.Exec("DELETE FROM translation_caches")
	return db
}

func uiTestRouter(h *UIi18nHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ui-i18n/:lang", h.Get)
	r.POST("/ui-i18n/:lang/fill", h.Fill)
	return r
}

func uiDo(r http.Handler, method, path, etag string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type uiFillResp struct {
	Total     int               `json:"total"`
	Count     int               `json:"count"`
	Remaining int               `json:"remaining"`
	Busy      bool              `json:"busy"`
	Warning   string            `json:"warning"`
	Added     map[string]string `json:"added"`
}

func TestUIFillAndGet(t *testing.T) {
	db := openUITestDB(t)
	cat := uiCatalog()
	h := NewUIi18nHandler(db)

	var mu sync.Mutex
	calls := 0
	h.translate = func(langName string, batch map[string]uiSrcPair) (map[string]string, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		out := map[string]string{}
		for id, p := range batch {
			out[id] = "[" + langName + "] " + p.EN
		}
		return out, nil
	}
	r := uiTestRouter(h)

	// Chưa dịch gì: Get trả 0 câu, đúng tổng
	w := uiDo(r, "GET", "/ui-i18n/de", "")
	if w.Code != 200 {
		t.Fatalf("GET de: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Total, Count int
		Translations map[string]string
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Total != len(cat.list) || got.Count != 0 || len(got.Translations) != 0 {
		t.Fatalf("ban đầu phải trống: total=%d count=%d", got.Total, got.Count)
	}

	// Một đợt Fill dịch đúng 1 batch (40 câu) vì translate giả chạy tức thì → gọi tiếp tới khi hết
	var last uiFillResp
	rounds := 0
	for {
		rounds++
		w = uiDo(r, "POST", "/ui-i18n/de/fill", "")
		if w.Code != 200 {
			t.Fatalf("fill: %d %s", w.Code, w.Body.String())
		}
		last = uiFillResp{}
		_ = json.Unmarshal(w.Body.Bytes(), &last)
		if last.Remaining == 0 || rounds > 3 {
			break
		}
	}
	if last.Remaining != 0 || last.Count != len(cat.list) {
		t.Fatalf("sau %d lượt còn thiếu %d (count %d/%d)", rounds, last.Remaining, last.Count, last.Total)
	}
	if calls != (len(cat.list)+uiFillBatch-1)/uiFillBatch {
		t.Fatalf("gọi AI %d lần, muốn %d", calls, (len(cat.list)+uiFillBatch-1)/uiFillBatch)
	}

	// Lần Fill tiếp theo không gọi AI nữa
	before := calls
	w = uiDo(r, "POST", "/ui-i18n/de/fill", "")
	_ = json.Unmarshal(w.Body.Bytes(), &last)
	if calls != before || last.Remaining != 0 {
		t.Fatalf("đã đủ mà vẫn gọi AI (%d -> %d)", before, calls)
	}

	// Get trả đủ + ETag/304; có bản dịch theo đúng câu tiếng Việt
	w = uiDo(r, "GET", "/ui-i18n/de", "")
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Count != len(cat.list) || len(got.Translations) != len(cat.list) {
		t.Fatalf("Get sau fill: count=%d translations=%d", got.Count, len(got.Translations))
	}
	for vi, en := range cat.en {
		if got.Translations[vi] != "[German] "+en {
			t.Fatalf("sai bản dịch của %q: %q", vi, got.Translations[vi])
		}
		break
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("thiếu ETag")
	}
	if w2 := uiDo(r, "GET", "/ui-i18n/de", etag); w2.Code != http.StatusNotModified {
		t.Fatalf("cùng ETag phải trả 304, được %d", w2.Code)
	}
	// Ngôn ngữ khác không bị lẫn
	w = uiDo(r, "GET", "/ui-i18n/fr", "")
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Count != 0 {
		t.Fatalf("fr phải trống, có %d", got.Count)
	}
	// Tiếng Anh lấy thẳng từ danh mục; tiếng Việt / ngôn ngữ lạ bị từ chối
	w = uiDo(r, "GET", "/ui-i18n/en", "")
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if w.Code != 200 || got.Count != len(cat.list) {
		t.Fatalf("en: %d count=%d", w.Code, got.Count)
	}
	if c := uiDo(r, "GET", "/ui-i18n/vi", "").Code; c != http.StatusBadRequest {
		t.Fatalf("vi: %d", c)
	}
	if c := uiDo(r, "POST", "/ui-i18n/xx/fill", "").Code; c != http.StatusNotFound {
		t.Fatalf("xx: %d", c)
	}
}

func TestUIFillRejectsBadTranslationsAndSurvivesAIFailure(t *testing.T) {
	db := openUITestDB(t)
	cat := uiCatalog()
	h := NewUIi18nHandler(db)
	r := uiTestRouter(h)

	// AI hỏng placeholder ở mọi câu → không câu nào được lưu
	h.translate = func(_ string, batch map[string]uiSrcPair) (map[string]string, error) {
		out := map[string]string{}
		for id, p := range batch {
			out[id] = strings.ReplaceAll(p.EN, "{0}", "") + " {9}"
		}
		return out, nil
	}
	w := uiDo(r, "POST", "/ui-i18n/es/fill", "")
	var resp uiFillResp
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Added) != 0 || resp.Remaining != len(cat.list) {
		t.Fatalf("bản dịch hỏng vẫn được nhận: added=%d remaining=%d", len(resp.Added), resp.Remaining)
	}

	// AI lỗi hẳn → vẫn trả 200 + cảnh báo, không lưu gì
	h.translate = func(string, map[string]uiSrcPair) (map[string]string, error) {
		return nil, errors.New("429 rate limit")
	}
	w = uiDo(r, "POST", "/ui-i18n/es/fill", "")
	resp = uiFillResp{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if w.Code != 200 || resp.Warning == "" || len(resp.Added) != 0 {
		t.Fatalf("AI lỗi: %d %s", w.Code, w.Body.String())
	}
	var n int64
	db.Model(&TranslationCache{}).Where("lang = ?", "es").Count(&n)
	if n != 0 {
		t.Fatalf("đã lưu %d dòng dù bản dịch hỏng / AI lỗi", n)
	}
}

func TestUIFillBusyWhenAnotherRunInProgress(t *testing.T) {
	db := openUITestDB(t)
	h := NewUIi18nHandler(db)
	r := uiTestRouter(h)
	h.translate = func(string, map[string]uiSrcPair) (map[string]string, error) { return map[string]string{}, nil }

	lk := h.langLock("it")
	lk.Lock()
	defer lk.Unlock()
	w := uiDo(r, "POST", "/ui-i18n/it/fill", "")
	var resp uiFillResp
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if w.Code != 200 || !resp.Busy {
		t.Fatalf("đang dịch dở mà không báo busy: %d %s", w.Code, w.Body.String())
	}
}
