package handlers

// Test tích hợp cho đồng bộ hộp thư & số phiên bản (inboxrev) trên PostgreSQL THẬT.
//
// Chạy:  TEST_DATABASE_URL=postgres://fbcrm:fbcrm_secret_password@localhost:5432/fbcrm_test?sslmode=disable go test ./internal/handlers -run Inbox -v
// An toàn: tên database BẮT BUỘC kết thúc bằng "_test" (test sẽ TRUNCATE các bảng hộp thư); không có biến môi trường thì bỏ qua.
// Facebook được giả lập bằng RoundTripper nên không gọi mạng.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/inboxrev"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeFB struct {
	body  atomic.Value // string
	calls atomic.Int32
}

func (f *fakeFB) RoundTrip(req *http.Request) (*http.Response, error) {
	f.calls.Add(1)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(f.body.Load().(string))),
		Request:    req,
	}, nil
}

type stmtCounter struct{ reads, writes atomic.Int64 }

func (c *stmtCounter) reset() { c.reads.Store(0); c.writes.Store(0) }

func (c *stmtCounter) attach(db *gorm.DB) {
	_ = db.Callback().Query().After("gorm:query").Register("test:count_read", func(*gorm.DB) { c.reads.Add(1) })
	w := func(*gorm.DB) { c.writes.Add(1) }
	_ = db.Callback().Create().After("gorm:create").Register("test:count_create", w)
	_ = db.Callback().Update().After("gorm:update").Register("test:count_update", w)
	_ = db.Callback().Delete().After("gorm:delete").Register("test:count_delete", w)
}

func openInboxTestDB(t *testing.T) *gorm.DB {
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
		t.Fatalf("TỪ CHỐI chạy trên database %q: tên phải kết thúc bằng _test (test sẽ xoá dữ liệu hộp thư)", name)
	}
	if err := db.AutoMigrate(&models.FacebookPage{}, &models.Conversation{}, &models.Message{}, &conversationWithCRM{}, &models.PageAISetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.Exec("TRUNCATE messages, conversations, facebook_pages RESTART IDENTITY CASCADE")
	return db
}

func fbMsg(id, fromID, fromName, text string, at time.Time) string {
	return fmt.Sprintf(`{"id":%q,"message":%q,"created_time":%q,"from":{"id":%q,"name":%q}}`,
		id, text, at.UTC().Format("2006-01-02T15:04:05-0700"), fromID, fromName)
}

func fbThread(id, psid, name string, unread int, updated time.Time, msgs ...string) string {
	return fmt.Sprintf(`{"id":%q,"updated_time":%q,"unread_count":%d,"participants":{"data":[{"id":"P1","name":"Shop A"},{"id":%q,"name":%q}]},"messages":{"data":[%s]}}`,
		id, updated.UTC().Format("2006-01-02T15:04:05-0700"), unread, psid, name, strings.Join(msgs, ","))
}

func fbPayload(threads ...string) string { return `{"data":[` + strings.Join(threads, ",") + `]}` }

func countRows(db *gorm.DB, table string) int64 {
	var n int64
	db.Table(table).Count(&n)
	return n
}

func TestInboxSyncIsIdempotentAndBumpsRevOnlyOnRealChanges(t *testing.T) {
	db := openInboxTestDB(t)
	inboxrev.Register(db)
	var cnt stmtCounter
	cnt.attach(db)

	const key = "test-master-key"
	enc, err := security.EncryptSecret("tok123", key)
	if err != nil {
		t.Fatal(err)
	}
	page := models.FacebookPage{PageID: "P1", Name: "Shop A", EncryptedPageToken: enc, IsConnected: true}
	if err := db.Create(&page).Error; err != nil {
		t.Fatal(err)
	}

	fb := &fakeFB{}
	h := &InboxHandler{DB: db, Cfg: &config.Config{JWTSecret: key}, HTTPClient: &http.Client{Transport: fb}}

	t0 := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	payload1 := fbPayload(
		fbThread("T1", "C1", "Lan", 1, t0,
			fbMsg("m2", "P1", "Shop A", "chào bạn", t0), // mới nhất đứng đầu (đúng thứ tự Facebook trả về)
			fbMsg("m1", "C1", "Lan", "xin chào", t0.Add(-time.Minute))),
		fbThread("T2", "C2", "Minh", 0, t0, fbMsg("m3", "C2", "Minh", "còn hàng không", t0)),
	)
	fb.body.Store(payload1)

	// ---- Lần 1: tạo mới ----
	revBefore := inboxrev.Current()
	n, errStr := h.syncSinglePageConversations(page, 25)
	if errStr != "" || n != 2 {
		t.Fatalf("lần 1: synced=%d err=%q", n, errStr)
	}
	if c, m := countRows(db, "conversations"), countRows(db, "messages"); c != 2 || m != 3 {
		t.Fatalf("lần 1: conversations=%d messages=%d, cần 2 và 3", c, m)
	}
	var c1 models.Conversation
	db.Where("thread_id = ?", "T1").First(&c1)
	if c1.CustomerName != "Lan" || c1.LastMessage != "chào bạn" || c1.UnreadCount != 1 || c1.CustomerPSID != "C1" {
		t.Fatalf("lần 1: dữ liệu hội thoại sai: %+v", c1)
	}
	if inboxrev.Current() == revBefore {
		t.Fatal("lần 1: tạo dữ liệu mới nhưng số phiên bản không tăng")
	}

	// ---- Lần 2: y hệt -> KHÔNG ghi gì, số phiên bản đứng yên ----
	time.Sleep(700 * time.Millisecond) // đợi lần tăng trễ 0,4s của lần 1 xong
	cnt.reset()
	revBefore = inboxrev.Current()
	n, errStr = h.syncSinglePageConversations(page, 25)
	if errStr != "" || n != 2 {
		t.Fatalf("lần 2: synced=%d err=%q", n, errStr)
	}
	if w := cnt.writes.Load(); w != 0 {
		t.Fatalf("lần 2: dữ liệu không đổi nhưng vẫn ghi %d lần (trước đây: 1 UPDATE cho mỗi hội thoại mỗi chu kỳ)", w)
	}
	if r := cnt.reads.Load(); r > 2 {
		t.Fatalf("lần 2: %d truy vấn đọc, mong đợi tối đa 2 (trước đây ~12 cho mỗi hội thoại)", r)
	}
	time.Sleep(100 * time.Millisecond)
	if inboxrev.Current() != revBefore {
		t.Fatal("lần 2: không có thay đổi nhưng số phiên bản vẫn tăng -> trình duyệt sẽ tải lại vô ích")
	}

	// ---- Rep chỉnh thông tin chăm sóc khách ----
	db.Exec("UPDATE conversations SET lead_status='follow_up', care_note='gọi lại chiều mai', customer_phone='0912345678' WHERE thread_id='T1'")

	// ---- Lần 3: khách nhắn thêm -> phải ghi, số phiên bản tăng, thông tin chăm sóc KHÔNG bị mất ----
	time.Sleep(700 * time.Millisecond)
	now := time.Now().Truncate(time.Second)
	payload3 := fbPayload(
		fbThread("T1", "C1", "Lan", 2, now,
			fbMsg("m4", "C1", "Lan", "giá bao nhiêu?", now),
			fbMsg("m2", "P1", "Shop A", "chào bạn", t0),
			fbMsg("m1", "C1", "Lan", "xin chào", t0.Add(-time.Minute))),
		fbThread("T2", "C2", "Minh", 0, t0, fbMsg("m3", "C2", "Minh", "còn hàng không", t0)),
	)
	fb.body.Store(payload3)
	cnt.reset()
	revBefore = inboxrev.Current()
	if n, errStr = h.syncSinglePageConversations(page, 25); errStr != "" || n != 2 {
		t.Fatalf("lần 3: synced=%d err=%q", n, errStr)
	}
	if m := countRows(db, "messages"); m != 4 {
		t.Fatalf("lần 3: messages=%d, cần 4", m)
	}
	db.Where("thread_id = ?", "T1").First(&c1)
	if c1.LastMessage != "giá bao nhiêu?" || c1.UnreadCount != 2 {
		t.Fatalf("lần 3: hội thoại chưa cập nhật: last=%q unread=%d", c1.LastMessage, c1.UnreadCount)
	}
	if c1.LeadStatus != "follow_up" || c1.CareNote != "gọi lại chiều mai" || c1.CustomerPhone != "0912345678" {
		t.Fatalf("lần 3: ĐỒNG BỘ ĐÃ GHI ĐÈ thông tin chăm sóc khách: lead=%q note=%q phone=%q", c1.LeadStatus, c1.CareNote, c1.CustomerPhone)
	}
	if w := cnt.writes.Load(); w < 2 || w > 3 {
		t.Fatalf("lần 3: %d lần ghi, mong đợi 2 (1 tin nhắn mới + 1 cập nhật hội thoại)", w)
	}
	if inboxrev.Current() == revBefore {
		t.Fatal("lần 3: có tin mới nhưng số phiên bản không tăng -> trình duyệt sẽ không thấy tin mới")
	}

	// ---- Hội thoại thứ 2 không bị đụng tới ----
	var c2 models.Conversation
	db.Where("thread_id = ?", "T2").First(&c2)
	if c2.LastMessage != "còn hàng không" || c2.UnreadCount != 0 {
		t.Fatalf("hội thoại T2 bị đổi ngoài ý muốn: %+v", c2)
	}
}

// 25 hội thoại × 10 tin nhắn = đúng cỡ một chu kỳ đồng bộ thật (limitThreads=25, messages.limit(10)).
func TestInboxSyncSteadyStateAtScale(t *testing.T) {
	db := openInboxTestDB(t)
	inboxrev.Register(db)
	var cnt stmtCounter
	cnt.attach(db)

	const key = "test-master-key"
	enc, _ := security.EncryptSecret("tok123", key)
	page := models.FacebookPage{PageID: "P1", Name: "Shop A", EncryptedPageToken: enc, IsConnected: true}
	db.Create(&page)

	t0 := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
	threads := make([]string, 0, 25)
	for i := 0; i < 25; i++ {
		msgs := make([]string, 0, 10)
		for j := 0; j < 10; j++ {
			from, name := fmt.Sprintf("C%d", i), fmt.Sprintf("Khách %d", i)
			if j%2 == 0 {
				from, name = "P1", "Shop A"
			}
			msgs = append(msgs, fbMsg(fmt.Sprintf("m_%d_%d", i, j), from, name, fmt.Sprintf("nội dung %d-%d", i, j), t0.Add(time.Duration(-j)*time.Minute)))
		}
		threads = append(threads, fbThread(fmt.Sprintf("T%d", i), fmt.Sprintf("C%d", i), fmt.Sprintf("Khách %d", i), i%3, t0, msgs...))
	}
	fb := &fakeFB{}
	fb.body.Store(fbPayload(threads...))
	h := &InboxHandler{DB: db, Cfg: &config.Config{JWTSecret: key}, HTTPClient: &http.Client{Transport: fb}}

	if n, e := h.syncSinglePageConversations(page, 25); e != "" || n != 25 {
		t.Fatalf("lần 1: synced=%d err=%q", n, e)
	}
	if c, m := countRows(db, "conversations"), countRows(db, "messages"); c != 25 || m != 250 {
		t.Fatalf("lần 1: conversations=%d messages=%d, cần 25 và 250", c, m)
	}

	time.Sleep(700 * time.Millisecond)
	cnt.reset()
	rev := inboxrev.Current()
	if n, e := h.syncSinglePageConversations(page, 25); e != "" || n != 25 {
		t.Fatalf("lần 2: synced=%d err=%q", n, e)
	}
	t.Logf("chu kỳ ổn định (25 hội thoại × 10 tin): %d truy vấn đọc, %d lần ghi", cnt.reads.Load(), cnt.writes.Load())
	if cnt.writes.Load() != 0 || cnt.reads.Load() > 2 {
		t.Fatalf("chu kỳ không có gì mới phải là ≤2 đọc và 0 ghi, thực tế %d đọc %d ghi", cnt.reads.Load(), cnt.writes.Load())
	}
	time.Sleep(100 * time.Millisecond)
	if inboxrev.Current() != rev {
		t.Fatal("không có thay đổi nhưng số phiên bản tăng")
	}
}

func TestInboxRevEndpointReadsMemoryOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &InboxHandler{} // không có DB: nếu endpoint chạm database sẽ panic
	r := gin.New()
	r.GET("/rev", h.InboxRev)

	call := func(url string) int64 {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: code=%d cache=%q", url, w.Code, w.Header().Get("Cache-Control"))
		}
		var out struct {
			Rev int64 `json:"rev"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Rev
	}

	lastInboxUse.Store(0)
	if got := call("/rev?bg=1"); got != inboxrev.Current() {
		t.Fatalf("rev=%d, want %d", got, inboxrev.Current())
	}
	if within(&lastInboxUse, time.Minute) {
		t.Fatal("tab ẩn (bg=1) không được tính là 'đang xem' — nếu không đồng bộ nền chạy mãi")
	}
	call("/rev")
	if !within(&lastInboxUse, time.Minute) {
		t.Fatal("tab đang xem phải đánh dấu 'đang có người dùng' để đồng bộ nền chạy")
	}
	before := inboxrev.Current()
	inboxrev.Bump()
	if got := call("/rev"); got != before+1 {
		t.Fatalf("sau Bump rev=%d, want %d", got, before+1)
	}
}
