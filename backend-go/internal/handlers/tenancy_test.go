package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/middleware"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Môi trường kiểm thử phân quyền: dựng lại đúng chuỗi middleware + route của máy chủ thật (không chạy worker nền)
type tenEnv struct {
	t   *testing.T
	db  *gorm.DB
	r   *gin.Engine
	cfg *config.Config
}

func newTenEnv(t *testing.T) *tenEnv {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("đặt TEST_DATABASE_URL (database tên kết thúc bằng _test) để chạy test tích hợp")
	}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	db.Raw("SELECT current_database()").Scan(&name)
	if !strings.HasSuffix(name, "_test") {
		t.Fatalf("TỪ CHỐI chạy trên database %q: tên phải kết thúc bằng _test", name)
	}
	cfg := &config.Config{JWTSecret: "ten-test-secret", FrontendURL: "http://localhost:4173"}
	_ = db.Exec(`DROP INDEX IF EXISTS idx_competitor_pages_page_id`).Error
	_ = db.Exec(`DROP INDEX IF EXISTS idx_competitor_posts_fb_post_id`).Error
	if err := db.AutoMigrate(&models.User{}, &models.FacebookPage{}, &models.Conversation{}, &conversationWithCRM{}, &models.Message{},
		&models.ScheduledPost{}, &models.CompetitorPage{}, &models.CompetitorPost{}, &models.PageAISetting{}, &SystemAIConfig{},
		&BroadcastCampaign{}, &BroadcastMessage{}); err != nil {
		t.Fatal(err)
	}
	migrateMarketingTables(db)
	sf := NewStorefrontHandler(db, cfg) // migrate bảng website
	db.Exec("TRUNCATE users, facebook_pages, conversations, messages, scheduled_posts, competitor_pages, competitor_posts, page_ai_settings, broadcast_campaigns, broadcast_messages, marketing_subscriptions, marketing_optin_requests, storefronts, products, product_variants RESTART IDENTITY CASCADE")
	middleware.InvalidateAllUsers()

	pageH := &FacebookPageHandler{DB: db, Cfg: cfg, HTTPClient: &http.Client{}}
	inboxH := &InboxHandler{DB: db, Cfg: cfg, HTTPClient: &http.Client{}}
	postH := &PostHandler{DB: db, Cfg: cfg}
	bcH := &BroadcastHandler{DB: db, Cfg: cfg, HTTPClient: &http.Client{}}
	compH := &CompetitorHandler{DB: db, Cfg: cfg, HTTPClient: &http.Client{}}
	aiH := &AIHandler{DB: db, Cfg: cfg, HTTPClient: &http.Client{}}
	authH := NewAuthHandler(db, cfg)
	custH := NewCustomerHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	jwt := middleware.JWTAuth(cfg.JWTSecret)
	tenant := middleware.Tenancy(db, true)
	noView := middleware.Tenancy(db, false)

	r.POST("/api/v1/auth/register", authH.Register)
	r.POST("/api/v1/auth/login", authH.Login)
	r.GET("/api/v1/auth/me", jwt, tenant, authH.Me)

	admin := r.Group("/api/v1/admin", jwt, noView, middleware.AdminOnly())
	admin.GET("/overview", custH.AdminOverview)
	admin.GET("/customers", custH.ListCustomers)
	admin.POST("/customers", custH.CreateCustomer)
	admin.GET("/customers/:id", custH.GetCustomer)
	admin.PATCH("/customers/:id", custH.UpdateCustomer)
	admin.POST("/customers/:id/password", custH.SetCustomerPassword)
	team := r.Group("/api/v1/team", jwt, tenant, middleware.OwnerOnly())
	team.GET("", custH.ListTeam)
	team.POST("", custH.CreateStaff)
	team.PATCH("/:id", custH.UpdateStaff)
	team.POST("/:id/password", custH.SetStaffPassword)
	team.DELETE("/:id", custH.DeleteStaff)

	pages := r.Group("/api/v1/pages", jwt, tenant)
	pw := []gin.HandlerFunc{middleware.RequireFeature(models.FeaturePages), middleware.OwnerOnly()}
	pages.GET("", middleware.RequireFeature(models.FeaturePages, models.FeatureInbox, models.FeatureAI, models.FeaturePosts, models.FeatureBroadcasts, models.FeatureCompetitors), pageH.ListPages)
	pages.POST("/connect", append(pw, pageH.ConnectPage)...)
	pages.PATCH("/:id/toggle", append(pw, pageH.TogglePage)...)
	pages.DELETE("/:id", append(pw, pageH.DisconnectPage)...)

	inbox := r.Group("/api/v1/inbox", jwt, tenant, middleware.RequireFeature(models.FeatureInbox))
	inbox.GET("/conversations", inboxH.ListConversations)
	inbox.GET("/conversations/:id/messages", inboxH.GetMessages)
	inbox.POST("/conversations/:id/messages", inboxH.SendMessage)
	inbox.PUT("/conversations/:id/crm", inboxH.UpdateConversationCRM)

	ai := r.Group("/api/v1/ai", jwt, tenant)
	aiUse := middleware.RequireFeature(models.FeatureAI, models.FeatureInbox)
	ai.GET("/settings", aiUse, aiH.ListAllPageSettings)
	ai.GET("/settings/:page_id", aiUse, aiH.GetPageSetting)
	ai.PUT("/settings/:page_id", aiUse, aiH.SavePageSetting)
	ai.GET("/config", middleware.AdminOnly(), aiH.GetGlobalAIConfig)

	posts := r.Group("/api/v1/posts", jwt, tenant, middleware.RequireFeature(models.FeaturePosts))
	posts.GET("", postH.ListScheduledPosts)
	posts.POST("", postH.CreateScheduledPost)
	posts.DELETE("/:id", postH.DeleteScheduledPost)
	posts.POST("/:id/publish-now", postH.PublishNowPost)

	comp := r.Group("/api/v1/competitors", jwt, tenant, middleware.RequireFeature(models.FeatureCompetitors))
	comp.GET("", compH.ListCompetitors)
	comp.POST("", compH.AddCompetitor)
	comp.DELETE("/:id", compH.DeleteCompetitor)

	bc := r.Group("/api/v1/broadcasts", jwt, tenant, middleware.RequireFeature(models.FeatureBroadcasts))
	bc.GET("", bcH.ListCampaigns)
	bc.POST("", bcH.CreateCampaign)
	bc.PATCH("/:id/status", bcH.ToggleStatus)
	bc.GET("/subscribers", bcH.ListSubscribers)

	sites := r.Group("/api/v1/storefront", jwt, tenant, middleware.RequireFeature(models.FeatureSites))
	sites.GET("/sites", sf.ListSites)
	sites.POST("/sites", middleware.OwnerOnly(), sf.CreateSite)
	sites.GET("/sites/:id", sf.GetSite)

	return &tenEnv{t: t, db: db, r: r, cfg: cfg}
}

// call gửi yêu cầu có token (và header tuỳ chọn)
func (e *tenEnv) call(token, method, path string, body any, hdr ...string) (int, map[string]any) {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out == nil {
		out = map[string]any{}
	}
	return w.Code, out
}

// register tạo tài khoản qua API đăng ký thật; trả về (id, token)
func (e *tenEnv) register(name, email string) (int64, string) {
	e.t.Helper()
	code, out := e.call("", "POST", "/api/v1/auth/register", map[string]any{"full_name": name, "email": email, "password": "secret123"})
	if code != http.StatusCreated {
		e.t.Fatalf("đăng ký %s: %d %v", email, code, out)
	}
	u := out["user"].(map[string]any)
	return int64(u["id"].(float64)), out["token"].(string)
}

func (e *tenEnv) login(email, pw string) (int, string) {
	code, out := e.call("", "POST", "/api/v1/auth/login", map[string]any{"email": email, "password": pw})
	tok, _ := out["token"].(string)
	return code, tok
}

func (e *tenEnv) connectPage(token, pageID, name string) (int, map[string]any) {
	return e.call(token, "POST", "/api/v1/pages/connect", map[string]any{"page_id": pageID, "name": name, "access_token": "EAAfaketoken" + pageID})
}

func (e *tenEnv) pageCount(token string) int {
	code, out := e.call(token, "GET", "/api/v1/pages", nil)
	if code != 200 {
		e.t.Fatalf("GET /pages: %d %v", code, out)
	}
	return len(out["pages"].([]any))
}

func (e *tenEnv) addConversation(ws int64, pageID, psid string) uint {
	cv := models.Conversation{WorkspaceID: uint(ws), PageID: pageID, PageName: "P", ThreadID: "t_" + psid + pageID, CustomerPSID: psid, CustomerName: "Khách " + psid, LastMessage: "xin chào"}
	if err := e.db.Create(&cv).Error; err != nil {
		e.t.Fatal(err)
	}
	e.db.Create(&models.Message{ConversationID: cv.ID, FBMessageID: fmt.Sprintf("m_%d", cv.ID), SenderType: "customer", Content: "xin chào", SentAt: cv.LastMessageTime})
	return cv.ID
}

func listLen(out map[string]any, key string) int {
	l, _ := out[key].([]any)
	return len(l)
}

// ----------------------------------------------------------------- Đăng ký / vai trò

func TestRegistrationCreatesAdminThenEmptyCustomers(t *testing.T) {
	e := newTenEnv(t)
	adminID, adminTok := e.register("Admin", "admin@x.vn")
	aID, aTok := e.register("Khách A", "a@x.vn")

	var admin, a models.User
	e.db.First(&admin, adminID)
	e.db.First(&a, aID)
	if admin.SystemRole != models.RoleAdmin || a.SystemRole != models.RoleCustomer {
		t.Fatalf("vai trò: %s / %s", admin.SystemRole, a.SystemRole)
	}
	if len(a.Features) != len(models.AllFeatures) || a.OwnerID != nil {
		t.Errorf("khách mới phải được dùng đủ trang, không có chủ: %+v", a.Features)
	}
	// Dữ liệu trống: khách hàng mới không thấy gì của người khác
	e.connectPage(adminTok, "PAGE_ADMIN", "Page của admin")
	e.addConversation(adminID, "PAGE_ADMIN", "psid1")
	if e.pageCount(aTok) != 0 {
		t.Error("khách mới phải thấy 0 Fanpage")
	}
	code, out := e.call(aTok, "GET", "/api/v1/inbox/conversations", nil)
	if code != 200 || listLen(out, "conversations") != 0 || listLen(out, "pages") != 0 {
		t.Errorf("hộp thư của khách mới phải trống: %d %v", code, out)
	}
	for _, p := range []string{"/api/v1/posts", "/api/v1/competitors", "/api/v1/broadcasts", "/api/v1/storefront/sites"} {
		if code, out := e.call(aTok, "GET", p, nil); code != 200 {
			t.Errorf("%s: %d %v", p, code, out)
		}
	}
	code, me := e.call(aTok, "GET", "/api/v1/auth/me", nil)
	tn := me["tenant"].(map[string]any)
	if code != 200 || int64(tn["workspace_id"].(float64)) != aID || tn["is_owner"] != true || len(tn["features"].([]any)) != len(models.AllFeatures) {
		t.Errorf("me: %v", me)
	}
	if code, _ := e.call("", "GET", "/api/v1/pages", nil); code != 401 {
		t.Errorf("không token phải 401, nhận %d", code)
	}
}

// ----------------------------------------------------------------- Cô lập dữ liệu giữa các khách hàng

func TestCustomersCannotSeeOrTouchEachOthersData(t *testing.T) {
	e := newTenEnv(t)
	e.register("Admin", "admin@x.vn")
	aID, aTok := e.register("Khách A", "a@x.vn")
	bID, bTok := e.register("Khách B", "b@x.vn")

	// Fanpage
	if code, out := e.connectPage(aTok, "PAGE_A", "Page A"); code != 201 {
		t.Fatalf("A kết nối: %d %v", code, out)
	}
	if code, out := e.connectPage(bTok, "PAGE_B", "Page B"); code != 201 {
		t.Fatalf("B kết nối: %d %v", code, out)
	}
	if e.pageCount(aTok) != 1 || e.pageCount(bTok) != 1 {
		t.Error("mỗi khách chỉ thấy Fanpage của mình")
	}
	if code, out := e.connectPage(bTok, "PAGE_A", "Cướp page"); code != http.StatusConflict {
		t.Errorf("B kết nối Fanpage của A phải 409: %d %v", code, out)
	}
	var pa models.FacebookPage
	e.db.Where("page_id = ?", "PAGE_A").First(&pa)
	if int64(pa.WorkspaceID) != aID {
		t.Fatalf("Fanpage A thuộc workspace %d, muốn %d", pa.WorkspaceID, aID)
	}
	if code, _ := e.call(bTok, "PATCH", fmt.Sprintf("/api/v1/pages/%d/toggle", pa.ID), nil); code != 404 {
		t.Errorf("B bật/tắt Page của A: %d", code)
	}
	if code, _ := e.call(bTok, "DELETE", fmt.Sprintf("/api/v1/pages/%d", pa.ID), nil); code != 200 {
		t.Logf("xoá Page của A bởi B trả %d (không được xoá thật)", code)
	}
	var still int64
	e.db.Model(&models.FacebookPage{}).Where("page_id = ?", "PAGE_A").Count(&still)
	if still != 1 {
		t.Fatal("B đã xoá được Fanpage của A")
	}

	// Hộp thư
	convA := e.addConversation(aID, "PAGE_A", "psA")
	e.addConversation(bID, "PAGE_B", "psB")
	code, out := e.call(aTok, "GET", "/api/v1/inbox/conversations", nil)
	if code != 200 || listLen(out, "conversations") != 1 || listLen(out, "pages") != 1 {
		t.Errorf("hộp thư A: %v", out)
	}
	code, out = e.call(bTok, "GET", "/api/v1/inbox/conversations", nil)
	if listLen(out, "conversations") != 1 || out["conversations"].([]any)[0].(map[string]any)["customer_name"] != "Khách psB" {
		t.Errorf("hộp thư B lẫn dữ liệu của A: %v", out)
	}
	if code, _ := e.call(bTok, "GET", fmt.Sprintf("/api/v1/inbox/conversations/%d/messages", convA), nil); code != 404 {
		t.Errorf("B đọc tin nhắn của A: %d", code)
	}
	if code, _ := e.call(bTok, "POST", fmt.Sprintf("/api/v1/inbox/conversations/%d/messages", convA), map[string]any{"content": "hi"}); code != 404 {
		t.Errorf("B gửi tin vào hội thoại của A: %d", code)
	}
	if code, _ := e.call(bTok, "PUT", fmt.Sprintf("/api/v1/inbox/conversations/%d/crm", convA), map[string]any{"care_note": "hack"}); code != 404 {
		t.Errorf("B sửa CRM của A: %d", code)
	}
	if code, _ := e.call(aTok, "GET", fmt.Sprintf("/api/v1/inbox/conversations/%d/messages", convA), nil); code != 200 {
		t.Errorf("A đọc hội thoại của mình: %d", code)
	}

	// Cài đặt AI theo Fanpage
	if code, _ := e.call(bTok, "GET", "/api/v1/ai/settings/PAGE_A", nil); code != 404 {
		t.Errorf("B đọc cài đặt AI của Page A: %d", code)
	}
	if code, _ := e.call(bTok, "PUT", "/api/v1/ai/settings/PAGE_A", map[string]any{"enabled": true, "tone": "x"}); code != 404 {
		t.Errorf("B ghi cài đặt AI của Page A: %d", code)
	}
	if code, _ := e.call(aTok, "PUT", "/api/v1/ai/settings/PAGE_A", map[string]any{"enabled": true, "tone": "thân thiện"}); code != 200 {
		t.Errorf("A ghi cài đặt AI của mình: %d", code)
	}
	if _, out := e.call(bTok, "GET", "/api/v1/ai/settings", nil); listLen(out, "settings") != 0 {
		t.Errorf("B thấy cài đặt AI của A: %v", out)
	}
	if _, out := e.call(aTok, "GET", "/api/v1/ai/settings", nil); listLen(out, "settings") != 1 {
		t.Errorf("A phải thấy cài đặt AI của mình: %v", out)
	}
	// khoá AI dùng chung: chỉ admin
	if code, _ := e.call(aTok, "GET", "/api/v1/ai/config", nil); code != 403 {
		t.Errorf("khách đọc cấu hình AI chung: %d", code)
	}

	// Bài đăng: chỉ đăng lên Fanpage của mình
	code, out = e.call(aTok, "POST", "/api/v1/posts", map[string]any{"content": "Khuyến mãi", "page_ids": []string{"PAGE_A", "PAGE_B"}, "scheduled_at": "2099-01-01T10:00:00Z"})
	if code != 201 {
		t.Fatalf("A lên lịch: %d %v", code, out)
	}
	post := out["post"].(map[string]any)
	if !strings.Contains(post["page_ids_json"].(string), "PAGE_A") || strings.Contains(post["page_ids_json"].(string), "PAGE_B") {
		t.Errorf("bài đăng của A dính Fanpage của B: %v", post["page_ids_json"])
	}
	if code, _ := e.call(bTok, "POST", "/api/v1/posts", map[string]any{"content": "x", "page_ids": []string{"PAGE_A"}, "scheduled_at": "2099-01-01T10:00:00Z"}); code != 400 {
		t.Errorf("B đăng bài lên Page của A: %d", code)
	}
	if _, out := e.call(bTok, "GET", "/api/v1/posts", nil); listLen(out, "posts") != 0 {
		t.Errorf("B thấy bài của A: %v", out)
	}
	postID := int(post["id"].(float64))
	if code, _ := e.call(bTok, "POST", fmt.Sprintf("/api/v1/posts/%d/publish-now", postID), nil); code != 404 {
		t.Errorf("B đăng ngay bài của A: %d", code)
	}
	e.call(bTok, "DELETE", fmt.Sprintf("/api/v1/posts/%d", postID), nil)
	if _, out := e.call(aTok, "GET", "/api/v1/posts", nil); listLen(out, "posts") != 1 {
		t.Error("B đã xoá được bài của A")
	}

	// Chiến dịch chăm sóc khách
	if code, _ := e.call(bTok, "POST", "/api/v1/broadcasts", map[string]any{"page_id": "PAGE_A", "name": "c", "templates": []string{"hi"}}); code != 404 {
		t.Errorf("B chạy chiến dịch trên Page của A: %d", code)
	}
	e.db.Create(&BroadcastCampaign{PageID: "PAGE_A", Name: "Của A", TemplatesJSON: "[]", Status: "running"})
	if _, out := e.call(bTok, "GET", "/api/v1/broadcasts", nil); listLen(out, "data") != 0 {
		t.Errorf("B thấy chiến dịch của A: %v", out)
	}
	if _, out := e.call(aTok, "GET", "/api/v1/broadcasts", nil); listLen(out, "data") != 1 {
		t.Errorf("A phải thấy chiến dịch của mình: %v", out)
	}
	var camp BroadcastCampaign
	e.db.First(&camp)
	e.call(bTok, "PATCH", fmt.Sprintf("/api/v1/broadcasts/%d/status", camp.ID), map[string]any{"status": "paused"})
	e.db.First(&camp, camp.ID)
	if camp.Status != "running" {
		t.Error("B đã tạm dừng được chiến dịch của A")
	}

	// Đối thủ: mỗi khách có danh sách riêng, kể cả theo dõi cùng một kênh
	for _, tok := range []string{aTok, bTok} {
		if code, out := e.call(tok, "POST", "/api/v1/competitors", map[string]any{"platform": "facebook", "page_input": "123456789", "name": "Đối thủ chung"}); code != 201 {
			t.Fatalf("thêm đối thủ: %d %v", code, out)
		}
	}
	_, outA := e.call(aTok, "GET", "/api/v1/competitors", nil)
	_, outB := e.call(bTok, "GET", "/api/v1/competitors", nil)
	if listLen(outA, "competitors") != 1 || listLen(outB, "competitors") != 1 {
		t.Errorf("mỗi khách phải có 1 đối thủ riêng: %d / %d", listLen(outA, "competitors"), listLen(outB, "competitors"))
	}
	compA := outA["competitors"].([]any)[0].(map[string]any)
	if code, _ := e.call(bTok, "DELETE", fmt.Sprintf("/api/v1/competitors/%d", int(compA["id"].(float64))), nil); code != 200 {
		t.Logf("DELETE đối thủ của A bởi B: %d", code)
	}
	if _, out := e.call(aTok, "GET", "/api/v1/competitors", nil); listLen(out, "competitors") != 1 {
		t.Error("B đã xoá được đối thủ của A")
	}
}

// ----------------------------------------------------------------- Trang được dùng + giới hạn

func TestAdminControlsFeaturesLimitsAndAccountLock(t *testing.T) {
	e := newTenEnv(t)
	_, adminTok := e.register("Admin", "admin@x.vn")
	aID, aTok := e.register("Khách A", "a@x.vn")

	// khách hàng không vào được khu quản trị
	for _, p := range []string{"/api/v1/admin/customers", "/api/v1/admin/overview"} {
		if code, _ := e.call(aTok, "GET", p, nil); code != 403 {
			t.Errorf("khách vào %s: %d", p, code)
		}
	}
	// tắt trang Hộp thư + Đăng bài
	code, out := e.call(adminTok, "PATCH", fmt.Sprintf("/api/v1/admin/customers/%d", aID), map[string]any{"features": []string{"pages", "sites", "competitors"}})
	if code != 200 {
		t.Fatalf("admin sửa: %d %v", code, out)
	}
	if code, out := e.call(aTok, "GET", "/api/v1/inbox/conversations", nil); code != 403 || out["code"] != "feature_disabled" {
		t.Errorf("hộp thư đã tắt: %d %v", code, out)
	}
	if code, _ := e.call(aTok, "GET", "/api/v1/posts", nil); code != 403 {
		t.Errorf("đăng bài đã tắt: %d", code)
	}
	if code, _ := e.call(aTok, "GET", "/api/v1/competitors", nil); code != 200 {
		t.Errorf("đối thủ vẫn bật: %d", code)
	}
	if code, _ := e.call(aTok, "GET", "/api/v1/pages", nil); code != 200 {
		t.Errorf("Fanpage vẫn bật: %d", code)
	}
	_, me := e.call(aTok, "GET", "/api/v1/auth/me", nil)
	if f := me["tenant"].(map[string]any)["features"].([]any); len(f) != 3 {
		t.Errorf("me.features: %v", f)
	}

	// giới hạn số Fanpage = 1
	e.call(adminTok, "PATCH", fmt.Sprintf("/api/v1/admin/customers/%d", aID), map[string]any{"max_pages": 1, "max_sites": 1})
	if code, _ := e.connectPage(aTok, "P1", "Page 1"); code != 201 {
		t.Fatalf("Page 1: %d", code)
	}
	if code, out := e.connectPage(aTok, "P2", "Page 2"); code != 403 || out["code"] != "limit_reached" {
		t.Errorf("vượt giới hạn Fanpage: %d %v", code, out)
	}
	if code, _ := e.connectPage(aTok, "P1", "Page 1 đổi tên"); code != 200 {
		t.Errorf("cập nhật lại Page đã có không bị tính vào giới hạn: %d", code)
	}
	if code, _ := e.call(aTok, "POST", "/api/v1/storefront/sites", map[string]any{"site_title": "Shop 1"}); code != 201 {
		t.Fatalf("site 1: %d", code)
	}
	if code, out := e.call(aTok, "POST", "/api/v1/storefront/sites", map[string]any{"site_title": "Shop 2"}); code != 403 {
		t.Errorf("vượt giới hạn website: %d %v", code, out)
	}

	// danh sách khách + số liệu
	code, out = e.call(adminTok, "GET", "/api/v1/admin/customers", nil)
	rows := out["customers"].([]any)
	if code != 200 || len(rows) != 1 {
		t.Fatalf("danh sách: %d %v", code, out)
	}
	st := rows[0].(map[string]any)["stats"].(map[string]any)
	if st["pages"].(float64) != 1 || st["sites"].(float64) != 1 {
		t.Errorf("số liệu: %v", st)
	}
	// đặt lại mật khẩu
	if code, _ := e.call(adminTok, "POST", fmt.Sprintf("/api/v1/admin/customers/%d/password", aID), map[string]any{"password": "matkhau-moi"}); code != 200 {
		t.Fatal("đặt lại mật khẩu")
	}
	if c, _ := e.login("a@x.vn", "secret123"); c != 401 {
		t.Error("mật khẩu cũ vẫn dùng được")
	}
	if c, _ := e.login("a@x.vn", "matkhau-moi"); c != 200 {
		t.Error("mật khẩu mới không đăng nhập được")
	}
	// khoá tài khoản: token cũ mất hiệu lực ngay
	e.call(adminTok, "PATCH", fmt.Sprintf("/api/v1/admin/customers/%d", aID), map[string]any{"is_active": false})
	if code, _ := e.call(aTok, "GET", "/api/v1/pages", nil); code != 401 {
		t.Errorf("tài khoản bị khoá phải 401: %d", code)
	}
	if c, _ := e.login("a@x.vn", "matkhau-moi"); c != 401 {
		t.Error("tài khoản bị khoá không được đăng nhập")
	}
	// không thể sửa admin qua API này
	var adminU models.User
	e.db.Where("system_role = ?", models.RoleAdmin).First(&adminU)
	if code, _ := e.call(adminTok, "PATCH", fmt.Sprintf("/api/v1/admin/customers/%d", adminU.ID), map[string]any{"is_active": false}); code != 403 {
		t.Errorf("sửa admin: %d", code)
	}
}

// ----------------------------------------------------------------- Nhân viên của khách hàng

func TestStaffWorkInsideTheirCustomerWorkspace(t *testing.T) {
	e := newTenEnv(t)
	_, adminTok := e.register("Admin", "admin@x.vn")
	aID, aTok := e.register("Khách A", "a@x.vn")
	_, bTok := e.register("Khách B", "b@x.vn")
	e.connectPage(aTok, "PAGE_A", "Page A")
	e.connectPage(bTok, "PAGE_B", "Page B")
	e.addConversation(aID, "PAGE_A", "psA")
	e.db.Model(&models.User{}).Where("id = ?", aID).Update("max_staff", 1)
	middleware.InvalidateAllUsers()

	code, out := e.call(aTok, "POST", "/api/v1/team", map[string]any{"full_name": "Nhân viên 1", "email": "s1@x.vn", "password": "matkhau1", "features": []string{"inbox", "posts", "sites"}})
	if code != 201 {
		t.Fatalf("tạo nhân viên: %d %v", code, out)
	}
	staffID := int64(out["staff"].(map[string]any)["id"].(float64))
	if code, out := e.call(aTok, "POST", "/api/v1/team", map[string]any{"full_name": "NV2", "email": "s2@x.vn", "password": "matkhau2"}); code != 403 || out["code"] != "limit_reached" {
		t.Errorf("vượt giới hạn nhân viên: %d %v", code, out)
	}
	if code, _ := e.call(aTok, "POST", "/api/v1/team", map[string]any{"full_name": "Trùng", "email": "s1@x.vn", "password": "matkhau2"}); code == 201 {
		t.Error("email trùng vẫn tạo được")
	}
	_, sTok := func() (int, string) { return e.login("s1@x.vn", "matkhau1") }()
	if sTok == "" {
		t.Fatal("nhân viên không đăng nhập được")
	}

	// nhân viên thấy dữ liệu của khách hàng A, không thấy của B
	code, out = e.call(sTok, "GET", "/api/v1/inbox/conversations", nil)
	if code != 200 || listLen(out, "conversations") != 1 || listLen(out, "pages") != 1 {
		t.Errorf("hộp thư nhân viên: %d %v", code, out)
	}
	code, me := e.call(sTok, "GET", "/api/v1/auth/me", nil)
	tn := me["tenant"].(map[string]any)
	if code != 200 || int64(tn["workspace_id"].(float64)) != aID || tn["is_owner"] != false || tn["owner_name"] != "Khách A" {
		t.Errorf("me của nhân viên: %v", me)
	}
	// trang không được cấp / việc chỉ chủ tài khoản
	if code, _ := e.call(sTok, "GET", "/api/v1/competitors", nil); code != 403 {
		t.Errorf("đối thủ không được cấp: %d", code)
	}
	if code, out := e.connectPage(sTok, "PAGE_S", "x"); code != 403 {
		t.Errorf("nhân viên kết nối Fanpage: %d %v", code, out)
	}
	if code, _ := e.call(sTok, "GET", "/api/v1/team", nil); code != 403 {
		t.Errorf("nhân viên xem /team: %d", code)
	}
	if code, out := e.call(sTok, "POST", "/api/v1/storefront/sites", map[string]any{"site_title": "x"}); code != 403 || out["code"] != "owner_only" {
		t.Errorf("nhân viên tạo website: %d %v", code, out)
	}
	if code, _ := e.call(sTok, "GET", "/api/v1/storefront/sites", nil); code != 200 {
		t.Errorf("nhân viên xem website của chủ: %d", code)
	}
	// không thể tự cấp thêm trang: chủ chỉ cấp được trang mà chính mình có
	e.call(adminTok, "PATCH", fmt.Sprintf("/api/v1/admin/customers/%d", aID), map[string]any{"features": []string{"inbox", "pages"}})
	if code, _ := e.call(sTok, "GET", "/api/v1/posts", nil); code != 403 {
		t.Errorf("chủ bị tắt posts thì nhân viên cũng mất: %d", code)
	}
	if code, _ := e.call(sTok, "GET", "/api/v1/inbox/conversations", nil); code != 200 {
		t.Errorf("inbox còn bật: %d", code)
	}
	// B không quản lý được nhân viên của A
	if code, _ := e.call(bTok, "PATCH", fmt.Sprintf("/api/v1/team/%d", staffID), map[string]any{"is_active": false}); code != 404 {
		t.Errorf("B sửa nhân viên của A: %d", code)
	}
	// khoá nhân viên
	e.call(aTok, "PATCH", fmt.Sprintf("/api/v1/team/%d", staffID), map[string]any{"is_active": false})
	if code, _ := e.call(sTok, "GET", "/api/v1/inbox/conversations", nil); code != 401 {
		t.Errorf("nhân viên bị khoá: %d", code)
	}
	e.call(aTok, "PATCH", fmt.Sprintf("/api/v1/team/%d", staffID), map[string]any{"is_active": true})
	// khoá khách hàng chủ → nhân viên cũng bị chặn
	e.call(adminTok, "PATCH", fmt.Sprintf("/api/v1/admin/customers/%d", aID), map[string]any{"is_active": false})
	if code, out := e.call(sTok, "GET", "/api/v1/inbox/conversations", nil); code != 403 {
		t.Errorf("chủ bị khoá thì nhân viên bị chặn: %d %v", code, out)
	}
	e.call(adminTok, "PATCH", fmt.Sprintf("/api/v1/admin/customers/%d", aID), map[string]any{"is_active": true})
	// xoá nhân viên
	if code, _ := e.call(aTok, "DELETE", fmt.Sprintf("/api/v1/team/%d", staffID), nil); code != 200 {
		t.Error("xoá nhân viên")
	}
	if code, _ := e.call(sTok, "GET", "/api/v1/inbox/conversations", nil); code != 401 {
		t.Errorf("nhân viên đã xoá: %d", code)
	}
}

// ----------------------------------------------------------------- Admin "xem như khách"

func TestAdminViewAsCustomerIsReadOnly(t *testing.T) {
	e := newTenEnv(t)
	adminID, adminTok := e.register("Admin", "admin@x.vn")
	aID, aTok := e.register("Khách A", "a@x.vn")
	e.connectPage(aTok, "PAGE_A", "Page A")
	e.connectPage(adminTok, "PAGE_ADMIN", "Page admin")
	e.addConversation(aID, "PAGE_A", "psA")
	e.addConversation(adminID, "PAGE_ADMIN", "psAdmin")
	view := []string{"X-Workspace", fmt.Sprint(aID)}

	// không có header: admin thấy dữ liệu của chính mình
	if _, out := e.call(adminTok, "GET", "/api/v1/inbox/conversations", nil); out["conversations"].([]any)[0].(map[string]any)["customer_name"] != "Khách psAdmin" {
		t.Errorf("admin không header phải thấy dữ liệu của mình: %v", out)
	}
	// có header: thấy đúng dữ liệu của khách A
	code, out := e.call(adminTok, "GET", "/api/v1/inbox/conversations", nil, view...)
	if code != 200 || listLen(out, "conversations") != 1 || out["conversations"].([]any)[0].(map[string]any)["customer_name"] != "Khách psA" {
		t.Errorf("xem như khách A: %d %v", code, out)
	}
	if _, out := e.call(adminTok, "GET", "/api/v1/pages", nil, view...); listLen(out, "pages") != 1 || out["pages"].([]any)[0].(map[string]any)["page_id"] != "PAGE_A" {
		t.Errorf("Fanpage khi xem như khách: %v", out)
	}
	_, me := e.call(adminTok, "GET", "/api/v1/auth/me", nil, view...)
	tn := me["tenant"].(map[string]any)
	if tn["viewing"] != true || int64(tn["workspace_id"].(float64)) != aID {
		t.Errorf("me khi xem như khách: %v", tn)
	}
	// chỉ đọc: mọi thao tác ghi bị chặn
	code, out = e.call(adminTok, "POST", "/api/v1/pages/connect", map[string]any{"page_id": "X", "name": "x", "access_token": "t"}, view...)
	if code != 403 || out["code"] != "view_only" {
		t.Errorf("ghi khi xem như khách: %d %v", code, out)
	}
	if code, _ := e.call(adminTok, "PUT", "/api/v1/ai/settings/PAGE_A", map[string]any{"enabled": true}, view...); code != 403 {
		t.Errorf("sửa khi xem như khách: %d", code)
	}
	// khách hàng không thể dùng header để xem người khác; header bị bỏ qua
	bID, bTok := e.register("Khách B", "b@x.vn")
	e.addConversation(bID, "PAGE_B", "psB")
	code, out = e.call(bTok, "GET", "/api/v1/inbox/conversations", nil, "X-Workspace", fmt.Sprint(aID))
	if code != 200 || listLen(out, "conversations") != 1 || out["conversations"].([]any)[0].(map[string]any)["customer_name"] != "Khách psB" {
		t.Errorf("khách dùng X-Workspace phải bị bỏ qua: %v", out)
	}
	// khu quản trị bỏ qua header (không bị chế độ chỉ đọc)
	if code, _ := e.call(adminTok, "PATCH", fmt.Sprintf("/api/v1/admin/customers/%d", aID), map[string]any{"max_pages": 3}, view...); code != 200 {
		t.Errorf("admin sửa khách khi còn header: %d", code)
	}
	if code, _ := e.call(adminTok, "GET", "/api/v1/inbox/conversations", nil, "X-Workspace", "999999"); code != 404 {
		t.Errorf("xem khách không tồn tại: %d", code)
	}
	// xem chi tiết một khách
	code, out = e.call(adminTok, "GET", fmt.Sprintf("/api/v1/admin/customers/%d", aID), nil)
	if code != 200 || listLen(out, "pages") != 1 {
		t.Errorf("chi tiết khách: %d %v", code, out)
	}
	if code, out = e.call(adminTok, "GET", "/api/v1/admin/overview", nil); code != 200 || out["customers"].(float64) != 2 {
		t.Errorf("tổng quan admin: %d %v", code, out)
	}
}

// ----------------------------------------------------------------- Chuyển dữ liệu cũ

func TestMigrateTenancyAssignsSharedDataToAdminAndNormalisesRoles(t *testing.T) {
	e := newTenEnv(t)
	hash, _ := utils.HashPassword("x12345")
	mk := func(email, role string) models.User {
		u := models.User{FullName: email, Email: email, PasswordHash: hash, AuthProvider: "local", SystemRole: role, IsActive: true}
		e.db.Create(&u)
		return u
	}
	admin := mk("admin@x.vn", "admin")
	mgr := mk("m@x.vn", "manager")
	staff := mk("s@x.vn", "staff")
	social := mk("g@x.vn", "user")
	e.db.Model(&models.User{}).Where("1 = 1").UpdateColumn("features", nil)

	// dữ liệu dùng chung kiểu cũ (chưa có chủ)
	e.db.Create(&models.FacebookPage{PageID: "OLD_PAGE", Name: "Cũ", EncryptedPageToken: "x"})
	e.db.Exec(`UPDATE facebook_pages SET workspace_id = 0`)
	e.db.Exec(`INSERT INTO conversations (page_id, thread_id, customer_ps_id, customer_name, workspace_id, created_at, updated_at, last_message_time) VALUES ('OLD_PAGE', 't1', 'p1', 'Khách cũ', 0, now(), now(), now())`)
	e.db.Exec(`INSERT INTO conversations (page_id, thread_id, customer_ps_id, customer_name, workspace_id, created_at, updated_at, last_message_time) VALUES ('GONE_PAGE', 't2', 'p2', 'Khách mồ côi', 0, now(), now(), now())`)
	e.db.Create(&models.ScheduledPost{Content: "bài cũ", PageIDsJSON: `["OLD_PAGE"]`, Status: "pending"})
	e.db.Exec(`UPDATE scheduled_posts SET workspace_id = 0`)
	e.db.Create(&models.CompetitorPage{PageID: "facebook:1", Name: "ĐT cũ"})
	e.db.Exec(`UPDATE competitor_pages SET workspace_id = 0`)

	MigrateTenancy(e.db)
	MigrateTenancy(e.db) // chạy lại không đổi gì

	var users []models.User
	e.db.Order("id").Find(&users)
	roles := map[string]string{}
	for _, u := range users {
		roles[u.Email] = u.SystemRole
		if u.Features == nil || len(u.Features) != len(models.AllFeatures) {
			t.Errorf("%s chưa có danh sách trang: %v", u.Email, u.Features)
		}
	}
	if roles["admin@x.vn"] != "admin" || roles["m@x.vn"] != "customer" || roles["s@x.vn"] != "customer" || roles["g@x.vn"] != "customer" {
		t.Errorf("vai trò sau chuyển đổi: %v", roles)
	}
	_, _, _ = mgr, staff, social
	var pg models.FacebookPage
	e.db.Where("page_id = ?", "OLD_PAGE").First(&pg)
	if int64(pg.WorkspaceID) != admin.ID {
		t.Errorf("Fanpage cũ phải thuộc admin: %d", pg.WorkspaceID)
	}
	var cs []models.Conversation
	e.db.Find(&cs)
	for _, c := range cs {
		if int64(c.WorkspaceID) != admin.ID {
			t.Errorf("hội thoại %s thuộc %d, muốn admin %d", c.CustomerName, c.WorkspaceID, admin.ID)
		}
	}
	var post models.ScheduledPost
	var comp models.CompetitorPage
	e.db.First(&post)
	e.db.First(&comp)
	if int64(post.WorkspaceID) != admin.ID || int64(comp.WorkspaceID) != admin.ID {
		t.Errorf("bài đăng / đối thủ cũ phải thuộc admin: %d %d", post.WorkspaceID, comp.WorkspaceID)
	}
	// khách hàng cũ (manager→customer) bắt đầu với dữ liệu trống
	tok, _ := utils.GenerateJWT(mgr.ID, "customer", e.cfg.JWTSecret)
	if e.pageCount(tok) != 0 {
		t.Error("khách hàng cũ không được thấy dữ liệu đã giao cho admin")
	}
	adminTok, _ := utils.GenerateJWT(admin.ID, "admin", e.cfg.JWTSecret)
	if e.pageCount(adminTok) != 1 {
		t.Error("admin phải thấy Fanpage cũ")
	}
}

// Hội thoại mới từ webhook thuộc đúng chủ Fanpage
func TestWebhookConversationBelongsToPageOwner(t *testing.T) {
	e := newTenEnv(t)
	e.register("Admin", "admin@x.vn")
	aID, aTok := e.register("Khách A", "a@x.vn")
	e.connectPage(aTok, "PAGE_A", "Page A")
	var page models.FacebookPage
	e.db.Where("page_id = ?", "PAGE_A").First(&page)
	wh := &WebhookHandler{DB: e.db, Cfg: e.cfg, HTTPClient: &http.Client{}, inFlight: map[uint]bool{}, cooldownMap: nil}
	wh.handleCustomerInteraction(page, "psid-9999", "", "", "", "", "")
	var cv models.Conversation
	if e.db.Where("page_id = ?", "PAGE_A").First(&cv).Error != nil {
		t.Fatal("webhook chưa tạo hội thoại")
	}
	if int64(cv.WorkspaceID) != aID {
		t.Errorf("hội thoại webhook thuộc %d, muốn %d", cv.WorkspaceID, aID)
	}
	code, out := e.call(aTok, "GET", "/api/v1/inbox/conversations", nil)
	if code != 200 || listLen(out, "conversations") != 1 {
		t.Errorf("A phải thấy hội thoại mới: %v", out)
	}
}

// Danh sách rỗng phải là mảng [] (null làm giao diện báo lỗi)
func TestAdminAndTeamListsAreArraysNotNull(t *testing.T) {
	e := newTenEnv(t)
	_, adminTok := e.register("Admin", "admin@x.vn")
	aID, aTok := e.register("Khách A", "a@x.vn")
	_, out := e.call(adminTok, "GET", fmt.Sprintf("/api/v1/admin/customers/%d", aID), nil)
	for _, k := range []string{"staff", "pages", "sites"} {
		if _, ok := out[k].([]any); !ok {
			t.Errorf("customers/:id %s phải là mảng, nhận %T", k, out[k])
		}
	}
	_, out = e.call(aTok, "GET", "/api/v1/team", nil)
	if _, ok := out["staff"].([]any); !ok {
		t.Errorf("team.staff phải là mảng, nhận %T", out["staff"])
	}
}
