package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"
	"fb-multipage-crm/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB          *gorm.DB
	Cfg         *config.Config
	GoogleOAuth *oauth2.Config
	FBOAuth     *oauth2.Config
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		DB:  db,
		Cfg: cfg,
		GoogleOAuth: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		FBOAuth: &oauth2.Config{
			ClientID:     cfg.FacebookAppID,
			ClientSecret: cfg.FacebookAppSecret,
			RedirectURL:  cfg.FacebookRedirectURL,
			Scopes: []string{
				"public_profile",
				"email",
				"pages_show_list",
				"pages_messaging",
				"pages_read_engagement",
				"pages_manage_metadata",
			},
			Endpoint: facebook.Endpoint,
		},
	}
}

func (h *AuthHandler) encKey() string {
	if h.Cfg != nil && h.Cfg.JWTSecret != "" {
		return h.Cfg.JWTSecret
	}
	return "default-dev-master-key-change-in-prod"
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		FullName string `json:"full_name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Thông tin đăng ký chưa hợp lệ (mật khẩu tối thiểu 6 ký tự)."})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi mã hóa mật khẩu."})
		return
	}

	user := models.User{
		FullName:     strings.TrimSpace(req.FullName),
		Email:        email,
		PasswordHash: hash,
		AuthProvider: "local",
		SystemRole:   h.resolveInitialRole(),
		IsActive:     true,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email này đã được sử dụng."})
		return
	}

	token, err := utils.GenerateJWT(user.ID, user.SystemRole, h.Cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tạo phiên đăng nhập."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"token": token, "user": user})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập đầy đủ email và mật khẩu."})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	var user models.User
	if err := h.DB.Where("email = ? AND is_active = TRUE", email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email hoặc mật khẩu không chính xác."})
		return
	}

	if user.PasswordHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Tài khoản này đăng ký bằng %s. Hãy bấm nút đăng nhập tương ứng.", user.AuthProvider),
		})
		return
	}

	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email hoặc mật khẩu không chính xác."})
		return
	}

	token, err := utils.GenerateJWT(user.ID, user.SystemRole, h.Cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tạo phiên đăng nhập."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var user models.User
	if err := h.DB.Where("id = ? AND is_active = TRUE", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Tài khoản không tồn tại hoặc đã bị khóa."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Đăng xuất thành công"})
}

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	if h.Cfg.GoogleClientID == "" || h.Cfg.GoogleClientSecret == "" {
		h.redirectWithError(c, "Chưa cấu hình GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET trong file .env.")
		return
	}
	state := utils.RandomState()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	c.Redirect(http.StatusTemporaryRedirect, h.GoogleOAuth.AuthCodeURL(state, oauth2.AccessTypeOnline))
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	if !h.verifyOAuthState(c) {
		h.redirectWithError(c, "Phiên xác thực Google không hợp lệ hoặc đã hết hạn.")
		return
	}

	code := c.Query("code")
	tok, err := h.GoogleOAuth.Exchange(context.Background(), code)
	if err != nil {
		h.redirectWithError(c, "Không thể xác thực mã đăng nhập Google.")
		return
	}

	client := h.GoogleOAuth.Client(context.Background(), tok)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil || resp.StatusCode >= 400 {
		h.redirectWithError(c, "Không thể lấy thông tin tài khoản Google.")
		return
	}
	defer resp.Body.Close()

	var profile struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil || profile.Email == "" {
		h.redirectWithError(c, "Không đọc được email từ tài khoản Google.")
		return
	}

	h.finishSocialLogin(c, "google", profile.ID, profile.Email, profile.Name, profile.Picture)
}

func (h *AuthHandler) FacebookLogin(c *gin.Context) {
	if h.Cfg.FacebookAppID == "" || h.Cfg.FacebookAppSecret == "" {
		h.redirectWithError(c, "Chưa cấu hình FACEBOOK_APP_ID / FACEBOOK_APP_SECRET trong file .env.")
		return
	}
	state := utils.RandomState()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	// auth_type=rerequest buộc Facebook hiển thị lại màn hình chọn tất cả Fanpage nếu trước đó bị bỏ sót
	authURL := h.FBOAuth.AuthCodeURL(state, oauth2.SetAuthURLParam("auth_type", "rerequest"))
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

func (h *AuthHandler) FacebookCallback(c *gin.Context) {
	if !h.verifyOAuthState(c) {
		h.redirectWithError(c, "Phiên xác thực Facebook không hợp lệ hoặc đã hết hạn.")
		return
	}

	code := c.Query("code")
	tok, err := h.FBOAuth.Exchange(context.Background(), code)
	if err != nil {
		h.redirectWithError(c, "Không thể xác thực mã đăng nhập Facebook.")
		return
	}

	endpoint := "https://graph.facebook.com/v21.0/me?fields=id,name,email,picture.type(large)&access_token=" + url.QueryEscape(tok.AccessToken)
	resp, err := http.Get(endpoint)
	if err != nil || resp.StatusCode >= 400 {
		h.redirectWithError(c, "Không thể lấy thông tin từ Facebook.")
		return
	}
	defer resp.Body.Close()

	var profile struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil || profile.ID == "" {
		h.redirectWithError(c, "Không đọc được hồ sơ Facebook.")
		return
	}

	// Tự động đổi sang Long-Lived User Token và quét toàn bộ Fanpage qua paging.next
	go h.syncAllPagesFromOAuthToken(tok.AccessToken)

	email := profile.Email
	if email == "" {
		email = fmt.Sprintf("fb_%s@facebook.local", profile.ID)
	}

	h.finishSocialLogin(c, "facebook", profile.ID, email, profile.Name, profile.Picture.Data.URL)
}

func (h *AuthHandler) syncAllPagesFromOAuthToken(shortToken string) {
	client := &http.Client{Timeout: 30 * time.Second}
	userToken := shortToken

	if h.Cfg != nil && h.Cfg.FacebookAppID != "" && h.Cfg.FacebookAppSecret != "" {
		exchangeURL := fmt.Sprintf(
			"https://graph.facebook.com/v21.0/oauth/access_token?grant_type=fb_exchange_token&client_id=%s&client_secret=%s&fb_exchange_token=%s",
			strings.TrimSpace(h.Cfg.FacebookAppID),
			strings.TrimSpace(h.Cfg.FacebookAppSecret),
			shortToken,
		)
		if resp, err := client.Get(exchangeURL); err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var res struct {
				AccessToken string `json:"access_token"`
			}
			if json.Unmarshal(body, &res) == nil && res.AccessToken != "" {
				userToken = res.AccessToken
			}
		}
	}

	nextURL := fmt.Sprintf(
		"https://graph.facebook.com/v21.0/me/accounts?fields=id,name,category,access_token,picture.type(large)&limit=100&access_token=%s",
		userToken,
	)

	for nextURL != "" {
		resp, err := client.Get(nextURL)
		if err != nil {
			break
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var fbResp struct {
			Data []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Category    string `json:"category"`
				AccessToken string `json:"access_token"`
				Picture     struct {
					Data struct {
						URL string `json:"url"`
					} `json:"data"`
				} `json:"picture"`
			} `json:"data"`
			Paging *struct {
				Next string `json:"next"`
			} `json:"paging"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if json.Unmarshal(body, &fbResp) != nil || fbResp.Error != nil {
			break
		}

		for _, item := range fbResp.Data {
			if item.ID == "" || item.AccessToken == "" {
				continue
			}
			encToken, err := security.EncryptSecret(item.AccessToken, h.encKey())
			if err != nil {
				continue
			}
			avatar := strings.TrimSpace(item.Picture.Data.URL)

			var page models.FacebookPage
			if h.DB.Where("page_id = ?", item.ID).First(&page).Error == nil {
				page.Name = item.Name
				page.Category = item.Category
				page.AvatarURL = avatar
				page.EncryptedPageToken = encToken
				page.IsConnected = true
				_ = h.DB.Save(&page).Error
			} else {
				_ = h.DB.Create(&models.FacebookPage{
					PageID:             item.ID,
					Name:               item.Name,
					Category:           item.Category,
					AvatarURL:          avatar,
					EncryptedPageToken: encToken,
					IsConnected:        true,
				}).Error
			}
		}

		if fbResp.Paging != nil && fbResp.Paging.Next != "" {
			nextURL = fbResp.Paging.Next
		} else {
			nextURL = ""
		}
	}
}

func (h *AuthHandler) finishSocialLogin(c *gin.Context, provider, providerID, email, name, avatar string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	var user models.User
	err := h.DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		user = models.User{
			FullName:     name,
			Email:        email,
			AvatarURL:    avatar,
			AuthProvider: provider,
			ProviderID:   providerID,
			SystemRole:   h.resolveInitialRole(),
			IsActive:     true,
		}
		if createErr := h.DB.Create(&user).Error; createErr != nil {
			h.redirectWithError(c, "Không thể tạo tài khoản từ "+provider)
			return
		}
	} else {
		if !user.IsActive {
			h.redirectWithError(c, "Tài khoản này đã bị vô hiệu hóa.")
			return
		}
		if user.AvatarURL == "" && avatar != "" {
			h.DB.Model(&user).Update("avatar_url", avatar)
		}
	}

	jwtToken, err := utils.GenerateJWT(user.ID, user.SystemRole, h.Cfg.JWTSecret)
	if err != nil {
		h.redirectWithError(c, "Không thể khởi tạo phiên đăng nhập.")
		return
	}

	target := fmt.Sprintf("%s/auth/callback?token=%s", strings.TrimRight(h.Cfg.FrontendURL, "/"), url.QueryEscape(jwtToken))
	c.Redirect(http.StatusTemporaryRedirect, target)
}

func (h *AuthHandler) resolveInitialRole() string {
	var count int64
	h.DB.Model(&models.User{}).Count(&count)
	if count == 0 {
		return "admin"
	}
	return "staff"
}

func (h *AuthHandler) verifyOAuthState(c *gin.Context) bool {
	cookieState, err := c.Cookie("oauth_state")
	if err != nil || cookieState == "" {
		return false
	}
	c.SetCookie("oauth_state", "", -1, "/", "", false, true)
	return cookieState == c.Query("state")
}

func (h *AuthHandler) redirectWithError(c *gin.Context, msg string) {
	target := fmt.Sprintf("%s/auth/callback?error=%s", strings.TrimRight(h.Cfg.FrontendURL, "/"), url.QueryEscape(msg))
	c.Redirect(http.StatusTemporaryRedirect, target)
}
