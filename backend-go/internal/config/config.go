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
}

func Load() *Config {
	_ = godotenv.Load()

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
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
