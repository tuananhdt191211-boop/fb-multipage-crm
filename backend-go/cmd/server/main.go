package main

import (
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/database"
	"fb-multipage-crm/internal/handlers"
	"fb-multipage-crm/internal/inboxrev"
	"fb-multipage-crm/internal/middleware"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func isOriginAllowed(origin, primaryFrontend string) bool {
	if origin == "" {
		return false
	}
	cleanOrigin := strings.TrimRight(strings.TrimSpace(origin), "/")
	if primaryFrontend != "" && cleanOrigin == primaryFrontend {
		return true
	}
	// Cho phép chạy local dev ở localhost / 127.0.0.1
	if strings.HasPrefix(cleanOrigin, "http://localhost:") ||
		strings.HasPrefix(cleanOrigin, "http://127.0.0.1:") ||
		cleanOrigin == "http://localhost" ||
		cleanOrigin == "http://127.0.0.1" {
		return true
	}
	return false
}

func main() {
	cfg := config.Load()

	// Mở cổng HTTP NGAY (chưa cần database). Nếu database đang lỗi (vd. Neon hết hạn mức) thì mọi API
	// trả 503 kèm lý do rõ ràng (có CORS nên giao diện hiện được thông báo thay vì lỗi mạng khó hiểu),
	// và máy chủ tự hồi phục khi database sống lại — không cần deploy lại.
	var current atomic.Pointer[gin.Engine]
	var lastDBError atomic.Value // string
	current.Store(maintenanceEngine(cfg, &lastDBError))

	go func() {
		db := database.ConnectWithRetry(cfg.DatabaseURL, func(err error, attempt int) {
			msg := database.ExplainError(err)
			lastDBError.Store(msg)
			if attempt <= 2 || attempt%5 == 0 {
				log.Printf("[DB] Chưa kết nối được database (lần %d):\n%s", attempt, msg)
			}
		})
		current.Store(buildRouter(cfg, db))
		log.Println("[DB] Database sẵn sàng — API đầy đủ đã hoạt động")
	}()

	log.Printf("Backend Golang đang chạy tại http://localhost:%s", cfg.Port)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { current.Load().ServeHTTP(w, r) }),
		ReadHeaderTimeout: 15 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// buildRouter dựng toàn bộ API (chỉ gọi khi database đã sẵn sàng).
func buildRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	// Mọi thay đổi ở bảng hộp thư tự tăng "số phiên bản" trong RAM (xem inboxrev)
	inboxrev.Register(db)

	authHandler := handlers.NewAuthHandler(db, cfg)
	postHandler := handlers.NewPostHandler(db, cfg)
	pageHandler := handlers.NewFacebookPageHandler(db, cfg)
	aiHandler := handlers.NewAIHandler(db, cfg)
	postAIHandler := handlers.NewPostAIHandler(db, cfg, aiHandler)
	videoDownloader := handlers.NewVideoDownloaderHandler()

	storefrontHandler := handlers.NewStorefrontHandler(db, cfg)
	storefrontHandler.AI = aiHandler
	storefrontHandler.RunAnalyticsJanitor() // dọn thống kê cũ mỗi ngày
	// Kho lưu trữ ngoài (R2/S3) cho ảnh & video sản phẩm; chưa cấu hình thì ảnh lưu trong PostgreSQL, tắt tải video lên
	storefrontHandler.Store = storage.New(storage.Config{
		Endpoint: cfg.S3Endpoint, Region: cfg.S3Region, Bucket: cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, PublicBase: cfg.S3PublicBase,
	})
	if storefrontHandler.Store != nil {
		log.Printf("[media] dùng kho lưu trữ ngoài (bucket %s) cho ảnh & video", cfg.S3Bucket)
	} else {
		log.Printf("[media] chưa cấu hình kho lưu trữ (S3_*/R2_*): ảnh lưu trong PostgreSQL, tắt tải video lên")
	}

	// Đa ngôn ngữ cho giao diện quản trị (bản dịch AI dùng chung, lưu trong translation_caches)
	uiI18nHandler := handlers.NewUIi18nHandler(db)
	uiI18nHandler.AI = aiHandler

	inboxHandler := handlers.NewInboxHandler(db, cfg)
	competitorHandler := handlers.NewCompetitorHandler(db, cfg)
	customerHandler := handlers.NewCustomerHandler(db)
	webhookHandler := handlers.NewWebhookHandler(db, cfg, aiHandler)
	broadcastHandler := handlers.NewBroadcastHandler(db, cfg)

	// Phân quyền theo khách hàng: chuẩn hoá vai trò, giao dữ liệu dùng chung trước đây cho admin (idempotent)
	handlers.MigrateTenancy(db)
	jwt := middleware.JWTAuth(cfg.JWTSecret)
	tenant := middleware.Tenancy(db, true)         // dùng được header X-Workspace (admin xem như khách, chỉ đọc)
	tenantNoView := middleware.Tenancy(db, false) // route quản trị: không "xem như khách"

	r := gin.Default()
	// Tự động chuẩn hóa URL nếu Frontend gửi thừa dấu '/' (ví dụ /api/v1//auth/register)
	r.RemoveExtraSlash = true
	r.HandleMethodNotAllowed = true

	r.Use(corsMiddleware(cfg))

	r.Static("/uploads", "./uploads")

	// Bản dịch giao diện quản trị: công khai để cả trang đăng nhập dùng được; dịch thêm bằng AI cần đăng nhập
	r.GET("/api/v1/ui-i18n/:lang", uiI18nHandler.Get)
	r.POST("/api/v1/ui-i18n/:lang/fill", middleware.JWTAuth(cfg.JWTSecret), uiI18nHandler.Fill)

	r.GET("/api/v1/storefront/public", storefrontHandler.GetPublicStorefront)
	r.GET("/api/v1/storefront/public/products/:pid", storefrontHandler.GetPublicProduct)
	r.GET("/api/v1/storefront/public/pages/:pslug", storefrontHandler.GetPublicPage)
	r.GET("/api/v1/storefront/public/seo", storefrontHandler.GetPublicSEO)
	r.GET("/api/v1/storefront/public/sitemap.xml", storefrontHandler.GetPublicSitemap)
	r.GET("/api/v1/storefront/media/:mid", storefrontHandler.ServeSiteMedia)
	// Giỏ hàng & đặt hàng trên web công khai
	r.GET("/api/v1/storefront/public/checkout-config", storefrontHandler.PublicCheckoutConfig)
	r.POST("/api/v1/storefront/public/orders", storefrontHandler.CreatePublicOrder)
	r.GET("/api/v1/storefront/public/orders/:code", storefrontHandler.GetPublicOrder)
	r.POST("/api/v1/storefront/public/orders/:code/pay", storefrontHandler.RetryVnpPayment)
	r.GET("/api/v1/storefront/public/vnpay-return", storefrontHandler.VnpayReturnCheck)
	r.POST("/api/v1/storefront/public/subscribe", storefrontHandler.PublicSubscribe)
	r.POST("/api/v1/storefront/public/track", storefrontHandler.TrackEvents)
	// Tài khoản khách hàng của từng website
	r.POST("/api/v1/storefront/public/customers/register", storefrontHandler.CustomerRegister)
	r.POST("/api/v1/storefront/public/customers/login", storefrontHandler.CustomerLogin)
	r.GET("/api/v1/storefront/public/customers/me", storefrontHandler.CustomerMe)
	r.PUT("/api/v1/storefront/public/customers/me", storefrontHandler.CustomerUpdate)
	r.GET("/api/v1/storefront/public/customers/orders", storefrontHandler.CustomerOrders)
	r.GET("/api/v1/storefront/public/rates", storefrontHandler.PublicRates)
	r.GET("/api/v1/payments/vnpay/ipn", storefrontHandler.VnpayIPN)
	r.POST("/api/v1/payments/stripe/webhook/:site", storefrontHandler.StripeWebhook)
	r.POST("/api/v1/storefront/public/orders/:code/confirm", storefrontHandler.ConfirmIntlPayment)
	r.GET("/api/v1/inbox/avatar-check/:page_id/:psid", inboxHandler.CheckCustomerAvatar)

	storefrontGroup := r.Group("/api/v1/storefront", jwt, tenant, middleware.RequireFeature(models.FeatureSites))
	{
		storefrontGroup.GET("", storefrontHandler.GetMyStorefront)
		storefrontGroup.PUT("/draft", storefrontHandler.SaveDraft)
		storefrontGroup.POST("/publish", storefrontHandler.Publish)
		// Multi-site: mỗi tài khoản nhiều website
		storefrontGroup.GET("/sites", storefrontHandler.ListSites)
		storefrontGroup.POST("/sites", middleware.OwnerOnly(), storefrontHandler.CreateSite)
		storefrontGroup.GET("/sites/:id", storefrontHandler.GetSite)
		storefrontGroup.PUT("/sites/:id/draft", storefrontHandler.SaveSiteDraft)
		storefrontGroup.POST("/sites/:id/publish", storefrontHandler.PublishSite)
		storefrontGroup.POST("/sites/:id/unpublish", storefrontHandler.UnpublishSite)
		storefrontGroup.DELETE("/sites/:id", middleware.OwnerOnly(), storefrontHandler.DeleteSite)
		storefrontGroup.GET("/sites/:id/pages", storefrontHandler.ListPages)
		storefrontGroup.POST("/sites/:id/pages", storefrontHandler.CreatePage)
		storefrontGroup.PUT("/sites/:id/pages/order", storefrontHandler.ReorderPages)
		storefrontGroup.PATCH("/sites/:id/pages/:pid", storefrontHandler.UpdatePage)
		storefrontGroup.DELETE("/sites/:id/pages/:pid", storefrontHandler.DeletePage)
		storefrontGroup.POST("/sites/:id/pages/:pid/duplicate", storefrontHandler.DuplicatePage)
		storefrontGroup.POST("/sites/:id/translate", storefrontHandler.TranslateStrings)
		storefrontGroup.POST("/sites/:id/seo/suggest", storefrontHandler.SuggestSEO)
		storefrontGroup.GET("/sites/:id/domain", storefrontHandler.GetSiteDomain)
		storefrontGroup.PUT("/sites/:id/domain", middleware.OwnerOnly(), storefrontHandler.SetSiteDomain)
		storefrontGroup.DELETE("/sites/:id/domain", middleware.OwnerOnly(), storefrontHandler.RemoveSiteDomain)
		storefrontGroup.GET("/sites/:id/products", storefrontHandler.ListProducts)
		storefrontGroup.POST("/sites/:id/products", storefrontHandler.CreateProduct)
		storefrontGroup.POST("/sites/:id/products/import", storefrontHandler.ImportProduct)
		storefrontGroup.GET("/sites/:id/products/:pid", storefrontHandler.GetProduct)
		storefrontGroup.PUT("/sites/:id/products/:pid", storefrontHandler.UpdateProduct)
		storefrontGroup.PATCH("/sites/:id/products/:pid", storefrontHandler.PatchProduct)
		storefrontGroup.DELETE("/sites/:id/products/:pid", storefrontHandler.DeleteProduct)
		// Thư viện ảnh của website (lưu trong DB)
		storefrontGroup.GET("/sites/:id/media", storefrontHandler.ListSiteMedia)
		storefrontGroup.POST("/sites/:id/media", storefrontHandler.UploadSiteMedia)
		storefrontGroup.POST("/sites/:id/media/video", storefrontHandler.UploadSiteVideo)
		storefrontGroup.POST("/sites/:id/media/import", storefrontHandler.ImportSiteMedia)
		storefrontGroup.GET("/media-config", storefrontHandler.MediaConfig)
		storefrontGroup.DELETE("/sites/:id/media/:mid", storefrontHandler.DeleteSiteMedia)
		// Đơn hàng, thanh toán, email đăng ký
		storefrontGroup.GET("/sites/:id/analytics", storefrontHandler.AnalyticsOverview)
		storefrontGroup.GET("/sites/:id/analytics/live", storefrontHandler.AnalyticsLive)
		storefrontGroup.GET("/sites/:id/analytics/abandoned", storefrontHandler.AnalyticsAbandoned)
		storefrontGroup.GET("/sites/:id/orders", storefrontHandler.ListOrders)
		storefrontGroup.PATCH("/sites/:id/orders/:oid", storefrontHandler.UpdateOrderStatus)
		storefrontGroup.GET("/sites/:id/payment-settings", storefrontHandler.GetPaymentSettings)
		storefrontGroup.PUT("/sites/:id/payment-settings", middleware.OwnerOnly(), storefrontHandler.SavePaymentSettings)
		storefrontGroup.GET("/sites/:id/subscribers", storefrontHandler.ListSiteSubscribers)
	}

	// Route trả ảnh đại diện khách hàng từ Graph API
	r.GET("/api/v1/inbox/avatar/:page_id/:psid", inboxHandler.GetCustomerAvatar)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	// Facebook Webhook công khai (Meta gọi trực tiếp không qua JWT)
	r.GET("/api/v1/webhook/facebook", webhookHandler.VerifyWebhook)
	r.POST("/api/v1/webhook/facebook", webhookHandler.HandleWebhook)

	// Route công khai (không cần đăng nhập trước)
	authPublic := r.Group("/api/v1/auth")
	{
		authPublic.POST("/register", authHandler.Register)
		authPublic.POST("/login", authHandler.Login)
		authPublic.GET("/google", authHandler.GoogleLogin)
		authPublic.GET("/google/callback", authHandler.GoogleCallback)
		authPublic.GET("/facebook", authHandler.FacebookLogin)
		authPublic.GET("/facebook/callback", authHandler.FacebookCallback)
	}

	// Route yêu cầu đăng nhập (JWT)
	authProtected := r.Group("/api/v1/auth", jwt)
	{
		authProtected.GET("/me", tenant, authHandler.Me)
		authProtected.POST("/logout", authHandler.Logout)
	}

	// Quản trị: admin theo dõi toàn bộ khách hàng (số liệu, khoá/mở, bật tắt trang, giới hạn)
	adminGroup := r.Group("/api/v1/admin", jwt, tenantNoView, middleware.AdminOnly())
	{
		adminGroup.GET("/overview", customerHandler.AdminOverview)
		adminGroup.GET("/customers", customerHandler.ListCustomers)
		adminGroup.POST("/customers", customerHandler.CreateCustomer)
		adminGroup.GET("/customers/:id", customerHandler.GetCustomer)
		adminGroup.PATCH("/customers/:id", customerHandler.UpdateCustomer)
		adminGroup.POST("/customers/:id/password", customerHandler.SetCustomerPassword)
	}

	// Nhân viên của khách hàng (chỉ chủ tài khoản quản lý)
	teamGroup := r.Group("/api/v1/team", jwt, tenant, middleware.OwnerOnly())
	{
		teamGroup.GET("", customerHandler.ListTeam)
		teamGroup.POST("", customerHandler.CreateStaff)
		teamGroup.PATCH("/:id", customerHandler.UpdateStaff)
		teamGroup.POST("/:id/password", customerHandler.SetStaffPassword)
		teamGroup.DELETE("/:id", customerHandler.DeleteStaff)
	}

	// Route gộp và quản lý Fanpage Facebook
	pagesGroup := r.Group("/api/v1/pages", jwt, tenant)
	pageWrite := []gin.HandlerFunc{middleware.RequireFeature(models.FeaturePages), middleware.OwnerOnly()}
	{
		pagesGroup.GET("", middleware.RequireFeature(models.FeaturePages, models.FeatureInbox, models.FeatureAI, models.FeaturePosts, models.FeatureBroadcasts, models.FeatureCompetitors), pageHandler.ListPages)
		pagesGroup.POST("/connect", append(pageWrite, pageHandler.ConnectPage)...)
		pagesGroup.POST("/sync", append(pageWrite, pageHandler.SyncFromFacebook)...)
		pagesGroup.PATCH("/:id/toggle", append(pageWrite, pageHandler.TogglePage)...)
		pagesGroup.DELETE("/:id", append(pageWrite, pageHandler.DisconnectPage)...)
	}

	inboxGroup := r.Group("/api/v1/inbox", jwt, tenant, middleware.RequireFeature(models.FeatureInbox))
	{
		inboxGroup.GET("/rev", inboxHandler.InboxRev) // "có tin mới chưa?" — đọc từ RAM, không chạm database
		inboxGroup.GET("/conversations", inboxHandler.ListConversations)
		inboxGroup.POST("/sync", inboxHandler.SyncMessages)
		inboxGroup.POST("/upload", inboxHandler.UploadInboxImages)
		inboxGroup.GET("/conversations/:id/messages", inboxHandler.GetMessages)
		inboxGroup.POST("/conversations/:id/messages", inboxHandler.SendMessage)
		inboxGroup.PUT("/conversations/:id/crm", inboxHandler.UpdateConversationCRM)
	}

	aiGroup := r.Group("/api/v1/ai", jwt, tenant)
	aiUse := middleware.RequireFeature(models.FeatureAI, models.FeatureInbox)
	{
		aiGroup.GET("/settings", aiUse, aiHandler.ListAllPageSettings)
		aiGroup.GET("/settings/:page_id", aiUse, aiHandler.GetPageSetting)
		aiGroup.PUT("/settings/:page_id", aiUse, aiHandler.SavePageSetting)
		aiGroup.POST("/suggest/:conversation_id", aiUse, aiHandler.SuggestReply)
		aiGroup.POST("/generate-script", aiUse, aiHandler.GenerateSalesScript)
		// Khoá AI dùng chung của hệ thống: chỉ admin
		aiGroup.GET("/config", middleware.AdminOnly(), aiHandler.GetGlobalAIConfig)
		aiGroup.POST("/config", middleware.AdminOnly(), aiHandler.SaveGlobalAIConfig)
		aiGroup.POST("/config/test", middleware.AdminOnly(), aiHandler.TestGlobalAIConfig)
	}

	// Route tự động lên lịch & đăng bài đa Fanpage
	postsGroup := r.Group("/api/v1/posts", jwt, tenant, middleware.RequireFeature(models.FeaturePosts))
	{
		postsGroup.GET("", postHandler.ListScheduledPosts)
		postsGroup.POST("", postHandler.CreateScheduledPost)
		postsGroup.POST("/ai-rewrite", postAIHandler.RewriteContent)
		postsGroup.POST("/upload-images", postAIHandler.UploadPostImages)
		postsGroup.POST("/upload-videos", postAIHandler.UploadPostVideos)
		postsGroup.POST("/fetch-social-video", videoDownloader.FetchSocialVideo)
		postsGroup.POST("/:id/publish-now", postHandler.PublishNowPost)
		postsGroup.DELETE("/:id", postHandler.DeleteScheduledPost)
	}

	// Route nghiên cứu & bám kênh đối thủ
	competitorsGroup := r.Group("/api/v1/competitors", jwt, tenant, middleware.RequireFeature(models.FeatureCompetitors))
	{
		competitorsGroup.GET("", competitorHandler.ListCompetitors)
		competitorsGroup.GET("/search", competitorHandler.SearchByKeyword)
		competitorsGroup.GET("/explore", competitorHandler.ExploreContent)
		competitorsGroup.POST("/explore/save", competitorHandler.SaveExploreItems)
		competitorsGroup.POST("", competitorHandler.AddCompetitor)
		competitorsGroup.POST("/:id/sync", competitorHandler.SyncCompetitorPosts)
		competitorsGroup.DELETE("/:id", competitorHandler.DeleteCompetitor)
		competitorsGroup.POST("/posts/manual", competitorHandler.CreateManualCompetitorPost)
		competitorsGroup.POST("/posts/:id/analyze", competitorHandler.AnalyzeAndRemixPost)
	}

	// Route chăm sóc khách cũ tự động (trong 24h & ngoài 24h, chống spam)
	broadcastsGroup := r.Group("/api/v1/broadcasts", jwt, tenant, middleware.RequireFeature(models.FeatureBroadcasts))
	{
		broadcastsGroup.GET("", broadcastHandler.ListCampaigns)
		broadcastsGroup.GET("/diagnose", broadcastHandler.DiagnosePage)
		broadcastsGroup.POST("", broadcastHandler.CreateCampaign)
		broadcastsGroup.POST("/upload-media", broadcastHandler.UploadMedia)
		broadcastsGroup.PATCH("/:id/status", broadcastHandler.ToggleStatus)

		// Marketing Messages: mời khách đăng ký nhận tin & quản lý người đăng ký
		broadcastsGroup.POST("/optin-requests", broadcastHandler.SendOptinRequests)
		broadcastsGroup.GET("/subscribers", broadcastHandler.ListSubscribers)
		broadcastsGroup.POST("/subscribers/sync", broadcastHandler.SyncSubscribers)

	}

	return r
}
