package main

import (
	"log"
	"net/http"
	"strings"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/database"
	"fb-multipage-crm/internal/handlers"
	"fb-multipage-crm/internal/middleware"

	"github.com/gin-gonic/gin"
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
	db := database.Connect(cfg.DatabaseURL)

	authHandler := handlers.NewAuthHandler(db, cfg)
	postHandler := handlers.NewPostHandler(db, cfg)
	pageHandler := handlers.NewFacebookPageHandler(db, cfg)
	aiHandler := handlers.NewAIHandler(db, cfg)
	postAIHandler := handlers.NewPostAIHandler(db, cfg, aiHandler)
	videoDownloader := handlers.NewVideoDownloaderHandler()

	storefrontHandler := handlers.NewStorefrontHandler(db, cfg)

	inboxHandler := handlers.NewInboxHandler(db, cfg)
	competitorHandler := handlers.NewCompetitorHandler(db, cfg)
	roleHandler := handlers.NewRoleHandler(db)
	webhookHandler := handlers.NewWebhookHandler(db, cfg, aiHandler)
	broadcastHandler := handlers.NewBroadcastHandler(db, cfg)

	r := gin.Default()
	// Tự động chuẩn hóa URL nếu Frontend gửi thừa dấu '/' (ví dụ /api/v1//auth/register)
	r.RemoveExtraSlash = true
	r.HandleMethodNotAllowed = true

	// Chuẩn hóa Frontend URL chính được phép gọi API (loại bỏ dấu '/' ở cuối nếu có)
	allowedOrigin := strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/")

	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		c.Header("Vary", "Origin")

		if allowedOrigin == "*" {
			if origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
			} else {
				c.Header("Access-Control-Allow-Origin", "*")
			}
		} else if isOriginAllowed(origin, allowedOrigin) {
			c.Header("Access-Control-Allow-Origin", strings.TrimRight(strings.TrimSpace(origin), "/"))
		} else if allowedOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowedOrigin)
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Origin")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.Static("/uploads", "./uploads")

	r.GET("/api/v1/storefront/public", storefrontHandler.GetPublicStorefront)
	r.GET("/api/v1/storefront/media/:mid", storefrontHandler.ServeSiteMedia)
	r.GET("/api/v1/inbox/avatar-check/:page_id/:psid", inboxHandler.CheckCustomerAvatar)

	storefrontGroup := r.Group("/api/v1/storefront", middleware.JWTAuth(cfg.JWTSecret))
	{
		storefrontGroup.GET("", storefrontHandler.GetMyStorefront)
		storefrontGroup.PUT("/draft", storefrontHandler.SaveDraft)
		storefrontGroup.POST("/publish", storefrontHandler.Publish)
		// Multi-site: mỗi tài khoản nhiều website
		storefrontGroup.GET("/sites", storefrontHandler.ListSites)
		storefrontGroup.POST("/sites", storefrontHandler.CreateSite)
		storefrontGroup.GET("/sites/:id", storefrontHandler.GetSite)
		storefrontGroup.PUT("/sites/:id/draft", storefrontHandler.SaveSiteDraft)
		storefrontGroup.POST("/sites/:id/publish", storefrontHandler.PublishSite)
		storefrontGroup.POST("/sites/:id/unpublish", storefrontHandler.UnpublishSite)
		storefrontGroup.DELETE("/sites/:id", storefrontHandler.DeleteSite)
		storefrontGroup.GET("/sites/:id/products", storefrontHandler.ListProducts)
		storefrontGroup.POST("/sites/:id/products", storefrontHandler.CreateProduct)
		storefrontGroup.PUT("/sites/:id/products/:pid", storefrontHandler.UpdateProduct)
		storefrontGroup.DELETE("/sites/:id/products/:pid", storefrontHandler.DeleteProduct)
		// Thư viện ảnh của website (lưu trong DB)
		storefrontGroup.GET("/sites/:id/media", storefrontHandler.ListSiteMedia)
		storefrontGroup.POST("/sites/:id/media", storefrontHandler.UploadSiteMedia)
		storefrontGroup.DELETE("/sites/:id/media/:mid", storefrontHandler.DeleteSiteMedia)
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
	authProtected := r.Group("/api/v1/auth", middleware.JWTAuth(cfg.JWTSecret))
	{
		authProtected.GET("/me", authHandler.Me)
		authProtected.POST("/logout", authHandler.Logout)
	}

	// Route phân cấp bậc & phân quyền (Admin & Manager)
	roles := r.Group("/api/v1/roles", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireRoles(db, "admin", "manager"))
	{
		roles.GET("/users", roleHandler.ListUsers)
		roles.PATCH("/users/:id/role", middleware.RequireRoles(db, "admin"), roleHandler.AssignRole)
		roles.PATCH("/users/:id/permissions", roleHandler.AssignStaffPermissions)
	}

	// Route gộp và quản lý Fanpage Facebook
	pagesGroup := r.Group("/api/v1/pages", middleware.JWTAuth(cfg.JWTSecret))
	{
		pagesGroup.GET("", pageHandler.ListPages)
		pagesGroup.POST("/connect", middleware.RequireRoles(db, "admin", "manager"), pageHandler.ConnectPage)
		pagesGroup.POST("/sync", pageHandler.SyncFromFacebook)
		pagesGroup.PATCH("/:id/toggle", pageHandler.TogglePage)
		pagesGroup.DELETE("/:id", pageHandler.DisconnectPage)
	}

	inboxGroup := r.Group("/api/v1/inbox", middleware.JWTAuth(cfg.JWTSecret))
	{
		inboxGroup.GET("/conversations", inboxHandler.ListConversations)
		inboxGroup.POST("/sync", inboxHandler.SyncMessages)
		inboxGroup.POST("/upload", inboxHandler.UploadInboxImages)
		inboxGroup.GET("/conversations/:id/messages", inboxHandler.GetMessages)
		inboxGroup.POST("/conversations/:id/messages", inboxHandler.SendMessage)
		inboxGroup.PUT("/conversations/:id/crm", inboxHandler.UpdateConversationCRM)
	}

	aiGroup := r.Group("/api/v1/ai", middleware.JWTAuth(cfg.JWTSecret))
	{
		aiGroup.GET("/settings", aiHandler.ListAllPageSettings)
		aiGroup.GET("/settings/:page_id", aiHandler.GetPageSetting)
		aiGroup.PUT("/settings/:page_id", aiHandler.SavePageSetting)
		aiGroup.POST("/suggest/:conversation_id", aiHandler.SuggestReply)
		aiGroup.POST("/generate-script", aiHandler.GenerateSalesScript)
		aiGroup.GET("/config", aiHandler.GetGlobalAIConfig)
		aiGroup.POST("/config", aiHandler.SaveGlobalAIConfig)
		aiGroup.POST("/config/test", aiHandler.TestGlobalAIConfig)
	}

	// Route tự động lên lịch & đăng bài đa Fanpage
	postsGroup := r.Group("/api/v1/posts", middleware.JWTAuth(cfg.JWTSecret))
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
	competitorsGroup := r.Group("/api/v1/competitors", middleware.JWTAuth(cfg.JWTSecret))
	{
		competitorsGroup.GET("", competitorHandler.ListCompetitors)
		competitorsGroup.GET("/search", competitorHandler.SearchByKeyword)
		competitorsGroup.POST("", competitorHandler.AddCompetitor)
		competitorsGroup.POST("/:id/sync", competitorHandler.SyncCompetitorPosts)
		competitorsGroup.DELETE("/:id", competitorHandler.DeleteCompetitor)
		competitorsGroup.POST("/posts/manual", competitorHandler.CreateManualCompetitorPost)
		competitorsGroup.POST("/posts/:id/analyze", competitorHandler.AnalyzeAndRemixPost)
	}

	// Route chăm sóc khách cũ tự động (trong 24h & ngoài 24h, chống spam)
	broadcastsGroup := r.Group("/api/v1/broadcasts", middleware.JWTAuth(cfg.JWTSecret))
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

	log.Printf("Backend Golang đang chạy tại http://localhost:%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
