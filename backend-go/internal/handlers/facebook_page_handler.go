package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"
)

type FacebookPageHandler struct {
	DB         *gorm.DB
	Cfg        *config.Config
	HTTPClient *http.Client
}

func NewFacebookPageHandler(db *gorm.DB, cfg *config.Config) *FacebookPageHandler {
	_ = db.AutoMigrate(&models.FacebookPage{})
	return &FacebookPageHandler{
		DB:         db,
		Cfg:        cfg,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *FacebookPageHandler) encKey() string {
	if h.Cfg != nil && h.Cfg.JWTSecret != "" {
		return h.Cfg.JWTSecret
	}
	return "default-dev-master-key-change-in-prod"
}

// exchangeToLongLivedUserToken đổi User Token ngắn hạn (2h) sang Long-Lived Token (60 ngày)
// để các Page Access Token lấy từ /me/accounts có thời hạn vĩnh viễn (không bao giờ hết hạn)
func (h *FacebookPageHandler) exchangeToLongLivedUserToken(shortToken string) string {
	if h.Cfg == nil || strings.TrimSpace(h.Cfg.FacebookAppID) == "" || strings.TrimSpace(h.Cfg.FacebookAppSecret) == "" {
		return shortToken
	}

	exchangeURL := fmt.Sprintf(
		"https://graph.facebook.com/v21.0/oauth/access_token?grant_type=fb_exchange_token&client_id=%s&client_secret=%s&fb_exchange_token=%s",
		strings.TrimSpace(h.Cfg.FacebookAppID),
		strings.TrimSpace(h.Cfg.FacebookAppSecret),
		shortToken,
	)

	resp, err := h.HTTPClient.Get(exchangeURL)
	if err != nil {
		return shortToken
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &res); err == nil && res.AccessToken != "" {
		return res.AccessToken
	}
	return shortToken
}

func (h *FacebookPageHandler) ListPages(c *gin.Context) {
	var pages []models.FacebookPage
	if err := h.DB.Where("workspace_id = ?", wsID(c)).Order("created_at DESC").Find(&pages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tải danh sách Fanpage"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pages": pages, "total": len(pages)})
}

type connectManualPageReq struct {
	PageID      string `json:"page_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Category    string `json:"category"`
	AvatarURL   string `json:"avatar_url"`
	AccessToken string `json:"access_token" binding:"required"`
}

// pageLimitReached: khách hàng đã dùng hết số Fanpage được cấp (0 = không giới hạn)
func (h *FacebookPageHandler) pageLimitReached(ws int64, max int, adding int64) bool {
	if max <= 0 {
		return false
	}
	var n int64
	h.DB.Model(&models.FacebookPage{}).Where("workspace_id = ?", ws).Count(&n)
	return n+adding > int64(max)
}

const msgPageTaken = "Fanpage này đã được kết nối bởi một tài khoản khác."

func msgPageLimit(max int) string {
	return fmt.Sprintf("Tài khoản của bạn chỉ được kết nối tối đa %d Fanpage. Hãy liên hệ admin để nâng giới hạn.", max)
}

func (h *FacebookPageHandler) ConnectPage(c *gin.Context) {
	ws := wsID(c)
	tn := tenantOf(c)
	var req connectManualPageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập đầy đủ Page ID, Tên Page và Page Access Token"})
		return
	}

	pageID := strings.TrimSpace(req.PageID)
	name := strings.TrimSpace(req.Name)
	token := strings.TrimSpace(req.AccessToken)
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "Fanpage"
	}
	avatar := strings.TrimSpace(req.AvatarURL)

	encToken, err := security.EncryptSecret(token, h.encKey())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể mã hóa Page Access Token"})
		return
	}

	var existing models.FacebookPage
	if h.DB.Where("page_id = ?", pageID).First(&existing).Error == nil {
		if existing.WorkspaceID != 0 && int64(existing.WorkspaceID) != ws {
			c.JSON(http.StatusConflict, gin.H{"error": msgPageTaken})
			return
		}
		existing.WorkspaceID = uint(ws)
		existing.Name = name
		existing.Category = category
		if avatar != "" {
			existing.AvatarURL = avatar
		}
		existing.EncryptedPageToken = encToken
		existing.IsConnected = true
		if err := h.DB.Save(&existing).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật Fanpage thất bại"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật Fanpage " + name, "page": existing})
		return
	}

	if h.pageLimitReached(ws, tn.MaxPages, 1) {
		c.JSON(http.StatusForbidden, gin.H{"error": msgPageLimit(tn.MaxPages), "code": "limit_reached"})
		return
	}
	newPage := models.FacebookPage{
		WorkspaceID:        uint(ws),
		PageID:             pageID,
		Name:               name,
		Category:           category,
		AvatarURL:          avatar,
		EncryptedPageToken: encToken,
		IsConnected:        true,
	}
	if err := h.DB.Create(&newPage).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lưu Fanpage mới"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Đã gộp Fanpage " + name + " thành công!", "page": newPage})
}

type syncGraphPagesReq struct {
	UserAccessToken string `json:"user_access_token" binding:"required"`
}

func (h *FacebookPageHandler) SyncFromFacebook(c *gin.Context) {
	ws := wsID(c)
	tn := tenantOf(c)
	var req syncGraphPagesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập User Access Token"})
		return
	}

	userToken := h.exchangeToLongLivedUserToken(strings.TrimSpace(req.UserAccessToken))
	nextURL := fmt.Sprintf(
		"https://graph.facebook.com/v21.0/me/accounts?fields=id,name,category,access_token,picture.type(large)&limit=100&access_token=%s",
		userToken,
	)

	syncedCount := 0
	skippedNoToken := 0
	skippedTaken := 0
	skippedLimit := 0

	// Lặp qua toàn bộ các trang paging.next để lấy sạch 100% Fanpage trong Via
	for nextURL != "" {
		resp, err := h.HTTPClient.Get(nextURL)
		if err != nil {
			if syncedCount == 0 {
				c.JSON(http.StatusBadGateway, gin.H{"error": "Không kết nối được tới Facebook Graph API"})
				return
			}
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

		if err := json.Unmarshal(body, &fbResp); err != nil {
			if syncedCount == 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi đọc phản hồi từ Facebook"})
				return
			}
			break
		}
		if fbResp.Error != nil {
			if syncedCount == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Facebook báo lỗi: " + fbResp.Error.Message})
				return
			}
			break
		}

		for _, item := range fbResp.Data {
			if item.ID == "" {
				continue
			}
			if item.AccessToken == "" {
				skippedNoToken++
				continue
			}
			encToken, err := security.EncryptSecret(item.AccessToken, h.encKey())
			if err != nil {
				continue
			}
			avatar := strings.TrimSpace(item.Picture.Data.URL)

			var page models.FacebookPage
			if h.DB.Where("page_id = ?", item.ID).First(&page).Error == nil {
				if page.WorkspaceID != 0 && int64(page.WorkspaceID) != ws {
					skippedTaken++ // Fanpage đang thuộc về tài khoản khác
					continue
				}
				page.WorkspaceID = uint(ws)
				page.Name = item.Name
				page.Category = item.Category
				page.AvatarURL = avatar
				page.EncryptedPageToken = encToken
				page.IsConnected = true
				_ = h.DB.Save(&page).Error
			} else {
				if h.pageLimitReached(ws, tn.MaxPages, 1) {
					skippedLimit++
					continue
				}
				_ = h.DB.Create(&models.FacebookPage{
					WorkspaceID:        uint(ws),
					PageID:             item.ID,
					Name:               item.Name,
					Category:           item.Category,
					AvatarURL:          avatar,
					EncryptedPageToken: encToken,
					IsConnected:        true,
				}).Error
			}
			syncedCount++
		}

		if fbResp.Paging != nil && fbResp.Paging.Next != "" {
			nextURL = fbResp.Paging.Next
		} else {
			nextURL = ""
		}
	}

	msg := fmt.Sprintf("Đã đồng bộ %d Fanpage từ Facebook!", syncedCount)
	if skippedNoToken > 0 {
		msg += fmt.Sprintf(" (Bỏ qua %d Page do Via chưa có quyền quản lý tin nhắn/token)", skippedNoToken)
	}

	if skippedTaken > 0 {
		msg += fmt.Sprintf(" (Bỏ qua %d Page đã được tài khoản khác kết nối)", skippedTaken)
	}
	if skippedLimit > 0 {
		msg += fmt.Sprintf(" (Bỏ qua %d Page vì đã đạt giới hạn %d Fanpage của tài khoản)", skippedLimit, tn.MaxPages)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": msg,
		"synced":  syncedCount,
	})
}

func (h *FacebookPageHandler) TogglePage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}
	var page models.FacebookPage
	if err := h.DB.Where("id = ? AND workspace_id = ?", id, wsID(c)).First(&page).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy Fanpage"})
		return
	}
	page.IsConnected = !page.IsConnected
	if err := h.DB.Save(&page).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể cập nhật trạng thái"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã đổi trạng thái " + page.Name, "page": page})
}

func (h *FacebookPageHandler) DisconnectPage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}
	if err := h.DB.Where("workspace_id = ?", wsID(c)).Delete(&models.FacebookPage{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể gỡ Fanpage"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã gỡ Fanpage khỏi danh sách gộp"})
}
