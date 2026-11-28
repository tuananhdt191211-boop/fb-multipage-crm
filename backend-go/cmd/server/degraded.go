package main

import (
	"net/http"
	"strings"
	"sync/atomic"

	"fb-multipage-crm/internal/config"

	"github.com/gin-gonic/gin"
)

// corsMiddleware cho phép frontend (và tên miền riêng của khách với API công khai) gọi API.
func corsMiddleware(cfg *config.Config) gin.HandlerFunc {
	// Chuẩn hóa Frontend URL chính được phép gọi API (loại bỏ dấu '/' ở cuối nếu có)
	allowedOrigin := strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		c.Header("Vary", "Origin")

		// API công khai của website bán hàng được gọi từ tên miền riêng của khách → cho mọi origin
		p := c.Request.URL.Path
		isPublicStore := strings.HasPrefix(p, "/api/v1/storefront/public") || strings.HasPrefix(p, "/api/v1/storefront/media/")

		if isPublicStore && origin != "" {
			c.Header("Access-Control-Allow-Origin", strings.TrimRight(strings.TrimSpace(origin), "/"))
		} else if allowedOrigin == "*" {
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
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Origin, X-Customer-Token, X-Workspace")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// maintenanceEngine phục vụ khi database chưa sẵn sàng (lỗi kết nối, Neon hết hạn mức...):
//   - /health vẫn trả 200 (Render không coi là sập và không khởi động lại liên tục)
//   - mọi đường dẫn khác trả 503 + JSON giải thích lý do, có đủ header CORS để giao diện đọc được
func maintenanceEngine(cfg *config.Config, lastErr *atomic.Value) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), corsMiddleware(cfg))

	reason := func() string {
		if v, ok := lastErr.Load().(string); ok && v != "" {
			return v
		}
		return "Đang kết nối database..."
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "degraded", "database": "unavailable"})
	})
	r.NoRoute(func(c *gin.Context) {
		c.Header("Retry-After", "60")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":  "Cơ sở dữ liệu tạm thời không khả dụng nên hệ thống chưa phục vụ được. Vui lòng thử lại sau ít phút. (Quản trị viên: xem log máy chủ hoặc console.neon.tech)",
			"code":   "db_unavailable",
			"detail": reason(),
		})
	})
	return r
}
