package handlers

// Xoá dữ liệu người dùng theo yêu cầu của Meta (Data Deletion Request Callback).
//
// Khi người dùng gỡ ứng dụng trong cài đặt Facebook và yêu cầu xoá dữ liệu, Meta gửi
// POST /api/v1/facebook/data-deletion với form "signed_request". Ta kiểm tra chữ ký bằng
// App Secret, xoá ngay dữ liệu lấy từ Facebook của người đó (mã truy cập Page, liên kết tài khoản
// Facebook, ảnh đại diện), ghi lại yêu cầu và trả về {url, confirmation_code} như Meta yêu cầu.
// Người dùng mở url để xem trạng thái: GET /api/v1/facebook/data-deletion/:code.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DataDeletionRequest struct {
	ID               uint       `gorm:"primaryKey" json:"-"`
	ConfirmationCode string     `gorm:"size:40;uniqueIndex;not null" json:"confirmation_code"`
	FacebookUserID   string     `gorm:"size:128;index" json:"-"`
	Status           string     `gorm:"size:20;not null" json:"status"` // completed | not_found
	Summary          string     `gorm:"type:text" json:"summary"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at"`
}

type DataDeletionHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewDataDeletionHandler(db *gorm.DB, cfg *config.Config) *DataDeletionHandler {
	_ = db.AutoMigrate(&DataDeletionRequest{})
	return &DataDeletionHandler{DB: db, Cfg: cfg}
}

type fbSignedPayload struct {
	Algorithm string `json:"algorithm"`
	UserID    string `json:"user_id"`
	IssuedAt  int64  `json:"issued_at"`
}

func b64urlDecode(s string) ([]byte, error) {
	s = strings.TrimRight(strings.TrimSpace(s), "=")
	return base64.RawURLEncoding.DecodeString(s)
}

// parseSignedRequest kiểm tra chữ ký HMAC-SHA256 của Meta và trả về nội dung.
func parseSignedRequest(signed, appSecret string) (fbSignedPayload, error) {
	var p fbSignedPayload
	if appSecret == "" {
		return p, errors.New("chưa cấu hình FACEBOOK_APP_SECRET")
	}
	parts := strings.SplitN(strings.TrimSpace(signed), ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return p, errors.New("signed_request sai định dạng")
	}
	sig, err := b64urlDecode(parts[0])
	if err != nil {
		return p, errors.New("chữ ký không hợp lệ")
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return p, errors.New("chữ ký không khớp")
	}
	raw, err := b64urlDecode(parts[1])
	if err != nil {
		return p, errors.New("nội dung không hợp lệ")
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, errors.New("nội dung không hợp lệ")
	}
	if !strings.EqualFold(p.Algorithm, "HMAC-SHA256") || strings.TrimSpace(p.UserID) == "" {
		return p, errors.New("thiếu thông tin người dùng")
	}
	return p, nil
}

func newConfirmationCode() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}

// eraseFacebookData xoá dữ liệu lấy từ Facebook của người dùng có ID Facebook này.
// Không xoá dữ liệu kinh doanh khác của tài khoản (website, đơn hàng) vì có thể thuộc về
// cả nhóm; người dùng yêu cầu xoá toàn bộ tài khoản qua email trong trang hướng dẫn.
func (h *DataDeletionHandler) eraseFacebookData(fbUserID string) (string, bool) {
	var users []models.User
	h.DB.Where("auth_provider = ? AND provider_id = ?", "facebook", fbUserID).Find(&users)
	if len(users) == 0 {
		return "Không tìm thấy tài khoản nào liên kết với Facebook này. Hệ thống không lưu dữ liệu Facebook của bạn.", false
	}

	pagesCleared := int64(0)
	for _, u := range users {
		// Chỉ chủ tài khoản (không phải nhân viên) mới là người kết nối Fanpage
		if u.OwnerID == nil {
			res := h.DB.Model(&models.FacebookPage{}).
				Where("workspace_id = ?", u.ID).
				Updates(map[string]any{
					"encrypted_page_token": "",
					"is_connected":         false,
					"webhook_subscribed":   false,
				})
			pagesCleared += res.RowsAffected
		}
		_ = h.DB.Model(&models.User{}).Where("id = ?", u.ID).Updates(map[string]any{
			"provider_id": "",
			"avatar_url":  "",
		}).Error
	}

	summary := "Đã xoá liên kết tài khoản Facebook và ảnh đại diện"
	if pagesCleared > 0 {
		summary += ", xoá mã truy cập và ngắt kết nối " + strconv.FormatInt(pagesCleared, 10) + " Fanpage"
	}
	return summary + ".", true
}

func (h *DataDeletionHandler) statusURL(code string) string {
	base := ""
	if h.Cfg != nil {
		base = strings.TrimRight(strings.TrimSpace(h.Cfg.FrontendURL), "/")
	}
	return base + "/data-deletion?code=" + code
}

// Callback POST /api/v1/facebook/data-deletion (Meta gọi, form signed_request)
func (h *DataDeletionHandler) Callback(c *gin.Context) {
	secret := ""
	if h.Cfg != nil {
		secret = strings.TrimSpace(h.Cfg.FacebookAppSecret)
	}
	payload, err := parseSignedRequest(c.PostForm("signed_request"), secret)
	if err != nil {
		log.Printf("[DataDeletion] Từ chối yêu cầu: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	summary, found := h.eraseFacebookData(payload.UserID)
	now := time.Now()
	req := DataDeletionRequest{
		ConfirmationCode: newConfirmationCode(),
		FacebookUserID:   payload.UserID,
		Status:           "completed",
		Summary:          summary,
		CompletedAt:      &now,
	}
	if !found {
		req.Status = "not_found"
	}
	if err := h.DB.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không ghi nhận được yêu cầu"})
		return
	}
	log.Printf("[DataDeletion] Mã %s: %s", req.ConfirmationCode, summary)

	c.JSON(http.StatusOK, gin.H{
		"url":               h.statusURL(req.ConfirmationCode),
		"confirmation_code": req.ConfirmationCode,
	})
}

// Status GET /api/v1/facebook/data-deletion/:code (công khai)
func (h *DataDeletionHandler) Status(c *gin.Context) {
	code := strings.ToUpper(strings.TrimSpace(c.Param("code")))
	if len(code) < 8 || len(code) > 40 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy mã xác nhận"})
		return
	}
	var req DataDeletionRequest
	if err := h.DB.Where("confirmation_code = ?", code).First(&req).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy mã xác nhận"})
		return
	}
	c.JSON(http.StatusOK, req)
}
