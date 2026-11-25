package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BroadcastCampaign struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	PageID          string    `gorm:"index;not null" json:"page_id"`
	Name            string    `gorm:"not null" json:"name"`
	TemplatesJSON   string    `gorm:"type:text;not null" json:"-"`
	TargetMode      string    `gorm:"default:'all'" json:"target_mode"` // all | within_24h | over_24h
	OutOfWindowTag  string    `gorm:"default:'POST_PURCHASE_UPDATE'" json:"out_of_window_tag"`
	MinDelaySec     int       `gorm:"default:4" json:"min_delay_sec"`
	MaxDelaySec     int       `gorm:"default:9" json:"max_delay_sec"`
	BatchSize       int       `gorm:"default:20" json:"batch_size"`
	BatchPauseSec   int       `gorm:"default:60" json:"batch_pause_sec"`
	Status          string    `gorm:"index;default:'running'" json:"status"` // running | paused | completed
	PauseReason     string    `json:"pause_reason"`
	TotalRecipients int       `json:"total_recipients"`
	SentCount       int       `json:"sent_count"`
	FailedCount     int       `json:"failed_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type BroadcastMessage struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	CampaignID     uint       `gorm:"index;not null" json:"campaign_id"`
	PageID         string     `gorm:"index;not null" json:"page_id"`
	ConversationID uint       `gorm:"index;not null" json:"conversation_id"`
	RecipientPSID  string     `gorm:"not null" json:"recipient_psid"`
	RecipientName  string     `json:"recipient_name"`
	RenderedText   string     `gorm:"type:text;not null" json:"rendered_text"`
	Status         string     `gorm:"index;default:'pending'" json:"status"` // pending | processing | sent | failed | cancelled
	FBMessageID    string     `json:"fb_message_id"`
	ErrorCode      int        `json:"error_code"`
	ErrorMessage   string     `json:"error_message"`
	SentAt         *time.Time `json:"sent_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type BroadcastHandler struct {
	DB         *gorm.DB
	Cfg        *config.Config
	HTTPClient *http.Client
	vnLoc      *time.Location
}

func NewBroadcastHandler(db *gorm.DB, cfg *config.Config) *BroadcastHandler {
	_ = db.AutoMigrate(&BroadcastCampaign{}, &BroadcastMessage{})
	_ = db.Exec(`
		ALTER TABLE conversations
		  ADD COLUMN IF NOT EXISTS is_unsubscribed BOOLEAN NOT NULL DEFAULT FALSE,
		  ADD COLUMN IF NOT EXISTS unsubscribed_at TIMESTAMPTZ,
		  ADD COLUMN IF NOT EXISTS last_auto_sent_at TIMESTAMPTZ,
		  ADD COLUMN IF NOT EXISTS last_customer_message_at TIMESTAMPTZ
	`).Error

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}

	h := &BroadcastHandler{
		DB:         db,
		Cfg:        cfg,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		vnLoc:      loc,
	}
	go h.startWorker()
	return h
}

func (h *BroadcastHandler) getEncryptionKey() string {
	if h.Cfg != nil && h.Cfg.JWTSecret != "" {
		return h.Cfg.JWTSecret
	}
	return "default-dev-master-key-change-in-prod"
}

func (h *BroadcastHandler) startWorker() {
	sentInBatch := 0
	consecutiveTagErrors := 0

	for {
		nowVN := time.Now().In(h.vnLoc)
		mins := nowVN.Hour()*60 + nowVN.Minute()
		// Chỉ chạy từ 08:30 đến 21:00 để tránh làm phiền khách ban đêm
		if mins < 8*60+30 || mins > 21*60 {
			time.Sleep(1 * time.Minute)
			continue
		}

		var msg BroadcastMessage
		err := h.DB.Table("broadcast_messages m").
			Select("m.*").
			Joins("JOIN broadcast_campaigns c ON c.id = m.campaign_id").
			Where("m.status = ? AND c.status = ?", "pending", "running").
			Order("m.id ASC").
			First(&msg).Error
		if err != nil {
			sentInBatch = 0
			time.Sleep(5 * time.Second)
			continue
		}

		// Đánh dấu đang xử lý
		if res := h.DB.Model(&BroadcastMessage{}).
			Where("id = ? AND status = ?", msg.ID, "pending").
			Update("status", "processing"); res.RowsAffected == 0 {
			continue
		}

		var camp BroadcastCampaign
		var page models.FacebookPage
		var conv models.Conversation
		if h.DB.First(&camp, msg.CampaignID).Error != nil ||
			h.DB.Where("page_id = ? AND is_connected = ?", msg.PageID, true).First(&page).Error != nil ||
			h.DB.First(&conv, msg.ConversationID).Error != nil {
			_ = h.DB.Model(&msg).Updates(map[string]any{"status": "failed", "error_message": "Không tìm thấy Page hoặc hội thoại"}).Error
			continue
		}

		pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.getEncryptionKey())
		if err != nil || pageToken == "" {
			_ = h.DB.Model(&msg).Updates(map[string]any{"status": "failed", "error_message": "Lỗi giải mã Page Token"}).Error
			continue
		}

		// Kiểm tra khách trong 24h hay quá 24h dựa trên lần nhắn cuối của khách
		var lastCustomerMsg models.Message
		hasCustMsg := h.DB.Where("conversation_id = ? AND sender_type = ?", conv.ID, "customer").
			Order("sent_at DESC, id DESC").First(&lastCustomerMsg).Error == nil

		within24h := hasCustMsg && time.Since(lastCustomerMsg.SentAt) < 23*time.Hour+45*time.Minute

		fbMsgID, errCode, errMsg := h.sendGraphMessage(page.PageID, pageToken, msg.RecipientPSID, msg.RenderedText, within24h, camp.OutOfWindowTag)

		if errCode == 0 && errMsg == "" {
			consecutiveTagErrors = 0
			now := time.Now()
			_ = h.DB.Model(&msg).Updates(map[string]any{
				"status":        "sent",
				"fb_message_id": fbMsgID,
				"sent_at":       &now,
			}).Error

			_ = h.DB.Model(&camp).UpdateColumn("sent_count", gorm.Expr("sent_count + 1")).Error
			_ = h.DB.Exec(`UPDATE conversations SET last_auto_sent_at = NOW(), last_message = ?, last_message_time = ? WHERE id = ?`,
				msg.RenderedText, now, conv.ID).Error

			// Lưu vào bảng messages để nhân viên thấy trong khung chat CRM
			_ = h.DB.Create(&models.Message{
				ConversationID: conv.ID,
				FBMessageID:    fbMsgID,
				SenderType:     "page",
				SenderName:     page.Name + " (CSKH)",
				Content:        msg.RenderedText,
				SentAt:         now,
			}).Error
		} else {
			_ = h.DB.Model(&msg).Updates(map[string]any{
				"status":        "failed",
				"error_code":    errCode,
				"error_message": errMsg,
			}).Error
			_ = h.DB.Model(&camp).UpdateColumn("failed_count", gorm.Expr("failed_count + 1")).Error

			// Circuit Breaker: Dừng ngay nếu dính rate-limit (4, 17, 32, 613) hoặc lỗi ngoài 24h liên tiếp 5 lần
			if errCode == 4 || errCode == 17 || errCode == 32 || errCode == 613 {
				_ = h.DB.Model(&camp).Updates(map[string]any{
					"status":       "paused",
					"pause_reason": fmt.Sprintf("Tự dừng do giới hạn tốc độ Meta (Code %d: %s)", errCode, errMsg),
				}).Error
			} else if errCode == 10 || errCode == 2022 {
				consecutiveTagErrors++
				if consecutiveTagErrors >= 5 {
					_ = h.DB.Model(&camp).Updates(map[string]any{
						"status":       "paused",
						"pause_reason": fmt.Sprintf("Tự dừng do 5 tin ngoài 24h bị Meta từ chối liên tiếp (%s)", errMsg),
					}).Error
					consecutiveTagErrors = 0
				}
			}
		}

		// Kiểm tra hoàn thành chiến dịch
		var remaining int64
		h.DB.Model(&BroadcastMessage{}).Where("campaign_id = ? AND status IN ?", camp.ID, []string{"pending", "processing"}).Count(&remaining)
		if remaining == 0 {
			_ = h.DB.Model(&camp).Where("status = ?", "running").Update("status", "completed").Error
		}

		sentInBatch++
		if camp.BatchSize > 0 && sentInBatch >= camp.BatchSize {
			sentInBatch = 0
			pause := camp.BatchPauseSec
			if pause < 30 {
				pause = 60
			}
			time.Sleep(time.Duration(pause) * time.Second)
			continue
		}

		minD := camp.MinDelaySec
		if minD < 3 {
			minD = 4
		}
		maxD := camp.MaxDelaySec
		if maxD < minD {
			maxD = minD + 5
		}
		time.Sleep(time.Duration(minD+rand.Intn(maxD-minD+1)) * time.Second)
	}
}

func (h *BroadcastHandler) sendGraphMessage(pageID, pageToken, psid, text string, within24h bool, tag string) (string, int, string) {
	messageObj := map[string]any{
		"text": text,
		"quick_replies": []map[string]string{
			{"content_type": "text", "title": "Tư vấn thêm", "payload": "REENGAGE_INTERESTED"},
			{"content_type": "text", "title": "Ngừng nhận tin", "payload": "OPT_OUT_BROADCAST"},
		},
	}

	var payload map[string]any
	if within24h {
		payload = map[string]any{
			"messaging_type": "RESPONSE",
			"recipient":      map[string]string{"id": psid},
			"message":        messageObj,
		}
	} else {
		if strings.TrimSpace(tag) == "" {
			tag = "POST_PURCHASE_UPDATE"
		}
		payload = map[string]any{
			"messaging_type": "MESSAGE_TAG",
			"tag":            tag,
			"recipient":      map[string]string{"id": psid},
			"message":        messageObj,
		}
	}

	bodyBytes, _ := json.Marshal(payload)
	sendURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages?access_token=%s", pageID, pageToken)
	resp, err := h.HTTPClient.Post(sendURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", -1, err.Error()
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var fbRes struct {
		MessageID string `json:"message_id"`
		Error     *struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBytes, &fbRes)

	if fbRes.Error != nil {
		code := fbRes.Error.Code
		if code == 0 {
			code = resp.StatusCode
		}
		return "", code, fbRes.Error.Message
	}
	return fbRes.MessageID, 0, ""
}

// CreateCampaign POST /api/v1/broadcasts
func (h *BroadcastHandler) CreateCampaign(c *gin.Context) {
	var req struct {
		PageID         string   `json:"page_id" binding:"required"`
		Name           string   `json:"name" binding:"required"`
		Templates      []string `json:"templates" binding:"required,min=1"`
		TargetMode     string   `json:"target_mode"` // all | within_24h | over_24h
		OutOfWindowTag string   `json:"out_of_window_tag"`
		CooldownDays   int      `json:"cooldown_days"`
		MaxRecipients  int      `json:"max_recipients"`
		MinDelaySec    int      `json:"min_delay_sec"`
		MaxDelaySec    int      `json:"max_delay_sec"`
		BatchSize      int      `json:"batch_size"`
		BatchPauseSec  int      `json:"batch_pause_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var validTemplates []string
	for _, t := range req.Templates {
		if s := strings.TrimSpace(t); s != "" {
			validTemplates = append(validTemplates, s)
		}
	}
	if len(validTemplates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cần ít nhất 1 mẫu nội dung"})
		return
	}

	if req.CooldownDays <= 0 {
		req.CooldownDays = 7
	}
	if req.MaxRecipients <= 0 || req.MaxRecipients > 500 {
		req.MaxRecipients = 200
	}
	if req.MinDelaySec < 3 {
		req.MinDelaySec = 4
	}
	if req.MaxDelaySec < req.MinDelaySec {
		req.MaxDelaySec = req.MinDelaySec + 5
	}
	if req.BatchSize <= 0 {
		req.BatchSize = 20
	}
	if req.BatchPauseSec < 30 {
		req.BatchPauseSec = 60
	}
	if req.OutOfWindowTag == "" {
		req.OutOfWindowTag = "POST_PURCHASE_UPDATE"
	}

	q := h.DB.Table("conversations").
		Where("page_id = ? AND COALESCE(is_unsubscribed, FALSE) = FALSE", req.PageID).
		Where("last_auto_sent_at IS NULL OR last_auto_sent_at <= ?", time.Now().AddDate(0, 0, -req.CooldownDays))

	switch req.TargetMode {
	case "within_24h":
		q = q.Where("last_message_time >= ?", time.Now().Add(-23*time.Hour-45*time.Minute))
	case "over_24h":
		q = q.Where("last_message_time < ?", time.Now().Add(-23*time.Hour-45*time.Minute))
	default:
		req.TargetMode = "all"
	}

	var convs []models.Conversation
	if err := q.Order("last_message_time DESC").Limit(req.MaxRecipients).Find(&convs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(convs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không có khách hàng nào đủ điều kiện gửi an toàn"})
		return
	}

	tplJSON, _ := json.Marshal(validTemplates)
	camp := BroadcastCampaign{
		PageID:          req.PageID,
		Name:            req.Name,
		TemplatesJSON:   string(tplJSON),
		TargetMode:      req.TargetMode,
		OutOfWindowTag:  req.OutOfWindowTag,
		MinDelaySec:     req.MinDelaySec,
		MaxDelaySec:     req.MaxDelaySec,
		BatchSize:       req.BatchSize,
		BatchPauseSec:   req.BatchPauseSec,
		Status:          "running",
		TotalRecipients: len(convs),
	}

	if err := h.DB.Create(&camp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var msgs []BroadcastMessage
	for _, cv := range convs {
		tpl := validTemplates[rand.Intn(len(validTemplates))]
		name := strings.TrimSpace(cv.CustomerName)
		if name == "" {
			name = "bạn"
		}
		rendered := strings.ReplaceAll(tpl, "{{full_name}}", name)
		rendered = strings.ReplaceAll(rendered, "{customer_name}", name)

		msgs = append(msgs, BroadcastMessage{
			CampaignID:     camp.ID,
			PageID:         req.PageID,
			ConversationID: cv.ID,
			RecipientPSID:  cv.CustomerPSID,
			RecipientName:  cv.CustomerName,
			RenderedText:   rendered,
			Status:         "pending",
		})
	}
	_ = h.DB.CreateInBatches(&msgs, 100).Error

	log.Printf("[Broadcast] Tạo chiến dịch #%d (%s) cho %d khách hàng", camp.ID, camp.Name, len(convs))
	c.JSON(http.StatusCreated, camp)
}

// ListCampaigns GET /api/v1/broadcasts
func (h *BroadcastHandler) ListCampaigns(c *gin.Context) {
	var list []BroadcastCampaign
	_ = h.DB.Order("created_at DESC").Limit(30).Find(&list).Error
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// ToggleStatus PATCH /api/v1/broadcasts/:id/status
func (h *BroadcastHandler) ToggleStatus(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || (body.Status != "running" && body.Status != "paused") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trạng thái không hợp lệ"})
		return
	}
	updates := map[string]any{"status": body.Status}
	if body.Status == "running" {
		updates["pause_reason"] = ""
	}
	_ = h.DB.Model(&BroadcastCampaign{}).Where("id = ?", id).Updates(updates).Error
	c.JSON(http.StatusOK, gin.H{"status": body.Status})
}
