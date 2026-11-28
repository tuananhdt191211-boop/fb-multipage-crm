package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"fb-multipage-crm/internal/config"

	"github.com/gin-gonic/gin"
)

// AI giả cho bài kiểm thử dịch: mô phỏng các lỗi hay gặp ngoài đời
type fakeTranslator struct {
	mu        sync.Mutex
	calls     int
	maxBatch  int
	rateLimit int      // số lần đầu trả 429
	wrapOnce  bool     // lần đầu bọc thêm {"translations": ...}
	dropPh    string   // câu (tiếng Việt) mà lần đầu AI làm mất chỗ trống {n}
	badJSON   int      // số lần đầu trả chuỗi không phải JSON
	numeric   bool     // trả số cho câu "100"
	seen      []string // mọi câu đã được gửi
}

func (f *fakeTranslator) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.rateLimit > 0 {
		f.rateLimit--
		w.WriteHeader(429)
		return
	}
	var req struct {
		Messages []struct{ Content string } `json:"messages"`
	}
	_ = json.Unmarshal(body, &req)
	var batch map[string]string
	_ = json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &batch)
	if len(batch) > f.maxBatch {
		f.maxBatch = len(batch)
	}
	reply := func(s string) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": s}}}})
	}
	if f.badJSON > 0 && len(batch) > 1 {
		f.badJSON--
		reply("Xin lỗi, đây là bản dịch: ...")
		return
	}
	out := map[string]any{}
	for k, v := range batch {
		f.seen = append(f.seen, v)
		tr := "EN:" + v
		if v == f.dropPh {
			tr = "EN: no placeholder here"
			f.dropPh = ""
		}
		if f.numeric && v == "100" {
			out[k] = 100
			continue
		}
		out[k] = tr
	}
	b, _ := json.Marshal(out)
	if f.wrapOnce {
		f.wrapOnce = false
		b, _ = json.Marshal(map[string]any{"translations": out})
	}
	reply(string(b))
}

func newTranslateEnv(t *testing.T, f *fakeTranslator) *productEnv {
	e := newProductEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	t.Setenv("AI_BASE_URL", srv.URL)
	t.Setenv("AI_API_KEY", "test")
	t.Setenv("AI_MODEL", "fake")
	e.h.AI = NewAIHandler(e.db, &config.Config{})
	e.db.Exec("TRUNCATE translation_caches RESTART IDENTITY")
	e.r.POST("/api/v1/storefront/sites/:id/translate", func(c *gin.Context) { c.Set("user_id", uint(7)); e.h.TranslateStrings(c) })
	old := translateRetryWait
	translateRetryWait = 5 * time.Millisecond
	t.Cleanup(func() { translateRetryWait = old })
	return e
}

func (e *productEnv) translate(texts map[string]string) (map[string]string, map[string]any) {
	_, out := e.do("POST", e.base()+"/translate", map[string]any{"lang": "en", "texts": texts})
	tr := map[string]string{}
	if m, ok := out["translations"].(map[string]any); ok {
		for k, v := range m {
			tr[k] = fmt.Sprint(v)
		}
	}
	return tr, out
}

func TestTranslateSurvivesRateLimitAndOddReplies(t *testing.T) {
	f := &fakeTranslator{rateLimit: 2, wrapOnce: true, numeric: true}
	e := newTranslateEnv(t, f)
	texts := map[string]string{"a": "Trang chủ", "b": "Sản phẩm nổi bật", "c": "100", "d": "Giỏ hàng ({n})"}
	tr, out := e.translate(texts)
	for k, v := range texts {
		want := "EN:" + v
		if v == "100" {
			want = "100"
		}
		if tr[k] != want {
			t.Errorf("%s: %q, muốn %q (kết quả %v)", k, tr[k], want, out)
		}
	}
	if out["failed"] != nil {
		t.Errorf("không được báo lỗi: %v", out)
	}
}

func TestTranslateSplitsBadBatchesAndKeepsPlaceholders(t *testing.T) {
	f := &fakeTranslator{badJSON: 1, dropPh: "Giỏ hàng ({n})"}
	e := newTranslateEnv(t, f)
	texts := map[string]string{}
	for i := 0; i < 12; i++ {
		texts[fmt.Sprintf("k%d", i)] = fmt.Sprintf("Câu số %d", i)
	}
	texts["cart"] = "Giỏ hàng ({n})"
	tr, out := e.translate(texts)
	if out["failed"] != nil {
		t.Fatalf("còn câu chưa dịch: %v", out)
	}
	for k, v := range texts {
		if tr[k] != "EN:"+v {
			t.Errorf("%s: %q", k, tr[k])
		}
	}
	if !strings.Contains(tr["cart"], "{n}") {
		t.Errorf("mất chỗ trống {n}: %q", tr["cart"])
	}
}

func TestTranslateReportsRealReasonAndUsesCache(t *testing.T) {
	f := &fakeTranslator{rateLimit: 1000}
	e := newTranslateEnv(t, f)
	_, out := e.translate(map[string]string{"a": "Trang chủ"})
	if out["failed"] == nil || out["reason"] == nil || !strings.Contains(fmt.Sprint(out["warning"]), "Lý do") {
		t.Fatalf("phải nói rõ lý do: %v", out)
	}

	// AI chạy lại bình thường: dịch xong, lần sau lấy từ bộ đệm (không gọi AI nữa)
	f.mu.Lock()
	f.rateLimit = 0
	f.mu.Unlock()
	tr, out := e.translate(map[string]string{"a": "Trang chủ"})
	if tr["a"] != "EN:Trang chủ" || out["failed"] != nil {
		t.Fatalf("%v %v", tr, out)
	}
	f.mu.Lock()
	calls := f.calls
	f.mu.Unlock()
	if tr, _ = e.translate(map[string]string{"x": "Trang chủ"}); tr["x"] != "EN:Trang chủ" {
		t.Fatal("không lấy từ bộ đệm")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls != calls {
		t.Errorf("gọi AI thêm %d lần dù đã có trong bộ đệm", f.calls-calls)
	}
}

func TestSplitTranslateBatchesRespectsLimits(t *testing.T) {
	src := map[string]string{}
	var hs []string
	for i := 0; i < 100; i++ {
		k := fmt.Sprintf("h%03d", i)
		src[k] = "x"
		hs = append(hs, k)
	}
	for _, g := range splitTranslateBatches(hs, src) {
		if len(g) > translateBatchMaxItems {
			t.Fatalf("lô %d câu", len(g))
		}
	}
	long := strings.Repeat("a", 4000)
	src2 := map[string]string{"p": long, "q": long, "r": "ngắn"}
	groups := splitTranslateBatches([]string{"p", "q", "r"}, src2)
	if len(groups) != 2 || len(groups[0]) != 1 || len(groups[1]) != 2 {
		t.Fatalf("chia theo ký tự sai: %v", groups)
	}
	if !translationUsable("Giỏ ({n})", "Cart ({n})") || translationUsable("Giỏ ({n})", "Cart") || translationUsable("a", " ") {
		t.Fatal("translationUsable sai")
	}
}
