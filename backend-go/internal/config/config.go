package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	FrontendURL         string
	DatabaseURL         string
	JWTSecret           string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleRedirectURL   string
	FacebookAppID       string
	FacebookAppSecret   string
	FacebookRedirectURL string
	GeminiAPIKey        string

	// Marketing Messages on Messenger (gửi tin cho khách đã đăng ký nhận tin, kể cả ngoài 24h)
	MetaAdAccountID     string // ID tài khoản quảng cáo (có hoặc không có tiền tố act_ đều được)
	MetaSystemUserToken string // System User Access Token: ads_management + pages_messaging + paid_marketing_messages
	MetaGraphVersion    string // Phiên bản Graph API dùng cho Marketing Messages
}

func Load() *Config {
	// .env.local (nếu có) được nạp TRƯỚC và thắng .env — dùng để chạy dev bằng database trên máy
	// mà không đụng vào cấu hình / khoá thật trong .env. File này nằm trong .gitignore và không có trên Render.
	// (Nạp riêng từng file vì godotenv.Load dừng ngay khi một file không tồn tại.)
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load(".env")

	return &Config{
		Port:                getEnv("PORT", "8080"),
		FrontendURL:         getEnv("FRONTEND_URL", "http://localhost:5173"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://fbcrm:fbcrm_secret_password@localhost:5432/fb_multipage_crm?sslmode=disable"),
		JWTSecret:           getEnv("JWT_SECRET", "dev_secret_change_me"),
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:   getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),
		FacebookAppID:       os.Getenv("FACEBOOK_APP_ID"),
		FacebookAppSecret:   os.Getenv("FACEBOOK_APP_SECRET"),
		FacebookRedirectURL: getEnv("FACEBOOK_REDIRECT_URL", "http://localhost:8080/api/v1/auth/facebook/callback"),
		GeminiAPIKey:        os.Getenv("GEMINI_API_KEY"),
		MetaAdAccountID:     os.Getenv("META_AD_ACCOUNT_ID"),
		MetaSystemUserToken: os.Getenv("META_SYSTEM_USER_TOKEN"),
		MetaGraphVersion:    getEnv("META_GRAPH_VERSION", "v23.0"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
